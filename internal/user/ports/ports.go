// Package ports declares the outbound interfaces the user-service depends on.
// Concrete implementations live under internal/user/infra/*.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
)

type UserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uint64) (*domain.User, error)
	FindByOIDCSubject(ctx context.Context, subject string) (*domain.User, error)
	UpdatePassword(ctx context.Context, id uint64, hash string) error
	UpdateLogin(ctx context.Context, id uint64, at time.Time, ip string) error
	UpdateTOTP(ctx context.Context, id uint64, secret string, enabled bool) error
	LinkOIDCSubject(ctx context.Context, id uint64, subject string) error
}

type RefreshRepo interface {
	Create(ctx context.Context, rt *domain.RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id uint64, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID uint64, at time.Time) error
}

type EmailCodeRepo interface {
	Create(ctx context.Context, c *domain.EmailCode) error
	FindLatestUnused(ctx context.Context, email, scene string) (*domain.EmailCode, error)
	IncAttempts(ctx context.Context, id uint64) error
	MarkUsed(ctx context.Context, id uint64, at time.Time) error
}

type LoginLogRepo interface {
	Append(ctx context.Context, l domain.LoginLog) error
}

type Mailer interface {
	SendCode(ctx context.Context, to, scene, code string) error
}

// AccessTokenBlacklist tracks revoked access-token jti values until they expire.
type AccessTokenBlacklist interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// RateLimiter implements a simple per-key fixed-window counter.
type RateLimiter interface {
	// Allow returns true if the key has not yet hit limit in window.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RiskHook 是 Phase 15-A 风控接入点。所有方法可空实现（Service.Deps.Risk 为 nil
// 时整个风控降级）。
type RiskHook interface {
	PreLogin(ctx context.Context, email string) error
	RegisterLoginFailure(ctx context.Context, email string)
	RegisterLoginSuccess(ctx context.Context, userID uint64, email, ip, ua, acceptLang string)
}

// AdminUserView is a flattened projection used by admin list / detail
// endpoints; intentionally separate from domain.User so we can include
// computed columns (traffic used %, plan name, etc).
type AdminUserView struct {
	ID            uint64
	Email         string
	Status        int
	Role          string
	PlanID        *uint64
	PlanExpireAt  *time.Time
	TrafficTotal  uint64
	TrafficUsed   uint64
	RateBpsUp     uint64
	RateBpsDown   uint64
	Banned        bool
	CreatedAt     time.Time
	LastLoginAt   *time.Time
}

// AdminUserRepo exposes admin-only read+mutation paths on the users table.
type AdminUserRepo interface {
	List(ctx context.Context, q string, limit, offset int) ([]AdminUserView, int64, error)
	SetBanned(ctx context.Context, id uint64, banned bool) error
	AdjustTraffic(ctx context.Context, id uint64, deltaBytes int64) error
	SetRateLimits(ctx context.Context, id uint64, upBps, downBps uint64) error
	OverallCounts(ctx context.Context) (AdminCounts, error)
}

// AdminCounts is the dashboard top-row metric.
type AdminCounts struct {
	TotalUsers   int64
	ActiveUsers  int64
	BannedUsers  int64
	ActivePlans  int64 // users with non-expired plan
}
