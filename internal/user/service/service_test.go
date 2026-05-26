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

// ----- in-memory fakes --------------------------------------------------

type fakeUserRepo struct {
	mu    sync.Mutex
	byID  map[uint64]*domain.User
	byEml map[string]*domain.User
	seq   uint64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byID: map[uint64]*domain.User{}, byEml: map[string]*domain.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byEml[u.Email]; ok {
		return domain.ErrEmailTaken
	}
	f.seq++
	u.ID = f.seq
	u.CreatedAt = time.Now()
	f.byID[u.ID] = u
	f.byEml[u.Email] = u
	return nil
}
func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byEml[email]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}
func (f *fakeUserRepo) FindByID(_ context.Context, id uint64) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}
func (f *fakeUserRepo) UpdatePassword(_ context.Context, id uint64, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		u.PasswordHash = hash
	}
	return nil
}
func (f *fakeUserRepo) UpdateLogin(_ context.Context, id uint64, at time.Time, ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		u.LastLoginAt = &at
		u.LastLoginIP = ip
	}
	return nil
}
func (f *fakeUserRepo) UpdateTOTP(_ context.Context, id uint64, secret string, enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		u.TOTPSecret = secret
		u.TOTPEnabled = enabled
	}
	return nil
}
func (f *fakeUserRepo) FindByOIDCSubject(_ context.Context, subject string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.OIDCSubject == subject && subject != "" {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (f *fakeUserRepo) LinkOIDCSubject(_ context.Context, id uint64, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		u.OIDCSubject = subject
	}
	return nil
}

type fakeRefreshRepo struct {
	mu   sync.Mutex
	rows map[string]*domain.RefreshToken
	seq  uint64
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{rows: map[string]*domain.RefreshToken{}}
}
func (f *fakeRefreshRepo) Create(_ context.Context, rt *domain.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	rt.ID = f.seq
	rt.CreatedAt = time.Now()
	f.rows[rt.TokenHash] = rt
	return nil
}
func (f *fakeRefreshRepo) FindByHash(_ context.Context, h string) (*domain.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[h]; ok {
		return r, nil
	}
	return nil, domain.ErrRefreshInvalid
}
func (f *fakeRefreshRepo) Revoke(_ context.Context, id uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			r.RevokedAt = &at
		}
	}
	return nil
}
func (f *fakeRefreshRepo) RevokeAllForUser(_ context.Context, uid uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.UserID == uid && r.RevokedAt == nil {
			r.RevokedAt = &at
		}
	}
	return nil
}

type fakeCodeRepo struct {
	mu   sync.Mutex
	rows []*domain.EmailCode
	seq  uint64
}

func newFakeCodeRepo() *fakeCodeRepo { return &fakeCodeRepo{} }
func (f *fakeCodeRepo) Create(_ context.Context, c *domain.EmailCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	c.ID = f.seq
	c.CreatedAt = time.Now()
	f.rows = append(f.rows, c)
	return nil
}
func (f *fakeCodeRepo) FindLatestUnused(_ context.Context, email, scene string) (*domain.EmailCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.rows) - 1; i >= 0; i-- {
		r := f.rows[i]
		if r.Email == email && r.Scene == scene && r.UsedAt == nil {
			return r, nil
		}
	}
	return nil, domain.ErrCodeNotFound
}
func (f *fakeCodeRepo) IncAttempts(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			r.Attempts++
		}
	}
	return nil
}
func (f *fakeCodeRepo) MarkUsed(_ context.Context, id uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			r.UsedAt = &at
		}
	}
	return nil
}

type fakeLogRepo struct{ n int }

func (f *fakeLogRepo) Append(_ context.Context, _ domain.LoginLog) error { f.n++; return nil }

type fakeMailer struct {
	last string
}

func (f *fakeMailer) SendCode(_ context.Context, to, scene, code string) error {
	f.last = code
	_ = to
	_ = scene
	return nil
}

type fakeBL struct {
	mu      sync.Mutex
	revoked map[string]struct{}
}

func newFakeBL() *fakeBL { return &fakeBL{revoked: map[string]struct{}{}} }
func (f *fakeBL) Revoke(_ context.Context, jti string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[jti] = struct{}{}
	return nil
}
func (f *fakeBL) IsRevoked(_ context.Context, jti string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.revoked[jti]
	return ok, nil
}

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return true, nil
}

