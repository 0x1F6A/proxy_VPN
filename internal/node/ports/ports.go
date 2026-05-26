// Package ports defines node-context outbound interfaces.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
)

type NodeRepo interface {
	List(ctx context.Context, onlyEnabled bool) ([]domain.Node, error)
	ListByGroups(ctx context.Context, groupIDs []uint64, onlyServiceable bool) ([]domain.Node, error)
	Get(ctx context.Context, id uint64) (*domain.Node, error)
	FindByTokenHash(ctx context.Context, hash string) (*domain.Node, error)
	Create(ctx context.Context, n *domain.Node) error
	Update(ctx context.Context, n *domain.Node) error
	Delete(ctx context.Context, id uint64) error
	UpsertHeartbeat(ctx context.Context, id uint64, hb Heartbeat, now time.Time) error
	// MarkStale flips online=false for nodes whose last_heartbeat_at < cutoff.
	MarkStale(ctx context.Context, cutoff time.Time) (int64, error)
}

type NodeGroupRepo interface {
	List(ctx context.Context) ([]domain.NodeGroup, error)
	Get(ctx context.Context, id uint64) (*domain.NodeGroup, error)
	Create(ctx context.Context, g *domain.NodeGroup) error
	Update(ctx context.Context, g *domain.NodeGroup) error
	Delete(ctx context.Context, id uint64) error
	// PlanGroups returns node_group_ids granted to a given plan via
	// plan_node_groups. Returns empty slice if the plan has no entries.
	PlanGroups(ctx context.Context, planID uint64) ([]uint64, error)
}

// SubscriberPort lets the node service read the subscriber (user) record by
// subscription token without depending on the user package. Implemented in the
// user infra layer.
type SubscriberPort interface {
	LookupBySubToken(ctx context.Context, token string) (*Subscriber, error)
}

type Subscriber struct {
	UserID       uint64
	UUID         string
	PlanID       *uint64
	PlanExpireAt *time.Time
	Status       int8
}

// Heartbeat is the per-tick payload from a node-agent.
type Heartbeat struct {
	CPUPercent      string
	MemPercent      string
	BandwidthInBps  uint64
	BandwidthOutBps uint64
	OnlineUsers     uint32
}
