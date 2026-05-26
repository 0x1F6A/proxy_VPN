// Package gormrepo backs internal/report/ports with raw-SQL queries against
// MySQL. The queries are intentionally written to take advantage of the
// (user_id, day) and (status, paid_at) indexes added by migrations 000001 /
// 000003 — keep them aligned if those indexes change.
package gormrepo

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/report/ports"
)

type Repo struct{ db *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{db: db} }

func (r *Repo) RevenueDaily(ctx context.Context, from, to time.Time) ([]ports.RevenuePoint, error) {
	type row struct {
		Day       time.Time
		OrderCnt  uint64
		PaidCents uint64
	}
	var rows []row
	q := `
        SELECT DATE(paid_at) AS day,
               COUNT(*)      AS order_cnt,
               CAST(SUM(paid_cny) * 100 AS UNSIGNED) AS paid_cents
        FROM orders
        WHERE status='paid' AND paid_at IS NOT NULL
          AND paid_at >= ? AND paid_at < ?
        GROUP BY DATE(paid_at)
        ORDER BY day ASC`
	if err := r.db.WithContext(ctx).Raw(q, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ports.RevenuePoint, len(rows))
	for i, v := range rows {
		out[i] = ports.RevenuePoint{Day: v.Day, OrderCnt: v.OrderCnt, PaidCents: v.PaidCents}
	}
	return out, nil
}

func (r *Repo) TrafficDaily(ctx context.Context, from, to time.Time) ([]ports.TrafficPoint, error) {
	type row struct {
		Day       time.Time
		UpBytes   uint64
		DownBytes uint64
		Users     uint64
	}
	var rows []row
	q := `
        SELECT day,
               SUM(up_bytes)   AS up_bytes,
               SUM(down_bytes) AS down_bytes,
               COUNT(DISTINCT user_id) AS users
        FROM traffic_daily
        WHERE day >= ? AND day < ?
        GROUP BY day
        ORDER BY day ASC`
	if err := r.db.WithContext(ctx).Raw(q, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ports.TrafficPoint, len(rows))
	for i, v := range rows {
		out[i] = ports.TrafficPoint{Day: v.Day, UpBytes: v.UpBytes, DownBytes: v.DownBytes, Users: v.Users}
	}
	return out, nil
}

func (r *Repo) OrderStatusCounts(ctx context.Context, from, to time.Time) ([]ports.OrderStatusCount, error) {
	var rows []ports.OrderStatusCount
	q := `
        SELECT status, COUNT(*) AS count
        FROM orders
        WHERE created_at >= ? AND created_at < ?
        GROUP BY status
        ORDER BY count DESC`
	if err := r.db.WithContext(ctx).Raw(q, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repo) Dashboard(ctx context.Context, now time.Time) (ports.DashboardSnapshot, error) {
	var snap ports.DashboardSnapshot
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	// 1) user counts (deleted_at is GORM soft-delete; respect it)
	var uc struct {
		Total   uint64
		Active  uint64
		Banned  uint64
	}
	uq := `
        SELECT
            COUNT(*) AS total,
            COALESCE(SUM(CASE WHEN COALESCE(banned,0)=0
                              AND subscription_end_at IS NOT NULL
                              AND subscription_end_at > ?
                              THEN 1 ELSE 0 END), 0) AS active,
            COALESCE(SUM(CASE WHEN COALESCE(banned,0)=1 THEN 1 ELSE 0 END), 0) AS banned
        FROM users
        WHERE deleted_at IS NULL`
	if err := r.db.WithContext(ctx).Raw(uq, now).Scan(&uc).Error; err != nil {
		return snap, err
	}
	snap.UsersTotal = uc.Total
	snap.UsersActive = uc.Active
	snap.UsersBanned = uc.Banned

	// 2) today's orders + revenue (in CNY string with 2 decimals)
	var oc struct {
		Orders uint64
		Paid   string
	}
	oq := `
        SELECT
            COALESCE(COUNT(*), 0) AS orders,
            COALESCE(FORMAT(SUM(paid_cny), 2), '0.00') AS paid
        FROM orders
        WHERE status='paid' AND paid_at >= ? AND paid_at < ?`
	if err := r.db.WithContext(ctx).Raw(oq, dayStart, dayEnd).Scan(&oc).Error; err != nil {
		return snap, err
	}
	snap.OrdersToday = oc.Orders
	if oc.Paid == "" {
		snap.RevenueTodayCNY = "0.00"
	} else {
		snap.RevenueTodayCNY = oc.Paid
	}

	// 3) today's traffic totals (the traffic_daily row keyed by today's
	//    DATE is updated continuously by the rollup task; if absent the
	//    sum is simply 0).
	var tc struct {
		Up   uint64
		Down uint64
	}
	tq := `
        SELECT COALESCE(SUM(up_bytes),0) AS up,
               COALESCE(SUM(down_bytes),0) AS down
        FROM traffic_daily
        WHERE day = DATE(?)`
	if err := r.db.WithContext(ctx).Raw(tq, dayStart).Scan(&tc).Error; err != nil {
		return snap, err
	}
	snap.TrafficTodayUp = tc.Up
	snap.TrafficTodayDown = tc.Down
	return snap, nil
}

// ensure interface satisfaction at compile time
var _ ports.ReportRepo = (*Repo)(nil)

// guard against unused import in some build tags
var _ = fmt.Sprintf
