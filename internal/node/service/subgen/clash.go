package subgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
)

// Clash renders a minimal Clash Meta YAML containing the proxies + a
// "PROXY" selector group. Returned as []byte (text/yaml).
func Clash(views []NodeView) []byte {
	views = expand(views)
	var b strings.Builder
	b.WriteString("# proxy_VPN subscription (clash-meta)\n")
	b.WriteString("proxies:\n")
	names := make([]string, 0, len(views))
	for _, v := range views {
		entry, name := clashProxy(v)
		if entry == "" {
			continue
		}
		b.WriteString(entry)
		names = append(names, name)
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: PROXY\n    type: select\n    proxies:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "      - %q\n", n)
	}
	b.WriteString("      - DIRECT\n")
	b.WriteString("rules:\n  - MATCH,PROXY\n")
	return []byte(b.String())
}

func clashProxy(v NodeView) (string, string) {
	n := v.Node
	jsonStr := func(m map[string]any) string {
		j, _ := json.Marshal(m)
		return string(j)
	}
	tls := map[string]any{}
	_ = json.Unmarshal(n.TLSConfig, &tls)
	trans := map[string]any{}
	_ = json.Unmarshal(n.TransportConfig, &trans)
	switch n.Protocol {
	case domain.ProtocolVLESSReality:
		m := map[string]any{
			"name":             n.Name,
			"type":             "vless",
			"server":           n.Address,
			"port":             n.Port,
			"uuid":             v.UserUUID,
			"network":          emptyDefault(n.Transport, "tcp"),
			"tls":              true,
			"flow":             "xtls-rprx-vision",
			"reality-opts":     tls,
			"client-fingerprint": valueOr(tls, "fp", "chrome"),
			"servername":       valueOr(tls, "sni", n.Address),
		}
		return "  - " + jsonStr(m) + "\n", n.Name
	case domain.ProtocolTrojan:
		m := map[string]any{
			"name":     n.Name,
			"type":     "trojan",
			"server":   n.Address,
			"port":     n.Port,
			"password": v.UserUUID,
			"sni":      valueOr(tls, "sni", n.Address),
			"network":  emptyDefault(n.Transport, "tcp"),
		}
		return "  - " + jsonStr(m) + "\n", n.Name
	case domain.ProtocolHysteria2:
		m := map[string]any{
			"name":     n.Name,
			"type":     "hysteria2",
			"server":   n.Address,
			"port":     n.Port,
			"password": v.UserUUID,
			"sni":      valueOr(tls, "sni", n.Address),
		}
		return "  - " + jsonStr(m) + "\n", n.Name
	case domain.ProtocolSS2022:
		m := map[string]any{
			"name":     n.Name,
			"type":     "ss",
			"server":   n.Address,
			"port":     n.Port,
			"cipher":   "2022-blake3-aes-128-gcm",
			"password": v.UserUUID,
		}
		return "  - " + jsonStr(m) + "\n", n.Name
	}
	return "", ""
}

func valueOr(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return def
}
