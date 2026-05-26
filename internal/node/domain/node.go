// Package domain holds node-context entities: Node, NodeGroup, and the
// protocol enumeration. Pure data + errors — no I/O.
package domain

import (
	"encoding/json"
	"time"
)

// Protocol values map to xray / sing-box outbound types.
const (
	ProtocolVLESSReality = "vless-reality"
	ProtocolTrojan       = "trojan"
	ProtocolHysteria2    = "hysteria2"
	ProtocolSS2022       = "ss-2022"

	NodeStatusDisabled    = 0
	NodeStatusEnabled     = 1
	NodeStatusMaintenance = 2

	// EngineXray runs the node config through Xray-core (default).
	EngineXray = "xray"
	// EngineSingBox runs the node config through sing-box.
	EngineSingBox = "sing-box"
)

// IsValidEngine reports whether s is a supported node engine value.
func IsValidEngine(s string) bool {
	switch s {
	case EngineXray, EngineSingBox:
		return true
	}
	return false
}

// NodeInbound describes one listener served by a node. The Node's primary
// fields (Protocol / Port / TLSConfig / Transport / TransportConfig) form
// the first/default inbound; Node.Inbounds may carry additional inbounds
// to enable multi-protocol mixed nodes.
type NodeInbound struct {
	Tag             string          `json:"tag,omitempty"`
	Protocol        string          `json:"protocol"`
	Port            uint32          `json:"port"`
	Transport       string          `json:"transport,omitempty"`
	TLSConfig       json.RawMessage `json:"tls_config,omitempty"`
	TransportConfig json.RawMessage `json:"transport_config,omitempty"`
}

type NodeGroup struct {
	ID        uint64
	Name      string
	Level     int
	Remark    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Node represents one outbound endpoint advertised to clients.
type Node struct {
	ID              uint64
	Name            string
	Region          string // US|JP|HK|SG|...
	Tags            string
	NodeGroupID     uint64
	Protocol        string
	Address         string
	Port            uint32
	TLSConfig       json.RawMessage // raw JSON; protocol-specific
	Transport       string          // tcp|ws|grpc|xhttp
	TransportConfig json.RawMessage
	RateMultiplier  string // DECIMAL(4,2) carried as string
	NodeTokenHash   string // sha256 of node bootstrap token
	Online          bool
	LastHeartbeatAt *time.Time
	CPUPercent      *string
	MemPercent      *string
	BandwidthInBps  *uint64
	BandwidthOutBps *uint64
	OnlineUsers     uint32
	Sort            int
	Status          int8
	Engine          string          // "xray" | "sing-box"; empty == "xray"
	Inbounds        json.RawMessage // optional []NodeInbound for multi-protocol nodes
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (n Node) IsServiceable() bool {
	return n.Status == NodeStatusEnabled && n.Online
}

// EffectiveEngine returns the engine to render with, defaulting to xray
// when the node has not been migrated to the new field yet.
func (n Node) EffectiveEngine() string {
	if n.Engine == "" {
		return EngineXray
	}
	return n.Engine
}

// AllInbounds returns the full inbound list for this node: the primary
// inbound built from the Node's own fields, followed by any extra inbounds
// declared in Node.Inbounds. Order is stable so config hashes are stable.
func (n Node) AllInbounds() []NodeInbound {
	out := []NodeInbound{{
		Tag:             "in-" + n.Protocol,
		Protocol:        n.Protocol,
		Port:            n.Port,
		Transport:       n.Transport,
		TLSConfig:       n.TLSConfig,
		TransportConfig: n.TransportConfig,
	}}
	if len(n.Inbounds) == 0 {
		return out
	}
	var extra []NodeInbound
	if err := json.Unmarshal(n.Inbounds, &extra); err != nil {
		return out
	}
	for i, in := range extra {
		if in.Tag == "" {
			extra[i].Tag = "in-" + in.Protocol + "-" + strconvUint(uint64(in.Port))
		}
	}
	return append(out, extra...)
}

func strconvUint(u uint64) string {
	// avoid importing strconv just for this; inline minimal base-10.
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}
