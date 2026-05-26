// Package ports defines repository contracts for admin reporting.
//
// All values are aggregations of MySQL tables (orders / traffic_daily /
// users). ClickHouse-backed per-event queries are intentionally out of scope
// for this layer; they belong to a future analytics service.
package ports

import (
	"context"
	"time"
)

// RevenuePoint is one bucket of the revenue timeseries.
type RevenuePoint struct {
	Day       time.Time
	OrderCnt  uint64
	PaidCents uint64 // sum(paid_cny) * 100
}

// TrafficPoint is one bucket of the traffic timeseries (aggregated across
// all users).
type TrafficPoint struct {
	Day       time.Time
	UpBytes   uint64
	DownBytes uint64
	Users     uint64
}

// OrderStatusCount returns how many orders sit in each status within a
// window. Useful for funnel charts.
type OrderStatusCount struct {
	Status string
	Count  uint64
}

// DashboardSnapshot is the at-a-glance KPI bundle for /admin/dashboard.
type DashboardSnapshot struct {
	UsersTotal       uint64
	UsersActive      uint64 // subscription valid + not banned
	UsersBanned      uint64
	OrdersToday      uint64
	RevenueTodayCNY  string // human, two decimals
	TrafficTodayUp   uint64
	TrafficTodayDown uint64
}

// ReportRepo reads aggregations for admin dashboards.
type ReportRepo interface {
	RevenueDaily(ctx context.Context, from, to time.Time) ([]RevenuePoint, error)
	TrafficDaily(ctx context.Context, from, to time.Time) ([]TrafficPoint, error)
	OrderStatusCounts(ctx context.Context, from, to time.Time) ([]OrderStatusCount, error)
	Dashboard(ctx context.Context, now time.Time) (DashboardSnapshot, error)
}
