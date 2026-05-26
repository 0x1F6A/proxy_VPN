// Package service implements user-bounded-context use cases. Each method is
// a transactional script that orchestrates ports + domain rules.
package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/idgen"
	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
)

// Deps bundles every dependency a Service needs. Wired once at startup.
type Deps struct {
	Users     ports.UserRepo
	Refresh   ports.RefreshRepo
	Codes     ports.EmailCodeRepo
	Logs      ports.LoginLogRepo
	Mailer    ports.Mailer
	Blacklist ports.AccessTokenBlacklist
	Rate      ports.RateLimiter
	JWT       *auth.JWT
	Cfg       *config.Config
}

// Service is the entry point for HTTP handlers.
type Service struct {
	d Deps
}

func New(d Deps) *Service { return &Service{d: d} }

// ----- helpers ----------------------------------------------------------

func normalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if _, err := mail.ParseAddress(s); err != nil {
		return "", domain.ErrEmailInvalid
	}
	return s, nil
}

func checkPasswordStrength(p string) error {
	if len(p) < 8 {
		return domain.ErrPasswordWeak
	}
	return nil
}

func randomDigits(n int) string {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		v, _ := rand.Int(rand.Reader, big.NewInt(10))
		out[i] = byte('0' + v.Int64())
	}
	return string(out)
}

// ----- use cases --------------------------------------------------------

// SendCode implements the "send me a verification code" flow. scene is one of
// register|reset_password|change_email.
func (s *Service) SendCode(ctx context.Context, email, scene string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	limit := s.d.Cfg.Rate.SendCodePerEmailMin
	if limit > 0 {
		ok, err := s.d.Rate.Allow(ctx, "sendcode:"+scene+":"+email, limit, time.Minute)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrCodeRateLimited
		}
	}
	code := randomDigits(6)
	c := &domain.EmailCode{
		Email:     email,
		Scene:     scene,
		CodeHash:  auth.SHA256Hex(code),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.d.Codes.Create(ctx, c); err != nil {
		return err
	}
	return s.d.Mailer.SendCode(ctx, email, scene, code)
}

func (s *Service) verifyCode(ctx context.Context, email, scene, code string) error {
	c, err := s.d.Codes.FindLatestUnused(ctx, email, scene)
	if err != nil {
		return domain.ErrCodeNotFound
	}
	if time.Now().After(c.ExpiresAt) {
		return domain.ErrCodeExpired
	}
	if c.Attempts >= 5 {
		return domain.ErrCodeMaxAttempts
	}
	if c.CodeHash != auth.SHA256Hex(code) {
		_ = s.d.Codes.IncAttempts(ctx, c.ID)
		return domain.ErrCodeMismatch
	}
	return s.d.Codes.MarkUsed(ctx, c.ID, time.Now())
}

// Register creates a new user after email-code verification.
func (s *Service) Register(ctx context.Context, email, password, code string) (*domain.User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if err := checkPasswordStrength(password); err != nil {
		return nil, err
	}
	if err := s.verifyCode(ctx, email, "register", code); err != nil {
		return nil, err
	}
	if existing, _ := s.d.Users.FindByEmail(ctx, email); existing != nil {
		return nil, domain.ErrEmailTaken
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		Email:             email,
		PasswordHash:      hash,
		UUID:              idgen.UUID(),
		Role:              "user",
		Status:            domain.StatusActive,
		BalanceCNY:        "0.00",
		DeviceLimit:       3,
		SubscriptionToken: idgen.SubscriptionToken(),
		InviteCode:        idgen.InviteCode(),
	}
	if err := s.d.Users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// TokenPair is returned by Login / Refresh.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (s *Service) issueTokens(ctx context.Context, u *domain.User, ip, ua string) (*TokenPair, error) {
	jti := idgen.HexN(32)
	at, exp, err := s.d.JWT.Issue(u.ID, u.Role, jti)
	if err != nil {
		return nil, err
	}
	rt, err := auth.RandomToken(48)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row := &domain.RefreshToken{
		UserID:    u.ID,
		TokenHash: auth.SHA256Hex(rt),
		UserAgent: ua,
		IP:        ip,
		ExpiresAt: now.Add(s.d.Cfg.JWT.RefreshTTL),
	}
	if err := s.d.Refresh.Create(ctx, row); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: at, RefreshToken: rt, ExpiresAt: exp}, nil
}

// Login authenticates by email+password (+TOTP if enrolled). On 2FA-enrolled
// accounts callers must pass totp.
func (s *Service) Login(ctx context.Context, email, password, totp, ip, ua string) (*TokenPair, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	limit := s.d.Cfg.Rate.LoginFailPerIPMin
	if limit > 0 {
		ok, err := s.d.Rate.Allow(ctx, "loginfail:"+ip, limit, time.Minute)
		if err == nil && !ok {
			return nil, fmt.Errorf("too many failed logins, try later")
		}
	}
	u, err := s.d.Users.FindByEmail(ctx, email)
	if err != nil || u == nil {
		_ = s.d.Logs.Append(ctx, domain.LoginLog{Email: email, Success: false, IP: ip, UserAgent: ua, Reason: "no_user"})
		return nil, domain.ErrInvalidCredentials
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		_ = s.d.Logs.Append(ctx, domain.LoginLog{UserID: &u.ID, Email: email, Success: false, IP: ip, UserAgent: ua, Reason: "bad_password"})
		return nil, domain.ErrInvalidCredentials
	}
	switch u.Status {
	case domain.StatusDisabled:
		return nil, domain.ErrUserDisabled
	case domain.StatusPending:
		return nil, domain.ErrUserPending
	}
	if u.TOTPEnabled {
		if totp == "" {
			return nil, domain.ErrTOTPRequired
		}
		if !auth.ValidateTOTP(u.TOTPSecret, totp) {
			_ = s.d.Logs.Append(ctx, domain.LoginLog{UserID: &u.ID, Email: email, Success: false, IP: ip, UserAgent: ua, Reason: "bad_totp"})
			return nil, domain.ErrTOTPInvalid
		}
	}
	pair, err := s.issueTokens(ctx, u, ip, ua)
	if err != nil {
		return nil, err
	}
	_ = s.d.Users.UpdateLogin(ctx, u.ID, time.Now(), ip)
	_ = s.d.Logs.Append(ctx, domain.LoginLog{UserID: &u.ID, Email: email, Success: true, IP: ip, UserAgent: ua})
	return pair, nil
}

