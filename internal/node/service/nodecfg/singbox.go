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

// RenderSingBox builds a sing-box (1.8+) server config JSON for the node.
// All Node.AllInbounds() are emitted as sing-box inbounds with the user
// list embedded directly. Unknown protocols abort the render.
//
// Output is a complete standalone document — the node-agent writes it to
// disk and reloads sing-box.
func RenderSingBox(n domain.Node, subs []ports.Subscriber) (Rendered, error) {
	sort.Slice(subs, func(i, j int) bool { return subs[i].UserID < subs[j].UserID })

	inbounds := []any{}
	for _, in := range n.AllInbounds() {
		built, err := singBoxInbound(in, subs)
		if err != nil {
			return Rendered{}, err
		}
		inbounds = append(inbounds, built)
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "warn", "timestamp": true},
		"dns": map[string]any{
			"servers": []map[string]any{
				{"tag": "google", "address": "https://8.8.8.8/dns-query"},
				{"tag": "local", "address": "local", "detour": "direct"},
			},
			"strategy": "ipv4_only",
		},
		"inbounds": inbounds,
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "block"},
		},
		"experimental": map[string]any{
			"v2ray_api": map[string]any{
				"listen": "127.0.0.1:8080",
				"stats":  map[string]any{"enabled": true, "users": singBoxAllUserTags(subs)},
			},
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return Rendered{}, err
	}
	sum := sha256.Sum256(raw)
	return Rendered{
		Version: hex.EncodeToString(sum[:8]),
		Format:  "sing-box",
		Config:  raw,
	}, nil
}

func singBoxInbound(in domain.NodeInbound, subs []ports.Subscriber) (map[string]any, error) {
	users := singBoxUsers(in.Protocol, subs)
	if users == nil {
		return nil, ErrUnsupportedProtocol{Protocol: in.Protocol}
	}

	base := map[string]any{
		"tag":         emptyOr(in.Tag, "in-"+in.Protocol),
		"listen":      "::",
		"listen_port": in.Port,
		"users":       users,
	}

	switch in.Protocol {
	case domain.ProtocolVLESSReality:
		base["type"] = "vless"
		base["tls"] = singBoxRealityTLS(in)
	case domain.ProtocolTrojan:
		base["type"] = "trojan"
		base["tls"] = singBoxStdTLS(in)
	case domain.ProtocolHysteria2:
		base["type"] = "hysteria2"
		base["tls"] = singBoxStdTLS(in)
	case domain.ProtocolSS2022:
		base["type"] = "shadowsocks"
		base["method"] = "2022-blake3-aes-128-gcm"
		// SS uses a single server-side password + per-user passwords
		// embedded in users. For simplicity we set an empty server key
		// and rely on per-user keys (operator may override via TLSConfig
		// JSON future enhancement).
		delete(base, "users")
		base["password"] = ""
		base["users"] = users
	}

	if t := emptyOr(in.Transport, ""); t != "" && t != "tcp" {
		base["transport"] = singBoxTransport(t, in.TransportConfig)
	}
	return base, nil
}

func singBoxUsers(protocol string, subs []ports.Subscriber) []map[string]any {
	out := make([]map[string]any, 0, len(subs))
	switch protocol {
	case domain.ProtocolVLESSReality:
		for _, s := range subs {
			out = append(out, map[string]any{"name": userEmail(s.UserID), "uuid": s.UUID, "flow": "xtls-rprx-vision"})
		}
	case domain.ProtocolTrojan, domain.ProtocolHysteria2:
		for _, s := range subs {
			out = append(out, map[string]any{"name": userEmail(s.UserID), "password": s.UUID})
		}
	case domain.ProtocolSS2022:
		for _, s := range subs {
			out = append(out, map[string]any{"name": userEmail(s.UserID), "password": s.UUID})
		}
	default:
		return nil
	}
	return out
}

func singBoxAllUserTags(subs []ports.Subscriber) []string {
	out := make([]string, 0, len(subs))
	for _, s := range subs {
		out = append(out, userEmail(s.UserID))
	}
	return out
}

// singBoxRealityTLS reads Reality params from the inbound's TLSConfig JSON.
// Expected keys: sni / pbk / sid / fp / private_key / short_ids.
func singBoxRealityTLS(in domain.NodeInbound) map[string]any {
	cfg := map[string]string{}
	_ = json.Unmarshal(in.TLSConfig, &cfg)
	shortIDs := []string{}
	if sid := cfg["sid"]; sid != "" {
		shortIDs = append(shortIDs, sid)
	}
	if extras, ok := decodeShortIDs(in.TLSConfig); ok {
		shortIDs = extras
	}
	return map[string]any{
		"enabled":     true,
		"server_name": emptyOr(cfg["sni"], cfg["server_name"]),
		"reality": map[string]any{
			"enabled":     true,
			"handshake":   map[string]any{"server": emptyOr(cfg["sni"], cfg["server_name"]), "server_port": 443},
			"private_key": cfg["private_key"],
			"short_id":    shortIDs,
		},
	}
}

func singBoxStdTLS(in domain.NodeInbound) map[string]any {
	cfg := map[string]string{}
	_ = json.Unmarshal(in.TLSConfig, &cfg)
	tls := map[string]any{
		"enabled":     true,
		"server_name": emptyOr(cfg["sni"], cfg["server_name"]),
	}
	if cert := cfg["certificate_path"]; cert != "" {
		tls["certificate_path"] = cert
	}
	if key := cfg["key_path"]; key != "" {
		tls["key_path"] = key
	}
	return tls
}

func singBoxTransport(network string, raw json.RawMessage) map[string]any {
	cfg := map[string]any{}
	_ = json.Unmarshal(raw, &cfg)
	cfg["type"] = network
	return cfg
}

// decodeShortIDs allows TLSConfig to carry a `short_ids` array.
func decodeShortIDs(raw json.RawMessage) ([]string, bool) {
	var t struct {
		ShortIDs []string `json:"short_ids"`
	}
	if err := json.Unmarshal(raw, &t); err != nil || len(t.ShortIDs) == 0 {
		return nil, false
	}
	return t.ShortIDs, true
}

// portKey returns a stable string for the inbound port (used in deterministic tags).
func portKey(p uint32) string { return strconv.FormatUint(uint64(p), 10) }
