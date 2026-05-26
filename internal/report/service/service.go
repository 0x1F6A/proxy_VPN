// Package service wraps report repositories with light-weight argument
// validation. Behaviour is intentionally thin — reports are read-only and
// the heavy lifting is in the SQL itself.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/report/ports"
)

var (
	ErrRangeInvalid = errors.New("report: from must precede to")
	ErrRangeTooLong = errors.New("report: range exceeds 366 days")
)

type Service struct{ repo ports.ReportRepo }

func New(repo ports.ReportRepo) *Service { return &Service{repo: repo} }

func (s *Service) validateRange(from, to time.Time) error {
	if !from.Before(to) {
		return ErrRangeInvalid
	}
	if to.Sub(from) > 366*24*time.Hour {
		return ErrRangeTooLong
	}
	return nil
}

func (s *Service) RevenueDaily(ctx context.Context, from, to time.Time) ([]ports.RevenuePoint, error) {
	if err := s.validateRange(from, to); err != nil {
		return nil, err
	}
	return s.repo.RevenueDaily(ctx, from, to)
}

func (s *Service) TrafficDaily(ctx context.Context, from, to time.Time) ([]ports.TrafficPoint, error) {
	if err := s.validateRange(from, to); err != nil {
		return nil, err
	}
	return s.repo.TrafficDaily(ctx, from, to)
}

func (s *Service) OrderStatusCounts(ctx context.Context, from, to time.Time) ([]ports.OrderStatusCount, error) {
	if err := s.validateRange(from, to); err != nil {
		return nil, err
	}
	return s.repo.OrderStatusCounts(ctx, from, to)
}

func (s *Service) Dashboard(ctx context.Context, now time.Time) (ports.DashboardSnapshot, error) {
	return s.repo.Dashboard(ctx, now)
}
