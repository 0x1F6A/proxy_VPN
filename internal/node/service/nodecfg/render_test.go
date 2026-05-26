package nodecfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

func TestRenderXray_VLESSReality(t *testing.T) {
	n := domain.Node{
		ID: 1, Name: "us-1", Protocol: domain.ProtocolVLESSReality,
		Address: "1.2.3.4", Port: 443, Transport: "tcp",
		TLSConfig: json.RawMessage(`{"sni":"example.com","pbk":"ABC"}`),
	}
	subs := []ports.Subscriber{
		{UserID: 2, UUID: "uuid-b"},
		{UserID: 1, UUID: "uuid-a"},
	}
	r, err := RenderXray(n, subs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Format != "xray" || r.Version == "" {
		t.Fatalf("bad render: %+v", r)
	}
	cfg := string(r.Config)
	for _, want := range []string{`"vless"`, `"uuid-a"`, `"uuid-b"`, `"reality"`, `"email":"u1"`, `"email":"u2"`} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q: %s", want, cfg)
		}
	}
	// Deterministic: same inputs -> same version.
	r2, _ := RenderXray(n, subs)
	if r2.Version != r.Version {
		t.Fatalf("version unstable: %s vs %s", r.Version, r2.Version)
	}
}

func TestRenderXray_UnsupportedProtocol(t *testing.T) {
	n := domain.Node{Protocol: "wireguard"}
	if _, err := RenderXray(n, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderXray_Trojan(t *testing.T) {
	n := domain.Node{Protocol: domain.ProtocolTrojan, Port: 8443}
	r, err := RenderXray(n, []ports.Subscriber{{UserID: 5, UUID: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Config), `"trojan"`) || !strings.Contains(string(r.Config), `"secret"`) {
		t.Fatalf("trojan render: %s", r.Config)
	}
}
