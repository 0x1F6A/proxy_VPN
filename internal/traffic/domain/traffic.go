// Package domain defines the traffic bounded context entities: usage
// events (per-(user,node) deltas reported by node-agent), per-user quota
// snapshot (mirrored from the users table), and the ban reason enum.
package domain

import (
	"errors"
	"time"
)

// UsageEvent is one row of (per-user, per-node) usage delta over a
// reporting interval — sent by node-agent to API, then forwarded to
// ClickHouse (or the MySQL fallback) by traffic.Service.
type UsageEvent struct {
	Ts        time.Time
	UserID    uint64
	NodeID    uint64
	Protocol  string
	UpBytes   uint64
	DownBytes uint64
}

// Quota mirrors the relevant per-user fields used by the traffic service.
// Bytes = TrafficTotal − TrafficUsed (clipped at 0).
type Quota struct {
	UserID         uint64
	TrafficTotal   uint64
	TrafficUsed    uint64
	TrafficResetAt *time.Time
	RateBpsUp      uint64
	RateBpsDown    uint64
	Banned         bool
}

func (q Quota) Remaining() uint64 {
	if q.TrafficUsed >= q.TrafficTotal {
		return 0
	}
	return q.TrafficTotal - q.TrafficUsed
}

func (q Quota) ShouldBan() bool {
	return q.TrafficTotal > 0 && q.TrafficUsed >= q.TrafficTotal
}

// BanReason explains why a user appears in the ban set.
type BanReason string

const (
	BanReasonOverQuota BanReason = "over_quota"
	BanReasonExpired   BanReason = "plan_expired"
	BanReasonAdmin     BanReason = "admin"
)

var (
	ErrUnknownSubscriber = errors.New("traffic: unknown subscriber token")
	ErrBootstrapAuth     = errors.New("traffic: bootstrap auth required")
)
