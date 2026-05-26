// Package nodecfg renders server-side proxy configuration (xray JSON) for a
// single node from the node's metadata + the active subscriber list. The
// node-agent fetches this from the control-plane and reloads the proxy.
//
// The output is intentionally conservative: one inbound per node (matching
// the node's protocol), uuid-keyed clients, TLS / Reality knobs sourced
// from Node.TLSConfig (already raw JSON). This keeps the contract simple
// so the node-agent can diff by hash and reload cheaply.
package nodecfg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

// Rendered is the payload returned to the node-agent. Version is a stable
// hash of the underlying inputs so the agent can skip writes when unchanged.
type Rendered struct {
	Version string          `json:"version"`
	Format  string          `json:"format"` // "xray" for now
	Config  json.RawMessage `json:"config"`
}

// RenderXray builds an xray config JSON for the given node + subscribers.
// Unknown protocols return an error so the node-agent surfaces it loudly.
func RenderXray(n domain.Node, subs []ports.Subscriber) (Rendered, error) {
	// Stable ordering: sort by UserID so the hash is deterministic.
	sort.Slice(subs, func(i, j int) bool { return subs[i].UserID < subs[j].UserID })
	clients := make([]map[string]any, 0, len(subs))
	switch n.Protocol {
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
		return Rendered{}, ErrUnsupportedProtocol{Protocol: n.Protocol}
	}

	inbound := map[string]any{
		"tag":      "in-" + n.Protocol,
		"protocol": xrayInboundProtocol(n.Protocol),
		"listen":   "0.0.0.0",
		"port":     n.Port,
		"settings": map[string]any{"clients": clients, "decryption": "none"},
	}
	if len(n.TLSConfig) > 0 {
		var tls map[string]any
		if err := json.Unmarshal(n.TLSConfig, &tls); err == nil {
			inbound["streamSettings"] = map[string]any{
				"network":         emptyOr(n.Transport, "tcp"),
				"security":        securityFor(n.Protocol),
				"realitySettings": tls,
			}
		}
	}

	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []any{inbound},
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

// ErrUnsupportedProtocol signals a node.Protocol the renderer doesn't know.
type ErrUnsupportedProtocol struct{ Protocol string }

func (e ErrUnsupportedProtocol) Error() string { return "nodecfg: unsupported protocol: " + e.Protocol }

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

func securityFor(p string) string {
	switch p {
	case domain.ProtocolVLESSReality:
		return "reality"
	case domain.ProtocolTrojan:
		return "tls"
	}
	return "none"
}

func emptyOr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func userEmail(id uint64) string { return "u" + strconv.FormatUint(id, 10) }
