package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/sla/domain"
)

type fakeProbeRepo struct {
	mu sync.Mutex
	ps []domain.Probe
}

func (f *fakeProbeRepo) Append(_ context.Context, p domain.Probe) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p.ID = uint64(len(f.ps) + 1)
	f.ps = append(f.ps, p)
	return nil
}
func (f *fakeProbeRepo) ListBetween(_ context.Context, from, to time.Time) ([]domain.Probe, error) {
	out := []domain.Probe{}
	for _, p := range f.ps {
		if !p.TS.Before(from) && p.TS.Before(to) {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeDailyRepo struct {
	mu sync.Mutex
	rs map[string]domain.DailyRollup
}

func newFakeDaily() *fakeDailyRepo { return &fakeDailyRepo{rs: map[string]domain.DailyRollup{}} }

func dailyKey(r domain.DailyRollup) string {
	return r.Day.Format("2006-01-02") + "|" + r.Region + "|" + r.Target
}

func (f *fakeDailyRepo) Upsert(_ context.Context, r domain.DailyRollup) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rs[dailyKey(r)] = r
	return nil
}
func (f *fakeDailyRepo) ListBetween(_ context.Context, from, to time.Time, target string) ([]domain.DailyRollup, error) {
	out := []domain.DailyRollup{}
	for _, r := range f.rs {
		if r.Day.Before(from) || !r.Day.Before(to) {
			continue
		}
		if target != "" && r.Target != target {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func TestRollupDay_Idempotent(t *testing.T) {
	probes := &fakeProbeRepo{}
	daily := newFakeDaily()
	svc := New(probes, daily)
	ctx := context.Background()

	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 50; i++ {
		_ = svc.Record(ctx, domain.Probe{
			TS: day.Add(time.Duration(i) * time.Minute),
			Region: "us-east-1", Target: "api",
			Success: i%10 != 0, // 10% failures
			LatencyMs: uint32(50 + i),
		})
	}
	n1, err := svc.RollupDay(ctx, day)
	if err != nil || n1 != 1 {
		t.Fatalf("rollup1: %v n=%d", err, n1)
	}
	got1 := daily.rs[dailyKey(domain.DailyRollup{Day: day, Region: "us-east-1", Target: "api"})]
	n2, err := svc.RollupDay(ctx, day)
	if err != nil || n2 != 1 {
		t.Fatalf("rollup2: %v n=%d", err, n2)
	}
	got2 := daily.rs[dailyKey(domain.DailyRollup{Day: day, Region: "us-east-1", Target: "api"})]
	if got1 != got2 {
		t.Fatalf("rollup not idempotent: %+v vs %+v", got1, got2)
	}
	if got1.SuccessCnt != 45 || got1.FailCnt != 5 {
		t.Fatalf("counts wrong: %+v", got1)
	}
	if got1.P50Ms == 0 || got1.P95Ms < got1.P50Ms {
		t.Fatalf("percentiles wrong: %+v", got1)
	}
}

func TestSummary_RangeValidation(t *testing.T) {
	svc := New(&fakeProbeRepo{}, newFakeDaily())
	_, err := svc.Summary(context.Background(), time.Now(), time.Now().Add(-time.Hour), "")
	if err == nil {
		t.Fatal("expected error on inverted range")
	}
}

func TestRecord_MissingTarget(t *testing.T) {
	svc := New(&fakeProbeRepo{}, newFakeDaily())
	if err := svc.Record(context.Background(), domain.Probe{}); err == nil {
		t.Fatal("expected error on missing target")
	}
}

func TestPercentiles_Edge(t *testing.T) {
	p50, p95, p99 := percentiles(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Fatalf("empty percentiles: %d %d %d", p50, p95, p99)
	}
	xs := []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	p50, p95, _ = percentiles(xs)
	if p50 == 0 || p95 == 0 {
		t.Fatal("missing percentiles")
	}
}

var _ = errors.New