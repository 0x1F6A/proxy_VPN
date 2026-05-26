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
)

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
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (n Node) IsServiceable() bool {
	return n.Status == NodeStatusEnabled && n.Online
}
