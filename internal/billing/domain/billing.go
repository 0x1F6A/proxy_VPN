// Package domain holds plan / data-pack / coupon / order entities and the
// invariants of the billing bounded context. Pure data + errors — no I/O.
package domain

import "time"

const (
	OrderTypePlan  = "plan"
	OrderTypePack  = "pack"
	OrderTypeTopup = "topup"

	OrderStatusPending   = "pending"
	OrderStatusPaid      = "paid"
	OrderStatusCancelled = "cancelled"
	OrderStatusExpired   = "expired"
	OrderStatusRefunded  = "refunded"

	PayMethodAlipay  = "alipay"
	PayMethodWechat  = "wechat"
	PayMethodUSDT    = "usdt_trc20"
	PayMethodBalance = "balance"
	PayMethodMock    = "mock" // dev / testing only

	CouponDiscountFixed   = 1
	CouponDiscountPercent = 2
)

type Plan struct {
	ID             uint64
	Name           string
	Description    string
	PriceCNY       string
	DurationDays   uint32
	TrafficGB      uint32
	DeviceLimit    uint32
	SpeedLimitMbps uint32
	NodeGroupID    uint64
	Tags           string
	Sort           int
	Status         int8
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DataPack struct {
	ID         uint64
	Name       string
	PriceCNY   string
	TrafficGB  uint32
	ValidDays  uint32
	AttachMode int8 // 1=并入当前周期 2=独立有效期
	Sort       int
	Status     int8
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Coupon struct {
	ID            uint64
	Code          string
	DiscountType  int8
	DiscountValue string
	MinAmount     string
	Applicable    string // plan|pack|all
	TotalQuota    uint32
	UsedCount     uint32
	PerUserLimit  uint32
	StartsAt      *time.Time
	ExpiresAt     *time.Time
	Status        int8
}

type Order struct {
	ID             uint64
	OrderNo        string
	UserID         uint64
	Type           string
	TargetID       uint64
	TargetSnapshot []byte // JSON
	AmountCNY      string
	DiscountCNY    string
	PaidCNY        string
	CouponCode     string
	PayMethod      string
	PayChannelNo   string
	Status         string
	ExpireAt       time.Time
	PaidAt         *time.Time
	IdempotencyKey string
	ClientIP       string
	Remark         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (o Order) IsPayable(now time.Time) bool {
	return o.Status == OrderStatusPending && now.Before(o.ExpireAt)
}
