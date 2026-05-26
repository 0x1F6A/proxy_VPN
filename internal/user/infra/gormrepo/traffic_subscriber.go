package gormrepo

import (
	"context"

	"gorm.io/gorm"
)

// TrafficSubscriberResolver implements traffic/ports.SubscriberResolver
// against the users table. Returns 0 (no error) for unknown tokens so
// callers can treat the row as "rejected" rather than failing the batch.
type TrafficSubscriberResolver struct{ db *gorm.DB }

func NewTrafficSubscriberResolver(db *gorm.DB) *TrafficSubscriberResolver {
	return &TrafficSubscriberResolver{db: db}
}

func (r *TrafficSubscriberResolver) UserIDByToken(ctx context.Context, token string) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM users WHERE subscription_token = ? LIMIT 1`, token,
	).Scan(&id).Error
	return id, err
}
