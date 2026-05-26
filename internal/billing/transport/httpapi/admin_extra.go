package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/ports"
)

// ----- coupons admin CRUD -----------------------------------------------

func (h *Handler) adminListCoupons(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, total, err := h.svc.AdminListCoupons(c.Request.Context(), q, limit, offset)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": rows}})
}

func (h *Handler) adminGetCoupon(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	row, err := h.svc.AdminGetCoupon(c.Request.Context(), id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": row})
}

func (h *Handler) adminCreateCoupon(c *gin.Context) {
	var cp domain.Coupon
	if err := c.ShouldBindJSON(&cp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.AdminCreateCoupon(c.Request.Context(), &cp); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cp})
}

func (h *Handler) adminUpdateCoupon(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cp domain.Coupon
	if err := c.ShouldBindJSON(&cp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	cp.ID = id
	if err := h.svc.AdminUpdateCoupon(c.Request.Context(), &cp); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cp})
}

func (h *Handler) adminDeleteCoupon(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminDeleteCoupon(c.Request.Context(), id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ----- orders admin list/detail -----------------------------------------

func (h *Handler) adminListOrders(c *gin.Context) {
	var f ports.OrderFilter
	f.Status = c.Query("status")
	f.Type = c.Query("type")
	f.OrderNo = c.Query("order_no")
	if v := c.Query("user_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			f.UserID = n
		}
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, total, err := h.svc.AdminListOrders(c.Request.Context(), f, limit, offset)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": rows}})
}

func (h *Handler) adminGetOrder(c *gin.Context) {
	no := c.Param("no")
	row, err := h.svc.AdminGetOrder(c.Request.Context(), no)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": row})
}
