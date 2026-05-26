// Package service implements SLA use cases: record probes, roll up daily
// summaries, and serve admin reports.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/sla/domain"
	"github.com/0x1F6A/proxy_VPN/internal/sla/ports"
)

type Service struct {
	probes ports.ProbeRepo
	daily  ports.DailyRepo
}

func New(probes ports.ProbeRepo, daily ports.DailyRepo) *Service {
	return &Service{probes: probes, daily: daily}
}

// Record persists a single probe outcome. Called by the prober worker.
func (s *Service) Record(ctx context.Context, p domain.Probe) error {
	if p.Target == "" {
		return errors.New("probe missing target")
	}
	if p.TS.IsZero() {
		p.TS = time.Now()
	}
	return s.probes.Append(ctx, p)
}

// RollupDay scans probes for a UTC day, aggregates by (region,target), and
// upserts into daily. Idempotent: re-runs overwrite the existing rollup.
func (s *Service) RollupDay(ctx context.Context, day time.Time) (int, error) {
	day = day.UTC().Truncate(24 * time.Hour)
	probes, err := s.probes.ListBetween(ctx, day, day.Add(24*time.Hour))
	if err != nil {
		return 0, fmt.Errorf("list probes: %w", err)
	}
	type key struct{ region, target string }
	groups := map[key]*group{}
	for _, p := range probes {
		k := key{p.Region, p.Target}
		g, ok := groups[k]
		if !ok {
			g = &group{}
			groups[k] = g
		}
		if p.Success {
			g.success++
			g.latencies = append(g.latencies, p.LatencyMs)
		} else {
			g.fail++
		}
	}
	written := 0
	for k, g := range groups {
		r := domain.DailyRollup{
			Day:        day,
			Region:     k.region,
			Target:     k.target,
			SuccessCnt: g.success,
			FailCnt:    g.fail,
		}
		r.P50Ms, r.P95Ms, r.P99Ms = percentiles(g.latencies)
		if err := s.daily.Upsert(ctx, r); err != nil {
			return written, fmt.Errorf("upsert %s/%s: %w", k.region, k.target, err)
		}
		written++
	}
	return written, nil
}

// Summary is the admin report payload.
type Summary struct {
	From    time.Time           `json:"from"`
	To      time.Time           `json:"to"`
	Buckets []SummaryBucket     `json:"buckets"`
	Overall map[string]Overall  `json:"overall"`
}

type SummaryBucket struct {
	Day        time.Time `json:"day"`
	Region     string    `json:"region"`
	Target     string    `json:"target"`
	SuccessCnt uint64    `json:"success_cnt"`
	FailCnt    uint64    `json:"fail_cnt"`
	UptimePct  float64   `json:"uptime_pct"`
	P50Ms      uint32    `json:"p50_ms"`
	P95Ms      uint32    `json:"p95_ms"`
	P99Ms      uint32    `json:"p99_ms"`
}

type Overall struct {
	UptimePct float64 `json:"uptime_pct"`
	P95Ms     uint32  `json:"p95_ms"`
	Samples   uint64  `json:"samples"`
}

// Summary returns per-day buckets and a per-target rollup across the range.
func (s *Service) Summary(ctx context.Context, from, to time.Time, target string) (*Summary, error) {
	if !to.After(from) {
		return nil, errors.New("to must be after from")
	}
	rows, err := s.daily.ListBetween(ctx, from.UTC(), to.UTC(), target)
	if err != nil {
		return nil, err
	}
	buckets := make([]SummaryBucket, 0, len(rows))
	type agg struct {
		success, fail uint64
		p95s          []uint32
	}
	overall := map[string]*agg{}
	for _, r := range rows {
		b := SummaryBucket{
			Day: r.Day, Region: r.Region, Target: r.Target,
			SuccessCnt: r.SuccessCnt, FailCnt: r.FailCnt,
			UptimePct: r.UptimeFraction() * 100,
			P50Ms:     r.P50Ms, P95Ms: r.P95Ms, P99Ms: r.P99Ms,
		}
		buckets = append(buckets, b)
		a, ok := overall[r.Target]
		if !ok {
			a = &agg{}
			overall[r.Target] = a
		}
		a.success += r.SuccessCnt
		a.fail += r.FailCnt
		if r.P95Ms > 0 {
			a.p95s = append(a.p95s, r.P95Ms)
		}
	}
	ov := map[string]Overall{}
	for t, a := range overall {
		total := a.success + a.fail
		up := 100.0
		if total > 0 {
			up = float64(a.success) / float64(total) * 100
		}
		var p95 uint32
		if len(a.p95s) > 0 {
			sort.Slice(a.p95s, func(i, j int) bool { return a.p95s[i] < a.p95s[j] })
			p95 = a.p95s[len(a.p95s)*95/100]
		}
		ov[t] = Overall{UptimePct: up, P95Ms: p95, Samples: total}
	}
	return &Summary{From: from, To: to, Buckets: buckets, Overall: ov}, nil
}

type group struct {
	success, fail uint64
	latencies     []uint32
}

func percentiles(xs []uint32) (p50, p95, p99 uint32) {
	if len(xs) == 0 {
		return 0, 0, 0
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	pick := func(p float64) uint32 {
		i := int(float64(len(xs)-1) * p)
		return xs[i]
	}
	return pick(0.50), pick(0.95), pick(0.99)
}
