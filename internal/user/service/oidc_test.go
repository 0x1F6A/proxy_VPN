package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
)

type fakeVerifier struct {
	id  OIDCIdentity
	err error
}

func (f *fakeVerifier) AuthCodeURL(state string) string {
	return "https://idp.test/authorize?state=" + state
}
func (f *fakeVerifier) Exchange(_ context.Context, _ string) (OIDCIdentity, error) {
	return f.id, f.err
}

type fakeStateStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeStateStore() *fakeStateStore { return &fakeStateStore{m: map[string]string{}} }

func (s *fakeStateStore) Save(_ context.Context, state, redirect string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[state] = redirect
	return nil
}
func (s *fakeStateStore) Consume(_ context.Context, state string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[state]
	if !ok {
		return "", false, nil
	}
	delete(s.m, state)
	return v, true, nil
}

func newOIDCService(t *testing.T, cfgOIDC config.OIDCConfig) (*Service, *fakeUserRepo) {
	t.Helper()
	users := newFakeUserRepo()
	deps := Deps{
		Users:     users,
		Refresh:   newFakeRefreshRepo(),
		Codes:     newFakeCodeRepo(),
		Logs:      &fakeLogRepo{},
		Mailer:    &fakeMailer{},
		Blacklist: newFakeBL(),
		Rate:      allowAllLimiter{},
		JWT:       auth.NewJWT("test-secret", time.Hour, "test", 0),
		Cfg:       &config.Config{OIDC: cfgOIDC},
	}
	return New(deps), users
}

func TestOIDC_BeginAuth(t *testing.T) {
	svc, _ := newOIDCService(t, config.OIDCConfig{Enabled: true, AllowedDomains: []string{"test.com"}})
	store := newFakeStateStore()
	ver := &fakeVerifier{}
	res, err := svc.BeginAuth(context.Background(), ver, store, "/admin")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if res.State == "" {
		t.Fatal("missing state")
	}
	if redirect, ok := store.m[res.State]; !ok || redirect != "/admin" {
		t.Fatalf("state not saved: %+v", store.m)
	}
}

func TestOIDC_CompleteAuth_AdminMappingAndAutoRegister(t *testing.T) {
	cfg := config.OIDCConfig{
		Enabled:        true,
		AllowedDomains: []string{"company.com"},
		AdminEmails:    []string{"boss@company.com"},
	}
	svc, users := newOIDCService(t, cfg)
	store := newFakeStateStore()
	store.m["s1"] = "/admin"
	ver := &fakeVerifier{id: OIDCIdentity{
		Subject: "google|123", Email: "boss@company.com", EmailVerified: true,
	}}
	pair, post, err := svc.CompleteAuth(context.Background(), ver, store, "s1", "code", "127.0.0.1", "ua")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if pair == nil || pair.AccessToken == "" {
		t.Fatal("missing tokens")
	}
	if post != "/admin" {
		t.Fatalf("post-login redirect lost: %q", post)
	}
	var u *domain.User
	for _, x := range users.byID {
		u = x
	}
	if u == nil || u.Role != "admin" || u.OIDCSubject != "google|123" {
		t.Fatalf("user wrong: %+v", u)
	}
}

func TestOIDC_CompleteAuth_DomainNotAllowed(t *testing.T) {
	cfg := config.OIDCConfig{Enabled: true, AllowedDomains: []string{"company.com"}}
	svc, _ := newOIDCService(t, cfg)
	store := newFakeStateStore()
	store.m["s1"] = ""
	ver := &fakeVerifier{id: OIDCIdentity{Subject: "x", Email: "rando@other.com", EmailVerified: true}}
	_, _, err := svc.CompleteAuth(context.Background(), ver, store, "s1", "code", "", "")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden got %v", err)
	}
}

func TestOIDC_CompleteAuth_StateMissing(t *testing.T) {
	svc, _ := newOIDCService(t, config.OIDCConfig{Enabled: true, AllowedDomains: []string{"t.com"}})
	store := newFakeStateStore()
	ver := &fakeVerifier{id: OIDCIdentity{Subject: "x", Email: "a@t.com", EmailVerified: true}}
	_, _, err := svc.CompleteAuth(context.Background(), ver, store, "missing", "code", "", "")
	if err == nil {
		t.Fatal("expected invalid state error")
	}
}

func TestOIDC_CompleteAuth_EmailNotVerified(t *testing.T) {
	svc, _ := newOIDCService(t, config.OIDCConfig{Enabled: true, AllowedDomains: []string{"t.com"}})
	store := newFakeStateStore()
	store.m["s1"] = ""
	ver := &fakeVerifier{id: OIDCIdentity{Subject: "x", Email: "a@t.com", EmailVerified: false}}
	_, _, err := svc.CompleteAuth(context.Background(), ver, store, "s1", "code", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
