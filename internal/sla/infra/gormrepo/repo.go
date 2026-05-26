// Package gormrepo backs internal/sla/ports with MySQL.
package gormrepo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/sla/domain"
)

type probeRow struct {
	ID        uint64 `gorm:"primaryKey"`
	TS        time.Time
	Region    string
	Target    string
	Success   bool
	LatencyMs uint32
	Err       *string
}

func (probeRow) TableName() string { return "sla_probes" }

type ProbeRepo struct{ db *gorm.DB }

func NewProbeRepo(db *gorm.DB) *ProbeRepo { return &ProbeRepo{db: db} }

func (r *ProbeRepo) Append(ctx context.Context, p domain.Probe) error {
	row := probeRow{
		TS: p.TS, Region: p.Region, Target: p.Target,
		Success: p.Success, LatencyMs: p.LatencyMs,
	}
	if p.Err != "" {
		s := p.Err
		if len(s) > 255 {
			s = s[:255]
		}
		row.Err = &s
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ProbeRepo) ListBetween(ctx context.Context, from, to time.Time) ([]domain.Probe, error) {
	var rows []probeRow
	if err := r.db.WithContext(ctx).
		Where("ts >= ? AND ts < ?", from, to).
		Order("ts ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Probe, len(rows))
	for i, x := range rows {
		var e string
		if x.Err != nil {
			e = *x.Err
		}
		out[i] = domain.Probe{
			ID: x.ID, TS: x.TS, Region: x.Region, Target: x.Target,
			Success: x.Success, LatencyMs: x.LatencyMs, Err: e,
		}
	}
	return out, nil
}

type dailyRow struct {
	ID         uint64 `gorm:"primaryKey"`
	Day        time.Time
	Region     string
	Target     string
	SuccessCnt uint64
	FailCnt    uint64
	P50Ms      uint32
	P95Ms      uint32
	P99Ms      uint32
	CreatedAt  time.Time
}

func (dailyRow) TableName() string { return "sla_daily" }

type DailyRepo struct{ db *gorm.DB }

func NewDailyRepo(db *gorm.DB) *DailyRepo { return &DailyRepo{db: db} }

func (r *DailyRepo) Upsert(ctx context.Context, x domain.DailyRollup) error {
	const q = `
INSERT INTO sla_daily (day, region, target, success_cnt, fail_cnt, p50_ms, p95_ms, p99_ms, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
  success_cnt=VALUES(success_cnt),
  fail_cnt=VALUES(fail_cnt),
  p50_ms=VALUES(p50_ms),
  p95_ms=VALUES(p95_ms),
  p99_ms=VALUES(p99_ms)`
	return r.db.WithContext(ctx).Exec(q,
		x.Day, x.Region, x.Target,
		x.SuccessCnt, x.FailCnt, x.P50Ms, x.P95Ms, x.P99Ms).Error
}

func (r *DailyRepo) ListBetween(ctx context.Context, from, to time.Time, target string) ([]domain.DailyRollup, error) {
	var rows []dailyRow
	q := r.db.WithContext(ctx).Where("day >= ? AND day < ?", from, to)
	if target != "" {
		q = q.Where("target = ?", target)
	}
	if err := q.Order("day ASC, target ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.DailyRollup, len(rows))
	for i, x := range rows {
		out[i] = domain.DailyRollup{
			Day: x.Day, Region: x.Region, Target: x.Target,
			SuccessCnt: x.SuccessCnt, FailCnt: x.FailCnt,
			P50Ms: x.P50Ms, P95Ms: x.P95Ms, P99Ms: x.P99Ms,
		}
	}
	return out, nil
}
