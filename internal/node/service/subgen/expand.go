package subgen

import "github.com/0x1F6A/proxy_VPN/internal/node/domain"

// expand flattens NodeViews so that each inbound becomes its own
// NodeView (with the Node's primary fields rewritten to the inbound's
// values). This lets a multi-protocol mixed node appear as multiple
// client entries in subscription output. Single-inbound nodes pass
// through with the name unchanged.
func expand(views []NodeView) []NodeView {
	out := make([]NodeView, 0, len(views))
	for _, v := range views {
		ins := v.Node.AllInbounds()
		if len(ins) <= 1 {
			out = append(out, v)
			continue
		}
		for _, in := range ins {
			n := v.Node
			n.Protocol = in.Protocol
			n.Port = in.Port
			n.Transport = in.Transport
			n.TLSConfig = in.TLSConfig
			n.TransportConfig = in.TransportConfig
			n.Name = v.Node.Name + " [" + in.Protocol + "]"
			out = append(out, NodeView{Node: n, UserUUID: v.UserUUID})
		}
		// Drop the primary inbound's empty extra copy: AllInbounds
		// already includes it, so we've covered everything above.
	}
	return out
}

// EngineFromName is unused but reserved for future per-engine subscription
// hints; declared here so the package compiles even if expand has no
// other consumers in tests.
var _ = domain.EngineXray
