// Package gormrepo implements the traffic ports against the same users
// table used by the user context plus the new traffic_daily /
// usage_event_fallback tables. We use raw SQL where convenient to avoid
// reshaping the userRow struct in the user package.
package gormrepo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/ports"
)

type QuotaRepo struct{ db *gorm.DB }

func NewQuotaRepo(db *gorm.DB) *QuotaRepo { return &QuotaRepo{db: db} }

func (r *QuotaRepo) GetQuota(ctx context.Context, userID uint64) (*domain.Quota, error) {
	var row struct {
		ID             uint64
		TrafficTotal   uint64
		TrafficUsed    uint64
		TrafficResetAt *time.Time
		RateBpsUp      uint64
		RateBpsDown    uint64
		Banned         uint8
	}
	err := r.db.WithContext(ctx).
		Table("users").
		Select("id, traffic_total, traffic_used, traffic_reset_at, rate_bps_up, rate_bps_down, banned").
		Where("id = ?", userID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &domain.Quota{
		UserID: row.ID, TrafficTotal: row.TrafficTotal, TrafficUsed: row.TrafficUsed,
		TrafficResetAt: row.TrafficResetAt, RateBpsUp: row.RateBpsUp,
		RateBpsDown: row.RateBpsDown, Banned: row.Banned != 0,
	}, nil
}

func (r *QuotaRepo) IncrTrafficUsed(ctx context.Context, userID uint64, delta uint64) (uint64, error) {
	if err := r.db.WithContext(ctx).Exec(
		`UPDATE users SET traffic_used = traffic_used + ? WHERE id = ?`, delta, userID,
	).Error; err != nil {
		return 0, err
	}
	var newUsed uint64
	if err := r.db.WithContext(ctx).
		Raw(`SELECT traffic_used FROM users WHERE id = ?`, userID).Scan(&newUsed).Error; err != nil {
		return 0, err
	}
	return newUsed, nil
}

type trafficDailyRow struct {
	UserID    uint64    `gorm:"column:user_id;primaryKey"`
	Day       time.Time `gorm:"column:day;primaryKey;type:date"`
	UpBytes   uint64    `gorm:"column:up_bytes"`
	DownBytes uint64    `gorm:"column:down_bytes"`
	UpdatedAt time.Time
}

func (trafficDailyRow) TableName() string { return "traffic_daily" }

func (r *QuotaRepo) UpsertDaily(ctx context.Context, userID uint64, day time.Time, up, down uint64) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO traffic_daily (user_id, day, up_bytes, down_bytes)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE up_bytes = up_bytes + VALUES(up_bytes),
		                         down_bytes = down_bytes + VALUES(down_bytes)`,
		userID, day, up, down,
	).Error
}

func (r *QuotaRepo) SetBanned(ctx context.Context, userID uint64, banned bool) error {
	v := 0
	if banned {
		v = 1
	}
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET banned = ? WHERE id = ?`, v, userID,
	).Error
}

func (r *QuotaRepo) ListBanCandidates(ctx context.Context, limit int) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM users
		 WHERE banned = 0
		   AND ((traffic_total > 0 AND traffic_used >= traffic_total)
		        OR (plan_expire_at IS NOT NULL AND plan_expire_at < NOW()))
		 LIMIT ?`, limit,
	).Scan(&ids).Error
	return ids, err
}

func (r *QuotaRepo) ListUnbanCandidates(ctx context.Context, limit int) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM users
		 WHERE banned = 1
		   AND (traffic_total = 0 OR traffic_used < traffic_total)
		   AND (plan_expire_at IS NULL OR plan_expire_at > NOW())
		 LIMIT ?`, limit,
	).Scan(&ids).Error
	return ids, err
}

func (r *QuotaRepo) SumDaily(ctx context.Context, userID uint64, from, to time.Time) ([]ports.DailyRow, error) {
	var rows []trafficDailyRow
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND day BETWEEN ? AND ?", userID, from, to).
		Order("day ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ports.DailyRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.DailyRow{Day: r.Day, UpBytes: r.UpBytes, DownBytes: r.DownBytes})
	}
	return out, nil
}

// UsageFallbackSink is a ports.UsageSink implementation that writes events
// to MySQL `usage_event_fallback`. Used when ClickHouse is disabled or
// returns an error.
type UsageFallbackSink struct{ db *gorm.DB }

func NewUsageFallbackSink(db *gorm.DB) *UsageFallbackSink { return &UsageFallbackSink{db: db} }

type usageEventFallbackRow struct {
	ID        uint64 `gorm:"primaryKey"`
	Ts        time.Time
	UserID    uint64
	NodeID    uint64
	Protocol  string
	UpBytes   uint64
	DownBytes uint64
	Flushed   uint8
}

func (usageEventFallbackRow) TableName() string { return "usage_event_fallback" }

func (s *UsageFallbackSink) Write(ctx context.Context, events []domain.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]usageEventFallbackRow, 0, len(events))
	for _, e := range events {
		rows = append(rows, usageEventFallbackRow{
			Ts: e.Ts, UserID: e.UserID, NodeID: e.NodeID, Protocol: e.Protocol,
			UpBytes: e.UpBytes, DownBytes: e.DownBytes,
		})
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, 200).Error
}
func (s *UsageFallbackSink) Close() error { return nil }
