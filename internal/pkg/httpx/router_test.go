package httpx

import (
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

func TestReadyz(t *testing.T) {
	r := NewRouter(Options{Version: "test", Logger: logger.New("error", "json")})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
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