func newSvc(t *testing.T) (*Service, *fakeMailer, *fakeUserRepo, *fakeRefreshRepo) {
	t.Helper()
	cfg := &config.Config{}
	cfg.JWT.AccessTTL = time.Hour
	cfg.JWT.RefreshTTL = 24 * time.Hour
	cfg.JWT.Issuer = "test"
	cfg.Rate.SendCodePerEmailMin = 0
	cfg.Rate.LoginFailPerIPMin = 0
	mailer := &fakeMailer{}
	users := newFakeUserRepo()
	refresh := newFakeRefreshRepo()
	deps := Deps{
		Users:     users,
		Refresh:   refresh,
		Codes:     newFakeCodeRepo(),
		Logs:      &fakeLogRepo{},
		Mailer:    mailer,
		Blacklist: newFakeBL(),
		Rate:      allowAllLimiter{},
		JWT:       auth.NewJWT("test-secret", time.Hour, "test", 0),
		Cfg:       cfg,
	}
	return New(deps), mailer, users, refresh
}

// ----- happy path ------------------------------------------------------

func TestRegisterLoginRefreshFlow(t *testing.T) {
	ctx := context.Background()
	svc, mailer, _, _ := newSvc(t)

	if err := svc.SendCode(ctx, "Alice@Example.com", "register"); err != nil {
		t.Fatalf("send code: %v", err)
	}
	if mailer.last == "" {
		t.Fatal("no code captured")
	}

	u, err := svc.Register(ctx, "alice@example.com", "hunter22", mailer.last)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.ID == 0 || u.InviteCode == "" || u.SubscriptionToken == "" {
		t.Fatalf("user not fully initialized: %+v", u)
	}

	pair, err := svc.Login(ctx, "alice@example.com", "hunter22", "", "1.2.3.4", "go-test")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("empty token pair")
	}

	pair2, err := svc.Refresh(ctx, pair.RefreshToken, "1.2.3.4", "go-test")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if pair2.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token should rotate")
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken, "", ""); !errors.Is(err, domain.ErrRefreshInvalid) {
		t.Fatalf("expected invalid on reused old token, got %v", err)
	}
}

// ----- failure modes ---------------------------------------------------

func TestRegisterRejectsWeakPassword(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Register(context.Background(), "x@y.com", "123", "000000")
	if !errors.Is(err, domain.ErrPasswordWeak) {
		t.Fatalf("expected weak password, got %v", err)
	}
}

func TestRegisterRejectsBadEmail(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Register(context.Background(), "not-an-email", "hunter22", "000000")
	if !errors.Is(err, domain.ErrEmailInvalid) {
		t.Fatalf("expected invalid email, got %v", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	svc, mailer, _, _ := newSvc(t)
	_ = svc.SendCode(ctx, "a@b.com", "register")
	_, _ = svc.Register(ctx, "a@b.com", "hunter22", mailer.last)
	_, err := svc.Login(ctx, "a@b.com", "wrong", "", "", "")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected invalid creds, got %v", err)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	svc, mailer, _, _ := newSvc(t)
	_ = svc.SendCode(ctx, "dup@x.com", "register")
	code1 := mailer.last
	if _, err := svc.Register(ctx, "dup@x.com", "hunter22", code1); err != nil {
		t.Fatal(err)
	}
	_ = svc.SendCode(ctx, "dup@x.com", "register")
	if _, err := svc.Register(ctx, "dup@x.com", "hunter22", mailer.last); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("expected email taken, got %v", err)
	}
}

func TestLogoutRevokesRefreshTokens(t *testing.T) {
	ctx := context.Background()
	svc, mailer, _, _ := newSvc(t)
	_ = svc.SendCode(ctx, "lo@x.com", "register")
	_, _ = svc.Register(ctx, "lo@x.com", "hunter22", mailer.last)
	pair, _ := svc.Login(ctx, "lo@x.com", "hunter22", "", "", "")
	claims, _ := svc.d.JWT.Parse(pair.AccessToken)
	if err := svc.Logout(ctx, claims); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken, "", ""); !errors.Is(err, domain.ErrRefreshInvalid) {
		t.Fatalf("refresh should fail after logout, got %v", err)
	}
}

func TestChangePasswordRevokesRefresh(t *testing.T) {
	ctx := context.Background()
	svc, mailer, _, _ := newSvc(t)
	_ = svc.SendCode(ctx, "cp@x.com", "register")
	u, _ := svc.Register(ctx, "cp@x.com", "hunter22", mailer.last)
	pair, _ := svc.Login(ctx, "cp@x.com", "hunter22", "", "", "")
	if err := svc.ChangePassword(ctx, u.ID, "hunter22", "new-hunter22"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken, "", ""); !errors.Is(err, domain.ErrRefreshInvalid) {
		t.Fatalf("refresh should be invalid after password change, got %v", err)
	}
}
