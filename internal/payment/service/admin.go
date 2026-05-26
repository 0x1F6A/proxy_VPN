package service

import (
	"context"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

func (s *Service) AdminListPayments(ctx context.Context, f ports.PaymentFilter, limit, offset int) ([]domain.Payment, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.d.Payments.AdminList(ctx, f, limit, offset)
}

func (s *Service) AdminGetPayment(ctx context.Context, id uint64) (*domain.Payment, error) {
	return s.d.Payments.FindByID(ctx, id)
}
