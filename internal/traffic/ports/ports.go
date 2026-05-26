// Package ports defines the outbound interfaces of the traffic context:
// the event sink (ClickHouse-or-fallback), the per-user quota repository
// (MySQL users table + traffic_daily), the ban cache (Redis SET), and a
// subscriber lookup so the service can resolve sub_token → user_id.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
)

// UsageSink ingests a batch of events. Implementations must be safe to
// call concurrently and should batch internally where appropriate.
type UsageSink interface {
	Write(ctx context.Context, events []domain.UsageEvent) error
	Close() error
}

// QuotaRepo encapsulates per-user quota reads + atomic increment of
// traffic_used + per-day rollup writes.
type QuotaRepo interface {
	GetQuota(ctx context.Context, userID uint64) (*domain.Quota, error)
	// IncrTrafficUsed atomically increments users.traffic_used and returns
	// the new value (post-increment). Used to detect over-quota crossings.
	IncrTrafficUsed(ctx context.Context, userID uint64, deltaBytes uint64) (uint64, error)
	UpsertDaily(ctx context.Context, userID uint64, day time.Time, up, down uint64) error
	SetBanned(ctx context.Context, userID uint64, banned bool) error
	// ListBanCandidates returns users whose traffic_used >= traffic_total
	// (or plan_expire_at past) that are not yet flagged banned.
	ListBanCandidates(ctx context.Context, limit int) ([]uint64, error)
	// ListUnbanCandidates returns banned users whose quota / expiry has
	// been replenished (used < total AND not expired).
	ListUnbanCandidates(ctx context.Context, limit int) ([]uint64, error)
	// SumDaily returns per-day totals for [from,to] inclusive (UTC dates).
	SumDaily(ctx context.Context, userID uint64, from, to time.Time) ([]DailyRow, error)
}

type DailyRow struct {
	Day       time.Time
	UpBytes   uint64
	DownBytes uint64
}

// BanCache is the hot path that node-agent consults for the active ban
// list. A small Redis SET keyed by all banned user ids.
type BanCache interface {
	Add(ctx context.Context, userIDs []uint64, ttl time.Duration) error
	Remove(ctx context.Context, userIDs []uint64) error
	Contains(ctx context.Context, userID uint64) (bool, error)
	List(ctx context.Context) ([]uint64, error)
}

// SubscriberResolver maps a node-agent-supplied sub token to its owning
// user id (and, optionally, the user's current rate caps).
type SubscriberResolver interface {
	UserIDByToken(ctx context.Context, token string) (uint64, error)
}
