// Package audit provides admin-action audit logging. The middleware records
// every admin POST/PUT/DELETE/PATCH call into the `admin_audit_logs` table
// via the configured Writer. Reads (GET/HEAD) are intentionally not logged
// to keep table volume manageable.
//
// Audit writes are best-effort and never block the response: failures are
// captured by the Writer implementation (e.g. logged) but never propagated
// to the gin handler chain.
package audit

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

// Record is a single admin action to be persisted.
type Record struct {
	AdminID   uint64
	Action    string // e.g. "POST /api/v1/admin/users/:id/ban"
	Target    string // best-effort: action target (path param value or path)
	Payload   string // truncated request body
	IP        string
	UserAgent string
	CreatedAt time.Time
}

// Writer persists Records. Implementations should be non-blocking or use
// their own goroutine pool — Middleware calls Write in a fresh goroutine
// so a slow Writer will not impact request latency.
type Writer interface {
	Write(ctx context.Context, rec Record)
}

// ClaimsExtractor fetches the auth.Claims for the current request. Pass the
// same extractor used by your admin RequireAuth middleware.
type ClaimsExtractor func(*gin.Context) *auth.Claims

const maxPayloadBytes = 4 << 10 // 4 KB

// Middleware returns a gin.HandlerFunc that audits POST/PUT/DELETE/PATCH
// requests under the group it is mounted on. GET/HEAD/OPTIONS are skipped.
//
// The middleware reads the request body, truncates it to 4 KB, and writes
// the audit record after the downstream handler returns successfully (4xx
// / 5xx responses are not audited so failed attempts don't pollute the log
// — adjust if you also want to record refused actions).
func Middleware(w Writer, extract ClaimsExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
		m := c.Request.Method
		if m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions {
			c.Next()
			return
		}
		var body []byte
		if c.Request.Body != nil {
			limited := io.LimitReader(c.Request.Body, maxPayloadBytes+1)
			b, _ := io.ReadAll(limited)
			body = b
			c.Request.Body = io.NopCloser(bytes.NewReader(b))
		}
		c.Next()
		if c.Writer.Status() >= 400 {
			return
		}
		cl := extract(c)
		var adminID uint64
		if cl != nil {
			adminID = cl.UID
		}
		target := c.Param("id")
		if target == "" {
			target = c.Param("no")
		}
		if target == "" {
			target = c.FullPath()
		}
		payload := string(body)
		if len(payload) > maxPayloadBytes {
			payload = payload[:maxPayloadBytes] + "...(truncated)"
		}
		rec := Record{
			AdminID:   adminID,
			Action:    m + " " + c.FullPath(),
			Target:    strings.TrimSpace(target),
			Payload:   payload,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			CreatedAt: time.Now(),
		}
		go w.Write(context.Background(), rec)
	}
}
