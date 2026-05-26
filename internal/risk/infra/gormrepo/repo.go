// Package gormrepo implements risk DeviceRepo + a UserLookup view over the
// users table.
package gormrepo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/risk/domain"
)

// ---------- DeviceRepo --------------------------------------------------

type deviceRow struct {
	ID          uint64 `gorm:"primaryKey"`
	UserID      uint64
	FPHash      string `gorm:"column:fp_hash"`
	IP          string `gorm:"column:ip"`
	UserAgent   string
	Country     string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
}

func (deviceRow) TableName() string { return "login_devices" }

type DeviceRepo struct{ db *gorm.DB }

func NewDeviceRepo(db *gorm.DB) *DeviceRepo { return &DeviceRepo{db: db} }

func (r *DeviceRepo) Upsert(ctx context.Context, d *domain.LoginDevice) error {
	// ON DUPLICATE KEY UPDATE last_seen_at + ip + country
	return r.db.WithContext(ctx).Exec(`
INSERT INTO login_devices (user_id, fp_hash, ip, user_agent, country, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  ip = VALUES(ip),
  user_agent = VALUES(user_agent),
  country = VALUES(country),
  last_seen_at = VALUES(last_seen_at),
  revoked_at = NULL
`, d.UserID, d.FPHash, d.IP, d.UserAgent, d.Country, d.FirstSeenAt, d.LastSeenAt).Error
}

func (r *DeviceRepo) ListByUser(ctx context.Context, userID uint64) ([]domain.LoginDevice, error) {
	var rows []deviceRow
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).Order("last_seen_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.LoginDevice, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.LoginDevice{
			ID: row.ID, UserID: row.UserID, FPHash: row.FPHash,
			IP: row.IP, UserAgent: row.UserAgent, Country: row.Country,
			FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt,
			RevokedAt: row.RevokedAt,
		})
	}
	return out, nil
}

func (r *DeviceRepo) Revoke(ctx context.Context, userID uint64, fpHash string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&deviceRow{}).
		Where("user_id = ? AND fp_hash = ?", userID, fpHash).
		Update("revoked_at", at).Error
}

// ---------- UserLookup --------------------------------------------------

type UserLookup struct{ db *gorm.DB }

func NewUserLookup(db *gorm.DB) *UserLookup { return &UserLookup{db: db} }

func (r *UserLookup) EmailAndCountry(ctx context.Context, userID uint64) (email, locale, lastCountry string, err error) {
	var row struct {
		Email            string
		Locale           string
		LastLoginCountry string
	}
	err = r.db.WithContext(ctx).Table("users").
		Select("email, locale, last_login_country").
		Where("id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", "", nil
	}
	return row.Email, row.Locale, row.LastLoginCountry, err
}

func (r *UserLookup) UpdateLastCountry(ctx context.Context, userID uint64, country string) error {
	return r.db.WithContext(ctx).Table("users").
		Where("id = ?", userID).
		Update("last_login_country", country).Error
}

func (r *UserLookup) RotateSubscriptionToken(ctx context.Context, userID uint64, newToken string, at time.Time) error {
	return r.db.WithContext(ctx).Table("users").
		Where("id = ?", userID).
		Updates(map[string]any{
			"subscription_token":         newToken,
			"subscribe_token_rotated_at": at,
		}).Error
}

func (r *UserLookup) SubscribeTokenByUser(ctx context.Context, userID uint64) (string, error) {
	var tok string
	err := r.db.WithContext(ctx).Table("users").
		Select("subscription_token").Where("id = ?", userID).
		Take(&tok).Error
	return tok, err
}
