// Package chgo adapts the github.com/ClickHouse/clickhouse-go/v2 driver to
// the small chsink.Driver interface used by the traffic sink. Keeping the
// adapter in its own subpackage lets chsink avoid a hard dependency on the
// CH driver (so the package is buildable in environments without CGO/CH).
package chgo

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Options mirrors the subset of clickhouse.Options that the sink needs.
// Addr is a host:port; the driver uses native protocol on port 9000 (or
// 9440 with TLS) by default.
type Options struct {
	Addr        string
	Database    string
	User        string
	Password    string
	TLS         bool
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// Conn is the driver wrapper exposed to chsink.
type Conn struct{ c chdriver.Conn }

// Open dials ClickHouse and verifies the connection with Ping. Caller owns
// the returned *Conn and must call Close.
func Open(ctx context.Context, opts Options) (*Conn, error) {
	if opts.Database == "" {
		opts.Database = "default"
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 5 * time.Second
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 10 * time.Second
	}
	chOpts := &clickhouse.Options{
		Addr: []string{opts.Addr},
		Auth: clickhouse.Auth{
			Database: opts.Database,
			Username: opts.User,
			Password: opts.Password,
		},
		DialTimeout: opts.DialTimeout,
		ReadTimeout: opts.ReadTimeout,
	}
	if opts.TLS {
		chOpts.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	c, err := clickhouse.Open(chOpts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	defer cancel()
	if err := c.Ping(pingCtx); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &Conn{c: c}, nil
}

// Exec implements chsink.Driver.
func (c *Conn) Exec(ctx context.Context, query string, args ...any) error {
	return c.c.Exec(ctx, query, args...)
}

// Close implements chsink.Driver.
func (c *Conn) Close() error { return c.c.Close() }

// Query exposes the underlying driver's Query for tests and ad-hoc reads.
// The sink itself never reads back; this is a convenience.
func (c *Conn) Query(ctx context.Context, query string, args ...any) (chdriver.Rows, error) {
	return c.c.Query(ctx, query, args...)
}

// EnsureDatabase creates the database if it does not yet exist. The
// chsink Bootstrap qualifies all DDL with the database name, so the
// database itself must exist before Bootstrap runs.
func (c *Conn) EnsureDatabase(ctx context.Context, name string) error {
	return c.c.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", name))
}
