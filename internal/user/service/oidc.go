// Package service — OIDC SSO flow. Issuer-agnostic; we accept any OIDC
// provider whose issuer URL exposes /.well-known/openid-configuration.
//
// Authorisation flow (handled by the transport layer):
//  1. user hits /api/v1/auth/oidc/login → handler calls BeginAuth, gets
//     state + redirect URL, 302
//  2. provider redirects back to /api/v1/auth/oidc/callback?code&state →
//     handler validates state then calls CompleteAuth
//  3. CompleteAuth verifies the ID token, applies the allow-list, finds
//     or creates the local user, then issues a JWT TokenPair
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/idgen"
	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
)

// OIDCVerifier is the narrow contract the OIDC service depends on.
// Production wires this to github.com/coreos/go-oidc; tests inject a fake.
type OIDCVerifier interface {
	// AuthCodeURL builds the provider authorize URL for the given state.
	AuthCodeURL(state string) string
	// Exchange swaps the auth code for a verified ID token claim set.
	Exchange(ctx context.Context, code string) (OIDCIdentity, error)
}

// OIDCIdentity is the subset of OIDC ID-token claims we use.
type OIDCIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// OIDCStateStore persists short-lived state tokens to defend against CSRF.
type OIDCStateStore interface {
	Save(ctx context.Context, state, redirect string, ttl time.Duration) error
	Consume(ctx context.Context, state string) (redirect string, ok bool, err error)
}

// BeginAuthResult is what the transport layer needs to perform the 302.
type BeginAuthResult struct {
	RedirectURL string
	State       string
}

// BeginAuth generates a fresh state, stashes the post-login redirect, and
// returns the provider authorize URL.
func (s *Service) BeginAuth(ctx context.Context, verifier OIDCVerifier, store OIDCStateStore, postLogin string) (*BeginAuthResult, error) {
	if verifier == nil || store == nil {
		return nil, errors.New("oidc not configured")
	}
	state := idgen.HexN(32)
	ttl := s.d.Cfg.OIDC.StateTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if err := store.Save(ctx, state, postLogin, ttl); err != nil {
		return nil, fmt.Errorf("save oidc state: %w", err)
	}
	return &BeginAuthResult{RedirectURL: verifier.AuthCodeURL(state), State: state}, nil
}

// CompleteAuth validates the state, exchanges the code, enforces the
// allow-list, finds or creates the local user, and issues tokens.
//
// Returns (TokenPair, postLoginURL, error).
func (s *Service) CompleteAuth(ctx context.Context, verifier OIDCVerifier, store OIDCStateStore, state, code, ip, ua string) (*TokenPair, string, error) {
	if verifier == nil || store == nil {
		return nil, "", errors.New("oidc not configured")
	}
	postLogin, ok, err := store.Consume(ctx, state)
	if err != nil {
		return nil, "", fmt.Errorf("consume oidc state: %w", err)
	}
	if !ok {
		return nil, "", errors.New("invalid or expired state")
	}
	id, err := verifier.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("oidc exchange: %w", err)
	}
	if id.Subject == "" {
		return nil, "", errors.New("oidc identity missing subject")
	}
	if id.Email == "" {
		return nil, "", errors.New("oidc identity missing email")
	}
	if !id.EmailVerified {
		return nil, "", errors.New("oidc email not verified by provider")
	}
	email := strings.ToLower(strings.TrimSpace(id.Email))
	if !s.oidcAllowed(email) {
		return nil, "", domain.ErrForbidden
	}
	role := "user"
	if s.oidcIsAdmin(email) {
		role = "admin"
	}
	u, err := s.findOrLinkOIDCUser(ctx, id.Subject, email, role)
	if err != nil {
		return nil, "", err
	}
	if u.Status == domain.StatusDisabled {
		return nil, "", domain.ErrUserDisabled
	}
	pair, err := s.issueTokens(ctx, u, ip, ua)
	if err != nil {
		return nil, "", err
	}
	_ = s.d.Users.UpdateLogin(ctx, u.ID, time.Now(), ip)
	return pair, postLogin, nil
}

func (s *Service) findOrLinkOIDCUser(ctx context.Context, subject, email, role string) (*domain.User, error) {
	if u, err := s.d.Users.FindByOIDCSubject(ctx, subject); err == nil && u != nil {
		return u, nil
	}
	if u, err := s.d.Users.FindByEmail(ctx, email); err == nil && u != nil {
		if err := s.d.Users.LinkOIDCSubject(ctx, u.ID, subject); err != nil {
			return nil, err
		}
		u.OIDCSubject = subject
		return u, nil
	}
	hash, err := auth.HashPassword(idgen.HexN(32))
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		Email:             email,
		PasswordHash:      hash,
		UUID:              idgen.UUID(),
		Role:              role,
		Status:            domain.StatusActive,
		BalanceCNY:        "0.00",
		DeviceLimit:       3,
		SubscriptionToken: idgen.SubscriptionToken(),
		InviteCode:        idgen.InviteCode(),
		OIDCSubject:       subject,
	}
	if err := s.d.Users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) oidcAllowed(email string) bool {
	cfg := s.d.Cfg.OIDC
	if contains(cfg.AdminEmails, email) || contains(cfg.AllowedEmails, email) {
		return true
	}
	if len(cfg.AllowedDomains) > 0 {
		if at := strings.LastIndex(email, "@"); at >= 0 {
			d := email[at+1:]
			for _, ad := range cfg.AllowedDomains {
				if strings.EqualFold(strings.TrimSpace(ad), d) {
					return true
				}
			}
		}
		return false
	}
	return len(cfg.AllowedEmails) == 0 && len(cfg.AdminEmails) == 0
}

func (s *Service) oidcIsAdmin(email string) bool {
	return contains(s.d.Cfg.OIDC.AdminEmails, email)
}

func contains(list []string, want string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), want) {
			return true
		}
	}
	return false
}

// EnsureOIDCConfigured panics if Service.New was wired without OIDC config;
// transport code uses this defensively when mounting routes.
func (s *Service) OIDCEnabled() bool {
	cfg := s.d.Cfg
	return cfg != nil && cfg.OIDC.Enabled && cfg.OIDC.Issuer != "" && cfg.OIDC.ClientID != ""
}

// guard against accidentally exposing config package indirectly
var _ = (*config.Config)(nil)