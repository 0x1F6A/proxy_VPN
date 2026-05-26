//go:build integration

package gormrepo_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/infra/gormrepo"
)

// seedUser inserts a minimal users row sufficient for QuotaRepo tests and
// returns its id. Only the columns referenced by QuotaRepo + NOT NULL
// schema fields are populated.
func seedUser(t *testing.T, db *gorm.DB, total, used uint64, banned bool) uint64 {
	t.Helper()
	b := 0
	if banned {
		b = 1
	}
	res := db.Exec(`INSERT INTO users
		(email, password_hash, uuid, invite_code, subscription_token,
		 traffic_total, traffic_used, banned)
		VALUES (?, 'x', UUID(), SUBSTR(REPLACE(UUID(),'-',''),1,8), SUBSTR(REPLACE(UUID(),'-',''),1,40), ?, ?, ?)`,
		time.Now().UnixNano(), total, used, b)
	if res.Error != nil {
		t.Fatalf("seed user: %v", res.Error)
	}
	var id uint64
	if err := db.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error; err != nil {
		t.Fatalf("last id: %v", err)
	}
	return id
}

func TestQuotaRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewQuotaRepo(db)
	ctx := context.Background()

	id := seedUser(t, db, 1000, 0, false)

	q, err := repo.GetQuota(ctx, id)
	if err != nil {
		t.Fatalf("GetQuota: %v", err)
	}
	if q.TrafficTotal != 1000 || q.TrafficUsed != 0 || q.Banned {
		t.Fatalf("unexpected quota: %+v", q)
	}

	newUsed, err := repo.IncrTrafficUsed(ctx, id, 300)
	if err != nil || newUsed != 300 {
		t.Fatalf("Incr1: used=%d err=%v", newUsed, err)
	}
	newUsed, err = repo.IncrTrafficUsed(ctx, id, 700)
	if err != nil || newUsed != 1000 {
		t.Fatalf("Incr2: used=%d err=%v", newUsed, err)
	}

	cands, err := repo.ListBanCandidates(ctx, 10)
	if err != nil {
		t.Fatalf("ListBanCandidates: %v", err)
	}
	foundBan := false
	for _, c := range cands {
		if c == id {
			foundBan = true
		}
	}
	if !foundBan {
		t.Fatalf("expected %d in ban candidates, got %v", id, cands)
	}

	if err := repo.SetBanned(ctx, id, true); err != nil {
		t.Fatalf("SetBanned: %v", err)
	}
	q, _ = repo.GetQuota(ctx, id)
	if !q.Banned {
		t.Fatalf("expected banned=true after SetBanned")
	}

	if err := db.Exec(`UPDATE users SET traffic_total = 10000 WHERE id = ?`, id).Error; err != nil {
		t.Fatalf("bump quota: %v", err)
	}
	unban, err := repo.ListUnbanCandidates(ctx, 10)
	if err != nil {
		t.Fatalf("ListUnbanCandidates: %v", err)
	}
	foundUnban := false
	for _, c := range unban {
		if c == id {
			foundUnban = true
		}
	}
	if !foundUnban {
		t.Fatalf("expected %d in unban candidates, got %v", id, unban)
	}

	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.UpsertDaily(ctx, id, day, 100, 200); err != nil {
		t.Fatalf("UpsertDaily 1: %v", err)
	}
	if err := repo.UpsertDaily(ctx, id, day, 50, 75); err != nil {
		t.Fatalf("UpsertDaily 2: %v", err)
	}
	rows, err := repo.SumDaily(ctx, id, day, day)
	if err != nil {
		t.Fatalf("SumDaily: %v", err)
	}
	if len(rows) != 1 || rows[0].UpBytes != 150 || rows[0].DownBytes != 275 {
		t.Fatalf("unexpected daily: %+v", rows)
	}
}

func TestUsageFallbackSinkIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	sink := gormrepo.NewUsageFallbackSink(db)
	ctx := context.Background()

	id := seedUser(t, db, 1000, 0, false)
	events := []domain.UsageEvent{
		{Ts: time.Now(), UserID: id, NodeID: 1, Protocol: "vless", UpBytes: 10, DownBytes: 20},
		{Ts: time.Now(), UserID: id, NodeID: 2, Protocol: "trojan", UpBytes: 30, DownBytes: 40},
	}
	if err := sink.Write(ctx, events); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM usage_event_fallback WHERE user_id = ?", id).Scan(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 fallback rows, got %d", n)
	}
}
