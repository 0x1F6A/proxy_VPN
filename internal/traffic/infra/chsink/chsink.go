// Package chsink writes UsageEvents to ClickHouse. If Enabled is false the
// sink is a no-op; callers should compose with a MySQL fallback sink for
// durability. The on-startup DDL bootstrap is idempotent.
package chsink

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/ports"
)

// Driver is the minimal subset of clickhouse-go's API the sink needs. An
// interface lets us unit-test without spinning up CH.
type Driver interface {
	Exec(ctx context.Context, query string, args ...any) error
	Close() error
}

type Config struct {
	Enabled      bool
	Database     string
	Table        string // default "traffic_events"
	FlushSize    int
	FlushTimeout time.Duration
	Fallback     ports.UsageSink // used when Enabled=false OR Driver write fails
}

type Sink struct {
	cfg    Config
	driver Driver
}

func New(cfg Config, drv Driver) (*Sink, error) {
	if cfg.Table == "" {
		cfg.Table = "traffic_events"
	}
	if cfg.FlushSize <= 0 {
		cfg.FlushSize = 500
	}
	if cfg.FlushTimeout <= 0 {
		cfg.FlushTimeout = 15 * time.Second
	}
	if cfg.Enabled && drv == nil {
		return nil, errors.New("chsink: Enabled=true requires a non-nil driver")
	}
	return &Sink{cfg: cfg, driver: drv}, nil
}

// Bootstrap creates the target table & rollup materialised view if needed.
// Safe to call repeatedly on app startup.
func (s *Sink) Bootstrap(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
		ts        DateTime,
		user_id   UInt64,
		node_id   UInt64,
		protocol  LowCardinality(String),
		up_bytes  UInt64,
		down_bytes UInt64
	) ENGINE = MergeTree PARTITION BY toYYYYMM(ts) ORDER BY (user_id, ts)`,
		s.cfg.Database, s.cfg.Table)
	if err := s.driver.Exec(ctx, ddl); err != nil {
		return err
	}
	// Per-user daily rollup materialised view. The downstream table is
	// SummingMergeTree so identical (user_id, day) rows merge their byte
	// counters; readers should still wrap reads in SUM() to handle
	// not-yet-merged parts.
	rollupTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.traffic_user_daily (
		day      Date,
		user_id  UInt64,
		up_bytes UInt64,
		down_bytes UInt64
	) ENGINE = SummingMergeTree PARTITION BY toYYYYMM(day) ORDER BY (user_id, day)`,
		s.cfg.Database)
	if err := s.driver.Exec(ctx, rollupTable); err != nil {
		return err
	}
	mv := fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.mv_traffic_user_daily
		TO %s.traffic_user_daily AS
		SELECT toDate(ts) AS day, user_id,
		       sum(up_bytes)   AS up_bytes,
		       sum(down_bytes) AS down_bytes
		FROM %s.%s
		GROUP BY day, user_id`,
		s.cfg.Database, s.cfg.Database, s.cfg.Database, s.cfg.Table)
	return s.driver.Exec(ctx, mv)
}

func (s *Sink) Write(ctx context.Context, events []domain.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	if !s.cfg.Enabled || s.driver == nil {
		if s.cfg.Fallback != nil {
			return s.cfg.Fallback.Write(ctx, events)
		}
		return nil
	}
	q := fmt.Sprintf(`INSERT INTO %s.%s (ts, user_id, node_id, protocol, up_bytes, down_bytes) VALUES`,
		s.cfg.Database, s.cfg.Table)
	for _, e := range events {
		if err := s.driver.Exec(ctx, q+" (?, ?, ?, ?, ?, ?)",
			e.Ts, e.UserID, e.NodeID, e.Protocol, e.UpBytes, e.DownBytes); err != nil {
			if s.cfg.Fallback != nil {
				return s.cfg.Fallback.Write(ctx, events)
			}
			return err
		}
	}
	return nil
}

func (s *Sink) Close() error {
	if s.driver != nil {
		return s.driver.Close()
	}
	return nil
}
