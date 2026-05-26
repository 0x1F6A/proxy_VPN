// Package testsupport provides shared helpers for integration tests that
// require real MySQL and Redis backends spun up via testcontainers-go.
//
// The contents are only compiled when the `integration` build tag is set,
// so the production build and the default `go test ./...` run do not
// require Docker.
package testsupport
