// Package rediskv 提供风控 Lockout + SubIPTracker 的 Redis 实现。
package rediskv

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// LockoutStore 用 INCR + EXPIRE 维护登录失败计数；锁定标记用单独 key。
type LockoutStore struct{ rdb *redis.Client }

func NewLockoutStore(rdb *redis.Client) *LockoutStore { return &LockoutStore{rdb: rdb} }

func (s *LockoutStore) IncrFail(ctx context.Context, key string, window time.Duration) (int, error) {
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 && window > 0 {
		_ = s.rdb.Expire(ctx, key, window).Err()
	}
	return int(n), nil
}

func (s *LockoutStore) ResetFail(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, key).Err()
}

func (s *LockoutStore) Lock(ctx context.Context, key string, ttl time.Duration) error {
	return s.rdb.Set(ctx, key, "1", ttl).Err()
}

func (s *LockoutStore) IsLocked(ctx context.Context, key string) (bool, error) {
	n, err := s.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// SubIPTracker 用 ZSET（score=unix epoch）维护订阅 token 滚动窗口内的 IP 集合。
type SubIPTracker struct{ rdb *redis.Client }

func NewSubIPTracker(rdb *redis.Client) *SubIPTracker { return &SubIPTracker{rdb: rdb} }

func (t *SubIPTracker) Touch(ctx context.Context, token, ip string, window time.Duration) (int, error) {
	k := "risk:sub:ips:" + token
	now := time.Now().Unix()
	cutoff := now - int64(window.Seconds())
	pipe := t.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, k, "-inf", itoa64(cutoff))
	pipe.ZAdd(ctx, k, redis.Z{Score: float64(now), Member: ip})
	pipe.Expire(ctx, k, window+time.Minute)
	cardCmd := pipe.ZCard(ctx, k)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return int(cardCmd.Val()), nil
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
