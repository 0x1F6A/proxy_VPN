package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
)

const (
	CtxClaimsKey = "auth_claims"
)

// AuthRequired parses the Authorization: Bearer header, verifies the JWT,
// checks the blacklist, and stores claims in the request context.
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": "missing bearer token"})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		claims, err := h.jwt.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": "invalid token: " + err.Error()})
			return
		}
		revoked, _ := h.blacklist.IsRevoked(c.Request.Context(), claims.JTI)
		if revoked {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 4001, "message": "token revoked"})
			return
		}
		c.Set(CtxClaimsKey, claims)
		c.Next()
	}
}

// ClaimsFrom extracts the claims previously stored by AuthRequired.
func ClaimsFrom(c *gin.Context) *auth.Claims {
	v, ok := c.Get(CtxClaimsKey)
	if !ok {
		return nil
	}
	return v.(*auth.Claims)
}
