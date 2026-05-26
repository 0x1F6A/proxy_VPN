// Package ports defines payment outbound interfaces. Providers implement
// PaymentProvider; persistence implements PaymentRepo / AddressPoolRepo;
// upstream billing callbacks happen through OrderPaidNotifier.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
)

// CreateRequest carries everything a provider needs to mint a payment.
type CreateRequest struct {
	OrderNo   string
	UserID    uint64
	AmountCNY string
	Subject   string // shown on alipay/wechat
	NotifyURL string // webhook callback URL (server-built)
	ReturnURL string // optional browser return
	ClientIP  string
	ExpireAt  time.Time
}

// CreateResult is what the provider returns to the client. For QR-based
// channels QRContent is the string to render; PayURL is a deep-link.
// For USDT, Address is filled and AmountToken is the on-chain transfer amount.
type CreateResult struct {
	ChannelTradeNo string
	QRContent      string
	PayURL         string
	Address        string
	AmountToken    string
}

// VerifiedNotify is the normalised view of a webhook payload after the
// provider has validated its signature.
type VerifiedNotify struct {
	OrderNo        string
	ChannelTradeNo string
	AmountCNY      string
	RawPayload     string
	Success        bool
}

// PaymentProvider abstracts a channel. Implementations: alipay, wechat,
// usdt-trc20, mock.
type PaymentProvider interface {
	Channel() domain.Channel
	Create(ctx context.Context, req CreateRequest) (*CreateResult, error)
	// VerifyNotify parses raw webhook payload (HTTP body + headers), validates
	// signature, and returns the normalised result.
	VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*VerifiedNotify, error)
	// QueryStatus actively queries the channel for a payment we created.
	// Used by reconcile tasks.
	QueryStatus(ctx context.Context, channelTradeNo string) (domain.Status, error)
}

// PaymentRepo is the storage port for Payment records.
type PaymentRepo interface {
	Create(ctx context.Context, p *domain.Payment) error
	FindByID(ctx context.Context, id uint64) (*domain.Payment, error)
	FindByOrder(ctx context.Context, orderNo string) ([]domain.Payment, error)
	FindActiveByOrder(ctx context.Context, orderNo string, channel domain.Channel) (*domain.Payment, error)
	FindByChannelTradeNo(ctx context.Context, channel domain.Channel, tradeNo string) (*domain.Payment, error)
	FindByAddress(ctx context.Context, channel domain.Channel, address, amountToken string) (*domain.Payment, error)
	MarkPaid(ctx context.Context, id uint64, tradeNo string, raw string, paidAt time.Time) error
	UpdateTradeNo(ctx context.Context, id uint64, tradeNo string) error
	ExpirePending(ctx context.Context, before time.Time) (int64, error)
	ListPendingByChannel(ctx context.Context, channel domain.Channel, limit int) ([]domain.Payment, error)
}

// AddressPoolRepo manages USDT one-time receiving addresses.
type AddressPoolRepo interface {
	Allocate(ctx context.Context, channel domain.Channel, orderNo string) (*domain.AddressLease, error)
	Release(ctx context.Context, channel domain.Channel, address string) error
	MarkUsed(ctx context.Context, channel domain.Channel, address string) error
	Seed(ctx context.Context, channel domain.Channel, addrs []string) error
	CountFree(ctx context.Context, channel domain.Channel) (int64, error)
}

// ChainScanCursor stores per-chain last-scanned block height.
type ChainScanCursor interface {
	Get(ctx context.Context, chain string) (int64, error)
	Set(ctx context.Context, chain string, block int64) error
}

// OrderPaidNotifier is implemented by the billing context. The payment
// service calls it once after a successful, verified payment to apply order
// effects. Must be idempotent.
type OrderPaidNotifier interface {
	HandlePaid(ctx context.Context, orderNo string, channelTradeNo string) error
	GetOrderAmount(ctx context.Context, orderNo string) (amountCNY string, userID uint64, err error)
}
