// Package httpapi exposes payment-context HTTP routes: webhook receivers
// for each channel (root-level, not under /api/v1, so channel callbacks
// hit a short stable URL), authed pay-an-order endpoint, and an authed
// payment-status lookup.
package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/service"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

type ClaimsExtractor func(*gin.Context) *auth.Claims

type Handler struct {
	svc           *service.Service
	authRequired  gin.HandlerFunc
	extractClaims ClaimsExtractor
}

func New(svc *service.Service, authMW gin.HandlerFunc, extract ClaimsExtractor) *Handler {
	return &Handler{svc: svc, authRequired: authMW, extractClaims: extract}
}

// Register mounts authed payment routes onto v1 and the channel webhook
// routes onto the root engine (so URLs are e.g. /pay/notify/alipay).
func (h *Handler) Register(v1 *gin.RouterGroup, root *gin.Engine) {
	authed := v1.Group("")
	authed.Use(h.authRequired)
	{
		authed.POST("/orders/:no/pay", h.pay)
		authed.GET("/payments/:id", h.getPayment)
	}
	root.POST("/pay/notify/:channel", h.notify)
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrUnknownTradeNo):
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": err.Error()})
	case errors.Is(err, domain.ErrChannelUnsupported),
		errors.Is(err, domain.ErrAmountMismatch),
		errors.Is(err, domain.ErrPaymentExpired),
		errors.Is(err, domain.ErrNoAddressAvailable):
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
	case errors.Is(err, domain.ErrSignatureInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": err.Error()})
	case errors.Is(err, domain.ErrProviderUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 5003, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}

type payReq struct {
	Channel string `json:"channel" binding:"required,oneof=alipay wechat usdt_trc20 mock"`
}

func (h *Handler) pay(c *gin.Context) {
	var req payReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	p, err := h.svc.CreatePayment(c.Request.Context(), c.Param("no"), domain.Channel(req.Channel), c.ClientIP())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id":           p.ID,
		"order_no":     p.OrderNo,
		"channel":      p.Channel,
		"qr_or_url":    p.QRorURL,
		"address":      p.Address,
		"amount_cny":   p.AmountCNY,
		"amount_token": p.AmountToken,
		"expired_at":   p.ExpiredAt,
		"status":       p.Status,
	}})
}

func (h *Handler) getPayment(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	cl := h.extractClaims(c)
	p, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		mapErr(c, err)
		return
	}
	if cl == nil || p.UserID != cl.UID {
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// notify is the channel webhook entry-point. Alipay/Wechat require
// channel-specific response bodies on success / failure to stop the
// gateway from retrying.
func (h *Handler) notify(c *gin.Context) {
	channel := domain.Channel(c.Param("channel"))
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	headers := map[string]string{}
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	err := h.svc.HandleNotify(c.Request.Context(), channel, headers, body)
	if err != nil {
		switch channel {
		case domain.ChannelAlipay:
			c.String(http.StatusOK, "fail")
		case domain.ChannelWechat:
			c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		}
		return
	}
	switch channel {
	case domain.ChannelAlipay:
		c.String(http.StatusOK, "success")
	case domain.ChannelWechat:
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS"})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
	}
}
