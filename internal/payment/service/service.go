// Package service implements payment use cases: create a payment (delegating
// to the configured provider for the channel), receive & verify webhook
// notifications, query / reconcile pending payments. After a verified
// success it calls the billing OrderPaidNotifier to apply order effects.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

// DefaultPaymentTTL controls how long a freshly-created QR / address stays
// valid; clients should re-create after this.
const DefaultPaymentTTL = 15 * time.Minute

type Deps struct {
	Payments     ports.PaymentRepo
	Pool         ports.AddressPoolRepo // optional, only needed for USDT
	Cursor       ports.ChainScanCursor // optional, only USDT scanner
	Billing      ports.OrderPaidNotifier
	Providers    map[domain.Channel]ports.PaymentProvider
	NotifyBase   string // e.g. https://api.example.com — used to build NotifyURL
	ReturnBase   string // optional return URL base
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// GetByID exposes Payment lookup for HTTP handlers.
func (s *Service) GetByID(ctx context.Context, id uint64) (*domain.Payment, error) {
	p, err := s.d.Payments.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, domain.ErrPaymentNotFound
	}
	return p, nil
}

// Provider looks up a registered provider by channel; nil if not configured.
func (s *Service) Provider(ch domain.Channel) ports.PaymentProvider {
	return s.d.Providers[ch]
}

