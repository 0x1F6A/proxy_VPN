# cmd/admin SPA build artefacts

Run `make web-admin-build` to populate this directory with the compiled
React Web Admin assets. `go:embed` in `web.go` consumes everything here at
build time. This README exists so `go build` works even before the SPA is
built (the directory is required to be non-empty).
