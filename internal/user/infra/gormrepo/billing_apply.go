package gormrepo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// BillingApplyRepo is a small adapter that implements the billing-context
// UserBillingPort: it applies plan / data-pack / topup effects on the user
// row. Lives here (user/infra/gormrepo) because it owns the users table.
type BillingApplyRepo struct{ db *gorm.DB }

func NewBillingApplyRepo(db *gorm.DB) *BillingApplyRepo { return &BillingApplyRepo{db: db} }

// ApplyPlan sets the user's active plan, resets traffic, and pushes the
// expiry date forward (if the user already had this plan unexpired, stack
// durations).
func (r *BillingApplyRepo) ApplyPlan(ctx context.Context, userID, planID uint64, durationDays, trafficGB, deviceLimit uint32) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u userRow
		if err := tx.First(&u, userID).Error; err != nil {
			return err
		}
		now := time.Now()
		base := now
		if u.PlanID != nil && *u.PlanID == planID && u.PlanExpireAt != nil && u.PlanExpireAt.After(now) {
			base = *u.PlanExpireAt
		}
		exp := base.AddDate(0, 0, int(durationDays))
		reset := now.AddDate(0, 1, 0)
		updates := map[string]any{
			"plan_id":          planID,
			"plan_expire_at":   exp,
			"traffic_total":    uint64(trafficGB) * 1024 * 1024 * 1024,
			"traffic_used":     0,
			"traffic_reset_at": reset,
			"device_limit":     deviceLimit,
		}
		return tx.Model(&userRow{}).Where("id = ?", userID).Updates(updates).Error
	})
}

// ApplyPack adds traffic to the user. Mode 1 = stack onto the current cycle;
// mode 2 = independent expiry (we still merge traffic onto current bucket in
// v1 for simplicity, but extend expiry).
func (r *BillingApplyRepo) ApplyPack(ctx context.Context, userID uint64, trafficGB, validDays uint32, attachMode int8) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u userRow
		if err := tx.First(&u, userID).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"traffic_total": gorm.Expr("traffic_total + ?", uint64(trafficGB)*1024*1024*1024),
		}
		if attachMode == 2 {
			now := time.Now()
			base := now
			if u.PlanExpireAt != nil && u.PlanExpireAt.After(now) {
				base = *u.PlanExpireAt
			}
			updates["plan_expire_at"] = base.AddDate(0, 0, int(validDays))
		}
		return tx.Model(&userRow{}).Where("id = ?", userID).Updates(updates).Error
	})
}

// ApplyTopup credits the user's balance.
func (r *BillingApplyRepo) ApplyTopup(ctx context.Context, userID uint64, amountCNY string) error {
	res := r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", userID).
		UpdateColumn("balance_cny", gorm.Expr("balance_cny + ?", amountCNY))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("user not found for topup")
	}
	return nil
}
