// Admin-facing operations for the user context: list, ban/unban, traffic
// adjustment, per-user rate limits, and a small dashboard summary.
//
// All methods require the caller to have already passed an admin/ops RBAC
// gate in the HTTP layer — there is no role check inside this file.
package service

import (
	"context"
	"errors"

	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
)

var ErrAdminNotConfigured = errors.New("admin user repo not configured")

func (s *Service) AdminListUsers(ctx context.Context, q string, limit, offset int) ([]ports.AdminUserView, int64, error) {
	if s.d.Admin == nil {
		return nil, 0, ErrAdminNotConfigured
	}
	return s.d.Admin.List(ctx, q, limit, offset)
}

func (s *Service) AdminSetBanned(ctx context.Context, id uint64, banned bool) error {
	if s.d.Admin == nil {
		return ErrAdminNotConfigured
	}
	return s.d.Admin.SetBanned(ctx, id, banned)
}

func (s *Service) AdminAdjustTraffic(ctx context.Context, id uint64, deltaBytes int64) error {
	if s.d.Admin == nil {
		return ErrAdminNotConfigured
	}
	return s.d.Admin.AdjustTraffic(ctx, id, deltaBytes)
}

func (s *Service) AdminSetRateLimits(ctx context.Context, id uint64, upBps, downBps uint64) error {
	if s.d.Admin == nil {
		return ErrAdminNotConfigured
	}
	return s.d.Admin.SetRateLimits(ctx, id, upBps, downBps)
}

func (s *Service) AdminOverallCounts(ctx context.Context) (ports.AdminCounts, error) {
	if s.d.Admin == nil {
		return ports.AdminCounts{}, ErrAdminNotConfigured
	}
	return s.d.Admin.OverallCounts(ctx)
}
