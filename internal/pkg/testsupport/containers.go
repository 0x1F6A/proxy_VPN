//go:build integration

package testsupport

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	mysqlmod "github.com/testcontainers/testcontainers-go/modules/mysql"
	redismod "github.com/testcontainers/testcontainers-go/modules/redis"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// findRepoRoot walks up from this file until it finds the directory that
// contains go.mod so tests can locate the migration scripts regardless of
// the working directory of the test invocation.
func findRepoRoot(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("go.mod not found from", file)
		}
		dir = parent
	}
}

// MigrationScripts returns the absolute paths of every *.up.sql migration
// in lexical (apply) order.
func MigrationScripts(tb testing.TB) []string {
	tb.Helper()
	migDir := filepath.Join(findRepoRoot(tb), "internal", "migrations")
	matches, err := filepath.Glob(filepath.Join(migDir, "*.up.sql"))
	if err != nil {
		tb.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		tb.Fatalf("no migrations found in %s", migDir)
	}
	return matches
}

// StartMySQL boots a disposable MySQL 8.0 container, applies every
// *.up.sql migration in internal/migrations and returns a ready-to-use
// *gorm.DB. The container is terminated automatically when the test
// finishes.
func StartMySQL(tb testing.TB) *gorm.DB {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := mysqlmod.Run(ctx,
		"mysql:8.0",
		mysqlmod.WithDatabase("proxyvpn"),
		mysqlmod.WithUsername("proxyvpn"),
		mysqlmod.WithPassword("proxyvpn"),
		mysqlmod.WithScripts(MigrationScripts(tb)...),
	)
	if err != nil {
		tb.Fatalf("start mysql: %v", err)
	}
	tb.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		_ = container.Terminate(stopCtx)
	})

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true", "charset=utf8mb4")
	if err != nil {
		tb.Fatalf("dsn: %v", err)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		tb.Fatalf("gorm open: %v", err)
	}
	return db
}

// StartRedis boots a disposable Redis 7 container and returns a connected
// client. Cleanup is registered on tb.
func StartRedis(tb testing.TB) *redis.Client {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := redismod.Run(ctx, "redis:7-alpine")
	if err != nil {
		tb.Fatalf("start redis: %v", err)
	}
	tb.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		_ = container.Terminate(stopCtx)
	})

	addr, err := container.Endpoint(ctx, "")
	if err != nil {
		tb.Fatalf("redis endpoint: %v", err)
	}
	cli := redis.NewClient(&redis.Options{Addr: addr})
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		tb.Fatalf("ping redis: %v", err)
	}
	tb.Cleanup(func() { _ = cli.Close() })
	return cli
}
