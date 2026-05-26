// Package redisban backs traffic.ports.BanCache with a Redis SET. Members
// are user ids encoded as decimal strings. A short TTL on the key forces
// a rebuild via the asynq RecomputeBans task in case Redis is wiped.
package redisban

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultKey = "traffic:ban:users"

type Cache struct {
	rdb *redis.Client
	key string
}

func New(rdb *redis.Client) *Cache { return &Cache{rdb: rdb, key: defaultKey} }

// WithKey overrides the Redis key (useful for tests).
func (c *Cache) WithKey(k string) *Cache { c.key = k; return c }

func (c *Cache) Add(ctx context.Context, userIDs []uint64, ttl time.Duration) error {
	if len(userIDs) == 0 {
		return nil
	}
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, strconv.FormatUint(id, 10))
	}
	if err := c.rdb.SAdd(ctx, c.key, args...).Err(); err != nil {
		return err
	}
	if ttl > 0 {
		_ = c.rdb.Expire(ctx, c.key, ttl).Err()
	}
	return nil
}

func (c *Cache) Remove(ctx context.Context, userIDs []uint64) error {
	if len(userIDs) == 0 {
		return nil
	}
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, strconv.FormatUint(id, 10))
	}
	return c.rdb.SRem(ctx, c.key, args...).Err()
}

func (c *Cache) Contains(ctx context.Context, userID uint64) (bool, error) {
	return c.rdb.SIsMember(ctx, c.key, strconv.FormatUint(userID, 10)).Result()
}

func (c *Cache) List(ctx context.Context) ([]uint64, error) {
	vals, err := c.rdb.SMembers(ctx, c.key).Result()
	if err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(vals))
	for _, v := range vals {
		if id, perr := strconv.ParseUint(v, 10, 64); perr == nil {
			out = append(out, id)
		}
	}
	return out, nil
}
