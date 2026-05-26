// Package wechatprov adapts wechatpay-apiv3/wechatpay-go to the payment
// ports interface, implementing the Native (QR-code) flow under API v3.
package wechatprov

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

type Config struct {
	MchID          string
	AppID          string
	SerialNo       string
	PrivateKeyPEM  string
	APIv3Key       string
	NotifyURLOverride string
}

type Provider struct {
	cfg     Config
	client  *core.Client
	handler *notify.Handler
	native  native.NativeApiService
}

// New constructs a Wechat Pay v3 provider. Returns an error if any
// required credential is empty. Bootstrapping registers the merchant with
// the platform-certificate downloader manager.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.MchID == "" || cfg.AppID == "" || cfg.SerialNo == "" ||
		cfg.PrivateKeyPEM == "" || cfg.APIv3Key == "" {
		return nil, errors.New("wechat: missing credentials")
	}
	privKey, err := utils.LoadPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("wechat: load priv key: %w", err)
	}
	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(cfg.MchID, cfg.SerialNo, privKey, cfg.APIv3Key),
	}
	cli, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("wechat: new client: %w", err)
	}
	// Register the downloader so the verifier can fetch & rotate the
	// platform certificate(s). Without this the verifier panics on first
	// notification.
	if err := downloader.MgrInstance().RegisterDownloaderWithPrivateKey(
		ctx, privKey, cfg.SerialNo, cfg.MchID, cfg.APIv3Key,
	); err != nil {
		return nil, fmt.Errorf("wechat: register downloader: %w", err)
	}
	visitor := downloader.MgrInstance().GetCertificateVisitor(cfg.MchID)
	handler, err := notify.NewRSANotifyHandler(cfg.APIv3Key, verifiers.NewSHA256WithRSAVerifier(visitor))
	if err != nil {
		return nil, fmt.Errorf("wechat: new notify handler: %w", err)
	}
	return &Provider{
		cfg:     cfg,
		client:  cli,
		handler: handler,
		native:  native.NativeApiService{Client: cli},
	}, nil
}

func (p *Provider) Channel() domain.Channel { return domain.ChannelWechat }

// Create issues a Native prepay request. AmountCNY is in yuan (string)
// and must be converted to integer cents.
func (p *Provider) Create(ctx context.Context, req ports.CreateRequest) (*ports.CreateResult, error) {
	cents := cnyToCents(req.AmountCNY)
	notifyURL := firstNonEmpty(p.cfg.NotifyURLOverride, req.NotifyURL)
	resp, _, err := p.native.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(p.cfg.AppID),
		Mchid:       core.String(p.cfg.MchID),
		Description: core.String(req.Subject),
		OutTradeNo:  core.String(req.OrderNo),
		NotifyUrl:   core.String(notifyURL),
		Amount: &native.Amount{
			Currency: core.String("CNY"),
			Total:    core.Int64(cents),
		},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.CodeUrl == nil {
		return nil, errors.New("wechat: empty code_url")
	}
	return &ports.CreateResult{
		ChannelTradeNo: "", // wechat assigns transaction_id after payment
		QRContent:      *resp.CodeUrl,
	}, nil
}

func (p *Provider) VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*ports.VerifiedNotify, error) {
	// Reconstruct an http.Request the notify handler understands.
	r, _ := http.NewRequest(http.MethodPost, "/notify", io.NopCloser(strings.NewReader(string(body))))
	r.Header = http.Header{}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	tx := new(payments.Transaction)
	if _, err := p.handler.ParseNotifyRequest(ctx, r, tx); err != nil {
		return nil, domain.ErrSignatureInvalid
	}
	out := &ports.VerifiedNotify{
		OrderNo:    strDeref(tx.OutTradeNo),
		ChannelTradeNo: strDeref(tx.TransactionId),
		RawPayload: string(body),
		Success:    strDeref(tx.TradeState) == "SUCCESS",
	}
	if tx.Amount != nil && tx.Amount.Total != nil {
		out.AmountCNY = centsToCNY(*tx.Amount.Total)
	}
	return out, nil
}

func (p *Provider) QueryStatus(ctx context.Context, channelTradeNo string) (domain.Status, error) {
	// channelTradeNo holds either wechat transaction_id (we'd prefix "wx-")
	// or our out_trade_no. We default to out_trade_no lookup.
	resp, _, err := p.native.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(strings.TrimPrefix(channelTradeNo, "wx-")),
		Mchid:      core.String(p.cfg.MchID),
	})
	if err != nil {
		return "", err
	}
	switch strDeref(resp.TradeState) {
	case "SUCCESS":
		return domain.StatusPaid, nil
	case "CLOSED", "REVOKED", "PAYERROR":
		return domain.StatusFailed, nil
	}
	return domain.StatusPending, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func cnyToCents(s string) int64 {
	// "12.34" → 1234
	dot := strings.Index(s, ".")
	if dot < 0 {
		n, _ := strconv.ParseInt(s, 10, 64)
		return n * 100
	}
	whole := s[:dot]
	frac := s[dot+1:]
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	return w*100 + f
}
func centsToCNY(c int64) string {
	yuan := c / 100
	cents := c % 100
	if cents < 0 {
		cents = -cents
	}
	return fmt.Sprintf("%d.%02d", yuan, cents)
}
