// Package httpapi exposes the traffic context HTTP routes:
//   - POST /api/v1/nodes/usage           (bootstrap-secret auth; node-agent → API batch upload)
//   - GET  /api/v1/nodes/banlist         (bootstrap-secret auth; node-agent polls)
//   - GET  /api/v1/me/usage              (user auth; current quota snapshot)
//   - GET  /api/v1/me/usage/daily        (user auth; per-day rollup)
package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/service"
)

type ClaimsExtractor func(*gin.Context) *auth.Claims

type Handler struct {
	svc             *service.Service
	bootstrapSecret string
	authRequired    gin.HandlerFunc
	extractClaims   ClaimsExtractor
}

func New(svc *service.Service, bootstrapSecret string, authMW gin.HandlerFunc, extract ClaimsExtractor) *Handler {
	return &Handler{
		svc: svc, bootstrapSecret: bootstrapSecret,
		authRequired: authMW, extractClaims: extract,
	}
}

func (h *Handler) Register(v1 *gin.RouterGroup) {
	bs := v1.Group("/nodes")
	bs.Use(h.bootstrapAuth())
	{
		bs.POST("/usage", h.postUsage)
		bs.GET("/banlist", h.getBanlist)
	}
	me := v1.Group("/me")
	me.Use(h.authRequired)
	{
		me.GET("/usage", h.getMyUsage)
		me.GET("/usage/daily", h.getMyDaily)
	}
}

func (h *Handler) bootstrapAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.bootstrapSecret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bootstrap secret not configured"})
			return
		}
		got := c.GetHeader("X-Bootstrap-Secret")
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.bootstrapSecret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad bootstrap secret"})
			return
		}
		c.Next()
	}
}

type usageReportReq struct {
	NodeID uint64 `json:"node_id" binding:"required"`
	Items  []struct {
		SubToken  string `json:"sub_token"`
		UserID    uint64 `json:"user_id"`
		Protocol  string `json:"protocol"`
		UpBytes   uint64 `json:"up_bytes"`
		DownBytes uint64 `json:"down_bytes"`
	} `json:"items" binding:"required"`
}

func (h *Handler) postUsage(c *gin.Context) {
	var req usageReportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]service.ReportItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.ReportItem{
			SubToken: it.SubToken, UserID: it.UserID, Protocol: it.Protocol,
			UpBytes: it.UpBytes, DownBytes: it.DownBytes,
		})
	}
	accepted, rejected, err := h.svc.ReportUsage(c.Request.Context(), req.NodeID, items)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accepted": accepted, "rejected": rejected})
}

func (h *Handler) getBanlist(c *gin.Context) {
	ids, err := h.svc.CurrentBans(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_ids": ids})
}

func (h *Handler) getMyUsage(c *gin.Context) {
	cl := h.extractClaims(c)
	if cl == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	q, err := h.svc.GetMyUsage(c.Request.Context(), cl.UID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"traffic_total":  q.TrafficTotal,
		"traffic_used":   q.TrafficUsed,
		"remaining":      q.Remaining(),
		"reset_at":       q.TrafficResetAt,
		"rate_bps_up":    q.RateBpsUp,
		"rate_bps_down":  q.RateBpsDown,
		"banned":         q.Banned,
	})
}

func (h *Handler) getMyDaily(c *gin.Context) {
	cl := h.extractClaims(c)
	if cl == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	from := parseDate(c.Query("from"), time.Now().AddDate(0, 0, -30))
	to := parseDate(c.Query("to"), time.Now())
	rows, err := h.svc.GetMyDaily(c.Request.Context(), cl.UID, from, to)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"day": r.Day.Format("2006-01-02"), "up_bytes": r.UpBytes, "down_bytes": r.DownBytes,
		})
	}
	c.JSON(http.StatusOK, gin.H{"days": out})
}

func parseDate(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		// Allow Unix seconds fallback to be forgiving.
		if sec, perr := strconv.ParseInt(s, 10, 64); perr == nil {
			return time.Unix(sec, 0)
		}
		return def
	}
	return t
}
