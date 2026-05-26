package gormrepo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
)

// AdminUserRepo implements user/ports.AdminUserRepo on the users table.
// Uses raw SQL for the columns added by traffic phase (rate_bps_*, banned)
// so we don't have to evolve userRow.
type AdminUserRepo struct{ db *gorm.DB }

func NewAdminUserRepo(db *gorm.DB) *AdminUserRepo { return &AdminUserRepo{db: db} }

func (r *AdminUserRepo) List(ctx context.Context, q string, limit, offset int) ([]ports.AdminUserView, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	base := r.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL")
	if q != "" {
		like := "%" + q + "%"
		base = base.Where("email LIKE ? OR CAST(id AS CHAR) = ?", like, q)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	type row struct {
		ID            uint64
		Email         string
		Status        int
		Role          string
		PlanID        *uint64
		PlanExpireAt  *time.Time
		TrafficTotal  uint64
		TrafficUsed   uint64
		RateBpsUp     uint64 `gorm:"column:rate_bps_up"`
		RateBpsDown   uint64 `gorm:"column:rate_bps_down"`
		Banned        bool
		CreatedAt     time.Time
		LastLoginAt   *time.Time
	}
	var rows []row
	err := base.Select(`id, email, status, role, plan_id, plan_expire_at,
		traffic_total, traffic_used,
		COALESCE(rate_bps_up, 0) AS rate_bps_up,
		COALESCE(rate_bps_down, 0) AS rate_bps_down,
		COALESCE(banned, 0) AS banned,
		created_at, last_login_at`).
		Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]ports.AdminUserView, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.AdminUserView{
			ID: r.ID, Email: r.Email, Status: r.Status, Role: r.Role,
			PlanID: r.PlanID, PlanExpireAt: r.PlanExpireAt,
			TrafficTotal: r.TrafficTotal, TrafficUsed: r.TrafficUsed,
			RateBpsUp: r.RateBpsUp, RateBpsDown: r.RateBpsDown,
			Banned: r.Banned, CreatedAt: r.CreatedAt, LastLoginAt: r.LastLoginAt,
		})
	}
	return out, total, nil
}

func (r *AdminUserRepo) SetBanned(ctx context.Context, id uint64, banned bool) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET banned = ?, updated_at = NOW() WHERE id = ?`, banned, id,
	).Error
}

func (r *AdminUserRepo) AdjustTraffic(ctx context.Context, id uint64, delta int64) error {
	// Allow negative deltas (refund). Clamp at 0 with GREATEST.
	if delta >= 0 {
		return r.db.WithContext(ctx).Exec(
			`UPDATE users SET traffic_total = traffic_total + ?, updated_at = NOW() WHERE id = ?`,
			uint64(delta), id,
		).Error
	}
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET traffic_total = GREATEST(CAST(traffic_total AS SIGNED) + ?, 0), updated_at = NOW() WHERE id = ?`,
		delta, id,
	).Error
}

func (r *AdminUserRepo) SetRateLimits(ctx context.Context, id uint64, up, down uint64) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE users SET rate_bps_up = ?, rate_bps_down = ?, updated_at = NOW() WHERE id = ?`,
		up, down, id,
	).Error
}

func (r *AdminUserRepo) OverallCounts(ctx context.Context) (ports.AdminCounts, error) {
	var c ports.AdminCounts
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total_users,
			SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_users,
			SUM(CASE WHEN COALESCE(banned, 0) = 1 THEN 1 ELSE 0 END) AS banned_users,
			SUM(CASE WHEN plan_id IS NOT NULL AND (plan_expire_at IS NULL OR plan_expire_at > NOW()) THEN 1 ELSE 0 END) AS active_plans
		FROM users WHERE deleted_at IS NULL
	`).Row().Scan(&c.TotalUsers, &c.ActiveUsers, &c.BannedUsers, &c.ActivePlans)
	return c, err
}
