// Package oidcprov adapts coreos/go-oidc + x/oauth2 to the
// user/service.OIDCVerifier contract. Keeping the adapter here means the
// service package stays import-light and trivially testable.
package oidcprov

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	usersvc "github.com/0x1F6A/proxy_VPN/internal/user/service"
)

// Config mirrors the bits of config.OIDCConfig we need at construction.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Verifier implements usersvc.OIDCVerifier.
type Verifier struct {
	oauthCfg *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New discovers the issuer (one HTTP call) and returns a ready Verifier.
func New(ctx context.Context, c Config) (*Verifier, error) {
	if c.Issuer == "" || c.ClientID == "" || c.RedirectURL == "" {
		return nil, errors.New("oidc: issuer, client_id, redirect_url are required")
	}
	provider, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discover: %w", err)
	}
	scopes := c.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}
	return &Verifier{
		oauthCfg: &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: c.ClientID}),
	}, nil
}

func (v *Verifier) AuthCodeURL(state string) string {
	return v.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (v *Verifier) Exchange(ctx context.Context, code string) (usersvc.OIDCIdentity, error) {
	tok, err := v.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return usersvc.OIDCIdentity{}, fmt.Errorf("oauth exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return usersvc.OIDCIdentity{}, errors.New("no id_token in oauth response")
	}
	idTok, err := v.verifier.Verify(ctx, rawID)
	if err != nil {
		return usersvc.OIDCIdentity{}, fmt.Errorf("verify id_token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return usersvc.OIDCIdentity{}, fmt.Errorf("decode claims: %w", err)
	}
	return usersvc.OIDCIdentity{
		Subject:       idTok.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
	}, nil
}
