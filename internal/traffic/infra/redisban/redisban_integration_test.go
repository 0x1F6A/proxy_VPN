//go:build integration

package redisban_test

import (
	"context"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/infra/redisban"
)

func TestBanCacheIntegration(t *testing.T) {
	rdb := testsupport.StartRedis(t)
	cache := redisban.New(rdb).WithKey("test:traffic:ban")
	ctx := context.Background()

	if err := cache.Add(ctx, []uint64{1, 2, 3}, 5*time.Minute); err != nil {
		t.Fatalf("Add: %v", err)
	}
	ok, err := cache.Contains(ctx, 2)
	if err != nil || !ok {
		t.Fatalf("Contains 2 = %v, %v; want true,nil", ok, err)
	}
	ok, err = cache.Contains(ctx, 99)
	if err != nil || ok {
		t.Fatalf("Contains 99 = %v, %v; want false,nil", ok, err)
	}

	if err := cache.Remove(ctx, []uint64{2}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	ok, _ = cache.Contains(ctx, 2)
	if ok {
		t.Fatalf("expected 2 removed")
	}

	list, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 members, got %v", list)
	}

	ttl, err := rdb.TTL(ctx, "test:traffic:ban").Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %v", ttl)
	}
}
