// Package service implements billing use cases: catalog browsing, coupon
// validation, order creation (idempotent), mock payment, and the background
// auto-cancel worker.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/ports"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/idgen"
)

// Order pending TTL — clients have this long to pay before the order is
// auto-cancelled by the worker.
const OrderPendingTTL = 15 * time.Minute

type Deps struct {
	Plans      ports.PlanRepo
	Packs      ports.DataPackRepo
	Coupons    ports.CouponRepo
	Orders     ports.OrderRepo
	UserApply  ports.UserBillingPort
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// ----- decimal helpers --------------------------------------------------

// decimal handles CNY using string-backed big.Rat to avoid float drift.
type decimal struct{ r *big.Rat }

func dec(s string) decimal {
	r := new(big.Rat)
	if s == "" {
		return decimal{r}
	}
	_, _ = r.SetString(s)
	return decimal{r}
}

func (d decimal) String() string { return d.r.FloatString(2) }

func (d decimal) Sub(o decimal) decimal { return decimal{new(big.Rat).Sub(d.r, o.r)} }
func (d decimal) Mul(o decimal) decimal { return decimal{new(big.Rat).Mul(d.r, o.r)} }
func (d decimal) Cmp(o decimal) int     { return d.r.Cmp(o.r) }

// ----- catalog ----------------------------------------------------------

func (s *Service) ListPlans(ctx context.Context, onlyActive bool) ([]domain.Plan, error) {
	return s.d.Plans.List(ctx, onlyActive)
}
func (s *Service) GetPlan(ctx context.Context, id uint64) (*domain.Plan, error) {
	return s.d.Plans.Get(ctx, id)
}
func (s *Service) CreatePlan(ctx context.Context, p *domain.Plan) error { return s.d.Plans.Create(ctx, p) }
func (s *Service) UpdatePlan(ctx context.Context, p *domain.Plan) error { return s.d.Plans.Update(ctx, p) }
func (s *Service) DeletePlan(ctx context.Context, id uint64) error      { return s.d.Plans.Delete(ctx, id) }

func (s *Service) ListPacks(ctx context.Context, onlyActive bool) ([]domain.DataPack, error) {
	return s.d.Packs.List(ctx, onlyActive)
}
func (s *Service) GetPack(ctx context.Context, id uint64) (*domain.DataPack, error) {
	return s.d.Packs.Get(ctx, id)
}
func (s *Service) CreatePack(ctx context.Context, p *domain.DataPack) error { return s.d.Packs.Create(ctx, p) }
func (s *Service) UpdatePack(ctx context.Context, p *domain.DataPack) error { return s.d.Packs.Update(ctx, p) }
func (s *Service) DeletePack(ctx context.Context, id uint64) error          { return s.d.Packs.Delete(ctx, id) }

// ----- coupon -----------------------------------------------------------

// QuoteCoupon returns the discount applicable to amount under the rules of
// the given coupon for this user/order-type. It does NOT consume the coupon.
func (s *Service) QuoteCoupon(ctx context.Context, code string, amount string, orderType string, userID uint64) (discount string, final string, err error) {
	c, err := s.d.Coupons.FindByCode(ctx, strings.TrimSpace(code))
	if err != nil || c == nil {
		return "", "", domain.ErrCouponNotFound
	}
	now := time.Now()
	if c.Status != 1 {
		return "", "", domain.ErrCouponExpired
	}
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return "", "", domain.ErrCouponExpired
	}
	if c.ExpiresAt != nil && now.After(*c.ExpiresAt) {
		return "", "", domain.ErrCouponExpired
	}
	if c.TotalQuota > 0 && c.UsedCount >= c.TotalQuota {
		return "", "", domain.ErrCouponExhausted
	}
	if c.Applicable != "all" && c.Applicable != orderType {
		return "", "", domain.ErrCouponWrongScope
	}
	amtD := dec(amount)
	if amtD.Cmp(dec(c.MinAmount)) < 0 {
		return "", "", domain.ErrCouponNotMet
	}
	used, _ := s.d.Coupons.CountUsedByUser(ctx, c.Code, userID)
	if c.PerUserLimit > 0 && used >= int(c.PerUserLimit) {
		return "", "", domain.ErrCouponExhausted
	}

	var disc decimal
	switch c.DiscountType {
	case domain.CouponDiscountFixed:
		disc = dec(c.DiscountValue)
	case domain.CouponDiscountPercent:
		pct := dec(c.DiscountValue)
		disc = amtD.Mul(pct).Mul(dec("0.01"))
	default:
		return "", "", domain.ErrCouponNotFound
	}
	if disc.Cmp(amtD) > 0 {
		disc = amtD
	}
	finalD := amtD.Sub(disc)
	return disc.String(), finalD.String(), nil
}

