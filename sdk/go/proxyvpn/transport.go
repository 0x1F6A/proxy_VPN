package proxyvpn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type envelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
}

// do issues an HTTP request, automatically attaching auth + parsing the envelope.
// If the call returns ErrAccessTokenInvalid (40101) it transparently refreshes
// once and retries. out may be nil to discard the data payload.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.doWithRetry(ctx, method, path, body, out, true)
}

// doNoAuth is for endpoints that must never carry tokens (login, register, refresh).
func (c *Client) doNoAuth(ctx context.Context, method, path string, body, out any) error {
	return c.doRaw(ctx, method, path, body, out, false)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body, out any, allowRefresh bool) error {
	err := c.doRaw(ctx, method, path, body, out, true)
	if err == nil {
		return nil
	}
	if !allowRefresh {
		return err
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == ErrAccessTokenInvalid.Code {
		c.mu.Lock()
		rt := c.refreshToken
		ver := c.tokenVersion
		c.mu.Unlock()
		if rt == "" {
			return err
		}
		if rerr := c.refreshLocked(ctx, rt, ver); rerr != nil {
			return err
		}
		return c.doRaw(ctx, method, path, body, out, true)
	}
	return err
}

func (c *Client) doRaw(ctx context.Context, method, path string, body, out any, withAuth bool) error {
	if c.cfg.BaseURL == "" {
		return fmt.Errorf("proxyvpn: BaseURL is required")
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("proxyvpn: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	url := strings.TrimRight(c.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("proxyvpn: new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if withAuth {
		c.mu.Lock()
		at := c.accessToken
		c.mu.Unlock()
		if at != "" {
			req.Header.Set("Authorization", "Bearer "+at)
		}
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("proxyvpn: http: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyvpn: read body: %w", err)
	}
	if len(raw) == 0 {
		if resp.StatusCode >= 400 {
			return &APIError{Code: resp.StatusCode * 100, Message: resp.Status, HTTPStatus: resp.StatusCode}
		}
		return nil
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("proxyvpn: decode envelope: %w (body=%s)", err, string(raw))
	}
	if env.Code != 0 {
		return &APIError{Code: env.Code, Message: env.Message, RequestID: env.RequestID, HTTPStatus: resp.StatusCode}
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("proxyvpn: decode data: %w", err)
		}
	}
	return nil
}

// doBytes returns the raw response body (used by subscription endpoints
// which serve non-JSON payloads).
func (c *Client) doBytes(ctx context.Context, method, path string) ([]byte, error) {
	if c.cfg.BaseURL == "" {
		return nil, fmt.Errorf("proxyvpn: BaseURL is required")
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Code: resp.StatusCode * 100, Message: resp.Status, HTTPStatus: resp.StatusCode}
	}
	return raw, nil
}
