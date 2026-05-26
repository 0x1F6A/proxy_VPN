package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
)

func TestHealthz(t *testing.T) {
	r := NewRouter(Options{Version: "test", Logger: logger.New("error", "json")})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `"version":"test"`) {
		t.Fatalf("version not echoed in body: %s", body)
	}
}

func TestReadyz_NoChecks(t *testing.T) {
	r := NewRouter(Options{Version: "test", Logger: logger.New("error", "json")})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyz_AllPass(t *testing.T) {
	r := NewRouter(Options{
		Version: "test", Logger: logger.New("error", "json"),
		ReadinessChecks: []ReadinessCheck{
			{Name: "mysql", Check: func(context.Context) error { return nil }},
			{Name: "redis", Check: func(context.Context) error { return nil }},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `"mysql":"ok"`) || !strings.Contains(string(body), `"redis":"ok"`) {
		t.Fatalf("expected ok per check, body=%s", body)
	}
}

func TestReadyz_OneFails(t *testing.T) {
	r := NewRouter(Options{
		Version: "test", Logger: logger.New("error", "json"),
		ReadinessChecks: []ReadinessCheck{
			{Name: "mysql", Check: func(context.Context) error { return nil }},
			{Name: "redis", Check: func(context.Context) error { return errors.New("boom") }},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `"status":"not_ready"`) || !strings.Contains(string(body), "boom") {
		t.Fatalf("expected failure body, got %s", body)
	}
}

func TestMetricsEndpointExposed(t *testing.T) {
	r := NewRouter(Options{Version: "test", Logger: logger.New("error", "json")})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
