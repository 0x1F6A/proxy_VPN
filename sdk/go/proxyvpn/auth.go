package proxyvpn

import (
	"context"
)

// TokenPair is the access + refresh token bundle returned by login/register/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// SendCode triggers an email verification code for the given scene.
//
// scene is one of: register | login | reset_password.
func (c *Client) SendCode(ctx context.Context, email, scene string) error {
	return c.doNoAuth(ctx, "POST", "/api/v1/auth/email/send-code", map[string]string{
		"email": email,
		"scene": scene,
	}, nil)
}

// RegisterEmail creates a new account using an emailed verification code.
func (c *Client) RegisterEmail(ctx context.Context, email, code, password, inviteCode string) (*TokenPair, error) {
	req := map[string]string{
		"email":    email,
		"code":     code,
		"password": password,
	}
	if inviteCode != "" {
		req["invite_code"] = inviteCode
	}
	var out TokenPair
	if err := c.doNoAuth(ctx, "POST", "/api/v1/auth/email/register", req, &out); err != nil {
		return nil, err
	}
	c.WithTokens(out.AccessToken, out.RefreshToken)
	return &out, nil
}

// LoginPassword logs in using email + password (+ optional TOTP code).
func (c *Client) LoginPassword(ctx context.Context, email, password, totp string) (*TokenPair, error) {
	req := map[string]string{"email": email, "password": password}
	if totp != "" {
		req["totp"] = totp
	}
	var out TokenPair
	if err := c.doNoAuth(ctx, "POST", "/api/v1/auth/login", req, &out); err != nil {
		return nil, err
	}
	c.WithTokens(out.AccessToken, out.RefreshToken)
	return &out, nil
}

// LoginEmailCode logs in using an emailed code (passwordless).
func (c *Client) LoginEmailCode(ctx context.Context, email, code string) (*TokenPair, error) {
	var out TokenPair
	if err := c.doNoAuth(ctx, "POST", "/api/v1/auth/email/login", map[string]string{
		"email": email,
		"code":  code,
	}, &out); err != nil {
		return nil, err
	}
	c.WithTokens(out.AccessToken, out.RefreshToken)
	return &out, nil
}

// RefreshToken exchanges a refresh token for a fresh access/refresh pair.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	var out TokenPair
	if err := c.doNoAuth(ctx, "POST", "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}, &out); err != nil {
		return nil, err
	}
	c.WithTokens(out.AccessToken, out.RefreshToken)
	return &out, nil
}

// Logout revokes the current refresh token server-side and clears local state.
func (c *Client) Logout(ctx context.Context) error {
	c.mu.Lock()
	rt := c.refreshToken
	c.mu.Unlock()
	if rt == "" {
		return nil
	}
	err := c.do(ctx, "POST", "/api/v1/auth/logout", map[string]string{"refresh_token": rt}, nil)
	c.WithTokens("", "")
	return err
}

// refreshLocked refreshes tokens iff no other refresh has already happened
// (token version check guards against thundering-herd refresh).
func (c *Client) refreshLocked(ctx context.Context, rt string, expectedVersion uint64) error {
	c.mu.Lock()
	if c.tokenVersion != expectedVersion {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	var out TokenPair
	if err := c.doNoAuth(ctx, "POST", "/api/v1/auth/refresh", map[string]string{
		"refresh_token": rt,
	}, &out); err != nil {
		return err
	}
	c.WithTokens(out.AccessToken, out.RefreshToken)
	return nil
}
