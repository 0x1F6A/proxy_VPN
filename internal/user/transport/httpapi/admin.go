package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) adminListUsers(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, total, err := h.svc.AdminListUsers(c.Request.Context(), q, limit, offset)
	if err != nil {
		mapErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":             r.ID,
			"email":          r.Email,
			"status":         r.Status,
			"role":           r.Role,
			"plan_id":        r.PlanID,
			"plan_expire_at": r.PlanExpireAt,
			"traffic_total":  r.TrafficTotal,
			"traffic_used":   r.TrafficUsed,
			"rate_bps_up":    r.RateBpsUp,
			"rate_bps_down":  r.RateBpsDown,
			"banned":         r.Banned,
			"created_at":     r.CreatedAt,
			"last_login_at":  r.LastLoginAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": out}})
}

func (h *Handler) adminSummary(c *gin.Context) {
	cnt, err := h.svc.AdminOverallCounts(c.Request.Context())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"total_users":   cnt.TotalUsers,
		"active_users":  cnt.ActiveUsers,
		"banned_users":  cnt.BannedUsers,
		"active_plans":  cnt.ActivePlans,
	}})
}

func (h *Handler) adminBanUser(c *gin.Context)   { h.adminSetBan(c, true) }
func (h *Handler) adminUnbanUser(c *gin.Context) { h.adminSetBan(c, false) }

func (h *Handler) adminSetBan(c *gin.Context, banned bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": "bad id"})
		return
	}
	if err := h.svc.AdminSetBanned(c.Request.Context(), id, banned); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

type adjustTrafficReq struct {
	DeltaBytes int64 `json:"delta_bytes" binding:"required"`
}

func (h *Handler) adminAdjustTraffic(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": "bad id"})
		return
	}
	var req adjustTrafficReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.AdminAdjustTraffic(c.Request.Context(), id, req.DeltaBytes); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

type setRateReq struct {
	UpBps   uint64 `json:"up_bps"`
	DownBps uint64 `json:"down_bps"`
}

func (h *Handler) adminSetRate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": "bad id"})
		return
	}
	var req setRateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.AdminSetRateLimits(c.Request.Context(), id, req.UpBps, req.DownBps); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
