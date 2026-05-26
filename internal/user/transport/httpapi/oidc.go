package httpapi

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// GET /api/v1/auth/oidc/login?next=/admin/dashboard
func (h *Handler) oidcLogin(c *gin.Context) {
	next := c.Query("next")
	res, err := h.svc.BeginAuth(c.Request.Context(), h.oidcVer, h.oidcStore, next)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.Redirect(http.StatusFound, res.RedirectURL)
}

// GET /api/v1/auth/oidc/callback?code=...&state=...
//
// Default behaviour is JSON response (so SPA popups can read tokens).
// If the post-login URL is set and the request prefers HTML (Accept
// header contains text/html) we 302 back with tokens in the fragment.
func (h *Handler) oidcCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": "missing code or state"})
		return
	}
	pair, postLogin, err := h.svc.CompleteAuth(c.Request.Context(), h.oidcVer, h.oidcStore, state, code, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		mapErr(c, err)
		return
	}
	if postLogin != "" && acceptsHTML(c) {
		q := url.Values{}
		q.Set("access_token", pair.AccessToken)
		q.Set("refresh_token", pair.RefreshToken)
		c.Redirect(http.StatusFound, postLogin+"#"+q.Encode())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pair})
}

func acceptsHTML(c *gin.Context) bool {
	a := c.GetHeader("Accept")
	for _, part := range []string{"text/html", "application/xhtml+xml"} {
		if containsFold(a, part) {
			return true
		}
	}
	return false
}

func containsFold(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if equalFoldASCII(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
