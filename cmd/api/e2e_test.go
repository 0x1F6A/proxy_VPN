//go:build e2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/httpx"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/storage"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
)

// fakeMailer captures the most recent code for each (email, scene) so the
// e2e test can complete the registration flow without an SMTP server.
type fakeMailer struct {
	mu    sync.Mutex
	codes map[string]string
}

func newFakeMailer() *fakeMailer { return &fakeMailer{codes: map[string]string{}} }

func (m *fakeMailer) SendCode(_ context.Context, to, scene, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[scene+":"+to] = code
	return nil
}

func (m *fakeMailer) Get(scene, to string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[scene+":"+to]
}

func TestAPIE2EUserOrderSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testsupport.StartMySQL(t)
	rdb := testsupport.StartRedis(t)

	mailer := newFakeMailer()
	prev := newMailer
	newMailer = func(_ config.SMTPConfig) ports.Mailer { return mailer }
	t.Cleanup(func() { newMailer = prev })

	cfg := &config.Config{
		App: config.AppConfig{Name: "proxy_VPN", Env: "test"},
		JWT: config.JWTConfig{
			Secret:           "e2e-test-secret-must-be-at-least-32-bytes-long!!",
			AccessTTL:        15 * time.Minute,
			RefreshTTL:       24 * time.Hour,
			Issuer:           "proxy_VPN-e2e",
			AllowedClockSkew: 30 * time.Second,
		},
		Rate: config.RateConfig{SendCodePerEmailMin: 100, LoginFailPerIPMin: 100},
		Node: config.NodeConfig{
			BootstrapSecret:  "node-bootstrap-secret",
			HeartbeatTimeout: time.Minute,
			SubscriptionBase: "http://127.0.0.1",
		},
		Payment: config.PaymentConfig{Mode: "mock", MockSecret: "mock-secret"},
		Traffic: config.TrafficConfig{
			ReportInterval: 30 * time.Second, BanCacheTTL: time.Minute,
			RateDefaultUpMbps: 100, RateDefaultDownMbps: 100,
		},
		ClickHouse: config.ClickHouseConfig{Enabled: false},
	}

	mySQL := &storage.MySQL{DB: db}
	redisStore := &storage.Redis{Client: rdb}

	router := httpx.NewRouter(httpx.Options{Version: "test", Logger: logger.New("error", "text")})
	mountUserAPI(router, cfg, mySQL, redisStore)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	cli := srv.Client()
	cli.Timeout = 10 * time.Second

	// Seed a node group + plan + plan_node_groups link so the order has a
	// valid plan target and the subscription has a group to resolve.
	if err := db.Exec(`INSERT INTO node_groups (id, name, level, remark) VALUES (1, 'default', 1, '')`).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Exec(`INSERT INTO plans (id, name, description, price_cny, duration_days, traffic_gb, device_limit, speed_limit_mbps, node_group_id, sort, status)
		VALUES (1, '月套餐', '30 days', 29.00, 30, 100, 3, 100, 1, 1, 1)`).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	if err := db.Exec(`INSERT INTO plan_node_groups (plan_id, node_group_id) VALUES (1, 1)`).Error; err != nil {
		t.Fatalf("seed pn: %v", err)
	}

	ctx := context.Background()
	email := "e2e@example.com"
	password := "Sup3rSecret!"

	// --- 1. send-code ---------------------------------------------------
	doJSON(t, ctx, cli, "POST", srv.URL+"/api/v1/auth/send-code", "",
		map[string]any{"email": email, "scene": "register"}, http.StatusOK)
	code := mailer.Get("register", email)
	if code == "" {
		t.Fatalf("mailer captured no code for %s", email)
	}

	// --- 2. register ----------------------------------------------------
	doJSON(t, ctx, cli, "POST", srv.URL+"/api/v1/auth/register", "",
		map[string]any{"email": email, "password": password, "code": code}, http.StatusOK)

	// --- 3. login -------------------------------------------------------
	loginResp := doJSON(t, ctx, cli, "POST", srv.URL+"/api/v1/auth/login", "",
		map[string]any{"email": email, "password": password}, http.StatusOK)
	data, _ := loginResp["data"].(map[string]any)
	accessTok, _ := data["access_token"].(string)
	if accessTok == "" {
		t.Fatalf("login missing access_token: %+v", loginResp)
	}

	// --- 4. /user/me — capture subscription_token -----------------------
	meResp := doJSON(t, ctx, cli, "GET", srv.URL+"/api/v1/user/me", accessTok, nil, http.StatusOK)
	meData, _ := meResp["data"].(map[string]any)
	subToken, _ := meData["subscription_token"].(string)
	if subToken == "" {
		t.Fatalf("/user/me missing subscription_token: %+v", meResp)
	}

	// --- 5. list plans (public) ----------------------------------------
	plansResp := doJSON(t, ctx, cli, "GET", srv.URL+"/api/v1/plans", "", nil, http.StatusOK)
	if plans, ok := plansResp["data"].([]any); !ok || len(plans) == 0 {
		t.Fatalf("/plans empty: %+v", plansResp)
	}

	// --- 6. create order (plan + mock pay channel) ---------------------
	orderResp := doJSON(t, ctx, cli, "POST", srv.URL+"/api/v1/orders", accessTok,
		map[string]any{"type": "plan", "target_id": 1, "pay_method": "mock"}, http.StatusOK)
	orderData, _ := orderResp["data"].(map[string]any)
	orderNo, _ := orderData["OrderNo"].(string)
	if orderNo == "" {
		t.Fatalf("create order missing OrderNo: %+v", orderResp)
	}

	// --- 7. mock-pay → plan applied ------------------------------------
	doJSON(t, ctx, cli, "POST", srv.URL+"/api/v1/orders/"+orderNo+"/mock-pay", accessTok, nil, http.StatusOK)

	// Verify the user's plan_id is now set.
	meResp = doJSON(t, ctx, cli, "GET", srv.URL+"/api/v1/user/me", accessTok, nil, http.StatusOK)
	meData, _ = meResp["data"].(map[string]any)
	if pid, ok := meData["plan_id"]; !ok || pid == nil {
		t.Fatalf("after mock-pay plan_id still nil: %+v", meData)
	}

	// --- 8. subscription -----------------------------------------------
	subURL := srv.URL + "/sub/" + subToken + "?format=v2ray"
	req, _ := http.NewRequestWithContext(ctx, "GET", subURL, nil)
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("sub status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("sub Content-Type = %q want text/plain*", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Subscription-Userinfo") == "" {
		t.Fatalf("sub missing Subscription-Userinfo header")
	}
}

// doJSON sends a JSON request (or no body if payload is nil) and parses the
// response into a generic map. It fails the test on transport errors, status
// code mismatches, or response decode errors.
func doJSON(t *testing.T, ctx context.Context, cli *http.Client, method, url, token string, payload any, wantStatus int) map[string]any {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s %s: %v", method, url, err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		t.Fatalf("req %s %s: %v", method, url, err)
	}
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
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s %s: %v (body=%s)", method, url, err, raw)
		}
	}
	if code, ok := out["code"]; ok {
		if n, ok := code.(float64); ok && n != 0 {
			t.Fatalf("%s %s -> business code=%v msg=%v", method, url, code, out["message"])
		}
	}
	return out
}
