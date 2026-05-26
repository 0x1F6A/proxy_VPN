// Package alipayprov adapts smartwalle/alipay/v3 to the payment ports
// interface, implementing the face-to-face (precreate → QR) flow.
package alipayprov

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	sdk "github.com/smartwalle/alipay/v3"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

type Config struct {
	AppID            string
	PrivateKey       string // PEM
	AliPayPublicKey  string // PEM
	Production       bool
	NotifyURLOverride string
}

type Provider struct {
	c   *sdk.Client
	cfg Config
}

// New constructs an Alipay provider. Returns nil + error if credentials
// are missing or invalid.
func New(cfg Config) (*Provider, error) {
	if cfg.AppID == "" || cfg.PrivateKey == "" || cfg.AliPayPublicKey == "" {
		return nil, errors.New("alipay: missing credentials")
	}
	c, err := sdk.New(cfg.AppID, cfg.PrivateKey, cfg.Production)
	if err != nil {
		return nil, err
	}
	if err := c.LoadAliPayPublicKey(cfg.AliPayPublicKey); err != nil {
		return nil, err
	}
	return &Provider{c: c, cfg: cfg}, nil
}

func (p *Provider) Channel() domain.Channel { return domain.ChannelAlipay }

func (p *Provider) Create(ctx context.Context, req ports.CreateRequest) (*ports.CreateResult, error) {
	param := sdk.TradePreCreate{}
	param.Subject = req.Subject
	param.OutTradeNo = req.OrderNo
	param.TotalAmount = req.AmountCNY
	param.NotifyURL = firstNonEmpty(p.cfg.NotifyURLOverride, req.NotifyURL)

	rsp, err := p.c.TradePreCreate(ctx, param)
	if err != nil {
		return nil, err
	}
	if !rsp.IsSuccess() {
		return nil, errors.New("alipay precreate: " + rsp.Msg + " " + rsp.SubMsg)
	}
	return &ports.CreateResult{
		// alipay does not return a trade no until paid; key on out_trade_no
		// (which equals our order_no) until first callback updates it.
		ChannelTradeNo: "",
		QRContent:      rsp.QRCode,
	}, nil
}

func (p *Provider) VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*ports.VerifiedNotify, error) {
	// Alipay sends application/x-www-form-urlencoded; we received raw body.
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	// Build a fake http.Request just to reuse the SDK verify helper.
	r := &http.Request{Method: http.MethodPost, Form: values, PostForm: values}
	r.Header = http.Header{}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	r.Body = http.NoBody
	notif, err := p.c.GetTradeNotification(r)
	if err != nil {
		return nil, domain.ErrSignatureInvalid
	}
	return &ports.VerifiedNotify{
		OrderNo:        notif.OutTradeNo,
		ChannelTradeNo: notif.TradeNo,
		AmountCNY:      notif.TotalAmount,
		RawPayload:     string(body),
		Success:        notif.TradeStatus == sdk.TradeStatusSuccess || notif.TradeStatus == sdk.TradeStatusFinished,
	}, nil
}

func (p *Provider) QueryStatus(ctx context.Context, channelTradeNo string) (domain.Status, error) {
	q := sdk.TradeQuery{}
	if strings.HasPrefix(channelTradeNo, "ali-") {
		q.TradeNo = strings.TrimPrefix(channelTradeNo, "ali-")
	} else {
		q.OutTradeNo = channelTradeNo
	}
	rsp, err := p.c.TradeQuery(ctx, q)
	if err != nil {
		return "", err
	}
	if !rsp.IsSuccess() {
		return domain.StatusPending, nil
	}
	switch rsp.TradeStatus {
	case sdk.TradeStatusSuccess, sdk.TradeStatusFinished:
		return domain.StatusPaid, nil
	case sdk.TradeStatusClosed:
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
