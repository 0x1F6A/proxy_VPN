//go:build integration

package gormrepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
)

func TestPaymentRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewPaymentRepo(db)
	ctx := context.Background()

	p := &domain.Payment{
		OrderNo: "ORD20260526000001", UserID: 1, Channel: domain.ChannelAlipay,
		AmountCNY: "12.34", AmountToken: "0", Status: domain.StatusPending,
		QRorURL: "alipay://qr/abc", ExpiredAt: time.Now().Add(15 * time.Minute),
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 {
		t.Fatalf("expected id")
	}

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil || got == nil || got.OrderNo != p.OrderNo {
		t.Fatalf("FindByID got=%+v err=%v", got, err)
	}

	active, err := repo.FindActiveByOrder(ctx, p.OrderNo, domain.ChannelAlipay)
	if err != nil || active == nil {
		t.Fatalf("FindActiveByOrder: %+v err=%v", active, err)
	}

	paidAt := time.Now()
	if err := repo.MarkPaid(ctx, p.ID, "TRADE-001", `{"raw":1}`, paidAt); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	got2, _ := repo.FindByID(ctx, p.ID)
	if got2.Status != domain.StatusPaid || got2.ChannelTradeNo != "TRADE-001" || got2.PaidAt == nil {
		t.Fatalf("after MarkPaid: %+v", got2)
	}

	// Idempotent trade-no lookup.
	byTrade, err := repo.FindByChannelTradeNo(ctx, domain.ChannelAlipay, "TRADE-001")
	if err != nil || byTrade == nil || byTrade.ID != p.ID {
		t.Fatalf("FindByChannelTradeNo: %+v err=%v", byTrade, err)
	}

	// Expire pending: insert a second pending payment that's already past
	// its expired_at, then call ExpirePending.
	pStale := &domain.Payment{
		OrderNo: "ORD20260526000002", UserID: 2, Channel: domain.ChannelAlipay,
		AmountCNY: "5.00", AmountToken: "0", Status: domain.StatusPending,
		ExpiredAt: time.Now().Add(-time.Minute),
	}
	if err := repo.Create(ctx, pStale); err != nil {
		t.Fatalf("Create stale: %v", err)
	}
	n, err := repo.ExpirePending(ctx, time.Now())
	if err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 expired, got %d", n)
	}
}

func TestAddressPoolRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewAddressPoolRepo(db)
	ctx := context.Background()

	if err := repo.Seed(ctx, domain.ChannelUSDT, []string{"TRX-AAA", "TRX-BBB", "TRX-CCC"}); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	// Re-seeding the same addresses must be a no-op (ON CONFLICT DO NOTHING).
	if err := repo.Seed(ctx, domain.ChannelUSDT, []string{"TRX-AAA"}); err != nil {
		t.Fatalf("Seed dup: %v", err)
	}
	free, err := repo.CountFree(ctx, domain.ChannelUSDT)
	if err != nil || free != 3 {
		t.Fatalf("CountFree = %d err=%v want 3", free, err)
	}

	l1, err := repo.Allocate(ctx, domain.ChannelUSDT, "ORD-A")
	if err != nil || l1 == nil {
		t.Fatalf("Allocate 1: %+v err=%v", l1, err)
	}
	l2, _ := repo.Allocate(ctx, domain.ChannelUSDT, "ORD-B")
	if l2 == nil || l2.Address == l1.Address {
		t.Fatalf("Allocate 2 must differ from %s; got %+v", l1.Address, l2)
	}
	free, _ = repo.CountFree(ctx, domain.ChannelUSDT)
	if free != 1 {
		t.Fatalf("after 2 allocs free=%d want 1", free)
	}

	if err := repo.Release(ctx, domain.ChannelUSDT, l1.Address); err != nil {
		t.Fatalf("Release: %v", err)
	}
	free, _ = repo.CountFree(ctx, domain.ChannelUSDT)
	if free != 2 {
		t.Fatalf("after release free=%d want 2", free)
	}

	if err := repo.MarkUsed(ctx, domain.ChannelUSDT, l2.Address); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
}

func TestChainScanCursorIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewChainScanCursor(db)
	ctx := context.Background()

	got, err := repo.Get(ctx, "tron")
	if err != nil {
		t.Fatalf("Get empty: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 for unseen chain, got %d", got)
	}

	if err := repo.Set(ctx, "tron", 12345); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, _ = repo.Get(ctx, "tron")
	if got != 12345 {
		t.Fatalf("after Set got %d want 12345", got)
	}

	// Upsert on the same key.
	if err := repo.Set(ctx, "tron", 99999); err != nil {
		t.Fatalf("Set 2: %v", err)
	}
	got, _ = repo.Get(ctx, "tron")
	if got != 99999 {
		t.Fatalf("after re-Set got %d want 99999", got)
	}
}
