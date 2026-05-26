// Package gormrepo implements user-context repositories on top of GORM/MySQL.
package gormrepo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
)

// ---------- ORM models -------------------------------------------------

type userRow struct {
	ID                uint64 `gorm:"primaryKey"`
	Email             string
	PasswordHash      string
	UUID              string
	Role              string
	Status            int
	BalanceCNY        string `gorm:"column:balance_cny;type:decimal(12,2)"`
	PlanID            *uint64
	PlanExpireAt      *time.Time
	TrafficTotal      uint64
	TrafficUsed       uint64
	TrafficResetAt    *time.Time
	DeviceLimit       uint32
	SubscriptionToken string
	TOTPSecret        string `gorm:"column:totp_secret"`
	TOTPEnabled       bool   `gorm:"column:totp_enabled"`
	InvitedBy         *uint64
	InviteCode        string
	LastLoginAt       *time.Time
	LastLoginIP       string
	OIDCSubject       *string `gorm:"column:oidc_subject"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

func (userRow) TableName() string { return "users" }

func toUserRow(u *domain.User) *userRow {
	var sub *string
	if u.OIDCSubject != "" {
		s := u.OIDCSubject
		sub = &s
	}
	return &userRow{
		ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, UUID: u.UUID,
		Role: u.Role, Status: u.Status, BalanceCNY: u.BalanceCNY,
		PlanID: u.PlanID, PlanExpireAt: u.PlanExpireAt,
		TrafficTotal: u.TrafficTotal, TrafficUsed: u.TrafficUsed,
		TrafficResetAt: u.TrafficResetAt, DeviceLimit: u.DeviceLimit,
		SubscriptionToken: u.SubscriptionToken, TOTPSecret: u.TOTPSecret,
		TOTPEnabled: u.TOTPEnabled, InvitedBy: u.InvitedBy,
		InviteCode: u.InviteCode, LastLoginAt: u.LastLoginAt,
		LastLoginIP: u.LastLoginIP,
		OIDCSubject: sub,
	}
}

func fromUserRow(r *userRow) *domain.User {
	sub := ""
	if r.OIDCSubject != nil {
		sub = *r.OIDCSubject
	}
	return &domain.User{
		ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, UUID: r.UUID,
		Role: r.Role, Status: r.Status, BalanceCNY: r.BalanceCNY,
		PlanID: r.PlanID, PlanExpireAt: r.PlanExpireAt,
		TrafficTotal: r.TrafficTotal, TrafficUsed: r.TrafficUsed,
		TrafficResetAt: r.TrafficResetAt, DeviceLimit: r.DeviceLimit,
		SubscriptionToken: r.SubscriptionToken, TOTPSecret: r.TOTPSecret,
		TOTPEnabled: r.TOTPEnabled, InvitedBy: r.InvitedBy,
		InviteCode: r.InviteCode, LastLoginAt: r.LastLoginAt,
		LastLoginIP: r.LastLoginIP,
		OIDCSubject: sub,
		CreatedAt:   r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// ---------- UserRepo ----------------------------------------------------

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	row := toUserRow(u)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	u.ID = row.ID
	u.CreatedAt = row.CreatedAt
	u.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromUserRow(&row), nil
}

func (r *UserRepo) FindByID(ctx context.Context, id uint64) (*domain.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromUserRow(&row), nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id uint64, hash string) error {
	return r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).
		Update("password_hash", hash).Error
}

func (r *UserRepo) UpdateLogin(ctx context.Context, id uint64, at time.Time, ip string) error {
	return r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_at": at, "last_login_ip": ip}).Error
}

func (r *UserRepo) UpdateTOTP(ctx context.Context, id uint64, secret string, enabled bool) error {
	return r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).
		Updates(map[string]any{"totp_secret": secret, "totp_enabled": enabled}).Error
}

func (r *UserRepo) FindByOIDCSubject(ctx context.Context, subject string) (*domain.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).Where("oidc_subject = ?", subject).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromUserRow(&row), nil
}

func (r *UserRepo) LinkOIDCSubject(ctx context.Context, id uint64, subject string) error {
	return r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).
		Update("oidc_subject", subject).Error
}

// ---------- RefreshRepo -------------------------------------------------

type refreshRow struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	TokenHash string
	UserAgent string
	IP        string `gorm:"column:ip"`
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (refreshRow) TableName() string { return "refresh_tokens" }

type RefreshRepo struct{ db *gorm.DB }

func NewRefreshRepo(db *gorm.DB) *RefreshRepo { return &RefreshRepo{db: db} }

func (r *RefreshRepo) Create(ctx context.Context, rt *domain.RefreshToken) error {
	row := &refreshRow{
		UserID: rt.UserID, TokenHash: rt.TokenHash, UserAgent: rt.UserAgent,
		IP: rt.IP, ExpiresAt: rt.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	rt.ID = row.ID
	rt.CreatedAt = row.CreatedAt
	return nil
}

func (r *RefreshRepo) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var row refreshRow
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRefreshInvalid
	}
	if err != nil {
		return nil, err
	}
	return &domain.RefreshToken{
		ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash,
		UserAgent: row.UserAgent, IP: row.IP, ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&refreshRow{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

func (r *RefreshRepo) RevokeAllForUser(ctx context.Context, userID uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&refreshRow{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
}

// ---------- EmailCodeRepo -----------------------------------------------

type emailCodeRow struct {
	ID        uint64 `gorm:"primaryKey"`
	Email     string
	Scene     string
	CodeHash  string
	Attempts  uint8
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (emailCodeRow) TableName() string { return "email_codes" }

type EmailCodeRepo struct{ db *gorm.DB }

func NewEmailCodeRepo(db *gorm.DB) *EmailCodeRepo { return &EmailCodeRepo{db: db} }

func (r *EmailCodeRepo) Create(ctx context.Context, c *domain.EmailCode) error {
	row := &emailCodeRow{
		Email: c.Email, Scene: c.Scene, CodeHash: c.CodeHash,
		ExpiresAt: c.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	c.ID = row.ID
	c.CreatedAt = row.CreatedAt
	return nil
}

func (r *EmailCodeRepo) FindLatestUnused(ctx context.Context, email, scene string) (*domain.EmailCode, error) {
	var row emailCodeRow
	err := r.db.WithContext(ctx).
		Where("email = ? AND scene = ? AND used_at IS NULL", email, scene).
		Order("id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &domain.EmailCode{
		ID: row.ID, Email: row.Email, Scene: row.Scene, CodeHash: row.CodeHash,
		Attempts: row.Attempts, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *EmailCodeRepo) IncAttempts(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&emailCodeRow{}).
		Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
}

func (r *EmailCodeRepo) MarkUsed(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&emailCodeRow{}).
		Where("id = ?", id).Update("used_at", at).Error
}

// ---------- LoginLogRepo ------------------------------------------------

type loginLogRow struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    *uint64
	Email     string
	Success   bool
	IP        string `gorm:"column:ip"`
	UserAgent string
	Reason    string
	CreatedAt time.Time
}

func (loginLogRow) TableName() string { return "login_logs" }

type LoginLogRepo struct{ db *gorm.DB }

func NewLoginLogRepo(db *gorm.DB) *LoginLogRepo { return &LoginLogRepo{db: db} }

func (r *LoginLogRepo) Append(ctx context.Context, l domain.LoginLog) error {
	return r.db.WithContext(ctx).Create(&loginLogRow{
		UserID: l.UserID, Email: l.Email, Success: l.Success,
		IP: l.IP, UserAgent: l.UserAgent, Reason: l.Reason,
	}).Error
}
