// Package service implements the traffic use cases: ingest node-agent
// usage reports, decrement per-user quotas, flag/un-flag the ban list, and
// expose user-facing aggregates.
package service

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/ports"
)

const DefaultBanTTL = 24 * time.Hour

type Deps struct {
	Sink     ports.UsageSink
	Quota    ports.QuotaRepo
	Bans     ports.BanCache
	Subs     ports.SubscriberResolver
	BanTTL   time.Duration
	Log      func(string, ...any)
}

type Service struct{ d Deps }

func New(d Deps) *Service {
	if d.BanTTL <= 0 {
		d.BanTTL = DefaultBanTTL
	}
	if d.Log == nil {
		d.Log = func(string, ...any) {}
	}
	return &Service{d: d}
}

// ReportItem is one row of a node-agent batch upload.
type ReportItem struct {
	SubToken  string
	UserID    uint64 // optional; resolver wins if both provided
	Protocol  string
	UpBytes   uint64
	DownBytes uint64
}

// ReportUsage ingests a batch of usage rows from one node. For each row it
// (a) resolves the user, (b) writes a UsageEvent to the sink, (c)
// increments users.traffic_used, (d) upserts the daily rollup, (e) flips
// the user's ban flag if they just crossed their quota.
func (s *Service) ReportUsage(ctx context.Context, nodeID uint64, items []ReportItem) (accepted int, rejected int, err error) {
	now := time.Now()
	events := make([]domain.UsageEvent, 0, len(items))
	day := now.UTC().Truncate(24 * time.Hour)
	for _, it := range items {
		uid := it.UserID
		if uid == 0 && it.SubToken != "" && s.d.Subs != nil {
			u, rerr := s.d.Subs.UserIDByToken(ctx, it.SubToken)
			if rerr != nil || u == 0 {
				rejected++
				continue
			}
			uid = u
		}
		if uid == 0 {
			rejected++
			continue
		}
		delta := it.UpBytes + it.DownBytes
		events = append(events, domain.UsageEvent{
			Ts: now, UserID: uid, NodeID: nodeID, Protocol: it.Protocol,
			UpBytes: it.UpBytes, DownBytes: it.DownBytes,
		})
		// Update aggregates regardless of sink outcome — sink failures fall
		// back to MySQL but we must not double-count traffic_used.
		newUsed, ierr := s.d.Quota.IncrTrafficUsed(ctx, uid, delta)
		if ierr != nil {
			s.d.Log("traffic.incr_used.error", "user", uid, "err", ierr)
			rejected++
			continue
		}
		if uerr := s.d.Quota.UpsertDaily(ctx, uid, day, it.UpBytes, it.DownBytes); uerr != nil {
			s.d.Log("traffic.daily.error", "user", uid, "err", uerr)
		}
		// Ban check: read quota total and compare. Cheap because the row
		// is hot from the increment above.
		if q, qerr := s.d.Quota.GetQuota(ctx, uid); qerr == nil && q != nil && q.TrafficTotal > 0 && newUsed >= q.TrafficTotal && !q.Banned {
			_ = s.d.Quota.SetBanned(ctx, uid, true)
			if s.d.Bans != nil {
				_ = s.d.Bans.Add(ctx, []uint64{uid}, s.d.BanTTL)
			}
			s.d.Log("traffic.banned.over_quota", "user", uid, "used", newUsed, "total", q.TrafficTotal)
		}
		accepted++
	}
	if len(events) > 0 && s.d.Sink != nil {
		if serr := s.d.Sink.Write(ctx, events); serr != nil {
			s.d.Log("traffic.sink.error", "err", serr)
		}
	}
	return accepted, rejected, nil
}

// GetMyUsage returns the current quota snapshot for one user.
func (s *Service) GetMyUsage(ctx context.Context, userID uint64) (*domain.Quota, error) {
	return s.d.Quota.GetQuota(ctx, userID)
}

// GetMyDaily fetches per-day rollup rows [from, to] from MySQL. (CH-backed
// detail queries are admin-only and live elsewhere.)
func (s *Service) GetMyDaily(ctx context.Context, userID uint64, from, to time.Time) ([]ports.DailyRow, error) {
	if to.Before(from) {
		from, to = to, from
	}
	if to.Sub(from) > 92*24*time.Hour {
		from = to.Add(-92 * 24 * time.Hour)
	}
	return s.d.Quota.SumDaily(ctx, userID, from, to)
}

// RecomputeBans rebuilds the ban cache & users.banned flag from the
// authoritative quota table. Intended to be run by an asynq task on a
// short cadence (1 min) so admin tweaks (e.g. quota top-up) propagate.
func (s *Service) RecomputeBans(ctx context.Context, batch int) (added, removed int, err error) {
	if batch <= 0 {
		batch = 200
	}
	bans, berr := s.d.Quota.ListBanCandidates(ctx, batch)
	if berr != nil {
		return 0, 0, berr
	}
	for _, u := range bans {
		_ = s.d.Quota.SetBanned(ctx, u, true)
	}
	if len(bans) > 0 && s.d.Bans != nil {
		_ = s.d.Bans.Add(ctx, bans, s.d.BanTTL)
	}
	added = len(bans)

	unbans, uerr := s.d.Quota.ListUnbanCandidates(ctx, batch)
	if uerr != nil {
		return added, 0, uerr
	}
	for _, u := range unbans {
		_ = s.d.Quota.SetBanned(ctx, u, false)
	}
	if len(unbans) > 0 && s.d.Bans != nil {
		_ = s.d.Bans.Remove(ctx, unbans)
	}
	removed = len(unbans)
	return
}

// CurrentBans returns the active ban list for node-agent consumption.
// Falls back to the authoritative QuotaRepo if the cache is empty (e.g.
// after a Redis flush).
func (s *Service) CurrentBans(ctx context.Context) ([]uint64, error) {
	if s.d.Bans == nil {
		return nil, nil
	}
	return s.d.Bans.List(ctx)
}
