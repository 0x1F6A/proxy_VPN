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

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
)

// MySQL wraps a *gorm.DB plus the underlying *sql.DB for ping access.
type MySQL struct {
	DB *gorm.DB
}

// NewMySQL opens the connection pool described by cfg and verifies it with a
// ping. Returns an error if either step fails.
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
	return &MySQL{DB: gormDB}, nil
}

// Ping verifies the MySQL connection is alive.
func (m *MySQL) Ping(ctx context.Context) error {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close releases the underlying connection pool.
func (m *MySQL) Close() error {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Redis is a thin wrapper around the official client so callers depend on a
// stable type while remaining free to access the embedded *redis.Client.
type Redis struct {
	*redis.Client
}

// NewRedis dials Redis and verifies the connection with a PING.
func NewRedis(cfg config.RedisConfig) (*Redis, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
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
