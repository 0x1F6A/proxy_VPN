package subgen_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/service/subgen"
)

func sampleViews() []subgen.NodeView {
	tls := json.RawMessage(`{"sni":"example.com","pbk":"abc","sid":"01","fp":"chrome"}`)
	return []subgen.NodeView{
		{UserUUID: "u-1", Node: domain.Node{
			Name: "US-1", Region: "US", Protocol: domain.ProtocolVLESSReality,
			Address: "us1.example.com", Port: 443, TLSConfig: tls, Transport: "tcp",
		}},
		{UserUUID: "u-1", Node: domain.Node{
			Name: "JP-1", Region: "JP", Protocol: domain.ProtocolHysteria2,
			Address: "jp1.example.com", Port: 8443, TLSConfig: tls,
		}},
		{UserUUID: "u-1", Node: domain.Node{
			Name: "HK-1", Region: "HK", Protocol: domain.ProtocolTrojan,
			Address: "hk1.example.com", Port: 443, TLSConfig: tls,
		}},
	}
}

func TestV2RaySubscription(t *testing.T) {
	out := subgen.V2Ray(sampleViews())
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"vless://u-1@us1.example.com:443", "hysteria2://u-1@jp1", "trojan://u-1@hk1", "#US-1", "#JP-1"} {
		if !strings.Contains(s, want) {
			t.Errorf("v2ray output missing %q\n---\n%s", want, s)
		}
	}
}

func TestClashSubscription(t *testing.T) {
	s := string(subgen.Clash(sampleViews()))
	for _, want := range []string{"proxies:", `"name":"US-1"`, `"type":"vless"`, `"type":"trojan"`, `"type":"hysteria2"`, "PROXY"} {
		if !strings.Contains(s, want) {
			t.Errorf("clash output missing %q\n---\n%s", want, s)
		}
	}
}

func TestSingBoxSubscription(t *testing.T) {
	s := string(subgen.SingBox(sampleViews()))
	for _, want := range []string{`"type": "vless"`, `"type": "trojan"`, `"type": "hysteria2"`, `"tag": "PROXY"`, `"final": "PROXY"`} {
		if !strings.Contains(s, want) {
			t.Errorf("singbox output missing %q\n---\n%s", want, s)
		}
	}
}