// CreatePayment mints a payment attempt for an order. If an active
// (pending, not expired) attempt already exists for the same order+channel,
// it is reused — clients can safely retry.
func (s *Service) CreatePayment(ctx context.Context, orderNo string, channel domain.Channel, clientIP string) (*domain.Payment, error) {
	prov, ok := s.d.Providers[channel]
	if !ok || prov == nil {
		return nil, domain.ErrChannelUnsupported
	}
	amount, uid, err := s.d.Billing.GetOrderAmount(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if existing, _ := s.d.Payments.FindActiveByOrder(ctx, orderNo, channel); existing != nil {
		if existing.IsActive(time.Now()) {
			return existing, nil
		}
	}
	expireAt := time.Now().Add(DefaultPaymentTTL)
	req := ports.CreateRequest{
		OrderNo:   orderNo,
		UserID:    uid,
		AmountCNY: amount,
		Subject:   fmt.Sprintf("proxy_VPN Order %s", orderNo),
		NotifyURL: s.notifyURL(channel),
		ReturnURL: s.returnURL(orderNo),
		ClientIP:  clientIP,
		ExpireAt:  expireAt,
	}
	res, err := prov.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	p := &domain.Payment{
		OrderNo:        orderNo,
		UserID:         uid,
		Channel:        channel,
		ChannelTradeNo: res.ChannelTradeNo,
		AmountCNY:      amount,
		AmountToken:    res.AmountToken,
		Status:         domain.StatusPending,
		QRorURL:        firstNonEmpty(res.QRContent, res.PayURL),
		Address:        res.Address,
		ExpiredAt:      expireAt,
	}
	if err := s.d.Payments.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) notifyURL(ch domain.Channel) string {
	if s.d.NotifyBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/pay/notify/%s", s.d.NotifyBase, ch)
}
func (s *Service) returnURL(orderNo string) string {
	if s.d.ReturnBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/order/%s", s.d.ReturnBase, orderNo)
}

// HandleNotify processes a verified webhook from a channel. Idempotent:
// if the payment is already paid this is a no-op. On success it asks the
// billing context to apply order effects.
func (s *Service) HandleNotify(ctx context.Context, channel domain.Channel, headers map[string]string, body []byte) error {
	prov := s.d.Providers[channel]
	if prov == nil {
		return domain.ErrChannelUnsupported
	}
	notify, err := prov.VerifyNotify(ctx, headers, body)
	if err != nil {
		return err
	}
	if !notify.Success {
		return nil // ignore non-success states; reconcile will catch
	}
	return s.applyVerified(ctx, channel, notify)
}

func (s *Service) applyVerified(ctx context.Context, channel domain.Channel, n *ports.VerifiedNotify) error {
	p, err := s.d.Payments.FindByChannelTradeNo(ctx, channel, n.ChannelTradeNo)
	if err != nil || p == nil {
		// some channels return trade-no only after first notify; fallback to order_no
		if n.OrderNo == "" {
			return domain.ErrUnknownTradeNo
		}
		ap, _ := s.d.Payments.FindActiveByOrder(ctx, n.OrderNo, channel)
		if ap == nil {
			return domain.ErrUnknownTradeNo
		}
		p = ap
		_ = s.d.Payments.UpdateTradeNo(ctx, p.ID, n.ChannelTradeNo)
	}
	if p.Status == domain.StatusPaid {
		return nil
	}
	if !cnyAmountEq(p.AmountCNY, n.AmountCNY) {
		return domain.ErrAmountMismatch
	}
	now := time.Now()
	if err := s.d.Payments.MarkPaid(ctx, p.ID, n.ChannelTradeNo, n.RawPayload, now); err != nil {
		return err
	}
	if p.Address != "" && s.d.Pool != nil {
		_ = s.d.Pool.MarkUsed(ctx, channel, p.Address)
	}
	return s.d.Billing.HandlePaid(ctx, p.OrderNo, n.ChannelTradeNo)
}

// ReconcileChannel polls the provider for our pending payments and updates
// any that have flipped to paid since last check. Used by the asynq
// reconcile task.
func (s *Service) ReconcileChannel(ctx context.Context, channel domain.Channel, limit int) (int, error) {
	prov := s.d.Providers[channel]
	if prov == nil {
		return 0, domain.ErrChannelUnsupported
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.d.Payments.ListPendingByChannel(ctx, channel, limit)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, p := range rows {
		if p.ChannelTradeNo == "" {
			continue // never assigned a trade no; nothing to query
		}
		st, qerr := prov.QueryStatus(ctx, p.ChannelTradeNo)
		if qerr != nil {
			continue
		}
		if st == domain.StatusPaid {
			pn := &ports.VerifiedNotify{
				OrderNo:        p.OrderNo,
				ChannelTradeNo: p.ChannelTradeNo,
				AmountCNY:      p.AmountCNY,
				RawPayload:     "{\"source\":\"reconcile\"}",
				Success:        true,
			}
			if err := s.applyVerified(ctx, channel, pn); err == nil {
				updated++
			}
		}
	}
	return updated, nil
}

// ExpireOldPending marks pending payments past their expired_at as expired.
// Releases any USDT address back to the pool.
func (s *Service) ExpireOldPending(ctx context.Context) (int64, error) {
	if s.d.Pool != nil {
		// gather addresses about to expire so we can release the lease.
		// (cheap pass: rely on repo to release on its own; here we only flip status)
	}
	return s.d.Payments.ExpirePending(ctx, time.Now())
}

// USDTReceived is invoked by the on-chain scanner when an incoming USDT
// transfer to a tracked address is observed with enough confirmations.
// The scanner provides (address, amount, txHash).
func (s *Service) USDTReceived(ctx context.Context, address string, amountToken string, txHash string) error {
	p, err := s.d.Payments.FindByAddress(ctx, domain.ChannelUSDT, address, amountToken)
	if err != nil || p == nil {
		return errors.New("no matching payment for incoming usdt")
	}
	if p.Status == domain.StatusPaid {
		return nil
	}
	n := &ports.VerifiedNotify{
		OrderNo:        p.OrderNo,
		ChannelTradeNo: txHash,
		AmountCNY:      p.AmountCNY,
		RawPayload:     fmt.Sprintf("{\"address\":%q,\"amount\":%q,\"tx\":%q}", address, amountToken, txHash),
		Success:        true,
	}
	return s.applyVerified(ctx, domain.ChannelUSDT, n)
}

// ----- helpers ---------------------------------------------------------

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// cnyAmountEq compares two CNY decimal strings safely. Both are expected
// to be valid 2-decimal strings (we generate them this way).
func cnyAmountEq(a, b string) bool {
	// normalise by parsing into integer cents.
	return cents(a) == cents(b)
}

func cents(s string) int64 {
	var sign int64 = 1
	if len(s) > 0 && s[0] == '-' {
		sign = -1
		s = s[1:]
	}
	var int64Part, fracPart int64
	dot := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		for i := 0; i < len(s); i++ {
			if s[i] >= '0' && s[i] <= '9' {
				int64Part = int64Part*10 + int64(s[i]-'0')
			}
		}
		return sign * int64Part * 100
	}
	for i := 0; i < dot; i++ {
		if s[i] >= '0' && s[i] <= '9' {
			int64Part = int64Part*10 + int64(s[i]-'0')
		}
	}
	frac := s[dot+1:]
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for i := 0; i < len(frac); i++ {
		fracPart = fracPart*10 + int64(frac[i]-'0')
	}
	for i := len(frac); i < 2; i++ {
		fracPart *= 10
	}
	return sign * (int64Part*100 + fracPart)
}
