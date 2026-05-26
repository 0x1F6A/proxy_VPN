// Package httpapi exposes ticket endpoints for both user self-service and
// admin back-office.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/ticket/domain"
	"github.com/0x1F6A/proxy_VPN/internal/ticket/ports"
	"github.com/0x1F6A/proxy_VPN/internal/ticket/service"
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

func (h *Handler) isAdmin(c *gin.Context) bool {
	switch h.roleOf(c) {
	case "admin", "ops":
		return true
	}
	return false
}

func (h *Handler) Register(r *gin.RouterGroup) {
	u := r.Group("/tickets").Use(h.authRequired)
	u.POST("", h.create)
	u.GET("", h.listMine)
	u.GET("/:id", h.detail)
	u.POST("/:id/messages", h.userReply)
	u.POST("/:id/close", h.close)

	a := r.Group("/admin/tickets").Use(h.authRequired, h.requireAdmin())
	a.GET("", h.adminList)
	a.POST("/:id/assign", h.adminAssign)
	a.POST("/:id/reply", h.adminReply)
	a.PATCH("/:id", h.adminPatch)
}

func (h *Handler) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.isAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 4003, "message": "insufficient role"})
			return
		}
		c.Next()
	}
}

type createReq struct {
	Subject     string   `json:"subject" binding:"required,max=200"`
	Category    string   `json:"category"`
	Priority    string   `json:"priority"`
	Body        string   `json:"body" binding:"required"`
	Attachments []string `json:"attachments"`
}

func (h *Handler) create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	t, err := h.svc.Create(c.Request.Context(), service.CreateInput{
		UserID:      h.uidOf(c),
		Subject:     req.Subject,
		Category:    req.Category,
		Priority:    domain.Priority(req.Priority),
		Body:        req.Body,
		Attachments: req.Attachments,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ticketView(t)})
}

func (h *Handler) listMine(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, total, err := h.svc.ListForUser(c.Request.Context(), h.uidOf(c), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"items": ticketViews(rows), "total": total}})
}

func (h *Handler) detail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, msgs, err := h.svc.Get(c.Request.Context(), id, h.uidOf(c), h.isAdmin(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"ticket":   ticketView(t),
		"messages": messageViews(msgs),
	}})
}

type replyReq struct {
	Body        string   `json:"body" binding:"required"`
	Attachments []string `json:"attachments"`
}

func (h *Handler) userReply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req replyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	_, m, err := h.svc.Reply(c.Request.Context(), service.ReplyInput{
		TicketID:    id,
		SenderID:    h.uidOf(c),
		SenderType:  domain.SenderUser,
		Body:        req.Body,
		Attachments: req.Attachments,
	}, false)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": messageView(m)})
}

func (h *Handler) close(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Close(c.Request.Context(), id, h.uidOf(c), h.isAdmin(c)); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (h *Handler) adminList(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	assignee, _ := strconv.ParseUint(c.DefaultQuery("assignee_id", "0"), 10, 64)
	rows, total, err := h.svc.ListForAdmin(c.Request.Context(), ports.ListFilter{
		Status:     c.Query("status"),
		Priority:   c.Query("priority"),
		AssigneeID: assignee,
		Keyword:    c.Query("q"),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"items": ticketViews(rows), "total": total}})
}

type assignReq struct {
	AssigneeID uint64 `json:"assignee_id" binding:"required"`
}

func (h *Handler) adminAssign(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req assignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	t, err := h.svc.Assign(c.Request.Context(), id, req.AssigneeID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ticketView(t)})
}

func (h *Handler) adminReply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req replyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	_, m, err := h.svc.Reply(c.Request.Context(), service.ReplyInput{
		TicketID:    id,
		SenderID:    h.uidOf(c),
		SenderType:  domain.SenderAdmin,
		Body:        req.Body,
		Attachments: req.Attachments,
	}, true)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": messageView(m)})
}

type patchReq struct {
	Status     *string `json:"status"`
	Priority   *string `json:"priority"`
	AssigneeID *uint64 `json:"assignee_id"`
}

func (h *Handler) adminPatch(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req patchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	in := service.PatchInput{AssigneeID: req.AssigneeID}
	if req.Status != nil {
		s := domain.Status(*req.Status)
		in.Status = &s
	}
	if req.Priority != nil {
		p := domain.Priority(*req.Priority)
		in.Priority = &p
	}
	t, err := h.svc.AdminPatch(c.Request.Context(), id, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ticketView(t)})
}

func writeErr(c *gin.Context, err error) {
	switch err {
	case domain.ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": err.Error()})
	case domain.ErrForbidden:
		c.JSON(http.StatusForbidden, gin.H{"code": 4003, "message": err.Error()})
	case domain.ErrClosed:
		c.JSON(http.StatusConflict, gin.H{"code": 4009, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}

func ticketView(t *domain.Ticket) gin.H {
	return gin.H{
		"id":          t.ID,
		"user_id":     t.UserID,
		"subject":     t.Subject,
		"category":    t.Category,
		"priority":    t.Priority,
		"status":      t.Status,
		"assignee_id": t.AssigneeID,
		"created_at":  t.CreatedAt,
		"updated_at":  t.UpdatedAt,
	}
}

func ticketViews(ts []domain.Ticket) []gin.H {
	out := make([]gin.H, 0, len(ts))
	for i := range ts {
		out = append(out, ticketView(&ts[i]))
	}
	return out
}

func messageView(m *domain.Message) gin.H {
	return gin.H{
		"id":          m.ID,
		"ticket_id":   m.TicketID,
		"sender_id":   m.SenderID,
		"sender_type": m.SenderType,
		"body":        m.Body,
		"attachments": m.Attachments,
		"created_at":  m.CreatedAt,
	}
}

func messageViews(ms []domain.Message) []gin.H {
	out := make([]gin.H, 0, len(ms))
	for i := range ms {
		out = append(out, messageView(&ms[i]))
	}
	return out
}
