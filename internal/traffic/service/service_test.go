package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/ports"
)

type fakeSubs map[string]uint64

func (f fakeSubs) UserIDByToken(_ context.Context, tok string) (uint64, error) {
	return f[tok], nil
}

type fakeQuota struct {
	used    map[uint64]uint64
	total   map[uint64]uint64
	banned  map[uint64]bool
	daily   map[uint64]map[string][2]uint64
	banCand []uint64
	unbCand []uint64
}

func newFakeQuota() *fakeQuota {
	return &fakeQuota{used: map[uint64]uint64{}, total: map[uint64]uint64{}, banned: map[uint64]bool{}, daily: map[uint64]map[string][2]uint64{}}
}

func (f *fakeQuota) IncrTrafficUsed(_ context.Context, uid, delta uint64) (uint64, error) {
	f.used[uid] += delta
	return f.used[uid], nil
}
func (f *fakeQuota) UpsertDaily(_ context.Context, uid uint64, day time.Time, up, down uint64) error {
	k := day.Format("2006-01-02")
	if f.daily[uid] == nil {
		f.daily[uid] = map[string][2]uint64{}
	}
	cur := f.daily[uid][k]
	cur[0] += up
	cur[1] += down
	f.daily[uid][k] = cur
	return nil
}
func (f *fakeQuota) GetQuota(_ context.Context, uid uint64) (*domain.Quota, error) {
	return &domain.Quota{UserID: uid, TrafficUsed: f.used[uid], TrafficTotal: f.total[uid], Banned: f.banned[uid]}, nil
}
func (f *fakeQuota) SetBanned(_ context.Context, uid uint64, b bool) error {
	f.banned[uid] = b
	return nil
}
func (f *fakeQuota) SumDaily(_ context.Context, uid uint64, _, _ time.Time) ([]ports.DailyRow, error) {
	return nil, nil
}
func (f *fakeQuota) ListBanCandidates(_ context.Context, _ int) ([]uint64, error) {
	return f.banCand, nil
}
func (f *fakeQuota) ListUnbanCandidates(_ context.Context, _ int) ([]uint64, error) {
	return f.unbCand, nil
}

type fakeBans struct {
	set map[uint64]bool
}

func (f *fakeBans) Add(_ context.Context, ids []uint64, _ time.Duration) error {
	for _, i := range ids {
		f.set[i] = true
	}
	return nil
}
func (f *fakeBans) Remove(_ context.Context, ids []uint64) error {
	for _, i := range ids {
		delete(f.set, i)
	}
	return nil
}
func (f *fakeBans) Contains(_ context.Context, id uint64) (bool, error) { return f.set[id], nil }
func (f *fakeBans) List(_ context.Context) ([]uint64, error) {
	out := make([]uint64, 0, len(f.set))
	for k := range f.set {
		out = append(out, k)
	}
	return out, nil
}

type fakeSink struct{ wrote int }

func (s *fakeSink) Write(_ context.Context, ev []domain.UsageEvent) error {
	s.wrote += len(ev)
	return nil
}
func (s *fakeSink) Close() error { return nil }

func TestReportUsage_HappyPath_AndBanOnOverQuota(t *testing.T) {
	q := newFakeQuota()
	q.total[1] = 1000 // 1KB cap
	bans := &fakeBans{set: map[uint64]bool{}}
	sink := &fakeSink{}
	s := New(Deps{
		Sink: sink, Quota: q, Bans: bans,
		Subs:   fakeSubs{"tok-1": 1, "tok-2": 2},
		BanTTL: time.Hour,
	})
	acc, rej, err := s.ReportUsage(context.Background(), 7, []ReportItem{
		{SubToken: "tok-1", Protocol: "vmess", UpBytes: 400, DownBytes: 700}, // -> 1100, over
		{SubToken: "tok-2", Protocol: "vmess", UpBytes: 100, DownBytes: 100},
		{SubToken: "unknown"}, // rejected
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if acc != 2 || rej != 1 {
		t.Fatalf("accepted=%d rejected=%d", acc, rej)
	}
	if !bans.set[1] {
		t.Fatalf("user 1 should be banned")
	}
	if bans.set[2] {
		t.Fatalf("user 2 should not be banned")
	}
	if sink.wrote != 2 {
		t.Fatalf("sink wrote=%d, want 2", sink.wrote)
	}
}

func TestRecomputeBans(t *testing.T) {
	q := newFakeQuota()
	q.banCand = []uint64{10, 11}
	q.unbCand = []uint64{20}
	bans := &fakeBans{set: map[uint64]bool{20: true}}
	s := New(Deps{Quota: q, Bans: bans})
	added, removed, err := s.RecomputeBans(context.Background(), 100)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if added != 2 || removed != 1 {
		t.Fatalf("added=%d removed=%d", added, removed)
	}
	if !bans.set[10] || !bans.set[11] || bans.set[20] {
		t.Fatalf("ban set mismatch: %v", bans.set)
	}
}

var _ ports.QuotaRepo = (*fakeQuota)(nil)
var _ ports.BanCache = (*fakeBans)(nil)
var _ ports.UsageSink = (*fakeSink)(nil)

// guard against unused error import on some Go versions
var _ = errors.New
