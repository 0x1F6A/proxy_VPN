package httpx

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
)

func TestMetrics_RecordsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Options{Version: "test", Logger: logger.New("info", "json")})
	r.GET("/api/v1/probe", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/probe", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}

	// scrape /metrics and assert the request shows up
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w2, req2)
	body, _ := io.ReadAll(w2.Body)
	if !strings.Contains(string(body), `http_requests_total{method="GET",route="/api/v1/probe",status="200"}`) {
		t.Fatalf("metrics missing request counter, body:\n%s", string(body))
	}
}
