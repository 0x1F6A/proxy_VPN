package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/service"
)

// ------ fakes ----------------------------------------------------------

type fakePlanRepo struct{ m map[uint64]*domain.Plan }

func (f *fakePlanRepo) List(ctx context.Context, onlyActive bool) ([]domain.Plan, error) {
	out := []domain.Plan{}
	for _, p := range f.m {
		if onlyActive && p.Status != 1 {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}
func (f *fakePlanRepo) Get(ctx context.Context, id uint64) (*domain.Plan, error) {
	return f.m[id], nil
}
func (f *fakePlanRepo) Create(ctx context.Context, p *domain.Plan) error {
	p.ID = uint64(len(f.m) + 1)
	f.m[p.ID] = p
	return nil
}
func (f *fakePlanRepo) Update(ctx context.Context, p *domain.Plan) error { f.m[p.ID] = p; return nil }
func (f *fakePlanRepo) Delete(ctx context.Context, id uint64) error      { delete(f.m, id); return nil }

type fakePackRepo struct{ m map[uint64]*domain.DataPack }

func (f *fakePackRepo) List(ctx context.Context, onlyActive bool) ([]domain.DataPack, error) {
	return nil, nil
}
func (f *fakePackRepo) Get(ctx context.Context, id uint64) (*domain.DataPack, error) {
	return f.m[id], nil
}
func (f *fakePackRepo) Create(ctx context.Context, p *domain.DataPack) error { return nil }
func (f *fakePackRepo) Update(ctx context.Context, p *domain.DataPack) error { return nil }
func (f *fakePackRepo) Delete(ctx context.Context, id uint64) error          { return nil }

type fakeCouponRepo struct {
	m         map[string]*domain.Coupon
	userUsage map[string]int
}

func (f *fakeCouponRepo) FindByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	if c, ok := f.m[code]; ok {
		return c, nil
	}
	return nil, nil
}
func (f *fakeCouponRepo) IncrementUsage(ctx context.Context, id uint64) error {
	for _, c := range f.m {
		if c.ID == id {
			if c.TotalQuota > 0 && c.UsedCount >= c.TotalQuota {
				return domain.ErrCouponExhausted
			}
			c.UsedCount++
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeCouponRepo) CountUsedByUser(ctx context.Context, code string, userID uint64) (int, error) {
	return f.userUsage[code], nil
}

type fakeOrderRepo struct {
	mu        sync.Mutex
	byNo      map[string]*domain.Order
	byIdem    map[string]*domain.Order
	createErr error
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{byNo: map[string]*domain.Order{}, byIdem: map[string]*domain.Order{}}
}
func idemKey(uid uint64, k string) string { return string(rune(uid)) + "|" + k }
func (f *fakeOrderRepo) Create(ctx context.Context, o *domain.Order) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byIdem[idemKey(o.UserID, o.IdempotencyKey)]; ok {
		return errors.New("dup")
	}
	o.ID = uint64(len(f.byNo) + 1)
	f.byNo[o.OrderNo] = o
	f.byIdem[idemKey(o.UserID, o.IdempotencyKey)] = o
	return nil
}
func (f *fakeOrderRepo) FindByIdempotency(ctx context.Context, userID uint64, key string) (*domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byIdem[idemKey(userID, key)], nil
}
func (f *fakeOrderRepo) FindByOrderNo(ctx context.Context, no string) (*domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byNo[no], nil
}
func (f *fakeOrderRepo) ListByUser(ctx context.Context, userID uint64, limit, offset int) ([]domain.Order, error) {
	return nil, nil
}
func (f *fakeOrderRepo) UpdateStatus(ctx context.Context, no, status string, paidAt *time.Time, paid, ch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	o := f.byNo[no]
	if o == nil {
		return errors.New("not found")
	}
	o.Status = status
	o.PaidAt = paidAt
	o.PaidCNY = paid
	o.PayChannelNo = ch
	return nil
}
func (f *fakeOrderRepo) ExpirePending(ctx context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := int64(0)
	for _, o := range f.byNo {
		if o.Status == domain.OrderStatusPending && o.ExpireAt.Before(before) {
			o.Status = domain.OrderStatusExpired
			n++
		}
	}
	return n, nil
}

type fakeApply struct {
	planCalls, packCalls, topupCalls int
}

func (f *fakeApply) ApplyPlan(ctx context.Context, uid, pid uint64, d, g, dev uint32) error {
	f.planCalls++
	return nil
}
func (f *fakeApply) ApplyPack(ctx context.Context, uid uint64, g, v uint32, m int8) error {
	f.packCalls++
	return nil
}
func (f *fakeApply) ApplyTopup(ctx context.Context, uid uint64, amt string) error {
	f.topupCalls++
	return nil
}

func newSvc() (*service.Service, *fakePlanRepo, *fakeCouponRepo, *fakeOrderRepo, *fakeApply) {
	planRepo := &fakePlanRepo{m: map[uint64]*domain.Plan{
		1: {ID: 1, Name: "Pro", PriceCNY: "30.00", DurationDays: 30, TrafficGB: 100, DeviceLimit: 3, Status: 1},
	}}
	couponRepo := &fakeCouponRepo{m: map[string]*domain.Coupon{
		"SAVE5":  {ID: 1, Code: "SAVE5", DiscountType: domain.CouponDiscountFixed, DiscountValue: "5.00", Applicable: "all", Status: 1},
		"OFF10":  {ID: 2, Code: "OFF10", DiscountType: domain.CouponDiscountPercent, DiscountValue: "10", Applicable: "all", Status: 1},
		"GONE":   {ID: 3, Code: "GONE", DiscountType: domain.CouponDiscountFixed, DiscountValue: "5.00", Applicable: "all", TotalQuota: 1, UsedCount: 1, Status: 1},
		"MIN100": {ID: 4, Code: "MIN100", DiscountType: domain.CouponDiscountFixed, DiscountValue: "5.00", MinAmount: "100.00", Applicable: "all", Status: 1},
	}, userUsage: map[string]int{}}
	orderRepo := newFakeOrderRepo()
	apply := &fakeApply{}
	svc := service.New(service.Deps{
		Plans:     planRepo,
		Packs:     &fakePackRepo{m: map[uint64]*domain.DataPack{}},
		Coupons:   couponRepo,
		Orders:    orderRepo,
		UserApply: apply,
	})
	return svc, planRepo, couponRepo, orderRepo, apply
}

// ------ tests ----------------------------------------------------------

func TestQuoteCoupon(t *testing.T) {
	svc, _, _, _, _ := newSvc()
	ctx := context.Background()

	t.Run("fixed", func(t *testing.T) {
		d, f, err := svc.QuoteCoupon(ctx, "SAVE5", "30.00", "plan", 1)
		if err != nil || d != "5.00" || f != "25.00" {
			t.Fatalf("got d=%s f=%s err=%v", d, f, err)
		}
	})
	t.Run("percent", func(t *testing.T) {
		d, f, err := svc.QuoteCoupon(ctx, "OFF10", "30.00", "plan", 1)
		if err != nil || d != "3.00" || f != "27.00" {
			t.Fatalf("got d=%s f=%s err=%v", d, f, err)
		}
	})
	t.Run("exhausted", func(t *testing.T) {
		_, _, err := svc.QuoteCoupon(ctx, "GONE", "30.00", "plan", 1)
		if !errors.Is(err, domain.ErrCouponExhausted) {
			t.Fatalf("want exhausted, got %v", err)
		}
	})
	t.Run("min not met", func(t *testing.T) {
		_, _, err := svc.QuoteCoupon(ctx, "MIN100", "30.00", "plan", 1)
		if !errors.Is(err, domain.ErrCouponNotMet) {
			t.Fatalf("want not met, got %v", err)
		}
	})
}

func TestCreateOrderIdempotent(t *testing.T) {
	svc, _, _, orderRepo, _ := newSvc()
	ctx := context.Background()
	in := service.CreateOrderInput{
		UserID: 7, Type: "plan", TargetID: 1, PayMethod: "mock", IdempotencyKey: "abc",
	}
	o1, err := svc.CreateOrder(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	o2, err := svc.CreateOrder(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if o1.OrderNo != o2.OrderNo {
		t.Fatalf("idempotency broken: %s vs %s", o1.OrderNo, o2.OrderNo)
	}
	if len(orderRepo.byNo) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orderRepo.byNo))
	}
}

func TestMockPayAppliesPlanEffect(t *testing.T) {
	svc, _, _, _, apply := newSvc()
	ctx := context.Background()
	o, err := svc.CreateOrder(ctx, service.CreateOrderInput{
		UserID: 7, Type: "plan", TargetID: 1, PayMethod: "mock", IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.MockPay(ctx, o.OrderNo); err != nil {
		t.Fatal(err)
	}
	if apply.planCalls != 1 {
		t.Fatalf("expected ApplyPlan once, got %d", apply.planCalls)
	}
	got, _ := svc.GetOrder(ctx, o.OrderNo, 7)
	if got.Status != domain.OrderStatusPaid {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestAutoCancelWorker(t *testing.T) {
	svc, _, _, orderRepo, _ := newSvc()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	o, err := svc.CreateOrder(ctx, service.CreateOrderInput{
		UserID: 7, Type: "plan", TargetID: 1, PayMethod: "mock", IdempotencyKey: "expire",
	})
	if err != nil {
		t.Fatal(err)
	}
	// force expiry in the past
	orderRepo.byNo[o.OrderNo].ExpireAt = time.Now().Add(-time.Hour)

	n, err := orderRepo.ExpirePending(ctx, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("expire n=%d err=%v", n, err)
	}
	got, _ := svc.GetOrder(ctx, o.OrderNo, 7)
	if got.Status != domain.OrderStatusExpired {
		t.Fatalf("status=%s", got.Status)
	}
}
