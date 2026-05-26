// Package storage wires the MySQL (GORM) and Redis connections used by all
// services. It exposes lightweight Ping helpers so the HTTP /readyz endpoint
// can verify dependency health without leaking driver internals.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
)

// MySQL wraps a *gorm.DB plus the underlying *sql.DB for ping access.
// When read replicas are configured the embedded *gorm.DB transparently
// routes SELECT queries to a read pool via gorm.io/plugin/dbresolver.
type MySQL struct {
	DB *gorm.DB
	// hasReplicas reports whether at least one read replica DSN was wired.
	hasReplicas bool
}

// NewMySQL opens the connection pool described by cfg and verifies it with a
// ping. When cfg.ReadReplicas is non-empty the dbresolver plugin is attached
// so that all SELECTs are load-balanced across the replica pool while INSERT
// / UPDATE / DELETE / DDL stay on the primary. Returns an error if either
// step fails.
func NewMySQL(cfg config.MySQLConfig) (*MySQL, error) {
	gormDB, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger:      gormlogger.Default.LogMode(gormlogger.Warn),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	hasReplicas := false
	if len(cfg.ReadReplicas) > 0 {
		replicas := make([]gorm.Dialector, 0, len(cfg.ReadReplicas))
		for _, dsn := range cfg.ReadReplicas {
			replicas = append(replicas, mysql.Open(dsn))
		}
		policy := dbresolver.RandomPolicy{}
		var loadBalancer dbresolver.Policy = policy
		if cfg.ResolverPolicy == "round_robin" {
			loadBalancer = &roundRobinPolicy{}
		}
		resolverCfg := dbresolver.Config{
			Replicas: replicas,
			Policy:   loadBalancer,
		}
		plugin := dbresolver.Register(resolverCfg).
			SetMaxOpenConns(cfg.MaxOpenConns).
			SetMaxIdleConns(cfg.MaxIdleConns).
			SetConnMaxLifetime(cfg.ConnMaxLifetime)
		if err := gormDB.Use(plugin); err != nil {
			return nil, fmt.Errorf("attach dbresolver: %w", err)
		}
		hasReplicas = true
	}
	return &MySQL{DB: gormDB, hasReplicas: hasReplicas}, nil
}

// HasReplicas reports whether read replicas were configured.
func (m *MySQL) HasReplicas() bool { return m.hasReplicas }

// Ping verifies the MySQL primary connection is alive.
func (m *MySQL) Ping(ctx context.Context) error {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// ReadPing verifies at least one read replica answers. When no replicas are
// configured it falls back to the primary Ping so callers can treat it as a
// generic "can we read?" probe.
func (m *MySQL) ReadPing(ctx context.Context) error {
	if !m.hasReplicas {
		return m.Ping(ctx)
	}
	// dbresolver routes raw SQL through the replica pool when we annotate
	// the session with `clauses.Read`. Use a trivial round-trip query.
	return m.DB.Clauses(dbresolver.Read).WithContext(ctx).
		Exec("SELECT 1").Error
}

// Close releases the underlying connection pool.
func (m *MySQL) Close() error {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// roundRobinPolicy is a tiny stateful balancer used when callers opt into
// "round_robin" via config. dbresolver only ships RandomPolicy by default.
type roundRobinPolicy struct {
	n uint32
}

func (p *roundRobinPolicy) Resolve(connPools []gorm.ConnPool) gorm.ConnPool {
	if len(connPools) == 0 {
		return nil
	}
	idx := p.n % uint32(len(connPools))
	p.n++
	return connPools[idx]
}

// Redis is a thin wrapper around the official client so callers depend on a
// stable type while remaining free to access the embedded *redis.Client.
// In sentinel mode the embedded client is a FailoverClient that transparently
// follows master re-elections.
type Redis struct {
	*redis.Client
}

// NewRedis dials Redis and verifies the connection with a PING. Standalone
// mode (default) targets cfg.Addr directly; sentinel mode wires
// redis.NewFailoverClient with cfg.MasterName + cfg.SentinelAddrs.
func NewRedis(cfg config.RedisConfig) (*Redis, error) {
	var rdb *redis.Client
	switch cfg.Mode {
	case "sentinel":
		if cfg.MasterName == "" || len(cfg.SentinelAddrs) == 0 {
			return nil, fmt.Errorf("redis: sentinel mode requires master_name + sentinel_addrs")
		}
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.SentinelAddrs,
			Password:      cfg.Password,
			DB:            cfg.DB,
		})
	case "", "standalone":
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	default:
		return nil, fmt.Errorf("redis: unknown mode %q (want standalone|sentinel)", cfg.Mode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Redis{Client: rdb}, nil
}

// Ping verifies the Redis connection is alive.
func (r *Redis) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}
