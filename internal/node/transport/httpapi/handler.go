// Package httpapi exposes node-context routes: admin CRUD, user-facing
// node list, node-agent register/heartbeat, and the public subscription
// endpoint.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
	"github.com/0x1F6A/proxy_VPN/internal/node/service"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

type ClaimsExtractor func(*gin.Context) *auth.Claims

type Handler struct {
	svc           *service.Service
	authRequired  gin.HandlerFunc
	extractClaims ClaimsExtractor
	// userPlanLoader returns the user's current plan_id (nil if no plan).
	userPlanLoader func(c *gin.Context, uid uint64) (*uint64, error)
}

func New(svc *service.Service, authMW gin.HandlerFunc, extract ClaimsExtractor,
	planLoader func(c *gin.Context, uid uint64) (*uint64, error)) *Handler {
	return &Handler{svc: svc, authRequired: authMW, extractClaims: extract, userPlanLoader: planLoader}
}

func (h *Handler) Register(v1 *gin.RouterGroup, root *gin.Engine) {
	// authed user list
	authed := v1.Group("/nodes")
	authed.Use(h.authRequired)
	authed.GET("", h.listForUser)

	// admin CRUD
	admin := v1.Group("/admin")
	admin.Use(h.authRequired, h.requireAdmin())
	{
		admin.GET("/node-groups", h.adminListGroups)
		admin.POST("/node-groups", h.adminCreateGroup)
		admin.PUT("/node-groups/:id", h.adminUpdateGroup)
		admin.DELETE("/node-groups/:id", h.adminDeleteGroup)

		admin.GET("/nodes", h.adminListNodes)
		admin.POST("/nodes", h.adminCreateNode) // returns bootstrap token (one-shot)
		admin.PUT("/nodes/:id", h.adminUpdateNode)
		admin.DELETE("/nodes/:id", h.adminDeleteNode)
	}

	// node-agent (no JWT; uses bootstrap + node token)
	agent := v1.Group("/node-agent")
	{
		agent.POST("/register", h.agentRegister)
		agent.POST("/heartbeat", h.agentHeartbeat)
		agent.POST("/config", h.agentConfig)
	}

	// public subscription, mounted on root (no /api/v1) so QR codes are short.
	root.GET("/sub/:token", h.subscription)
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
	case errors.Is(err, domain.ErrNodeNotFound),
		errors.Is(err, domain.ErrNodeGroupNotFound),
		errors.Is(err, domain.ErrSubTokenInvalid):
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": err.Error()})
	case errors.Is(err, domain.ErrBootstrapForbidden),
		errors.Is(err, domain.ErrNodeAuth):
		c.JSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": err.Error()})
	case errors.Is(err, domain.ErrUnsupportedProto),
		errors.Is(err, domain.ErrSubFormatUnknown),
		errors.Is(err, domain.ErrNoPlanGranted):
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}

// ----- user-facing -----------------------------------------------------

