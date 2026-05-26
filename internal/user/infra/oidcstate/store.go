// Package oidcstate is a Redis-backed OIDCStateStore. State entries hold
// the post-login redirect URL and expire after StateTTL.
package oidcstate

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct{ rdb *redis.Client }

func New(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func key(state string) string { return "oidc:state:" + state }

func (s *Store) Save(ctx context.Context, state, redirect string, ttl time.Duration) error {
	if state == "" {
		return errors.New("empty state")
	}
	return s.rdb.Set(ctx, key(state), redirect, ttl).Err()
}

func (s *Store) Consume(ctx context.Context, state string) (string, bool, error) {
	if state == "" {
		return "", false, nil
	}
	v, err := s.rdb.GetDel(ctx, key(state)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}
