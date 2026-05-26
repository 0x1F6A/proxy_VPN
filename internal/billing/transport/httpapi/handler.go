// Package httpapi exposes billing-context routes: catalog (public), orders
// (user), admin CRUD for plans/data-packs.
package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/service"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

// ClaimsExtractor lets us share the existing user/transport AuthRequired
// middleware without importing the user package directly.
type ClaimsExtractor func(*gin.Context) *auth.Claims

type Handler struct {
	svc           *service.Service
	authRequired  gin.HandlerFunc
	extractClaims ClaimsExtractor
}

func New(svc *service.Service, authMW gin.HandlerFunc, extract ClaimsExtractor) *Handler {
	return &Handler{svc: svc, authRequired: authMW, extractClaims: extract}
}

// Register mounts billing routes onto v1. Public list endpoints + authed
// order endpoints + admin-role-gated catalog management.
func (h *Handler) Register(v1 *gin.RouterGroup) {
	// public catalog
	v1.GET("/plans", h.listPlans)
	v1.GET("/plans/:id", h.getPlan)
	v1.GET("/data-packs", h.listPacks)
	v1.GET("/data-packs/:id", h.getPack)
	v1.POST("/coupons/quote", h.quoteCoupon) // body: code,amount,type — no auth so users can preview anonymously

	// authed orders
	authed := v1.Group("")
	authed.Use(h.authRequired)
	{
		authed.POST("/orders", h.createOrder)
		authed.GET("/orders", h.listMyOrders)
		authed.GET("/orders/:no", h.getOrder)
		authed.POST("/orders/:no/cancel", h.cancelOrder)
		authed.POST("/orders/:no/mock-pay", h.mockPay) // dev/test only
	}

	// admin catalog management
	admin := v1.Group("/admin")
	admin.Use(h.authRequired, h.requireAdmin())
	{
		admin.POST("/plans", h.adminCreatePlan)
		admin.PUT("/plans/:id", h.adminUpdatePlan)
		admin.DELETE("/plans/:id", h.adminDeletePlan)

		admin.POST("/data-packs", h.adminCreatePack)
		admin.PUT("/data-packs/:id", h.adminUpdatePack)
		admin.DELETE("/data-packs/:id", h.adminDeletePack)
	}
}

func (h *Handler) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := h.extractClaims(c)
		if cl == nil || cl.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 4003, "message": "admin role required"})
			return
		}
		c.Next()
	}
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrPlanNotFound),
		errors.Is(err, domain.ErrPackNotFound),
		errors.Is(err, domain.ErrCouponNotFound),
		errors.Is(err, domain.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": err.Error()})
	case errors.Is(err, domain.ErrPlanInactive),
		errors.Is(err, domain.ErrPackInactive),
		errors.Is(err, domain.ErrCouponExpired),
		errors.Is(err, domain.ErrCouponExhausted),
		errors.Is(err, domain.ErrCouponNotMet),
		errors.Is(err, domain.ErrCouponWrongScope),
		errors.Is(err, domain.ErrInvalidType),
		errors.Is(err, domain.ErrOrderNotPayable):
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
	case errors.Is(err, domain.ErrOrderConflict):
		c.JSON(http.StatusConflict, gin.H{"code": 4009, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}

// ----- catalog ---------------------------------------------------------

func (h *Handler) listPlans(c *gin.Context) {
	onlyActive := c.DefaultQuery("active", "true") == "true"
	rows, err := h.svc.ListPlans(c.Request.Context(), onlyActive)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}
func (h *Handler) getPlan(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p, err := h.svc.GetPlan(c.Request.Context(), id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}
func (h *Handler) listPacks(c *gin.Context) {
	onlyActive := c.DefaultQuery("active", "true") == "true"
	rows, err := h.svc.ListPacks(c.Request.Context(), onlyActive)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}
func (h *Handler) getPack(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	p, err := h.svc.GetPack(c.Request.Context(), id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

type quoteCouponReq struct {
	Code   string `json:"code" binding:"required"`
	Amount string `json:"amount" binding:"required"`
	Type   string `json:"type" binding:"required,oneof=plan pack topup"`
}

func (h *Handler) quoteCoupon(c *gin.Context) {
	var req quoteCouponReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	var uid uint64
	if cl := h.extractClaims(c); cl != nil {
		uid = cl.UID
	}
	disc, final, err := h.svc.QuoteCoupon(c.Request.Context(), req.Code, req.Amount, req.Type, uid)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"discount": disc, "final": final, "original": req.Amount,
	}})
}

// ----- orders ----------------------------------------------------------

type createOrderReq struct {
	Type        string `json:"type" binding:"required,oneof=plan pack topup"`
	TargetID    uint64 `json:"target_id"`
	TopupAmount string `json:"topup_amount"`
	CouponCode  string `json:"coupon_code"`
	PayMethod   string `json:"pay_method" binding:"required,oneof=alipay wechat usdt_trc20 balance mock"`
}

func (h *Handler) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	cl := h.extractClaims(c)
	o, err := h.svc.CreateOrder(c.Request.Context(), service.CreateOrderInput{
		UserID:         cl.UID,
		Type:           req.Type,
		TargetID:       req.TargetID,
		TopupAmount:    req.TopupAmount,
		CouponCode:     req.CouponCode,
		PayMethod:      req.PayMethod,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		ClientIP:       c.ClientIP(),
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": o})
}

func (h *Handler) getOrder(c *gin.Context) {
	cl := h.extractClaims(c)
	o, err := h.svc.GetOrder(c.Request.Context(), c.Param("no"), cl.UID)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": o})
}

func (h *Handler) listMyOrders(c *gin.Context) {
	cl := h.extractClaims(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := h.svc.ListMyOrders(c.Request.Context(), cl.UID, limit, offset)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

func (h *Handler) cancelOrder(c *gin.Context) {
	cl := h.extractClaims(c)
	if err := h.svc.CancelOrder(c.Request.Context(), c.Param("no"), cl.UID); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *Handler) mockPay(c *gin.Context) {
	cl := h.extractClaims(c)
	if _, err := h.svc.GetOrder(c.Request.Context(), c.Param("no"), cl.UID); err != nil {
		mapErr(c, err)
		return
	}
	if err := h.svc.MockPay(c.Request.Context(), c.Param("no")); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ----- admin -----------------------------------------------------------

func (h *Handler) adminCreatePlan(c *gin.Context) {
	var p domain.Plan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.CreatePlan(c.Request.Context(), &p); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}
func (h *Handler) adminUpdatePlan(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p domain.Plan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	p.ID = id
	if err := h.svc.UpdatePlan(c.Request.Context(), &p); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}
func (h *Handler) adminDeletePlan(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeletePlan(c.Request.Context(), id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *Handler) adminCreatePack(c *gin.Context) {
	var p domain.DataPack
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.CreatePack(c.Request.Context(), &p); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}
func (h *Handler) adminUpdatePack(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p domain.DataPack
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	p.ID = id
	if err := h.svc.UpdatePack(c.Request.Context(), &p); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}
func (h *Handler) adminDeletePack(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeletePack(c.Request.Context(), id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
