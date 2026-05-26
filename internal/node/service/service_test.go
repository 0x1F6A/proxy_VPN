package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
	"github.com/0x1F6A/proxy_VPN/internal/node/service"
)

// ------- fakes ---------------------------------------------------------

type fakeNodeRepo struct {
	mu        sync.Mutex
	rows      []domain.Node
	hbCalls   int
	staleCuts []time.Time
}

func (f *fakeNodeRepo) List(ctx context.Context, onlyEnabled bool) ([]domain.Node, error) {
	out := []domain.Node{}
	for _, n := range f.rows {
		if onlyEnabled && n.Status != domain.NodeStatusEnabled {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}
func (f *fakeNodeRepo) ListByGroups(ctx context.Context, gids []uint64, only bool) ([]domain.Node, error) {
	in := map[uint64]bool{}
	for _, g := range gids {
		in[g] = true
	}
	out := []domain.Node{}
	for _, n := range f.rows {
		if !in[n.NodeGroupID] {
			continue
		}
		if only && !n.IsServiceable() {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}
func (f *fakeNodeRepo) Get(ctx context.Context, id uint64) (*domain.Node, error) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}
func (f *fakeNodeRepo) FindByTokenHash(ctx context.Context, hash string) (*domain.Node, error) {
	for i := range f.rows {
		if f.rows[i].NodeTokenHash == hash {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}
func (f *fakeNodeRepo) Create(ctx context.Context, n *domain.Node) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	n.ID = uint64(len(f.rows) + 1)
	f.rows = append(f.rows, *n)
	return nil
}
func (f *fakeNodeRepo) Update(ctx context.Context, n *domain.Node) error { return nil }
func (f *fakeNodeRepo) Delete(ctx context.Context, id uint64) error      { return nil }
func (f *fakeNodeRepo) UpsertHeartbeat(ctx context.Context, id uint64, hb ports.Heartbeat, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hbCalls++
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].Online = true
			f.rows[i].LastHeartbeatAt = &now
		}
	}
	return nil
}
func (f *fakeNodeRepo) MarkStale(ctx context.Context, cutoff time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.staleCuts = append(f.staleCuts, cutoff)
	n := int64(0)
	for i := range f.rows {
		if f.rows[i].Online && (f.rows[i].LastHeartbeatAt == nil || f.rows[i].LastHeartbeatAt.Before(cutoff)) {
			f.rows[i].Online = false
			n++
		}
	}
	return n, nil
}

type fakeGroupRepo struct {
	planMap map[uint64][]uint64
}

func (f *fakeGroupRepo) List(ctx context.Context) ([]domain.NodeGroup, error) { return nil, nil }
func (f *fakeGroupRepo) Get(ctx context.Context, id uint64) (*domain.NodeGroup, error) {
	return nil, nil
}
func (f *fakeGroupRepo) Create(ctx context.Context, g *domain.NodeGroup) error { return nil }
func (f *fakeGroupRepo) Update(ctx context.Context, g *domain.NodeGroup) error { return nil }
func (f *fakeGroupRepo) Delete(ctx context.Context, id uint64) error           { return nil }
func (f *fakeGroupRepo) PlanGroups(ctx context.Context, pid uint64) ([]uint64, error) {
	return f.planMap[pid], nil
}

type fakeSubs struct{ m map[string]*ports.Subscriber }

func (f *fakeSubs) LookupBySubToken(ctx context.Context, token string) (*ports.Subscriber, error) {
	return f.m[token], nil
}

func (f *fakeSubs) ListActive(_ context.Context, _ int) ([]ports.Subscriber, error) {
	out := make([]ports.Subscriber, 0, len(f.m))
	for _, v := range f.m {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out, nil
}

// ------- helpers -------------------------------------------------------

func newSvc(t *testing.T) (*service.Service, *fakeNodeRepo) {
	t.Helper()
	nodes := &fakeNodeRepo{}
	groups := &fakeGroupRepo{planMap: map[uint64][]uint64{
		7: {10, 20},
	}}
	pid := uint64(7)
	exp := time.Now().Add(time.Hour)
	subs := &fakeSubs{m: map[string]*ports.Subscriber{
		"tok-active":  {UserID: 1, UUID: "abc", PlanID: &pid, PlanExpireAt: &exp, Status: 1},
		"tok-noplan":  {UserID: 2, UUID: "xyz", PlanID: nil, Status: 1},
		"tok-expired": {UserID: 3, UUID: "old", PlanID: &pid, PlanExpireAt: ptrTime(time.Now().Add(-time.Hour)), Status: 1},
	}}
	svc := service.New(service.Deps{
		Nodes: nodes, Groups: groups, Subs: subs,
		BootstrapSecret: "BOOT-SECRET", HeartbeatTimeout: time.Minute,
	})
	return svc, nodes
}

func ptrTime(t time.Time) *time.Time { return &t }

// ------- tests ---------------------------------------------------------

func TestIssueBootstrapAndAgentRegister(t *testing.T) {
	svc, nodes := newSvc(t)
	ctx := context.Background()
	n := &domain.Node{Name: "JP-1", Region: "JP", Protocol: domain.ProtocolVLESSReality,
		Address: "jp1.example.com", Port: 443, NodeGroupID: 10, Status: domain.NodeStatusEnabled}
	tok, err := svc.IssueBootstrapToken(ctx, n)
	if err != nil || len(tok) != 32 {
		t.Fatalf("token=%s err=%v", tok, err)
	}
	if nodes.rows[0].NodeTokenHash == tok {
		t.Fatal("hash should not equal token")
	}

	// register with wrong bootstrap → forbidden
	if _, err := svc.AgentRegister(ctx, "wrong", tok); err == nil {
		t.Fatal("expected error for wrong bootstrap")
	}
	// register with wrong token → auth err
	if _, err := svc.AgentRegister(ctx, "BOOT-SECRET", "nope"); err == nil {
		t.Fatal("expected error for wrong token")
	}
	// correct
	got, err := svc.AgentRegister(ctx, "BOOT-SECRET", tok)
	if err != nil || got.ID != n.ID {
		t.Fatalf("register failed: %v", err)
	}
}

func TestHeartbeatAndStaleMarker(t *testing.T) {
	svc, nodes := newSvc(t)
	ctx := context.Background()
	tok, _ := svc.IssueBootstrapToken(ctx, &domain.Node{
		Name: "X", Region: "US", Protocol: domain.ProtocolTrojan,
		Address: "a", Port: 443, NodeGroupID: 10, Status: domain.NodeStatusEnabled,
	})
	if _, err := svc.AgentHeartbeat(ctx, tok, ports.Heartbeat{CPUPercent: "1.0", MemPercent: "2.0"}); err != nil {
		t.Fatal(err)
	}
	if !nodes.rows[0].Online {
		t.Fatal("expected online after heartbeat")
	}

	// rewind last_heartbeat_at to far past
	old := time.Now().Add(-time.Hour)
	nodes.rows[0].LastHeartbeatAt = &old
	if n, err := nodes.MarkStale(ctx, time.Now().Add(-time.Minute)); err != nil || n != 1 {
		t.Fatalf("MarkStale n=%d err=%v", n, err)
	}
	if nodes.rows[0].Online {
		t.Fatal("expected offline after stale marker")
	}
}

func TestSubscriptionRendering(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	// add 2 serviceable nodes in groups 10 and 20
	for _, n := range []*domain.Node{
		{Name: "US-1", Region: "US", Protocol: domain.ProtocolVLESSReality,
			Address: "us1", Port: 443, NodeGroupID: 10, Status: domain.NodeStatusEnabled},
		{Name: "JP-1", Region: "JP", Protocol: domain.ProtocolHysteria2,
			Address: "jp1", Port: 443, NodeGroupID: 20, Status: domain.NodeStatusEnabled},
	} {
		tok, _ := svc.IssueBootstrapToken(ctx, n)
		_, _ = svc.AgentHeartbeat(ctx, tok, ports.Heartbeat{})
	}

	t.Run("active user gets v2ray base64", func(t *testing.T) {
		body, ct, err := svc.Subscription(ctx, "tok-active", "v2ray")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ct, "text/plain") {
			t.Fatalf("ct=%s", ct)
		}
		if len(body) == 0 {
			t.Fatal("empty body")
		}
	})
	t.Run("clash format", func(t *testing.T) {
		body, ct, err := svc.Subscription(ctx, "tok-active", "clash")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ct, "yaml") || !strings.Contains(string(body), "proxies:") {
			t.Fatalf("ct=%s body=%s", ct, body)
		}
	})
	t.Run("singbox format", func(t *testing.T) {
		body, ct, err := svc.Subscription(ctx, "tok-active", "sing-box")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ct, "json") || !strings.Contains(string(body), `"PROXY"`) {
			t.Fatalf("ct=%s body=%s", ct, body)
		}
	})
	t.Run("no plan", func(t *testing.T) {
		if _, _, err := svc.Subscription(ctx, "tok-noplan", "v2ray"); err != domain.ErrNoPlanGranted {
			t.Fatalf("want NoPlanGranted, got %v", err)
		}
	})
	t.Run("expired plan", func(t *testing.T) {
		if _, _, err := svc.Subscription(ctx, "tok-expired", "v2ray"); err != domain.ErrNoPlanGranted {
			t.Fatalf("want NoPlanGranted, got %v", err)
		}
	})
	t.Run("invalid token", func(t *testing.T) {
		if _, _, err := svc.Subscription(ctx, "garbage", "v2ray"); err != domain.ErrSubTokenInvalid {
			t.Fatalf("want ErrSubTokenInvalid, got %v", err)
		}
	})
	t.Run("bad format", func(t *testing.T) {
		if _, _, err := svc.Subscription(ctx, "tok-active", "surge"); err != domain.ErrSubFormatUnknown {
			t.Fatalf("want ErrSubFormatUnknown, got %v", err)
		}
	})
}

func TestListForUserFilters(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	_, _ = svc.IssueBootstrapToken(ctx, &domain.Node{
		Name: "US-1", Region: "US", Protocol: domain.ProtocolVLESSReality,
		Address: "us1", Port: 443, NodeGroupID: 10, Status: domain.NodeStatusEnabled,
	})
	_, _ = svc.IssueBootstrapToken(ctx, &domain.Node{
		Name: "X-OUT", Region: "X", Protocol: domain.ProtocolVLESSReality,
		Address: "x", Port: 443, NodeGroupID: 999, Status: domain.NodeStatusEnabled,
	})
	// nothing online yet — only nodes with HB show up
	nopid := svc
	rows, _ := nopid.ListForUser(ctx, nil)
	if len(rows) != 0 {
		t.Fatal("expected empty for nil plan")
	}
	pid := uint64(7)
	rows, _ = svc.ListForUser(ctx, &pid)
	if len(rows) != 0 {
		t.Fatalf("expected 0 (none online), got %d", len(rows))
	}
}
