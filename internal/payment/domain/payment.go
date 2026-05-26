// Package domain defines the payment bounded context entities and state
// machine. A Payment is one attempt to settle an Order via a specific
// channel (alipay / wechat / usdt-trc20). Multiple attempts may exist per
// order (e.g. when the previous QR expired) — only one may reach `paid`.
package domain

import "time"

type Channel string

const (
	ChannelAlipay Channel = "alipay"
	ChannelWechat Channel = "wechat"
	ChannelUSDT   Channel = "usdt_trc20"
	ChannelMock   Channel = "mock"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusExpired  Status = "expired"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

// Payment is a single payment attempt for an Order.
type Payment struct {
	ID             uint64
	OrderNo        string
	UserID         uint64
	Channel        Channel
	ChannelTradeNo string
	AmountCNY      string
	AmountToken    string // USDT amount, kept as string-decimal
	Status         Status
	QRorURL        string // alipay/wechat: qr content; USDT: deeplink optional
	Address        string // USDT receiving address
	RawNotify      string // last verified callback payload
	PaidAt         *time.Time
	ExpiredAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (p Payment) IsActive(now time.Time) bool {
	return p.Status == StatusPending && now.Before(p.ExpiredAt)
}

// AddressLease represents a USDT one-time receiving address allocated to
// a specific order. Released back to the pool after success / expiry.
type AddressLease struct {
	ID          uint64
	Channel     Channel
	Address     string
	Status      string // free|allocated|used
	OrderNo     string
	AllocatedAt *time.Time
	ReleasedAt  *time.Time
}
