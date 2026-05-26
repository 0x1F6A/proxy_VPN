package audit

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

type memWriter struct {
	mu   sync.Mutex
	recs []Record
}

func (m *memWriter) Write(_ context.Context, r Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recs = append(m.recs, r)
}

func (m *memWriter) snapshot() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, len(m.recs))
	copy(out, m.recs)
	return out
}

func TestMiddleware_SkipsReads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &memWriter{}
	r := gin.New()
	extract := func(c *gin.Context) *auth.Claims { return &auth.Claims{UID: 7, Role: "admin"} }
	r.Use(Middleware(w, extract))
	r.GET("/admin/x", func(c *gin.Context) { c.String(200, "ok") })

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if len(w.snapshot()) != 0 {
		t.Fatalf("GET should not be audited, got %d recs", len(w.snapshot()))
	}
}

func TestMiddleware_WritesPostRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &memWriter{}
	done := make(chan struct{})
	wrapped := &waitWriter{inner: w, done: done}
	r := gin.New()
	extract := func(c *gin.Context) *auth.Claims { return &auth.Claims{UID: 42, Role: "admin"} }
	r.Use(Middleware(wrapped, extract))
	r.POST("/admin/users/:id/ban", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	body := []byte(`{"reason":"abuse"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/99/ban", bytes.NewReader(body))
	req.Header.Set("User-Agent", "ut")
	r.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	<-done // wait for async write
	recs := w.snapshot()
	if len(recs) != 1 {
		t.Fatalf("want 1 audit rec, got %d", len(recs))
	}
	got := recs[0]
	if got.AdminID != 42 {
		t.Errorf("admin id: want 42, got %d", got.AdminID)
	}
	if got.Action != "POST /admin/users/:id/ban" {
		t.Errorf("action: %q", got.Action)
	}
	if got.Target != "99" {
		t.Errorf("target: want 99, got %q", got.Target)
	}
	if string(got.Payload) != `{"reason":"abuse"}` {
		t.Errorf("payload: %q", got.Payload)
	}
	if got.UserAgent != "ut" {
		t.Errorf("ua: %q", got.UserAgent)
	}
}

func TestMiddleware_SkipsOn4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &memWriter{}
	r := gin.New()
	extract := func(c *gin.Context) *auth.Claims { return &auth.Claims{UID: 1, Role: "admin"} }
	r.Use(Middleware(w, extract))
	r.POST("/admin/x", func(c *gin.Context) { c.JSON(400, gin.H{"err": "bad"}) })

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/x", nil)
	r.ServeHTTP(rr, req)

	if len(w.snapshot()) != 0 {
		t.Fatalf("4xx should not be audited, got %d", len(w.snapshot()))
	}
}

// waitWriter signals on done after the inner writer's first call so tests
// can synchronise against the audit goroutine.
type waitWriter struct {
	inner *memWriter
	once  sync.Once
	done  chan struct{}
}

func (w *waitWriter) Write(ctx context.Context, r Record) {
	w.inner.Write(ctx, r)
	w.once.Do(func() { close(w.done) })
}
