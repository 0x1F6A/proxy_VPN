// Package ports defines the billing-context outbound interfaces.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
)

type PlanRepo interface {
	List(ctx context.Context, onlyActive bool) ([]domain.Plan, error)
	Get(ctx context.Context, id uint64) (*domain.Plan, error)
	Create(ctx context.Context, p *domain.Plan) error
	Update(ctx context.Context, p *domain.Plan) error
	Delete(ctx context.Context, id uint64) error
}

type DataPackRepo interface {
	List(ctx context.Context, onlyActive bool) ([]domain.DataPack, error)
	Get(ctx context.Context, id uint64) (*domain.DataPack, error)
	Create(ctx context.Context, p *domain.DataPack) error
	Update(ctx context.Context, p *domain.DataPack) error
	Delete(ctx context.Context, id uint64) error
}

type CouponRepo interface {
	FindByCode(ctx context.Context, code string) (*domain.Coupon, error)
	// IncrementUsage atomically increments used_count, refusing if exhausted.
	IncrementUsage(ctx context.Context, id uint64) error
	CountUsedByUser(ctx context.Context, code string, userID uint64) (int, error)

	// Admin CRUD
	List(ctx context.Context, q string, limit, offset int) ([]domain.Coupon, int64, error)
	Get(ctx context.Context, id uint64) (*domain.Coupon, error)
	Create(ctx context.Context, c *domain.Coupon) error
	Update(ctx context.Context, c *domain.Coupon) error
	Delete(ctx context.Context, id uint64) error
}

// OrderFilter filters orders for admin listing. Zero-value fields are ignored.
type OrderFilter struct {
	Status  string
	Type    string
	UserID  uint64
	OrderNo string
	From    *time.Time
	To      *time.Time
}

type OrderRepo interface {
	Create(ctx context.Context, o *domain.Order) error
	FindByIdempotency(ctx context.Context, userID uint64, key string) (*domain.Order, error)
	FindByOrderNo(ctx context.Context, no string) (*domain.Order, error)
	ListByUser(ctx context.Context, userID uint64, limit, offset int) ([]domain.Order, error)
	AdminList(ctx context.Context, f OrderFilter, limit, offset int) ([]domain.Order, int64, error)
	UpdateStatus(ctx context.Context, no string, status string, paidAt *time.Time, paid string, channelNo string) error
	ExpirePending(ctx context.Context, before time.Time) (int64, error)
}

// UserBillingPort applies plan / pack / topup effects on the user aggregate
// after a successful payment. Lives in the user bounded context.
type UserBillingPort interface {
	ApplyPlan(ctx context.Context, userID, planID uint64, durationDays, trafficGB, deviceLimit uint32) error
	ApplyPack(ctx context.Context, userID uint64, trafficGB, validDays uint32, attachMode int8) error
	ApplyTopup(ctx context.Context, userID uint64, amountCNY string) error
}
