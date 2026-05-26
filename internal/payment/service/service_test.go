package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
	"github.com/0x1F6A/proxy_VPN/internal/payment/provider/mockprov"
	"github.com/0x1F6A/proxy_VPN/internal/payment/service"
)

type fakePaymentRepo struct {
	mu    sync.Mutex
	id    uint64
	store map[uint64]*domain.Payment
}

func newFakeRepo() *fakePaymentRepo {
	return &fakePaymentRepo{store: map[uint64]*domain.Payment{}}
}
func (r *fakePaymentRepo) Create(_ context.Context, p *domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.id++
	p.ID = r.id
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	cp := *p
	r.store[p.ID] = &cp
	return nil
}
func (r *fakePaymentRepo) FindByID(_ context.Context, id uint64) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.store[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}
func (r *fakePaymentRepo) FindByOrder(_ context.Context, no string) ([]domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Payment
	for _, p := range r.store {
		if p.OrderNo == no {
			out = append(out, *p)
		}
	}
	return out, nil
}
func (r *fakePaymentRepo) FindActiveByOrder(_ context.Context, no string, ch domain.Channel) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.store {
		if p.OrderNo == no && p.Channel == ch && p.Status == domain.StatusPending {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}
func (r *fakePaymentRepo) FindByChannelTradeNo(_ context.Context, ch domain.Channel, tradeNo string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.store {
		if p.Channel == ch && p.ChannelTradeNo == tradeNo && tradeNo != "" {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}
func (r *fakePaymentRepo) FindByAddress(_ context.Context, ch domain.Channel, addr, amt string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.store {
		if p.Channel == ch && p.Address == addr && p.AmountToken == amt {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}
func (r *fakePaymentRepo) MarkPaid(_ context.Context, id uint64, tradeNo, raw string, t time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.store[id]
	if !ok {
		return errors.New("not found")
	}
	if p.Status == domain.StatusPaid {
		return nil
	}
	p.Status = domain.StatusPaid
	p.ChannelTradeNo = tradeNo
	p.RawNotify = raw
	p.PaidAt = &t
	return nil
}
func (r *fakePaymentRepo) UpdateTradeNo(_ context.Context, id uint64, tradeNo string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.store[id]; ok {
		p.ChannelTradeNo = tradeNo
	}
	return nil
}
func (r *fakePaymentRepo) ExpirePending(_ context.Context, before time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, p := range r.store {
		if p.Status == domain.StatusPending && p.ExpiredAt.Before(before) {
			p.Status = domain.StatusExpired
			n++
		}
	}
	return n, nil
}
func (r *fakePaymentRepo) ListPendingByChannel(_ context.Context, ch domain.Channel, limit int) ([]domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Payment
	for _, p := range r.store {
		if p.Channel == ch && p.Status == domain.StatusPending {
			out = append(out, *p)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

type fakeBilling struct {
	amount string
	uid    uint64
	paid   []string
}

func (b *fakeBilling) HandlePaid(_ context.Context, orderNo, _ string) error {
	b.paid = append(b.paid, orderNo)
	return nil
}
func (b *fakeBilling) GetOrderAmount(_ context.Context, _ string) (string, uint64, error) {
	return b.amount, b.uid, nil
}

func newSvc(t *testing.T) (*service.Service, *mockprov.Provider, *fakeBilling, *fakePaymentRepo) {
	t.Helper()
	mp := mockprov.New(mockprov.Config{Channel: domain.ChannelMock, Secret: "test-secret"})
	repo := newFakeRepo()
	bill := &fakeBilling{amount: "12.50", uid: 42}
	svc := service.New(service.Deps{
		Payments:   repo,
		Billing:    bill,
		Providers:  map[domain.Channel]ports.PaymentProvider{domain.ChannelMock: mp},
		NotifyBase: "http://test",
	})
	return svc, mp, bill, repo
}

func TestCreatePaymentReuseActive(t *testing.T) {
	svc, _, _, repo := newSvc(t)
	ctx := context.Background()
	p1, err := svc.CreatePayment(ctx, "ORD1", domain.ChannelMock, "1.2.3.4")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	p2, err := svc.CreatePayment(ctx, "ORD1", domain.ChannelMock, "1.2.3.4")
	if err != nil {
		t.Fatalf("create2: %v", err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("expected reuse of payment id %d, got %d", p1.ID, p2.ID)
	}
	if got := len(repo.store); got != 1 {
		t.Fatalf("expected 1 stored payment, got %d", got)
	}
}

func TestHandleNotifySuccessIdempotent(t *testing.T) {
	svc, mp, bill, _ := newSvc(t)
	ctx := context.Background()
	p, err := svc.CreatePayment(ctx, "ORD2", domain.ChannelMock, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	headers, body, err := mp.SignNotifyFields(p.OrderNo, p.ChannelTradeNo, p.AmountCNY, true)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := svc.HandleNotify(ctx, domain.ChannelMock, headers, body); err != nil {
		t.Fatalf("notify1: %v", err)
	}
	if err := svc.HandleNotify(ctx, domain.ChannelMock, headers, body); err != nil {
		t.Fatalf("notify2 (idempotent): %v", err)
	}
	if len(bill.paid) != 1 {
		t.Fatalf("billing.HandlePaid called %d times, want 1", len(bill.paid))
	}
}

func TestHandleNotifyAmountMismatch(t *testing.T) {
	svc, mp, _, _ := newSvc(t)
	ctx := context.Background()
	p, _ := svc.CreatePayment(ctx, "ORD3", domain.ChannelMock, "")
	headers, body, _ := mp.SignNotifyFields(p.OrderNo, p.ChannelTradeNo, "99.99", true)
	err := svc.HandleNotify(ctx, domain.ChannelMock, headers, body)
	if !errors.Is(err, domain.ErrAmountMismatch) {
		t.Fatalf("expected ErrAmountMismatch, got %v", err)
	}
}

func TestHandleNotifyBadSignature(t *testing.T) {
	svc, mp, _, _ := newSvc(t)
	ctx := context.Background()
	p, _ := svc.CreatePayment(ctx, "ORD4", domain.ChannelMock, "")
	headers, body, _ := mp.SignNotifyFields(p.OrderNo, p.ChannelTradeNo, p.AmountCNY, true)
	headers["X-Mock-Sign"] = "deadbeef"
	if err := svc.HandleNotify(ctx, domain.ChannelMock, headers, body); err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestExpireOldPending(t *testing.T) {
	svc, _, _, repo := newSvc(t)
	ctx := context.Background()
	p, _ := svc.CreatePayment(ctx, "ORD5", domain.ChannelMock, "")
	repo.mu.Lock()
	repo.store[p.ID].ExpiredAt = time.Now().Add(-time.Minute)
	repo.mu.Unlock()
	n, err := svc.ExpireOldPending(ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 expired, got %d", n)
	}
}
