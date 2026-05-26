// Package httpapi exposes user-bounded-context use cases over HTTP.
package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
	"github.com/0x1F6A/proxy_VPN/internal/user/service"
)

type Handler struct {
	svc       *service.Service
	jwt       *auth.JWT
	blacklist ports.AccessTokenBlacklist
	oidcVer   service.OIDCVerifier
	oidcStore service.OIDCStateStore
}

func New(svc *service.Service, jwt *auth.JWT, bl ports.AccessTokenBlacklist) *Handler {
	return &Handler{svc: svc, jwt: jwt, blacklist: bl}
}

// WithOIDC enables /auth/oidc/* routes. Pass both verifier and state
// store; if either is nil OIDC routes are skipped.
func (h *Handler) WithOIDC(v service.OIDCVerifier, s service.OIDCStateStore) *Handler {
	h.oidcVer = v
	h.oidcStore = s
	return h
}

// Register attaches all user routes to the given router group. The /user/*
// subtree requires authentication.
func (h *Handler) Register(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.POST("/send-code", h.sendCode)
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
		auth.POST("/refresh", h.refresh)
		auth.POST("/logout", h.AuthRequired(), h.logout)
		if h.oidcVer != nil && h.oidcStore != nil {
			auth.GET("/oidc/login", h.oidcLogin)
			auth.GET("/oidc/callback", h.oidcCallback)
		}
	}
	u := r.Group("/user").Use(h.AuthRequired())
	{
		u.GET("/me", h.me)
		u.POST("/password", h.changePassword)
		u.POST("/2fa/enroll", h.totpEnroll)
		u.POST("/2fa/verify", h.totpVerify)
		u.POST("/2fa/disable", h.totpDisable)
	}

	admin := r.Group("/admin/users").Use(h.AuthRequired(), h.requireRole("admin", "ops"))
	{
		admin.GET("", h.adminListUsers)
		admin.GET("/summary", h.adminSummary)
		admin.POST("/:id/ban", h.adminBanUser)
		admin.POST("/:id/unban", h.adminUnbanUser)
		admin.POST("/:id/traffic", h.adminAdjustTraffic)
		admin.POST("/:id/rate", h.adminSetRate)
	}
}

func (h *Handler) requireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := ClaimsFrom(c)
		if cl == nil {
			c.AbortWithStatus(401)
			return
		}
		for _, a := range allowed {
			if cl.Role == a {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, gin.H{"code": 4003, "message": "insufficient role"})
	}
}

// ---- error mapping -----------------------------------------------------

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrEmailInvalid),
		errors.Is(err, domain.ErrPasswordWeak),
		errors.Is(err, domain.ErrCodeMismatch),
		errors.Is(err, domain.ErrCodeExpired),
		errors.Is(err, domain.ErrCodeNotFound),
		errors.Is(err, domain.ErrTOTPRequired),
		errors.Is(err, domain.ErrTOTPInvalid),
		errors.Is(err, domain.ErrTOTPNotEnrolled),
		errors.Is(err, domain.ErrTOTPAlreadyEnrolled):
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrRefreshInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": err.Error()})
	case errors.Is(err, domain.ErrUserDisabled),
		errors.Is(err, domain.ErrUserPending),
		errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": 4003, "message": err.Error()})
	case errors.Is(err, domain.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 4004, "message": err.Error()})
	case errors.Is(err, domain.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"code": 4009, "message": err.Error()})
	case errors.Is(err, domain.ErrCodeRateLimited),
		errors.Is(err, domain.ErrCodeMaxAttempts):
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 4029, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": err.Error()})
	}
}
