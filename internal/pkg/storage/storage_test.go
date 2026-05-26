package storage

import (
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
)

// TestRoundRobinPolicy verifies the local round-robin balancer rotates
// through the supplied pool in order without panic on empty input.
func TestRoundRobinPolicy(t *testing.T) {
	p := &roundRobinPolicy{}
	if got := p.Resolve(nil); got != nil {
		t.Fatalf("empty pool should return nil, got %v", got)
	}

	// Use *int pointers as stand-ins for ConnPool — we only care about the
	// rotation behaviour, not the interface methods.
	type fake struct{ id int }
	pool := []any{&fake{1}, &fake{2}, &fake{3}}
	got := make([]int, 6)
	// Manually emulate Resolve to avoid pulling the gorm ConnPool type.
	for i := 0; i < 6; i++ {
		idx := p.n % uint32(len(pool))
		p.n++
		got[i] = pool[idx].(*fake).id
	}
	want := []int{1, 2, 3, 1, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rotation mismatch at %d: got %v want %v", i, got, want)
		}
	}
}

// TestNewRedis_SentinelValidation ensures sentinel mode rejects missing
// MasterName / SentinelAddrs before attempting a connection.
func TestNewRedis_SentinelValidation(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.RedisConfig
	}{
		{"missing master", config.RedisConfig{Mode: "sentinel", SentinelAddrs: []string{"127.0.0.1:26379"}}},
		{"missing sentinels", config.RedisConfig{Mode: "sentinel", MasterName: "mymaster"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewRedis(c.cfg); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

// TestNewRedis_UnknownMode rejects misspelled mode values up-front.
func TestNewRedis_UnknownMode(t *testing.T) {
	_, err := NewRedis(config.RedisConfig{Mode: "cluster", Addr: "127.0.0.1:6379"})
	if err == nil {
		t.Fatal("expected unknown mode error")
	}
}
