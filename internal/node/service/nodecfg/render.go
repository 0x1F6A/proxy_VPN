// Package nodecfg renders server-side proxy configuration (xray or sing-box
// JSON) for a single node from the node's metadata + the active subscriber
// list. The node-agent fetches this from the control-plane and reloads the
// proxy.
//
// A node may declare multiple inbounds (mixed-protocol node) via
// Node.Inbounds; renderers expand all of them into the engine's inbound
// list. Hashes are deterministic so the agent can diff cheaply.
package nodecfg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

// Rendered is the payload returned to the node-agent. Version is a stable
// hash of the underlying inputs so the agent can skip writes when unchanged.
type Rendered struct {
	Version string          `json:"version"`
	Format  string          `json:"format"` // "xray" or "sing-box"
	Config  json.RawMessage `json:"config"`
}

// RenderXray builds an xray config JSON for the given node + subscribers.
// All inbounds advertised by the node are emitted. Unknown protocols on
// any inbound abort the render so the node-agent surfaces the error loudly.
func RenderXray(n domain.Node, subs []ports.Subscriber) (Rendered, error) {
	sort.Slice(subs, func(i, j int) bool { return subs[i].UserID < subs[j].UserID })

	inbounds := []any{}
	for _, in := range n.AllInbounds() {
		built, err := xrayInbound(in, subs)
		if err != nil {
			return Rendered{}, err
		}
		inbounds = append(inbounds, built)
	}

	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  inbounds,
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}},
		// Per-user stats prefix lets `xray api statsquery -pattern user>>>` work.
		"policy": map[string]any{
			"levels": map[string]any{"0": map[string]any{"statsUserUplink": true, "statsUserDownlink": true}},
			"system": map[string]any{"statsInboundUplink": true, "statsInboundDownlink": true},
		},
		"stats": map[string]any{},
		"api":   map[string]any{"tag": "api", "services": []string{"StatsService"}},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return Rendered{}, err
	}
	sum := sha256.Sum256(raw)
	return Rendered{
		Version: hex.EncodeToString(sum[:8]),
		Format:  "xray",
		Config:  raw,
	}, nil
}

func xrayInbound(in domain.NodeInbound, subs []ports.Subscriber) (map[string]any, error) {
	clients := make([]map[string]any, 0, len(subs))
	switch in.Protocol {
	case domain.ProtocolVLESSReality:
		for _, s := range subs {
			clients = append(clients, map[string]any{"id": s.UUID, "flow": "xtls-rprx-vision", "email": userEmail(s.UserID)})
		}
	case domain.ProtocolTrojan:
		for _, s := range subs {
			clients = append(clients, map[string]any{"password": s.UUID, "email": userEmail(s.UserID)})
		}
	case domain.ProtocolHysteria2:
		for _, s := range subs {
			clients = append(clients, map[string]any{"password": s.UUID, "email": userEmail(s.UserID)})
		}
	case domain.ProtocolSS2022:
		for _, s := range subs {
			clients = append(clients, map[string]any{"method": "2022-blake3-aes-128-gcm", "password": s.UUID, "email": userEmail(s.UserID)})
		}
	default:
		return nil, ErrUnsupportedProtocol{Protocol: in.Protocol}
	}

	inbound := map[string]any{
		"tag":      emptyOr(in.Tag, "in-"+in.Protocol),
		"protocol": xrayInboundProtocol(in.Protocol),
		"listen":   "0.0.0.0",
		"port":     in.Port,
		"settings": map[string]any{"clients": clients, "decryption": "none"},
	}
	if len(in.TLSConfig) > 0 {
		var tls map[string]any
		if err := json.Unmarshal(in.TLSConfig, &tls); err == nil {
			inbound["streamSettings"] = map[string]any{
				"network":         emptyOr(in.Transport, "tcp"),
				"security":        xraySecurityFor(in.Protocol),
				"realitySettings": tls,
			}
		}
	}
	return inbound, nil
}

func xrayInboundProtocol(p string) string {
	switch p {
	case domain.ProtocolVLESSReality:
		return "vless"
	case domain.ProtocolTrojan:
		return "trojan"
	case domain.ProtocolHysteria2:
		return "hysteria2"
	case domain.ProtocolSS2022:
		return "shadowsocks"
	}
	return p
}

func xraySecurityFor(p string) string {
	switch p {
	case domain.ProtocolVLESSReality:
		return "reality"
	case domain.ProtocolTrojan:
		return "tls"
	}
	return "none"
}
