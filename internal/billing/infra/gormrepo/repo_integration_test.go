//go:build integration

package gormrepo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
)

func TestPlanRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewPlanRepo(db)
	ctx := context.Background()

	p := &domain.Plan{
		Name: "月套餐", Description: "30 天 100 GB", PriceCNY: "29.00",
		DurationDays: 30, TrafficGB: 100, DeviceLimit: 3, SpeedLimitMbps: 100,
		NodeGroupID: 1, Tags: "hot", Sort: 1, Status: 1,
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 {
		t.Fatalf("expected id")
	}

	got, err := repo.Get(ctx, p.ID)
	if err != nil || got.Name != p.Name {
		t.Fatalf("Get: %+v err=%v", got, err)
	}

	p.Description = "30 天 200 GB"
	p.TrafficGB = 200
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, p.ID)
	if got.TrafficGB != 200 {
		t.Fatalf("after Update TrafficGB=%d want 200", got.TrafficGB)
	}

	// Inactive should still show in List(false).
	p2 := &domain.Plan{Name: "季套餐", PriceCNY: "79.00", DurationDays: 90, TrafficGB: 300, DeviceLimit: 3, NodeGroupID: 1, Status: 0}
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}
	all, _ := repo.List(ctx, false)
	if len(all) < 2 {
		t.Fatalf("List(false) len=%d want >=2", len(all))
	}
	active, _ := repo.List(ctx, true)
	for _, pl := range active {
		if pl.Status != 1 {
			t.Fatalf("List(true) returned inactive plan %+v", pl)
		}
	}

	if err := repo.Delete(ctx, p2.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, p2.ID); !errors.Is(err, domain.ErrPlanNotFound) {
		t.Fatalf("after Delete expected ErrPlanNotFound, got %v", err)
	}
}

func TestDataPackRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewDataPackRepo(db)
	ctx := context.Background()

	pk := &domain.DataPack{Name: "20GB", PriceCNY: "9.90", TrafficGB: 20, ValidDays: 30, AttachMode: 1, Status: 1}
	if err := repo.Create(ctx, pk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx, pk.ID)
	if err != nil || got.TrafficGB != 20 {
		t.Fatalf("Get: %+v err=%v", got, err)
	}
	pk.PriceCNY = "8.80"
	if err := repo.Update(ctx, pk); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, pk.ID)
	if got.PriceCNY != "8.80" {
		t.Fatalf("price=%s want 8.80", got.PriceCNY)
	}
	list, _ := repo.List(ctx, true)
	if len(list) != 1 {
		t.Fatalf("List active len=%d want 1", len(list))
	}
}

func TestCouponRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	coupons := gormrepo.NewCouponRepo(db)
	orders := gormrepo.NewOrderRepo(db)
	ctx := context.Background()

	// Direct insert via gorm to get a row (CouponRepo has no Create).
	type couponRow struct {
		ID            uint64 `gorm:"primaryKey"`
		Code          string
		DiscountType  int8
		DiscountValue string `gorm:"type:decimal(12,2)"`
		MinAmount     string `gorm:"type:decimal(12,2)"`
		Applicable    string
		TotalQuota    uint32
		UsedCount     uint32
		PerUserLimit  uint32
		StartsAt      *time.Time
		ExpiresAt     *time.Time
		Status        int8
	}
	row := &couponRow{
		Code: "WELCOME10", DiscountType: 1, DiscountValue: "10.00", MinAmount: "0.00",
		Applicable: "all", TotalQuota: 2, PerUserLimit: 1, Status: 1,
	}
	if err := db.WithContext(ctx).Table("coupons").Create(row).Error; err != nil {
		t.Fatalf("seed coupon: %v", err)
	}

	got, err := coupons.FindByCode(ctx, "WELCOME10")
	if err != nil || got.Code != "WELCOME10" {
		t.Fatalf("FindByCode: %+v err=%v", got, err)
	}
	if _, err := coupons.FindByCode(ctx, "NOPE"); !errors.Is(err, domain.ErrCouponNotFound) {
		t.Fatalf("expected ErrCouponNotFound, got %v", err)
	}

	if err := coupons.IncrementUsage(ctx, got.ID); err != nil {
		t.Fatalf("Increment 1: %v", err)
	}
	if err := coupons.IncrementUsage(ctx, got.ID); err != nil {
		t.Fatalf("Increment 2: %v", err)
	}
	if err := coupons.IncrementUsage(ctx, got.ID); !errors.Is(err, domain.ErrCouponExhausted) {
		t.Fatalf("expected ErrCouponExhausted, got %v", err)
	}

	// Seed a paid order using the coupon, verify CountUsedByUser.
	o := &domain.Order{
		OrderNo: "ORD-COUPON-1", UserID: 42, Type: "plan", TargetID: 1, TargetSnapshot: []byte(`{}`),
		AmountCNY: "29.00", DiscountCNY: "10.00", PaidCNY: "19.00",
		CouponCode: "WELCOME10", PayMethod: "alipay", Status: domain.OrderStatusPaid,
		ExpireAt: time.Now().Add(time.Hour), IdempotencyKey: "key-cp-1",
	}
	if err := orders.Create(ctx, o); err != nil {
		t.Fatalf("Create order: %v", err)
	}
	n, err := coupons.CountUsedByUser(ctx, "WELCOME10", 42)
	if err != nil || n != 1 {
		t.Fatalf("CountUsedByUser n=%d err=%v want 1", n, err)
	}
}

func TestOrderRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewOrderRepo(db)
	ctx := context.Background()

	o := &domain.Order{
		OrderNo: "ORD-ORD-001", UserID: 7, Type: "plan", TargetID: 1, TargetSnapshot: []byte(`{"plan":1}`),
		AmountCNY: "29.00", DiscountCNY: "0.00", PaidCNY: "0.00",
		PayMethod: "alipay", Status: domain.OrderStatusPending,
		ExpireAt: time.Now().Add(15 * time.Minute), IdempotencyKey: "idem-001", ClientIP: "127.0.0.1",
	}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dup, err := repo.FindByIdempotency(ctx, 7, "idem-001")
	if err != nil || dup == nil || dup.OrderNo != "ORD-ORD-001" {
		t.Fatalf("FindByIdempotency: %+v err=%v", dup, err)
	}
	miss, err := repo.FindByIdempotency(ctx, 7, "nope")
	if err != nil || miss != nil {
		t.Fatalf("FindByIdempotency miss: %+v err=%v", miss, err)
	}

	got, err := repo.FindByOrderNo(ctx, "ORD-ORD-001")
	if err != nil || got.UserID != 7 {
		t.Fatalf("FindByOrderNo: %+v err=%v", got, err)
	}
	if _, err := repo.FindByOrderNo(ctx, "MISSING"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}

	now := time.Now()
	if err := repo.UpdateStatus(ctx, "ORD-ORD-001", domain.OrderStatusPaid, &now, "29.00", "TRADE-XYZ"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ = repo.FindByOrderNo(ctx, "ORD-ORD-001")
	if got.Status != domain.OrderStatusPaid || got.PayChannelNo != "TRADE-XYZ" || got.PaidAt == nil || got.PaidCNY != "29.00" {
		t.Fatalf("after UpdateStatus: %+v", got)
	}

	// Stale pending order → ExpirePending flips it.
	stale := &domain.Order{
		OrderNo: "ORD-STALE-1", UserID: 7, Type: "plan", TargetID: 1, TargetSnapshot: []byte(`{}`),
		AmountCNY: "29.00", DiscountCNY: "0.00", PaidCNY: "0.00", PayMethod: "alipay",
		Status: domain.OrderStatusPending, ExpireAt: time.Now().Add(-time.Minute),
		IdempotencyKey: "idem-stale-1",
	}
	if err := repo.Create(ctx, stale); err != nil {
		t.Fatalf("Create stale: %v", err)
	}
	n, err := repo.ExpirePending(ctx, time.Now())
	if err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if n < 1 {
		t.Fatalf("ExpirePending n=%d want >=1", n)
	}
	got, _ = repo.FindByOrderNo(ctx, "ORD-STALE-1")
	if got.Status != domain.OrderStatusExpired {
		t.Fatalf("after ExpirePending status=%s want expired", got.Status)
	}

	list, err := repo.ListByUser(ctx, 7, 10, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByUser len=%d err=%v want 2", len(list), err)
	}
}
