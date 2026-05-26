// Package mockprov is a deterministic in-memory PaymentProvider used in dev
// environments and unit tests. It generates predictable QR strings and
// validates callbacks by a shared HMAC instead of real provider signatures.
package mockprov

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

type Config struct {
	Channel domain.Channel
	Secret  string // HMAC secret for callback signing
}

type Provider struct {
	cfg Config
	mu  sync.Mutex
	// state holds artificial channel-side status; lets tests flip a
	// payment to "paid" via SimulatePaid before asking QueryStatus.
	state map[string]domain.Status
}

func New(cfg Config) *Provider {
	if cfg.Channel == "" {
		cfg.Channel = domain.ChannelMock
	}
	return &Provider{cfg: cfg, state: map[string]domain.Status{}}
}

func (p *Provider) Channel() domain.Channel { return p.cfg.Channel }

func (p *Provider) Create(ctx context.Context, req ports.CreateRequest) (*ports.CreateResult, error) {
	tradeNo := fmt.Sprintf("mock-%s-%d", req.OrderNo, time.Now().UnixNano())
	res := &ports.CreateResult{
		ChannelTradeNo: tradeNo,
		QRContent:      fmt.Sprintf("mock://pay?order=%s&amount=%s", req.OrderNo, req.AmountCNY),
		PayURL:         fmt.Sprintf("https://mock.local/pay/%s", req.OrderNo),
	}
	if p.cfg.Channel == domain.ChannelUSDT {
		res.Address = fmt.Sprintf("Tmock%020x", time.Now().UnixNano())
		res.AmountToken = req.AmountCNY // 1:1 in tests
	}
	p.mu.Lock()
	p.state[tradeNo] = domain.StatusPending
	p.mu.Unlock()
	return res, nil
}

// SimulatePaid is a test-only hook to flip a trade no to paid.
func (p *Provider) SimulatePaid(tradeNo string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state[tradeNo] = domain.StatusPaid
}

type mockNotifyBody struct {
	OrderNo   string `json:"order_no"`
	TradeNo   string `json:"trade_no"`
	AmountCNY string `json:"amount_cny"`
	Success   bool   `json:"success"`
}

// SignNotify is a helper used by tests / dev to mint a valid payload.
func (p *Provider) SignNotify(b mockNotifyBody) (body []byte, sig string) {
	body, _ = json.Marshal(b)
	mac := hmac.New(sha256.New, []byte(p.cfg.Secret))
	_, _ = mac.Write(body)
	return body, hex.EncodeToString(mac.Sum(nil))
}

// SignNotifyFields wraps SignNotify with primitive arguments — convenient
// for tests in other packages where mockNotifyBody is unexported.
func (p *Provider) SignNotifyFields(orderNo, tradeNo, amountCNY string, success bool) (headers map[string]string, body []byte, err error) {
	body, sig := p.SignNotify(mockNotifyBody{OrderNo: orderNo, TradeNo: tradeNo, AmountCNY: amountCNY, Success: success})
	return map[string]string{"X-Mock-Sign": sig}, body, nil
}

func (p *Provider) VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*ports.VerifiedNotify, error) {
	sig := headers["X-Mock-Sign"]
	if sig == "" {
		sig = headers["x-mock-sign"]
	}
	if p.cfg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(p.cfg.Secret))
		_, _ = mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(want)) {
			return nil, domain.ErrSignatureInvalid
		}
	}
	var b mockNotifyBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	return &ports.VerifiedNotify{
		OrderNo:        b.OrderNo,
		ChannelTradeNo: b.TradeNo,
		AmountCNY:      b.AmountCNY,
		RawPayload:     string(body),
		Success:        b.Success,
	}, nil
}

func (p *Provider) QueryStatus(ctx context.Context, tradeNo string) (domain.Status, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st, ok := p.state[tradeNo]
	if !ok {
		return "", domain.ErrUnknownTradeNo
	}
	return st, nil
}
