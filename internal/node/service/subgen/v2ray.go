// Package subgen renders a list of nodes into client subscription formats:
// v2ray (base64-encoded share-link list), Clash Meta YAML, and Sing-box JSON.
// All generators take the same NodeView slice so callers feed cooked data
// without leaking persistence types into clients.
package subgen

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
)

// NodeView is the projection passed to renderers. It bundles the per-user
// secret (UUID/password) with the node's public addressing so generators
// don't need to know about user-context types.
type NodeView struct {
	Node     domain.Node
	UserUUID string
}

// V2Ray renders one base64-encoded blob of share links separated by newline,
// which is what v2rayN / v2rayNG / Shadowrocket consume.
func V2Ray(views []NodeView) string {
	lines := make([]string, 0, len(views))
	for _, v := range views {
		if link := shareLink(v); link != "" {
			lines = append(lines, link)
		}
	}
	joined := strings.Join(lines, "\n")
	return base64.StdEncoding.EncodeToString([]byte(joined))
}

// shareLink emits a single protocol share URI. TLSConfig / TransportConfig
// JSON is parsed best-effort; unknown keys are ignored.
func shareLink(v NodeView) string {
	n := v.Node
	switch n.Protocol {
	case domain.ProtocolVLESSReality:
		params := url.Values{}
		params.Set("encryption", "none")
		params.Set("security", "reality")
		params.Set("type", emptyDefault(n.Transport, "tcp"))
		mergeJSON(params, n.TLSConfig)        // sni, pbk, sid, fp, spx
		mergeJSON(params, n.TransportConfig) // host, path, serviceName
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
			v.UserUUID, n.Address, n.Port, params.Encode(), url.QueryEscape(n.Name))
	case domain.ProtocolTrojan:
		params := url.Values{}
		params.Set("security", "tls")
		params.Set("type", emptyDefault(n.Transport, "tcp"))
		mergeJSON(params, n.TLSConfig)
		mergeJSON(params, n.TransportConfig)
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s",
			v.UserUUID, n.Address, n.Port, params.Encode(), url.QueryEscape(n.Name))
	case domain.ProtocolHysteria2:
		params := url.Values{}
		mergeJSON(params, n.TLSConfig) // sni, insecure, obfs, obfs-password
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s",
			v.UserUUID, n.Address, n.Port, params.Encode(), url.QueryEscape(n.Name))
	case domain.ProtocolSS2022:
		// ss://BASE64(method:password)@host:port#name — use UUID as password.
		userInfo := base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:" + v.UserUUID))
		return fmt.Sprintf("ss://%s@%s:%d#%s",
			userInfo, n.Address, n.Port, url.QueryEscape(n.Name))
	default:
		return ""
	}
}

func mergeJSON(params url.Values, raw []byte) {
	if len(raw) == 0 {
		return
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	for k, val := range m {
		params.Set(k, fmt.Sprint(val))
	}
}

func emptyDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
