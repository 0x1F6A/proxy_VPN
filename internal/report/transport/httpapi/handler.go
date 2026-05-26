// Package httpapi exposes admin-only report endpoints.
//
// All routes require an authenticated request whose JWT carries an admin or
// ops role. Authentication is provided by the caller as a Gin middleware
// (`authRequired`) and a claims accessor (`roleOf`) so this package does
// not import the user bounded context.
package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/report/service"
)

type Handler struct {
	svc          *service.Service
	authRequired gin.HandlerFunc
	roleOf       func(*gin.Context) string
}

func New(svc *service.Service, authRequired gin.HandlerFunc, roleOf func(*gin.Context) string) *Handler {
	return &Handler{svc: svc, authRequired: authRequired, roleOf: roleOf}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/admin").Use(h.authRequired, h.requireAdminOrOps())
	{
		g.GET("/reports/revenue", h.revenue)
		g.GET("/reports/traffic", h.traffic)
		g.GET("/reports/orders", h.orders)
		g.GET("/dashboard", h.dashboard)
	}
}

func (h *Handler) requireAdminOrOps() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := h.roleOf(c)
		if role != "admin" && role != "ops" && role != "finance" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 4003, "message": "insufficient role"})
			return
		}
		c.Next()
	}
}

// parseRange reads ?from=YYYY-MM-DD&to=YYYY-MM-DD; defaults to last 30 days
// when either bound is missing.
func parseRange(c *gin.Context) (time.Time, time.Time, error) {
	fromS := c.Query("from")
	toS := c.Query("to")
	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	from := to.Add(-30 * 24 * time.Hour)
	var err error
	if fromS != "" {
		from, err = time.Parse("2006-01-02", fromS)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid from")
		}
	}
	if toS != "" {
		to, err = time.Parse("2006-01-02", toS)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid to")
		}
		to = to.Add(24 * time.Hour) // make 'to' exclusive end of day
	}
	return from, to, nil
}

func (h *Handler) revenue(c *gin.Context) {
	from, to, err := parseRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	pts, err := h.svc.RevenueDaily(c.Request.Context(), from, to)
	if err != nil {
		mapErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(pts))
	for _, p := range pts {
		items = append(items, gin.H{
			"day":            p.Day.Format("2006-01-02"),
			"order_count":    p.OrderCnt,
			"paid_cents":     p.PaidCents,
			"paid_cny":       strconv.FormatFloat(float64(p.PaidCents)/100.0, 'f', 2, 64),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) traffic(c *gin.Context) {
	from, to, err := parseRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	pts, err := h.svc.TrafficDaily(c.Request.Context(), from, to)
	if err != nil {
		mapErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(pts))
	for _, p := range pts {
		items = append(items, gin.H{
			"day":        p.Day.Format("2006-01-02"),
			"up_bytes":   p.UpBytes,
			"down_bytes": p.DownBytes,
			"users":      p.Users,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) orders(c *gin.Context) {
	from, to, err := parseRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	pts, err := h.svc.OrderStatusCounts(c.Request.Context(), from, to)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": pts})
}

func (h *Handler) dashboard(c *gin.Context) {
	snap, err := h.svc.Dashboard(c.Request.Context(), time.Now().UTC())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":  snap.UsersTotal,
			"active": snap.UsersActive,
			"banned": snap.UsersBanned,
		},
		"today": gin.H{
			"orders":      snap.OrdersToday,
			"revenue_cny": snap.RevenueTodayCNY,
			"up_bytes":    snap.TrafficTodayUp,
			"down_bytes":  snap.TrafficTodayDown,
		},
	})
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRangeInvalid),
		errors.Is(err, service.ErrRangeTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}
