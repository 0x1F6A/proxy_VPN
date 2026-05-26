package mockprov_test

import (
	"context"
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/provider/mockprov"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	p := mockprov.New(mockprov.Config{Channel: domain.ChannelAlipay, Secret: "topsecret"})
	headers, body, _ := p.SignNotifyFields("ORD1", "trade-1", "9.99", true)
	v, err := p.VerifyNotify(context.Background(), headers, body)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v.OrderNo != "ORD1" || v.ChannelTradeNo != "trade-1" || v.AmountCNY != "9.99" || !v.Success {
		t.Fatalf("unexpected verified: %+v", v)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	p := mockprov.New(mockprov.Config{Channel: domain.ChannelAlipay, Secret: "topsecret"})
	_, body, _ := p.SignNotifyFields("ORD1", "trade-1", "9.99", true)
	if _, err := p.VerifyNotify(context.Background(), map[string]string{"X-Mock-Sign": "bad"}, body); err == nil {
		t.Fatal("expected error on bad signature")
	}
}