// ----- order creation ---------------------------------------------------

type CreateOrderInput struct {
	UserID         uint64
	Type           string // plan|pack|topup
	TargetID       uint64
	TopupAmount    string // only for type=topup
	CouponCode     string
	PayMethod      string
	IdempotencyKey string
	ClientIP       string
}

// CreateOrder is idempotent over (UserID, IdempotencyKey). If a matching
// pending order exists it is returned unchanged.
func (s *Service) CreateOrder(ctx context.Context, in CreateOrderInput) (*domain.Order, error) {
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = idgen.UUID()
	}
	if existing, _ := s.d.Orders.FindByIdempotency(ctx, in.UserID, in.IdempotencyKey); existing != nil {
		return existing, nil
	}

	var amount string
	var snapshot []byte

	switch in.Type {
	case domain.OrderTypePlan:
		p, err := s.d.Plans.Get(ctx, in.TargetID)
		if err != nil || p == nil {
			return nil, domain.ErrPlanNotFound
		}
		if p.Status != 1 {
			return nil, domain.ErrPlanInactive
		}
		amount = p.PriceCNY
		snapshot, _ = json.Marshal(p)
	case domain.OrderTypePack:
		p, err := s.d.Packs.Get(ctx, in.TargetID)
		if err != nil || p == nil {
			return nil, domain.ErrPackNotFound
		}
		if p.Status != 1 {
			return nil, domain.ErrPackInactive
		}
		amount = p.PriceCNY
		snapshot, _ = json.Marshal(p)
	case domain.OrderTypeTopup:
		if dec(in.TopupAmount).Cmp(dec("0.01")) < 0 {
			return nil, fmt.Errorf("topup amount must be >= 0.01")
		}
		amount = in.TopupAmount
	default:
		return nil, domain.ErrInvalidType
	}

	discount := "0.00"
	if in.CouponCode != "" {
		d, _, err := s.QuoteCoupon(ctx, in.CouponCode, amount, in.Type, in.UserID)
		if err != nil {
			return nil, err
		}
		discount = d
	}
	final := dec(amount).Sub(dec(discount))

	now := time.Now()
	o := &domain.Order{
		OrderNo:        genOrderNo(now),
		UserID:         in.UserID,
		Type:           in.Type,
		TargetID:       in.TargetID,
		TargetSnapshot: snapshot,
		AmountCNY:      dec(amount).String(),
		DiscountCNY:    discount,
		PaidCNY:        "0.00",
		CouponCode:     in.CouponCode,
		PayMethod:      in.PayMethod,
		Status:         domain.OrderStatusPending,
		ExpireAt:       now.Add(OrderPendingTTL),
		IdempotencyKey: in.IdempotencyKey,
		ClientIP:       in.ClientIP,
	}
	_ = final // amount stored as amount; final = amount - discount is computed at pay time
	if err := s.d.Orders.Create(ctx, o); err != nil {
		// race with parallel idempotent insert: fetch and return.
		if existing, ferr := s.d.Orders.FindByIdempotency(ctx, in.UserID, in.IdempotencyKey); ferr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return o, nil
}

// genOrderNo: yyyymmddHHMMSS + 10 random hex chars = 24 chars.
func genOrderNo(t time.Time) string {
	return t.UTC().Format("20060102150405") + idgen.HexN(10)
}

