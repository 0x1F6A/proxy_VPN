// Package ports declares the outbound interfaces the SLA service depends on.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/sla/domain"
)

// ProbeRepo persists individual probe outcomes.
type ProbeRepo interface {
	Append(ctx context.Context, p domain.Probe) error
	// ListBetween returns probes ordered by ts ascending. Used by the
	// rollup job; callers should cap the range to one day.
	ListBetween(ctx context.Context, from, to time.Time) ([]domain.Probe, error)
}

// DailyRepo persists / queries pre-aggregated daily rollups.
type DailyRepo interface {
	Upsert(ctx context.Context, r domain.DailyRollup) error
	ListBetween(ctx context.Context, from, to time.Time, target string) ([]domain.DailyRollup, error)
}
