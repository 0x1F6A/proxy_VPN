//go:build integration

package chsink_test

import (
	"context"
	"testing"
	"time"

	chmod "github.com/testcontainers/testcontainers-go/modules/clickhouse"

	"github.com/0x1F6A/proxy_VPN/internal/traffic/domain"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/infra/chsink"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/infra/chsink/chgo"
)

// startClickHouse spins up a disposable CH container and returns a driver
// connected against it. Cleanup is registered on tb.
func startClickHouse(tb testing.TB) *chgo.Conn {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := chmod.Run(ctx,
		"clickhouse/clickhouse-server:24.3-alpine",
		chmod.WithUsername("default"),
		chmod.WithPassword("ch-test"),
		chmod.WithDatabase("default"),
	)
	if err != nil {
		tb.Fatalf("start clickhouse: %v", err)
	}
	tb.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		_ = container.Terminate(stopCtx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		tb.Fatalf("host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		tb.Fatalf("port: %v", err)
	}
	conn, err := chgo.Open(ctx, chgo.Options{
		Addr:     host + ":" + port.Port(),
		Database: "default",
		User:     "default",
		Password: "ch-test",
	})
	if err != nil {
		tb.Fatalf("chgo open: %v", err)
	}
	tb.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestChSinkEndToEnd(t *testing.T) {
	conn := startClickHouse(t)
	ctx := context.Background()

	if err := conn.EnsureDatabase(ctx, "proxyvpn"); err != nil {
		t.Fatalf("EnsureDatabase: %v", err)
	}

	sink, err := chsink.New(chsink.Config{
		Enabled:  true,
		Database: "proxyvpn",
	}, conn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := sink.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Calling Bootstrap a second time must be a no-op (idempotent).
	if err := sink.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap (re-run): %v", err)
	}

	day := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	events := []domain.UsageEvent{
		{Ts: day, UserID: 7, NodeID: 1, Protocol: "vless", UpBytes: 100, DownBytes: 200},
		{Ts: day.Add(time.Hour), UserID: 7, NodeID: 1, Protocol: "vless", UpBytes: 50, DownBytes: 75},
		{Ts: day, UserID: 8, NodeID: 2, Protocol: "trojan", UpBytes: 300, DownBytes: 400},
	}
	if err := sink.Write(ctx, events); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify raw event readback.
	type row struct {
		User uint64
		Up   uint64
		Down uint64
	}
	rows, err := conn.Query(ctx, `SELECT user_id, sum(up_bytes), sum(down_bytes)
		FROM proxyvpn.traffic_events GROUP BY user_id ORDER BY user_id`)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.User, &r.Up, &r.Down); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	want := []row{{7, 150, 275}, {8, 300, 400}}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row[%d] = %+v want %+v", i, got[i], want[i])
		}
	}

	// Verify the materialised view aggregated correctly. SummingMergeTree
	// merges asynchronously, so wrap counters in SUM().
	mvRows, err := conn.Query(ctx, `SELECT user_id, sum(up_bytes), sum(down_bytes)
		FROM proxyvpn.traffic_user_daily WHERE day = '2026-05-01'
		GROUP BY user_id ORDER BY user_id`)
	if err != nil {
		t.Fatalf("query mv: %v", err)
	}
	defer mvRows.Close()
	got = got[:0]
	for mvRows.Next() {
		var r row
		if err := mvRows.Scan(&r.User, &r.Up, &r.Down); err != nil {
			t.Fatalf("mv scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 2 || got[0] != (row{7, 150, 275}) || got[1] != (row{8, 300, 400}) {
		t.Fatalf("mv rows = %+v, want [{7 150 275} {8 300 400}]", got)
	}
}
