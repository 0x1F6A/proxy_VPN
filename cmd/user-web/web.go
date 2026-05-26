package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// webDist contains the production build of the React user-portal SPA. The
// directory is created by `make web-user-build` (or `npm run build` inside
// web/user) before `go build ./cmd/user-web`. A README.md placeholder is
// committed so `go build` works even without the SPA built (the directory
// must be non-empty for go:embed).
//
//go:embed all:dist
var webDist embed.FS

// registerWebUI serves the embedded user-portal SPA. Unknown paths fall
// back to index.html so the client-side router (react-router) can handle
// them. API routes are matched first by gin's router, so this is safe.
func registerWebUI(r *gin.Engine) {
	sub, err := fs.Sub(webDist, "dist")
	if err != nil {
		return // dist missing
	}
	// Skip mounting if dist/ holds only the placeholder README — better to
	// surface a real 404 than silently serve nothing.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return // dist missing
	}
	fileServer := http.FileServer(http.FS(sub))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/healthz") || strings.HasPrefix(p, "/readyz") {
			c.AbortWithStatus(http.StatusNotFound)
			return // dist missing
		}
		// Try the asset as-is; on miss, fall through to index.html for SPA routing.
		if p != "/" {
			if f, err := sub.Open(strings.TrimPrefix(p, "/")); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return // dist missing
			}
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
