// seed-admin 创建/覆盖一个 admin 账号，便于本地手动联调 Web Admin。
//
// 用法:
//   go run ./cmd/seed-admin \
//       --dsn 'root:root@tcp(127.0.0.1:3306)/proxy_vpn?charset=utf8mb4&parseTime=true&loc=UTC' \
//       --email admin@local.test --password admin123 [--role admin]
package main

import (
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

func main() {
	dsn := flag.String("dsn", "root:root@tcp(127.0.0.1:3306)/proxy_vpn?charset=utf8mb4&parseTime=true&loc=UTC", "MySQL DSN")
	email := flag.String("email", "admin@local.test", "admin email")
	password := flag.String("password", "admin123", "admin password (plain)")
	role := flag.String("role", "admin", "role: admin|ops|finance")
	flag.Parse()

	if !strings.ContainsRune(*email, '@') {
		log.Fatalf("invalid email: %s", *email)
	}
	if len(*password) < 6 {
		log.Fatalf("password too short (min 6)")
	}
	switch *role {
	case "admin", "ops", "finance":
	default:
		log.Fatalf("invalid role: %s", *role)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v (是否已 docker compose up -d?)", err)
	}

	uid := uuid.NewString()
	subToken := randomHex(20)
	inviteCode := strings.ToUpper(randomHex(4))[:8]
	now := time.Now().UTC()

	_, err = db.Exec(`
INSERT INTO users (email, password_hash, uuid, role, status, subscription_token, invite_code, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), role = VALUES(role), status = 1, updated_at = VALUES(updated_at)`,
		*email, hash, uid, *role, subToken, inviteCode, now, now)
	if err != nil {
		log.Fatalf("insert: %v", err)
	}

	var id uint64
	if err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, *email).Scan(&id); err != nil {
		log.Fatalf("select id: %v", err)
	}
	fmt.Printf("✅ admin seeded:\n  id        = %d\n  email     = %s\n  password  = %s\n  role      = %s\n", id, *email, *password, *role)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("rand: %v", err)
	}
	return fmt.Sprintf("%x", b)
}
