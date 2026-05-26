// Package service — admin-only operations on the billing catalog and orders.
// User-facing logic lives in service.go.
package service

import (
	"context"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/ports"
)

// ----- Coupons (admin CRUD) ---------------------------------------------

func (s *Service) AdminListCoupons(ctx context.Context, q string, limit, offset int) ([]domain.Coupon, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.d.Coupons.List(ctx, q, limit, offset)
}

func (s *Service) AdminGetCoupon(ctx context.Context, id uint64) (*domain.Coupon, error) {
	return s.d.Coupons.Get(ctx, id)
}

func (s *Service) AdminCreateCoupon(ctx context.Context, c *domain.Coupon) error {
	return s.d.Coupons.Create(ctx, c)
}

func (s *Service) AdminUpdateCoupon(ctx context.Context, c *domain.Coupon) error {
	return s.d.Coupons.Update(ctx, c)
}

func (s *Service) AdminDeleteCoupon(ctx context.Context, id uint64) error {
	return s.d.Coupons.Delete(ctx, id)
}

// ----- Orders (admin list & detail) -------------------------------------

func (s *Service) AdminListOrders(ctx context.Context, f ports.OrderFilter, limit, offset int) ([]domain.Order, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.d.Orders.AdminList(ctx, f, limit, offset)
}

func (s *Service) AdminGetOrder(ctx context.Context, no string) (*domain.Order, error) {
	return s.d.Orders.FindByOrderNo(ctx, no)
}
