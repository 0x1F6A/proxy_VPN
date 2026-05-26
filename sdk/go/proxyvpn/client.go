// Package proxyvpn is the official Go SDK for the proxy_VPN platform.
//
// It exposes a typed [Client] that wraps every public REST endpoint
// documented in docs/api.md, transparently handling token refresh,
// unified response envelopes and error code mapping.
//
// Minimal usage:
//
//	c := proxyvpn.New(proxyvpn.Config{BaseURL: "https://api.example.com"})
//	tok, err := c.LoginPassword(ctx, "alice@example.com", "secret")
//	if err != nil { return err }
//	c.WithTokens(tok.AccessToken, tok.RefreshToken)
//	me, _ := c.Me(ctx)
//	fmt.Println(me.Email)
package proxyvpn

import (
	"net/http"
	"sync"
	"time"
)

// Config configures a SDK [Client].
type Config struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Timeout    time.Duration
}

// Client is the typed proxy_VPN API client. It is safe for concurrent use.
type Client struct {
	cfg Config
	hc  *http.Client

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	tokenVersion uint64
}

// New constructs a [Client]. BaseURL is required.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		cfg.HTTPClient = &http.Client{Timeout: timeout}
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "proxyvpn-go-sdk/0.1"
	}
	return &Client{cfg: cfg, hc: cfg.HTTPClient}
}

// WithTokens injects pre-existing access + refresh tokens (e.g. loaded from disk).
func (c *Client) WithTokens(access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = access
	c.refreshToken = refresh
	c.tokenVersion++
}

// Tokens returns the currently held access + refresh tokens.
func (c *Client) Tokens() (access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken, c.refreshToken
}