func (s *Service) GetOrder(ctx context.Context, no string, userID uint64) (*domain.Order, error) {
	o, err := s.d.Orders.FindByOrderNo(ctx, no)
	if err != nil || o == nil {
		return nil, domain.ErrOrderNotFound
	}
	if o.UserID != userID {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

// GetOrderAmount looks up an order's net payable amount (after discount)
// and owning user. Implements payment.ports.OrderPaidNotifier.
func (s *Service) GetOrderAmount(ctx context.Context, no string) (string, uint64, error) {
	o, err := s.d.Orders.FindByOrderNo(ctx, no)
	if err != nil {
		return "", 0, err
	}
	if o == nil {
		return "", 0, domain.ErrOrderNotFound
	}
	amount := dec(o.AmountCNY).Sub(dec(o.DiscountCNY)).String()
	return amount, o.UserID, nil
}

func (s *Service) ListMyOrders(ctx context.Context, userID uint64, limit, offset int) ([]domain.Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.d.Orders.ListByUser(ctx, userID, limit, offset)
}

// CancelOrder allows a user to cancel a pending order before payment.
func (s *Service) CancelOrder(ctx context.Context, no string, userID uint64) error {
	o, err := s.GetOrder(ctx, no, userID)
	if err != nil {
		return err
	}
	if o.Status != domain.OrderStatusPending {
		return domain.ErrOrderNotPayable
	}
	return s.d.Orders.UpdateStatus(ctx, no, domain.OrderStatusCancelled, nil, o.PaidCNY, "")
}

// MockPay marks the order as paid and applies the resulting effects on the
// user. Used by the dev mock provider and by tests; real providers will call
// the same downstream logic via service.HandlePaid.
func (s *Service) MockPay(ctx context.Context, no string) error {
	return s.HandlePaid(ctx, no, "mock-"+idgen.HexN(12))
}

// HandlePaid is the single entry point invoked by any payment provider after
// verifying a successful payment. Idempotent: if already paid → no-op.
func (s *Service) HandlePaid(ctx context.Context, no string, channelNo string) error {
	o, err := s.d.Orders.FindByOrderNo(ctx, no)
	if err != nil || o == nil {
		return domain.ErrOrderNotFound
	}
	if o.Status == domain.OrderStatusPaid {
		return nil
	}
	if !o.IsPayable(time.Now()) {
		return domain.ErrOrderNotPayable
	}
	final := dec(o.AmountCNY).Sub(dec(o.DiscountCNY)).String()
	now := time.Now()
	if err := s.d.Orders.UpdateStatus(ctx, no, domain.OrderStatusPaid, &now, final, channelNo); err != nil {
		return err
	}
	if o.CouponCode != "" {
		if c, _ := s.d.Coupons.FindByCode(ctx, o.CouponCode); c != nil {
			_ = s.d.Coupons.IncrementUsage(ctx, c.ID)
		}
	}
	return s.applyOrderEffects(ctx, o)
}

func (s *Service) applyOrderEffects(ctx context.Context, o *domain.Order) error {
	if s.d.UserApply == nil {
		return nil
	}
	switch o.Type {
	case domain.OrderTypePlan:
		var snap domain.Plan
		if err := json.Unmarshal(o.TargetSnapshot, &snap); err != nil {
			return err
		}
		return s.d.UserApply.ApplyPlan(ctx, o.UserID, snap.ID, snap.DurationDays, snap.TrafficGB, snap.DeviceLimit)
	case domain.OrderTypePack:
		var snap domain.DataPack
		if err := json.Unmarshal(o.TargetSnapshot, &snap); err != nil {
			return err
		}
		return s.d.UserApply.ApplyPack(ctx, o.UserID, snap.TrafficGB, snap.ValidDays, snap.AttachMode)
	case domain.OrderTypeTopup:
		return s.d.UserApply.ApplyTopup(ctx, o.UserID, dec(o.AmountCNY).Sub(dec(o.DiscountCNY)).String())
	}
	return errors.New("unknown order type")
}

// ----- auto-cancel worker ----------------------------------------------

// AutoCancelExpired flips pending orders past their expire_at to cancelled.
// Exposed as a single-shot for asynq scheduling.
func (s *Service) AutoCancelExpired(ctx context.Context) (int64, error) {
	return s.d.Orders.ExpirePending(ctx, time.Now())
}

// RunAutoCancelLoop expires pending orders past their expire_at every tick.
// Caller cancels the context to stop. Suitable until we wire Asynq in a
// later phase.
func (s *Service) RunAutoCancelLoop(ctx context.Context, tick time.Duration, log func(string, ...any)) {
	if tick <= 0 {
		tick = time.Minute
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := s.d.Orders.ExpirePending(ctx, time.Now())
			if err != nil {
				log("billing.auto_cancel error", "err", err)
				continue
			}
			if n > 0 {
				log("billing.auto_cancel expired", "count", n)
			}
		}
	}
}
