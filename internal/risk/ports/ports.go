// Package ports declares risk-service outbound interfaces.
package ports

import (
	"context"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/risk/domain"
)

// DeviceRepo persists login devices.
type DeviceRepo interface {
	Upsert(ctx context.Context, d *domain.LoginDevice) error
	ListByUser(ctx context.Context, userID uint64) ([]domain.LoginDevice, error)
	Revoke(ctx context.Context, userID uint64, fpHash string, at time.Time) error
}

// CountryLookup 抽象 geoip 查询，便于单测注入桩。
type CountryLookup interface {
	Country(ip string) string
}

// LockoutStore 用 Redis 实现失败次数计数 + 锁定标记。
type LockoutStore interface {
	IncrFail(ctx context.Context, key string, window time.Duration) (int, error)
	ResetFail(ctx context.Context, key string) error
	Lock(ctx context.Context, key string, ttl time.Duration) error
	IsLocked(ctx context.Context, key string) (bool, error)
}

// SubIPTracker 用 Redis ZSET 记录订阅 token 在窗口内出现过的 IP。
type SubIPTracker interface {
	// Touch 添加 IP，trim 掉 windowExpire 之前的记录，返回当前唯一 IP 数。
	Touch(ctx context.Context, token, ip string, window time.Duration) (int, error)
}

// AlertMailer 推送风险告警邮件（异地登录、token revoke）。
type AlertMailer interface {
	SendRiskAlert(ctx context.Context, to, locale, kind string, args map[string]string) error
}

// UserLookup 风控需要的最小用户视图。
type UserLookup interface {
	EmailAndCountry(ctx context.Context, userID uint64) (email, locale, lastCountry string, err error)
	UpdateLastCountry(ctx context.Context, userID uint64, country string) error
	RotateSubscriptionToken(ctx context.Context, userID uint64, newToken string, at time.Time) error
	SubscribeTokenByUser(ctx context.Context, userID uint64) (string, error)
}
