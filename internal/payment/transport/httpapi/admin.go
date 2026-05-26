package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

func (h *Handler) adminListPayments(c *gin.Context) {
	var f ports.PaymentFilter
	if v := c.Query("status"); v != "" {
		f.Status = domain.Status(v)
	}
	if v := c.Query("channel"); v != "" {
		f.Channel = domain.Channel(v)
	}
	f.OrderNo = c.Query("order_no")
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
	rows, total, err := h.svc.AdminListPayments(c.Request.Context(), f, limit, offset)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"total": total, "items": rows}})
}

func (h *Handler) adminGetPayment(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	row, err := h.svc.AdminGetPayment(c.Request.Context(), id)
	if err != nil {
		mapErr(c, err)
		return
	}
	if row == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": "payment not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": row})
}
