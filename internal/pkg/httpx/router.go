// Package httpx wires the base HTTP router with health, readiness, metrics
// and minimal middlewares. Business handlers should be registered on top of
// the returned router by their respective service packages.
package httpx

import (
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
)

type Options struct {
	Version string
	Logger  *logger.Logger
}

// NewRouter returns a gin Engine pre-configured with recovery, request ID,
// structured access logging, /healthz, /readyz and /metrics.
func NewRouter(opt Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(requestid.New())
	r.Use(accessLog(opt.Logger))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": opt.Version,
		})
	})

	// readyz is intentionally minimal in Phase 0 — Phase 1 wires DB/Redis pings.
	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
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
