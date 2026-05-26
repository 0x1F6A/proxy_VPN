// Package httpapi exposes SLA admin reports.
package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/sla/service"
)

type Handler struct {
	svc          *service.Service
	authRequired gin.HandlerFunc
	roleOf       func(*gin.Context) string
}

func New(svc *service.Service, auth gin.HandlerFunc, roleOf func(*gin.Context) string) *Handler {
	return &Handler{svc: svc, authRequired: auth, roleOf: roleOf}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/admin/reports").Use(h.authRequired, h.requireAdminOrOps())
	g.GET("/sla", h.sla)
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

func (h *Handler) sla(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	target := c.Query("target")
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "from must be YYYY-MM-DD"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "to must be YYYY-MM-DD"})
		return
	}
	to = to.Add(24 * time.Hour)
	summary, err := h.svc.Summary(c.Request.Context(), from, to, target)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": summary})
}
