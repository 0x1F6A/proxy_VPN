package subgen

import (
	"encoding/json"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
)

// SingBox renders a sing-box 1.8 config skeleton (outbounds only). Caller
// can embed it under their own log / inbounds / dns section if needed; we
// produce a valid standalone document for v1.
func SingBox(views []NodeView) []byte {
	views = expand(views)
	outs := []map[string]any{}
	tags := []string{}
	for _, v := range views {
		o, tag := singBoxOutbound(v)
		if o == nil {
			continue
		}
		outs = append(outs, o)
		tags = append(tags, tag)
	}
	outs = append(outs, map[string]any{"type": "selector", "tag": "PROXY", "outbounds": tags},
		map[string]any{"type": "direct", "tag": "direct"},
		map[string]any{"type": "block", "tag": "block"})

	doc := map[string]any{
		"log":       map[string]any{"level": "info"},
		"outbounds": outs,
		"route": map[string]any{
			"final": "PROXY",
		},
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return b
}

func singBoxOutbound(v NodeView) (map[string]any, string) {
	n := v.Node
	tls := map[string]any{}
	_ = json.Unmarshal(n.TLSConfig, &tls)
	trans := map[string]any{}
	_ = json.Unmarshal(n.TransportConfig, &trans)
	switch n.Protocol {
	case domain.ProtocolVLESSReality:
		o := map[string]any{
			"type":        "vless",
			"tag":         n.Name,
			"server":      n.Address,
			"server_port": n.Port,
			"uuid":        v.UserUUID,
			"flow":        "xtls-rprx-vision",
			"tls": map[string]any{
				"enabled":     true,
				"server_name": valueOr(tls, "sni", n.Address),
				"reality": map[string]any{
					"enabled":    true,
					"public_key": valueOr(tls, "pbk", ""),
					"short_id":   valueOr(tls, "sid", ""),
				},
				"utls": map[string]any{
					"enabled":     true,
					"fingerprint": valueOr(tls, "fp", "chrome"),
				},
			},
		}
		return o, n.Name
	case domain.ProtocolTrojan:
		o := map[string]any{
			"type":        "trojan",
			"tag":         n.Name,
			"server":      n.Address,
			"server_port": n.Port,
			"password":    v.UserUUID,
			"tls": map[string]any{
				"enabled":     true,
				"server_name": valueOr(tls, "sni", n.Address),
			},
		}
		return o, n.Name
	case domain.ProtocolHysteria2:
		o := map[string]any{
			"type":        "hysteria2",
			"tag":         n.Name,
			"server":      n.Address,
			"server_port": n.Port,
			"password":    v.UserUUID,
			"tls": map[string]any{
				"enabled":     true,
				"server_name": valueOr(tls, "sni", n.Address),
			},
		}
		return o, n.Name
	case domain.ProtocolSS2022:
		o := map[string]any{
			"type":        "shadowsocks",
			"tag":         n.Name,
			"server":      n.Address,
			"server_port": n.Port,
			"method":      "2022-blake3-aes-128-gcm",
			"password":    v.UserUUID,
		}
		return o, n.Name
	}
	return nil, ""
}
