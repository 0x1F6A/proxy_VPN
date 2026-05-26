package nodecfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

func TestRenderSingBox_VLESSReality(t *testing.T) {
	n := domain.Node{
		ID: 1, Name: "us-1", Protocol: domain.ProtocolVLESSReality,
		Address: "1.2.3.4", Port: 443, Transport: "tcp",
		TLSConfig: json.RawMessage(`{"sni":"example.com","private_key":"PK","short_ids":["00","beef"]}`),
	}
	subs := []ports.Subscriber{
		{UserID: 2, UUID: "uuid-b"},
		{UserID: 1, UUID: "uuid-a"},
	}
	r, err := RenderSingBox(n, subs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Format != "sing-box" || r.Version == "" {
		t.Fatalf("bad render: %+v", r)
	}
	cfg := string(r.Config)
	for _, want := range []string{`"vless"`, `"uuid-a"`, `"uuid-b"`, `"reality"`, `"u1"`, `"u2"`, `"private_key":"PK"`, `"beef"`} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q: %s", want, cfg)
		}
	}
	r2, _ := RenderSingBox(n, subs)
	if r2.Version != r.Version {
		t.Fatalf("singbox version unstable: %s vs %s", r.Version, r2.Version)
	}
}

func TestRenderSingBox_AllProtocols(t *testing.T) {
	subs := []ports.Subscriber{{UserID: 1, UUID: "secret-uuid"}}
	cases := []struct {
		proto string
		want  string
	}{
		{domain.ProtocolTrojan, `"trojan"`},
		{domain.ProtocolHysteria2, `"hysteria2"`},
		{domain.ProtocolSS2022, `"shadowsocks"`},
	}
	for _, c := range cases {
		n := domain.Node{Protocol: c.proto, Port: 9000, TLSConfig: json.RawMessage(`{"sni":"x.com"}`)}
		r, err := RenderSingBox(n, subs)
		if err != nil {
			t.Fatalf("%s: %v", c.proto, err)
		}
		if !strings.Contains(string(r.Config), c.want) || !strings.Contains(string(r.Config), `"secret-uuid"`) {
			t.Fatalf("%s render missing fields: %s", c.proto, r.Config)
		}
	}
}

func TestRenderSingBox_UnsupportedProtocol(t *testing.T) {
	n := domain.Node{Protocol: "wireguard"}
	if _, err := RenderSingBox(n, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderXray_MultiInbound(t *testing.T) {
	extra, _ := json.Marshal([]domain.NodeInbound{
		{Protocol: domain.ProtocolHysteria2, Port: 8443, TLSConfig: json.RawMessage(`{"sni":"hy.com"}`)},
	})
	n := domain.Node{
		Protocol: domain.ProtocolVLESSReality, Port: 443,
		TLSConfig: json.RawMessage(`{"sni":"example.com","pbk":"ABC"}`),
		Inbounds:  extra,
	}
	r, err := RenderXray(n, []ports.Subscriber{{UserID: 1, UUID: "u1"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg := string(r.Config)
	// Both inbounds present.
	if !strings.Contains(cfg, `"vless"`) || !strings.Contains(cfg, `"hysteria2"`) {
		t.Fatalf("expected both vless and hy2 inbounds: %s", cfg)
	}
	// Decoded structure: inbounds len == 2.
	var doc struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(r.Config, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Inbounds) != 2 {
		t.Fatalf("want 2 inbounds, got %d", len(doc.Inbounds))
	}
}

func TestRenderSingBox_MultiInbound(t *testing.T) {
	extra, _ := json.Marshal([]domain.NodeInbound{
		{Protocol: domain.ProtocolTrojan, Port: 8443, TLSConfig: json.RawMessage(`{"sni":"trojan.com"}`)},
	})
	n := domain.Node{
		Protocol: domain.ProtocolHysteria2, Port: 443,
		TLSConfig: json.RawMessage(`{"sni":"hy.com"}`),
		Inbounds:  extra,
	}
	r, err := RenderSingBox(n, []ports.Subscriber{{UserID: 1, UUID: "u1"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(r.Config, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Inbounds) != 2 {
		t.Fatalf("want 2 inbounds, got %d", len(doc.Inbounds))
	}
}

func TestNode_EffectiveEngine(t *testing.T) {
	if (domain.Node{}).EffectiveEngine() != domain.EngineXray {
		t.Fatal("default should be xray")
	}
	if (domain.Node{Engine: domain.EngineSingBox}).EffectiveEngine() != domain.EngineSingBox {
		t.Fatal("explicit sing-box should be sing-box")
	}
}

func TestNode_AllInbounds_PrimaryOnly(t *testing.T) {
	n := domain.Node{Protocol: domain.ProtocolTrojan, Port: 443}
	ins := n.AllInbounds()
	if len(ins) != 1 || ins[0].Protocol != domain.ProtocolTrojan || ins[0].Port != 443 {
		t.Fatalf("bad: %+v", ins)
	}
}

func TestNode_AllInbounds_WithExtras(t *testing.T) {
	extra, _ := json.Marshal([]domain.NodeInbound{
		{Protocol: domain.ProtocolHysteria2, Port: 8443},
		{Protocol: domain.ProtocolSS2022, Port: 8388},
	})
	n := domain.Node{Protocol: domain.ProtocolTrojan, Port: 443, Inbounds: extra}
	ins := n.AllInbounds()
	if len(ins) != 3 {
		t.Fatalf("want 3, got %d", len(ins))
	}
	if ins[0].Protocol != domain.ProtocolTrojan || ins[1].Protocol != domain.ProtocolHysteria2 || ins[2].Protocol != domain.ProtocolSS2022 {
		t.Fatalf("order wrong: %+v", ins)
	}
}