// Refresh rotates a refresh token: revoke the old, issue a new pair.
func (s *Service) Refresh(ctx context.Context, refreshToken, ip, ua string) (*TokenPair, error) {
	row, err := s.d.Refresh.FindByHash(ctx, auth.SHA256Hex(refreshToken))
	if err != nil || row == nil || !row.IsActive(time.Now()) {
		return nil, domain.ErrRefreshInvalid
	}
	u, err := s.d.Users.FindByID(ctx, row.UserID)
	if err != nil || u == nil {
		return nil, domain.ErrUserNotFound
	}
	if err := s.d.Refresh.Revoke(ctx, row.ID, time.Now()); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, u, ip, ua)
}

// Logout revokes the current access token (via blacklist) and all refresh
// tokens for this user. We intentionally revoke all refresh sessions on logout
// to keep behavior simple in v1; a future change can scope this per-session.
func (s *Service) Logout(ctx context.Context, claims *auth.Claims) error {
	ttl := time.Until(claims.ExpiresAt.Time) + time.Minute
	if ttl > 0 {
		_ = s.d.Blacklist.Revoke(ctx, claims.JTI, ttl)
	}
	return s.d.Refresh.RevokeAllForUser(ctx, claims.UID, time.Now())
}

// ChangePassword requires the current password and updates to the new hash.
// All refresh tokens are revoked for safety.
func (s *Service) ChangePassword(ctx context.Context, uid uint64, oldPwd, newPwd string) error {
	if err := checkPasswordStrength(newPwd); err != nil {
		return err
	}
	u, err := s.d.Users.FindByID(ctx, uid)
	if err != nil || u == nil {
		return domain.ErrUserNotFound
	}
	if !auth.CheckPassword(u.PasswordHash, oldPwd) {
		return domain.ErrInvalidCredentials
	}
	hash, err := auth.HashPassword(newPwd)
	if err != nil {
		return err
	}
	if err := s.d.Users.UpdatePassword(ctx, uid, hash); err != nil {
		return err
	}
	_ = s.d.Refresh.RevokeAllForUser(ctx, uid, time.Now())
	return nil
}

// Me returns the current user profile.
func (s *Service) Me(ctx context.Context, uid uint64) (*domain.User, error) {
	u, err := s.d.Users.FindByID(ctx, uid)
	if err != nil || u == nil {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

// EnrollTOTP generates and persists a (not-yet-active) secret, returning the
// QR PNG bytes. Call VerifyTOTP to activate.
type TOTPEnrollment struct {
	Secret string
	URL    string
	QRPNG  []byte
}

func (s *Service) EnrollTOTP(ctx context.Context, uid uint64) (*TOTPEnrollment, error) {
	u, err := s.d.Users.FindByID(ctx, uid)
	if err != nil || u == nil {
		return nil, domain.ErrUserNotFound
	}
	if u.TOTPEnabled {
		return nil, domain.ErrTOTPAlreadyEnrolled
	}
	secret, url, qr, err := auth.EnrollTOTP(s.d.Cfg.JWT.Issuer, u.Email)
	if err != nil {
		return nil, err
	}
	if err := s.d.Users.UpdateTOTP(ctx, uid, secret, false); err != nil {
		return nil, err
	}
	return &TOTPEnrollment{Secret: secret, URL: url, QRPNG: qr}, nil
}

func (s *Service) VerifyTOTP(ctx context.Context, uid uint64, code string) error {
	u, err := s.d.Users.FindByID(ctx, uid)
	if err != nil || u == nil {
		return domain.ErrUserNotFound
	}
	if u.TOTPSecret == "" {
		return domain.ErrTOTPNotEnrolled
	}
	if !auth.ValidateTOTP(u.TOTPSecret, code) {
		return domain.ErrTOTPInvalid
	}
	return s.d.Users.UpdateTOTP(ctx, uid, u.TOTPSecret, true)
}

func (s *Service) DisableTOTP(ctx context.Context, uid uint64, code string) error {
	u, err := s.d.Users.FindByID(ctx, uid)
	if err != nil || u == nil {
		return domain.ErrUserNotFound
	}
	if !u.TOTPEnabled {
		return domain.ErrTOTPNotEnrolled
	}
	if !auth.ValidateTOTP(u.TOTPSecret, code) {
		return domain.ErrTOTPInvalid
	}
	return s.d.Users.UpdateTOTP(ctx, uid, "", false)
}
