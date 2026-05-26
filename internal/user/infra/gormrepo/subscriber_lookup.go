package gormrepo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	nodeports "github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

// SubscriberLookupRepo implements node/ports.SubscriberPort by reading the
// users table. Kept in the user infra layer to honor the dependency rule
// (node depends on the abstraction, user implements it).
type SubscriberLookupRepo struct{ db *gorm.DB }

func NewSubscriberLookupRepo(db *gorm.DB) *SubscriberLookupRepo {
	return &SubscriberLookupRepo{db: db}
}

func (r *SubscriberLookupRepo) LookupBySubToken(ctx context.Context, token string) (*nodeports.Subscriber, error) {
	var row userRow
	if err := r.db.WithContext(ctx).Where("subscription_token = ?", token).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &nodeports.Subscriber{
		UserID: row.ID, UUID: row.UUID,
		PlanID: row.PlanID, PlanExpireAt: row.PlanExpireAt,
		Status: int8(row.Status),
	}, nil
}

// ListActive returns enabled, non-expired subscribers. `banned=0` is also
// enforced so the rendered node config never includes a flagged user.
func (r *SubscriberLookupRepo) ListActive(ctx context.Context, limit int) ([]nodeports.Subscriber, error) {
	q := r.db.WithContext(ctx).Model(&userRow{}).
		Where("status = ? AND COALESCE(banned, 0) = 0", 1).
		Where("plan_expire_at IS NULL OR plan_expire_at > NOW()")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []userRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]nodeports.Subscriber, 0, len(rows))
	for _, row := range rows {
		out = append(out, nodeports.Subscriber{
			UserID: row.ID, UUID: row.UUID,
			PlanID: row.PlanID, PlanExpireAt: row.PlanExpireAt,
			Status: int8(row.Status),
		})
	}
	return out, nil
}
