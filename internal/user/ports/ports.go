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
	UpdatePassword(ctx context.Context, id uint64, hash string) error
	UpdateLogin(ctx context.Context, id uint64, at time.Time, ip string) error
	UpdateTOTP(ctx context.Context, id uint64, secret string, enabled bool) error
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
