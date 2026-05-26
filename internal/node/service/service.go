// Package service implements node-context use cases: admin CRUD,
// node-agent register / heartbeat, user-facing node listing, subscription
// rendering, and the offline-marker background worker.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
	"github.com/0x1F6A/proxy_VPN/internal/node/service/subgen"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/idgen"
)

type Deps struct {
	Nodes            ports.NodeRepo
	Groups           ports.NodeGroupRepo
	Subs             ports.SubscriberPort
	BootstrapSecret  string
	HeartbeatTimeout time.Duration
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func validProto(p string) bool {
	switch p {
	case domain.ProtocolVLESSReality, domain.ProtocolTrojan,
		domain.ProtocolHysteria2, domain.ProtocolSS2022:
		return true
	}
	return false
}

// ----- group CRUD ------------------------------------------------------

func (s *Service) ListGroups(ctx context.Context) ([]domain.NodeGroup, error) {
	return s.d.Groups.List(ctx)
}
func (s *Service) CreateGroup(ctx context.Context, g *domain.NodeGroup) error {
	return s.d.Groups.Create(ctx, g)
}
func (s *Service) UpdateGroup(ctx context.Context, g *domain.NodeGroup) error {
	return s.d.Groups.Update(ctx, g)
}
func (s *Service) DeleteGroup(ctx context.Context, id uint64) error {
	return s.d.Groups.Delete(ctx, id)
}

// ----- node CRUD -------------------------------------------------------

func (s *Service) ListNodes(ctx context.Context, onlyEnabled bool) ([]domain.Node, error) {
	return s.d.Nodes.List(ctx, onlyEnabled)
}
func (s *Service) GetNode(ctx context.Context, id uint64) (*domain.Node, error) {
	n, err := s.d.Nodes.Get(ctx, id)
	if err != nil || n == nil {
		return nil, domain.ErrNodeNotFound
	}
	return n, nil
}
func (s *Service) CreateNode(ctx context.Context, n *domain.Node) error {
	if !validProto(n.Protocol) {
		return domain.ErrUnsupportedProto
	}
	if n.NodeTokenHash == "" {
		// fresh-create path: caller may inject a token externally; otherwise
		// generate one and HASH only (token returned via separate channel by
		// the admin handler if it ever calls this directly).
		n.NodeTokenHash = sha256hex(idgen.HexN(32))
	}
	if n.RateMultiplier == "" {
		n.RateMultiplier = "1.00"
	}
	return s.d.Nodes.Create(ctx, n)
}
func (s *Service) UpdateNode(ctx context.Context, n *domain.Node) error {
	if !validProto(n.Protocol) {
		return domain.ErrUnsupportedProto
	}
	return s.d.Nodes.Update(ctx, n)
}
func (s *Service) DeleteNode(ctx context.Context, id uint64) error {
	return s.d.Nodes.Delete(ctx, id)
}

// IssueBootstrapToken creates a new node row + returns the plaintext bootstrap
// token (shown once to the admin). Token is stored as sha256 in DB.
func (s *Service) IssueBootstrapToken(ctx context.Context, n *domain.Node) (token string, err error) {
	token = idgen.HexN(32)
	n.NodeTokenHash = sha256hex(token)
	if err = s.CreateNode(ctx, n); err != nil {
		return "", err
	}
	return token, nil
}

// ----- agent register & heartbeat -------------------------------------

// AgentRegister authenticates a node-agent with the global bootstrap secret +
// the node's token. Returns the node row on success.
func (s *Service) AgentRegister(ctx context.Context, bootstrap, nodeToken string) (*domain.Node, error) {
	if bootstrap == "" || bootstrap != s.d.BootstrapSecret {
		return nil, domain.ErrBootstrapForbidden
	}
	n, err := s.d.Nodes.FindByTokenHash(ctx, sha256hex(nodeToken))
	if err != nil || n == nil {
		return nil, domain.ErrNodeAuth
	}
	return n, nil
}

func (s *Service) AgentHeartbeat(ctx context.Context, nodeToken string, hb ports.Heartbeat) (*domain.Node, error) {
	n, err := s.d.Nodes.FindByTokenHash(ctx, sha256hex(nodeToken))
	if err != nil || n == nil {
		return nil, domain.ErrNodeAuth
	}
	now := time.Now()
	if err := s.d.Nodes.UpsertHeartbeat(ctx, n.ID, hb, now); err != nil {
		return nil, err
	}
	return n, nil
}

// MarkStaleNow flips nodes offline if their last_heartbeat_at is older
// than HeartbeatTimeout. Exposed as a single-shot for asynq scheduling.
func (s *Service) MarkStaleNow(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-s.d.HeartbeatTimeout)
	return s.d.Nodes.MarkStale(ctx, cutoff)
}

// RunStaleMarker periodically marks nodes offline if their last_heartbeat_at
// is older than HeartbeatTimeout.
func (s *Service) RunStaleMarker(ctx context.Context, tick time.Duration, log func(string, ...any)) {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			cutoff := now.Add(-s.d.HeartbeatTimeout)
			if n, err := s.d.Nodes.MarkStale(ctx, cutoff); err != nil {
				if log != nil {
					log("node stale-marker failed", "err", err)
				}
			} else if n > 0 && log != nil {
				log("node stale-marker", "marked_offline", n)
			}
		}
	}
}

// ----- subscription ----------------------------------------------------

// Subscription returns a rendered subscription body + content-type given a
// user's subscription token and the requested format.
func (s *Service) Subscription(ctx context.Context, token, format string) (body []byte, contentType string, err error) {
	sub, err := s.d.Subs.LookupBySubToken(ctx, token)
	if err != nil || sub == nil {
		return nil, "", domain.ErrSubTokenInvalid
	}
	if sub.PlanID == nil || (sub.PlanExpireAt != nil && sub.PlanExpireAt.Before(time.Now())) {
		return nil, "", domain.ErrNoPlanGranted
	}
	groups, err := s.d.Groups.PlanGroups(ctx, *sub.PlanID)
	if err != nil {
		return nil, "", err
	}
	nodes, err := s.d.Nodes.ListByGroups(ctx, groups, true)
	if err != nil {
		return nil, "", err
	}
	views := make([]subgen.NodeView, 0, len(nodes))
	for _, n := range nodes {
		views = append(views, subgen.NodeView{Node: n, UserUUID: sub.UUID})
	}
	switch strings.ToLower(format) {
	case "", "v2ray":
		return []byte(subgen.V2Ray(views)), "text/plain; charset=utf-8", nil
	case "clash":
		return subgen.Clash(views), "text/yaml; charset=utf-8", nil
	case "singbox", "sing-box":
		return subgen.SingBox(views), "application/json; charset=utf-8", nil
	default:
		return nil, "", domain.ErrSubFormatUnknown
	}
}

// ----- user-facing list -----------------------------------------------

// ListForUser returns the nodes a user can see based on their plan's groups.
// If user has no plan, returns an empty slice (not an error) so the API
// surface can render a tip in the UI.
func (s *Service) ListForUser(ctx context.Context, planID *uint64) ([]domain.Node, error) {
	if planID == nil {
		return []domain.Node{}, nil
	}
	groups, err := s.d.Groups.PlanGroups(ctx, *planID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return []domain.Node{}, nil
	}
	return s.d.Nodes.ListByGroups(ctx, groups, true)
}

// helper for handlers
var ErrInvalidInput = errors.New("invalid input")
