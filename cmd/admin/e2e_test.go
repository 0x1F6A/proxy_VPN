//go:build integration && e2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/audit"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/httpx"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/storage"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
	"github.com/0x1F6A/proxy_VPN/internal/user/transport/httpapi"
	"golang.org/x/crypto/bcrypt"
)

// TestAdminE2ELoginListBan walks the admin happy-path: admin user logs in,
// hits /api/v1/admin/users to list, then bans the seeded fixture user, and
// finally verifies an audit row was written. Requires MySQL + Redis (via
// testcontainers) and the `e2e` build tag.
func TestAdminE2ELoginListBan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testsupport.StartMySQL(t)
	rdb := testsupport.StartRedis(t)

	cfg := &config.Config{
		App: config.AppConfig{Name: "proxy_VPN", Env: "test"},
		JWT: config.JWTConfig{
			Secret:           "admin-e2e-secret-must-be-at-least-32-bytes-long!",
			AccessTTL:        15 * time.Minute,
			RefreshTTL:       24 * time.Hour,
			Issuer:           "proxy_VPN-admin-e2e",
			AllowedClockSkew: 30 * time.Second,
		},
		Rate:       config.RateConfig{SendCodePerEmailMin: 100, LoginFailPerIPMin: 100},
		Node:       config.NodeConfig{BootstrapSecret: "boot", HeartbeatTimeout: time.Minute},
		Payment:    config.PaymentConfig{Mode: "mock", MockSecret: "mock"},
		Traffic:    config.TrafficConfig{ReportInterval: 30 * time.Second, BanCacheTTL: time.Minute, RateDefaultUpMbps: 100, RateDefaultDownMbps: 100},
		ClickHouse: config.ClickHouseConfig{Enabled: false},
	}

	mySQL := &storage.MySQL{DB: db}
	redisStore := &storage.Redis{Client: rdb}

	router := httpx.NewRouter(httpx.Options{Version: "test", Logger: logger.New("error", "text")})
	writer := audit.NewGormWriter(db, nil)
	router.Use(adminOnly(audit.Middleware(writer, httpapi.ClaimsFrom)))
	mountAdminAPI(router, cfg, mySQL, redisStore)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	cli := srv.Client()
	cli.Timeout = 10 * time.Second
	ctx := context.Background()

	// Seed admin + target user directly via SQL. The admin user is what the
	// console operator logs in as; the target user is the one we'll ban.
	hash, _ := bcrypt.GenerateFromPassword([]byte("AdminPass!1"), bcrypt.DefaultCost)
	if err := db.Exec(`INSERT INTO users (id,email,password_hash,uuid,subscription_token,invite_code,status,role,created_at,updated_at) VALUES (1001,'admin@example.com',?, '11111111-1111-1111-1111-111111111111','sub-admin-token-aaaaaaaaaaaaaaaaaaaaaa','ADM00001',1,'admin',NOW(),NOW())`, string(hash)).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id,email,password_hash,uuid,subscription_token,invite_code,status,role,created_at,updated_at) VALUES (1002,'target@example.com',?, '22222222-2222-2222-2222-222222222222','sub-target-token-bbbbbbbbbbbbbbbbbbbb','USR00002',1,'user',NOW(),NOW())`, string(hash)).Error; err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// --- 1. login as admin ----------------------------------------------
	loginResp := adminDo(t, ctx, cli, "POST", srv.URL+"/api/v1/auth/login", "",
		map[string]any{"email": "admin@example.com", "password": "AdminPass!1"}, http.StatusOK)
	data, _ := loginResp["data"].(map[string]any)
	tok, _ := data["access_token"].(string)
	if tok == "" {
		t.Fatalf("login missing access_token: %+v", loginResp)
	}

	// --- 2. GET /admin/users (read; should NOT be audited) --------------
	listResp := adminDo(t, ctx, cli, "GET", srv.URL+"/api/v1/admin/users?page=1&page_size=10", tok, nil, http.StatusOK)
	if _, ok := listResp["data"]; !ok {
		t.Fatalf("list missing data: %+v", listResp)
	}

	// --- 3. POST /admin/users/:id/ban (write; should be audited) --------
	adminDo(t, ctx, cli, "POST", srv.URL+"/api/v1/admin/users/1002/ban", tok,
		map[string]any{"reason": "e2e-test"}, http.StatusOK)

	// audit goroutine writes asynchronously — give it a beat.
	time.Sleep(300 * time.Millisecond)

	var auditCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM admin_audit_logs WHERE admin_id=1001 AND action LIKE 'POST %ban%'`).Scan(&auditCount).Error; err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount < 1 {
		t.Fatalf("expected >=1 audit row, got %d", auditCount)
	}
}

func adminDo(t *testing.T, ctx context.Context, cli *http.Client, method, url, token string, payload any, wantStatus int) map[string]any {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, url, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s -> status=%d want=%d body=%s", method, url, resp.StatusCode, wantStatus, raw)
	}
	var out map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if code, ok := out["code"]; ok {
		if n, ok := code.(float64); ok && n != 0 {
			t.Fatalf("%s %s -> business code=%v msg=%v", method, url, code, out["message"])
		}
	}
	return out
}
