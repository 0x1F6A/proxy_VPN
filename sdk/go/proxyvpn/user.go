package proxyvpn

import (
	"context"
	"time"
)

// User describes the current account.
type User struct {
	ID             uint64    `json:"id"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	Balance        int64     `json:"balance"`
	PlanID         uint64    `json:"plan_id"`
	PlanExpireAt   time.Time `json:"plan_expire_at"`
	TrafficUsed    uint64    `json:"traffic_used"`
	TrafficQuota   uint64    `json:"traffic_quota"`
	InviteCode     string    `json:"invite_code"`
	TOTPEnabled    bool      `json:"totp_enabled"`
	SubscribeToken string    `json:"subscribe_token"`
	CreatedAt      time.Time `json:"created_at"`
}

// Me returns the currently authenticated user.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.do(ctx, "GET", "/api/v1/user/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ChangePassword updates the account password (old required).
func (c *Client) ChangePassword(ctx context.Context, oldPwd, newPwd string) error {
	return c.do(ctx, "POST", "/api/v1/user/change-password", map[string]string{
		"old_password": oldPwd,
		"new_password": newPwd,
	}, nil)
}

// TOTPEnrollment is returned by Setup2FA, ready to be shown as a QR code.
type TOTPEnrollment struct {
	Secret    string   `json:"secret"`
	OTPAuth   string   `json:"otpauth_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// Setup2FA begins TOTP enrollment.
func (c *Client) Setup2FA(ctx context.Context) (*TOTPEnrollment, error) {
	var out TOTPEnrollment
	if err := c.do(ctx, "POST", "/api/v1/user/2fa/setup", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Confirm2FA finalises TOTP enrollment by verifying a code from the authenticator app.
func (c *Client) Confirm2FA(ctx context.Context, code string) error {
	return c.do(ctx, "POST", "/api/v1/user/2fa/confirm", map[string]string{"code": code}, nil)
}

// Disable2FA turns off TOTP. password is required as a sanity check.
func (c *Client) Disable2FA(ctx context.Context, password string) error {
	return c.do(ctx, "POST", "/api/v1/user/2fa/disable", map[string]string{"password": password}, nil)
}
