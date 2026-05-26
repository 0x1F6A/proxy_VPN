// Package httpapi exposes risk-control admin endpoints + user self-service
// (subscription token rotate).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/risk/service"
)

type Handler struct {
	svc          *service.Service
	authRequired gin.HandlerFunc
	roleOf       func(*gin.Context) string
	uidOf        func(*gin.Context) uint64
}

func New(svc *service.Service, auth gin.HandlerFunc, roleOf func(*gin.Context) string, uidOf func(*gin.Context) uint64) *Handler {
	return &Handler{svc: svc, authRequired: auth, roleOf: roleOf, uidOf: uidOf}
}

// RegisterAdmin 挂 admin 端点（要求 admin/ops 角色）。
func (h *Handler) RegisterAdmin(r *gin.RouterGroup) {
	g := r.Group("/admin/users").Use(h.authRequired, h.requireAdminOrOps())
	g.GET("/:id/devices", h.listDevices)
	g.DELETE("/:id/devices/:fp", h.revokeDevice)
	g.POST("/:id/subscribe-token/rotate", h.adminRotate)
}

// RegisterUser 挂用户自助端点（要求登录）。
func (h *Handler) RegisterUser(r *gin.RouterGroup) {
	u := r.Group("/user").Use(h.authRequired)
	u.POST("/subscribe-token/rotate", h.userRotate)
}

func (h *Handler) requireAdminOrOps() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch h.roleOf(c) {
		case "admin", "ops":
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 4003, "message": "insufficient role"})
	}
}

func (h *Handler) listDevices(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "bad user id"})
		return
	}
	rows, err := h.svc.ListDevices(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, d := range rows {
		out = append(out, gin.H{
			"fp":            d.FPHash,
			"ip":            d.IP,
			"user_agent":    d.UserAgent,
			"country":       d.Country,
			"first_seen_at": d.FirstSeenAt,
			"last_seen_at":  d.LastSeenAt,
			"revoked":       d.IsRevoked(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

func (h *Handler) revokeDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "bad user id"})
		return
	}
	fp := c.Param("fp")
	if err := h.svc.RevokeDevice(c.Request.Context(), id, fp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (h *Handler) adminRotate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "bad user id"})
		return
	}
	tok, err := h.svc.RotateSubscriptionToken(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"subscription_token": tok}})
}

func (h *Handler) userRotate(c *gin.Context) {
	uid := h.uidOf(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": "unauthorized"})
		return
	}
	tok, err := h.svc.RotateSubscriptionToken(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"subscription_token": tok}})
}
