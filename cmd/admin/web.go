package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// webDist contains the production build of the React admin UI. The directory
// is created by `make web-admin-build` (or `npm run build` inside web/admin)
// before `go build ./cmd/admin`. The embed is conditional via build tag so
// developers running `go build` without a frontend build still get a working
// binary (the placeholder file below is committed).
//
//go:embed all:dist
var webDist embed.FS

// registerWebUI serves the embedded admin SPA. Unknown paths fall back to
// index.html so the client-side router (react-router) can handle them.
// API routes are matched first by gin's router, so this is safe.
func registerWebUI(r *gin.Engine) {
	sub, err := fs.Sub(webDist, "dist")
	if err != nil {
		return
	}
	// Quick existence check — if dist/index.html is missing (placeholder
	// only) skip mounting so the operator gets a 404 instead of an empty
	// page that masks the real issue.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/healthz") || strings.HasPrefix(p, "/readyz") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		// Try the asset as-is; on miss, fall through to index.html for SPA routing.
		if p != "/" {
			if f, err := sub.Open(strings.TrimPrefix(p, "/")); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
