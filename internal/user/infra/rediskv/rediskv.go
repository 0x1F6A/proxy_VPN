// Package rediskv provides Redis-backed implementations of the user ports
// (access-token blacklist, rate limiter).
package rediskv

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Blacklist struct{ rdb *redis.Client }

func NewBlacklist(rdb *redis.Client) *Blacklist { return &Blacklist{rdb: rdb} }

func key(jti string) string { return "jwt:revoked:" + jti }

func (b *Blacklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return b.rdb.Set(ctx, key(jti), "1", ttl).Err()
}

func (b *Blacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := b.rdb.Exists(ctx, key(jti)).Result()
	return n > 0, err
}

// Limiter is a fixed-window counter. Not perfect (boundary bursts) but plenty
// for register/login throttling in v1.
type Limiter struct{ rdb *redis.Client }

func NewLimiter(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

func (l *Limiter) Allow(ctx context.Context, k string, limit int, window time.Duration) (bool, error) {
	rk := "rl:" + k
	n, err := l.rdb.Incr(ctx, rk).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, rk, window).Err()
	}
	return n <= int64(limit), nil
}

// itoa keeps the package-level helper available without importing strconv at
// call sites; reserved for future use.
var _ = strconv.Itoa