func (h *Handler) listForUser(c *gin.Context) {
	cl := h.extractClaims(c)
	planID, err := h.userPlanLoader(c, cl.UID)
	if err != nil {
		mapErr(c, err)
		return
	}
	rows, err := h.svc.ListForUser(c.Request.Context(), planID)
	if err != nil {
		mapErr(c, err)
		return
	}
	// strip sensitive fields before returning to user
	out := make([]gin.H, 0, len(rows))
	for _, n := range rows {
		out = append(out, gin.H{
			"id": n.ID, "name": n.Name, "region": n.Region, "tags": n.Tags,
			"protocol": n.Protocol, "online": n.Online,
			"rate_multiplier": n.RateMultiplier, "sort": n.Sort,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// ----- admin: node groups ---------------------------------------------

func (h *Handler) adminListGroups(c *gin.Context) {
	rows, err := h.svc.ListGroups(c.Request.Context())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}
func (h *Handler) adminCreateGroup(c *gin.Context) {
	var g domain.NodeGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.CreateGroup(c.Request.Context(), &g); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": g})
}
func (h *Handler) adminUpdateGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var g domain.NodeGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	g.ID = id
	if err := h.svc.UpdateGroup(c.Request.Context(), &g); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": g})
}
func (h *Handler) adminDeleteGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteGroup(c.Request.Context(), id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

// ----- admin: nodes ---------------------------------------------------

func (h *Handler) adminListNodes(c *gin.Context) {
	rows, err := h.svc.ListNodes(c.Request.Context(), false)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": rows})
}

type createNodeReq struct {
	Name            string          `json:"name" binding:"required"`
	Region          string          `json:"region" binding:"required"`
	Tags            string          `json:"tags"`
	NodeGroupID     uint64          `json:"node_group_id" binding:"required"`
	Protocol        string          `json:"protocol" binding:"required"`
	Address         string          `json:"address" binding:"required"`
	Port            uint32          `json:"port" binding:"required"`
	TLSConfig       json.RawMessage `json:"tls_config"`
	Transport       string          `json:"transport"`
	TransportConfig json.RawMessage `json:"transport_config"`
	RateMultiplier  string          `json:"rate_multiplier"`
	Sort            int             `json:"sort"`
	Status          int8            `json:"status"`
}

func (h *Handler) adminCreateNode(c *gin.Context) {
	var req createNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	n := &domain.Node{
		Name: req.Name, Region: req.Region, Tags: req.Tags,
		NodeGroupID: req.NodeGroupID, Protocol: req.Protocol,
		Address: req.Address, Port: req.Port,
		TLSConfig: req.TLSConfig, Transport: req.Transport, TransportConfig: req.TransportConfig,
		RateMultiplier: req.RateMultiplier, Sort: req.Sort, Status: req.Status,
	}
	if n.Status == 0 {
		n.Status = domain.NodeStatusEnabled
	}
	token, err := h.svc.IssueBootstrapToken(c.Request.Context(), n)
	if err != nil {
		mapErr(c, err)
		return
	}
	// One-shot reveal of bootstrap token.
	c.JSON(200, gin.H{"code": 0, "data": gin.H{
		"node":            n,
		"bootstrap_token": token,
	}})
}

func (h *Handler) adminUpdateNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req createNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	n := &domain.Node{
		ID: id, Name: req.Name, Region: req.Region, Tags: req.Tags,
		NodeGroupID: req.NodeGroupID, Protocol: req.Protocol,
		Address: req.Address, Port: req.Port,
		TLSConfig: req.TLSConfig, Transport: req.Transport, TransportConfig: req.TransportConfig,
		RateMultiplier: req.RateMultiplier, Sort: req.Sort, Status: req.Status,
	}
	if err := h.svc.UpdateNode(c.Request.Context(), n); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": n})
}

func (h *Handler) adminDeleteNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteNode(c.Request.Context(), id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

// ----- node-agent -----------------------------------------------------

type registerReq struct {
	Bootstrap string `json:"bootstrap" binding:"required"`
	NodeToken string `json:"node_token" binding:"required"`
}

func (h *Handler) agentRegister(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	n, err := h.svc.AgentRegister(c.Request.Context(), req.Bootstrap, req.NodeToken)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{
		"node_id":   n.ID,
		"name":      n.Name,
		"region":    n.Region,
		"protocol":  n.Protocol,
		"address":   n.Address,
		"port":      n.Port,
	}})
}

type heartbeatReq struct {
	NodeToken       string `json:"node_token" binding:"required"`
	CPUPercent      string `json:"cpu_percent"`
	MemPercent      string `json:"mem_percent"`
	BandwidthInBps  uint64 `json:"bandwidth_in_bps"`
	BandwidthOutBps uint64 `json:"bandwidth_out_bps"`
	OnlineUsers     uint32 `json:"online_users"`
}

func (h *Handler) agentHeartbeat(c *gin.Context) {
	var req heartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	_, err := h.svc.AgentHeartbeat(c.Request.Context(), req.NodeToken, ports.Heartbeat{
		CPUPercent: req.CPUPercent, MemPercent: req.MemPercent,
		BandwidthInBps: req.BandwidthInBps, BandwidthOutBps: req.BandwidthOutBps,
		OnlineUsers: req.OnlineUsers,
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

type agentConfigReq struct {
	NodeToken string `json:"node_token" binding:"required"`
	KnownHash string `json:"known_hash"`
}

func (h *Handler) agentConfig(c *gin.Context) {
	var req agentConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	r, err := h.svc.AgentConfig(c.Request.Context(), req.NodeToken)
	if err != nil {
		mapErr(c, err)
		return
	}
	if req.KnownHash != "" && req.KnownHash == r.Version {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"version": r.Version, "unchanged": true}})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{
		"version": r.Version,
		"format":  r.Format,
		"config":  r.Config,
	}})
}

// ----- subscription ---------------------------------------------------

func (h *Handler) subscription(c *gin.Context) {
	token := c.Param("token")
	format := c.DefaultQuery("format", "v2ray")
	body, ct, err := h.svc.Subscription(c.Request.Context(), token, format)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.Header("Content-Type", ct)
	// Subscription user-info header (used by Clash / Surge / Shadowrocket).
	c.Header("Subscription-Userinfo", "upload=0; download=0; total=0; expire=0")
	c.Header("Profile-Update-Interval", "24")
	c.Data(http.StatusOK, ct, body)
}
