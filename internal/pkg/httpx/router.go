// Package httpx wires the base HTTP router with health, readiness, metrics
// and minimal middlewares. Business handlers should be registered on top of
// the returned router by their respective service packages.
package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
)

// ReadinessCheck describes a single dependency probed by /readyz. Name is
// reported in the response so operators can pinpoint failures.
type ReadinessCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

type Options struct {
	Version          string
	Logger           *logger.Logger
	ReadinessChecks  []ReadinessCheck
	ReadinessTimeout time.Duration
}

// NewRouter returns a gin Engine pre-configured with recovery, request ID,
// structured access logging, /healthz, /readyz and /metrics.
func NewRouter(opt Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(requestid.New())
	r.Use(accessLog(opt.Logger))
	r.Use(Metrics())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": opt.Version,
		})
	})

	timeout := opt.ReadinessTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	r.GET("/readyz", func(c *gin.Context) {
		results := make(map[string]string, len(opt.ReadinessChecks))
		status := http.StatusOK
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		for _, chk := range opt.ReadinessChecks {
			if err := chk.Check(ctx); err != nil {
				results[chk.Name] = "fail: " + err.Error()
				status = http.StatusServiceUnavailable
				continue
			}
			results[chk.Name] = "ok"
		}
		body := gin.H{"status": "ready", "checks": results}
		if status != http.StatusOK {
			body["status"] = "not_ready"
		}
		c.JSON(status, body)
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r
}

func accessLog(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		log.Info("http_access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"client_ip", c.ClientIP(),
			"request_id", requestid.Get(c),
		)
	}
}
