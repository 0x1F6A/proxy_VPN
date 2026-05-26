# cmd/user-web SPA build artefacts

Run `make web-user-build` to populate this directory with the compiled
React user portal SPA. `go:embed` in `web.go` consumes everything here at
build time. This README exists so `go build` works even before the SPA is
built (the directory is required to be non-empty).
