//go:build integration

package gormrepo_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
)

func TestGroupRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewGroupRepo(db)
	ctx := context.Background()

	g := &domain.NodeGroup{Name: "premium-asia", Level: 10, Remark: "JP/HK/SG"}
	if err := repo.Create(ctx, g); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == 0 {
		t.Fatalf("expected id")
	}
	got, err := repo.Get(ctx, g.ID)
	if err != nil || got.Name != "premium-asia" {
		t.Fatalf("Get: %+v err=%v", got, err)
	}
	g.Remark = "JP/HK/SG/KR"
	if err := repo.Update(ctx, g); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, g.ID)
	if got.Remark != "JP/HK/SG/KR" {
		t.Fatalf("after Update remark=%s", got.Remark)
	}
	list, _ := repo.List(ctx)
	if len(list) != 1 {
		t.Fatalf("List len=%d want 1", len(list))
	}
	if err := repo.Delete(ctx, g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	miss, _ := repo.Get(ctx, g.ID)
	if miss != nil {
		t.Fatalf("expected nil after Delete, got %+v", miss)
	}
}

func TestNodeRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	groups := gormrepo.NewGroupRepo(db)
	nodes := gormrepo.NewNodeRepo(db)
	ctx := context.Background()

	g := &domain.NodeGroup{Name: "default", Level: 1}
	if err := groups.Create(ctx, g); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	n := &domain.Node{
		Name: "us-west-1", Region: "US", Tags: "premium",
		NodeGroupID: g.ID, Protocol: "vless", Address: "1.2.3.4", Port: 443,
		TLSConfig: json.RawMessage(`{"sni":"example.com"}`),
		Transport: "tcp", TransportConfig: json.RawMessage(`{}`),
		RateMultiplier: "1.00", NodeTokenHash: "hash-us-1",
		Sort: 100, Status: domain.NodeStatusEnabled,
	}
	if err := nodes.Create(ctx, n); err != nil {
		t.Fatalf("Create node: %v", err)
	}

	got, err := nodes.Get(ctx, n.ID)
	if err != nil || got.Address != "1.2.3.4" {
		t.Fatalf("Get: %+v err=%v", got, err)
	}

	byHash, err := nodes.FindByTokenHash(ctx, "hash-us-1")
	if err != nil || byHash == nil || byHash.ID != n.ID {
		t.Fatalf("FindByTokenHash: %+v err=%v", byHash, err)
	}
	miss, _ := nodes.FindByTokenHash(ctx, "nope")
	if miss != nil {
		t.Fatalf("expected nil, got %+v", miss)
	}

	// Heartbeat → online + populated metrics.
	now := time.Now().UTC().Truncate(time.Second)
	hb := ports.Heartbeat{
		CPUPercent: "25.5", MemPercent: "40.0",
		BandwidthInBps: 100_000, BandwidthOutBps: 200_000, OnlineUsers: 7,
	}
	if err := nodes.UpsertHeartbeat(ctx, n.ID, hb, now); err != nil {
		t.Fatalf("UpsertHeartbeat: %v", err)
	}
	got, _ = nodes.Get(ctx, n.ID)
	if !got.Online || got.OnlineUsers != 7 || got.BandwidthInBps == nil || *got.BandwidthInBps != 100_000 {
		t.Fatalf("after heartbeat: %+v", got)
	}
	if got.LastHeartbeatAt == nil || got.LastHeartbeatAt.Unix() != now.Unix() {
		t.Fatalf("LastHeartbeatAt mismatch: %+v vs %v", got.LastHeartbeatAt, now)
	}

	// ListByGroups serviceable only.
	list, err := nodes.ListByGroups(ctx, []uint64{g.ID}, true)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByGroups serviceable len=%d err=%v want 1", len(list), err)
	}
	empty, _ := nodes.ListByGroups(ctx, nil, true)
	if len(empty) != 0 {
		t.Fatalf("ListByGroups with no ids must be empty, got %d", len(empty))
	}

	// Disabled node should not appear in serviceable.
	off := &domain.Node{
		Name: "jp-1", Region: "JP", NodeGroupID: g.ID, Protocol: "vless",
		Address: "5.6.7.8", Port: 443, NodeTokenHash: "hash-jp-1",
		Transport: "tcp", RateMultiplier: "1.00", Status: domain.NodeStatusDisabled,
	}
	if err := nodes.Create(ctx, off); err != nil {
		t.Fatalf("Create off: %v", err)
	}
	list, _ = nodes.ListByGroups(ctx, []uint64{g.ID}, true)
	if len(list) != 1 {
		t.Fatalf("serviceable should still be 1 (disabled excluded), got %d", len(list))
	}
	listAll, _ := nodes.List(ctx, false)
	if len(listAll) != 2 {
		t.Fatalf("List(false) want 2, got %d", len(listAll))
	}
	listEnabled, _ := nodes.List(ctx, true)
	if len(listEnabled) != 1 {
		t.Fatalf("List(true) want 1, got %d", len(listEnabled))
	}

	// MarkStale: cutoff in the future flips our heartbeated node back to
	// offline (last_heartbeat_at < cutoff).
	affected, err := nodes.MarkStale(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	if affected < 1 {
		t.Fatalf("MarkStale affected=%d want >=1", affected)
	}
	got, _ = nodes.Get(ctx, n.ID)
	if got.Online {
		t.Fatalf("expected offline after MarkStale, got online")
	}

	// Update.
	n.Name = "us-west-1-renamed"
	n.Sort = 200
	if err := nodes.Update(ctx, n); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = nodes.Get(ctx, n.ID)
	if got.Name != "us-west-1-renamed" || got.Sort != 200 {
		t.Fatalf("after Update: %+v", got)
	}

	if err := nodes.Delete(ctx, n.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	miss2, _ := nodes.Get(ctx, n.ID)
	if miss2 != nil {
		t.Fatalf("expected nil after Delete, got %+v", miss2)
	}
}
