package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/risk/domain"
)

func TestFingerprintStableAcrossLastOctet(t *testing.T) {
	a := Fingerprint("ua", "en", "1.2.3.4")
	b := Fingerprint("ua", "en", "1.2.3.99")
	if a != b {
		t.Fatalf("fingerprint should be /24-stable: %s vs %s", a, b)
	}
	c := Fingerprint("ua", "en", "1.2.4.4")
	if a == c {
		t.Fatal("different /24 should produce different fingerprints")
	}
}

// ----- fake collaborators ----------------------------------------------

type fakeLockout struct {
	mu     sync.Mutex
	fails  map[string]int
	locks  map[string]time.Time
}

func newFakeLockout() *fakeLockout {
	return &fakeLockout{fails: map[string]int{}, locks: map[string]time.Time{}}
}

func (f *fakeLockout) IncrFail(_ context.Context, k string, _ time.Duration) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails[k]++
	return f.fails[k], nil
}
func (f *fakeLockout) ResetFail(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.fails, k)
	return nil
}
func (f *fakeLockout) Lock(_ context.Context, k string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locks[k] = time.Now().Add(ttl)
	return nil
}
func (f *fakeLockout) IsLocked(_ context.Context, k string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	exp, ok := f.locks[k]
	return ok && time.Now().Before(exp), nil
}

type fakeDevices struct {
	mu   sync.Mutex
	rows []domain.LoginDevice
}

func (f *fakeDevices) Upsert(_ context.Context, d *domain.LoginDevice) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.rows {
		if r.UserID == d.UserID && r.FPHash == d.FPHash {
			f.rows[i].LastSeenAt = d.LastSeenAt
			return nil
		}
	}
	f.rows = append(f.rows, *d)
	return nil
}
func (f *fakeDevices) ListByUser(_ context.Context, uid uint64) ([]domain.LoginDevice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.LoginDevice
	for _, r := range f.rows {
		if r.UserID == uid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeDevices) Revoke(_ context.Context, uid uint64, fp string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].UserID == uid && f.rows[i].FPHash == fp {
			t := at
			f.rows[i].RevokedAt = &t
		}
	}
	return nil
}

type fakeGeo struct{ ipToCountry map[string]string }

func (f *fakeGeo) Country(ip string) string { return f.ipToCountry[ip] }

type fakeUsers struct {
	email, locale, country string
	updated                string
	rotated                string
}

func (f *fakeUsers) EmailAndCountry(_ context.Context, _ uint64) (string, string, string, error) {
	return f.email, f.locale, f.country, nil
}
func (f *fakeUsers) UpdateLastCountry(_ context.Context, _ uint64, c string) error {
	f.updated = c
	return nil
}
func (f *fakeUsers) RotateSubscriptionToken(_ context.Context, _ uint64, newTok string, _ time.Time) error {
	f.rotated = newTok
	return nil
}
func (f *fakeUsers) SubscribeTokenByUser(_ context.Context, _ uint64) (string, error) {
	return f.rotated, nil
}

type fakeMailer struct{ last map[string]string; kind string }

func (m *fakeMailer) SendRiskAlert(_ context.Context, to, locale, kind string, args map[string]string) error {
	m.kind = kind
	m.last = map[string]string{"to": to, "locale": locale}
	for k, v := range args {
		m.last[k] = v
	}
	return nil
}

type fakeSubIPs struct{ uniq int }

func (f *fakeSubIPs) Touch(_ context.Context, _, _ string, _ time.Duration) (int, error) {
	f.uniq++
	return f.uniq, nil
}

// ----- tests ------------------------------------------------------------

func TestPreLoginNotEnabled(t *testing.T) {
	s := New(Deps{})
	if err := s.PreLogin(context.Background(), "a@b"); err != nil {
		t.Fatalf("disabled service should always allow: %v", err)
	}
}

func TestLockoutThreshold(t *testing.T) {
	lo := newFakeLockout()
	s := New(Deps{
		Cfg:     config.RiskConfig{Enabled: true, LoginLockThreshold: 3, LoginLockDuration: time.Minute},
		Lockout: lo,
	})
	for i := 0; i < 3; i++ {
		s.RegisterLoginFailure(context.Background(), "alice@x.com")
	}
	if err := s.PreLogin(context.Background(), "alice@x.com"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked, got %v", err)
	}
}

func TestRegisterLoginSuccessGeoAlert(t *testing.T) {
	dev := &fakeDevices{}
	users := &fakeUsers{email: "u@x", locale: "en", country: "US"}
	mailer := &fakeMailer{}
	geo := &fakeGeo{ipToCountry: map[string]string{"2.2.2.2": "JP"}}
	lo := newFakeLockout()
	s := New(Deps{
		Cfg:     config.RiskConfig{Enabled: true},
		Devices: dev, Users: users, Mailer: mailer, GeoIP: geo, Lockout: lo,
	})
	s.RegisterLoginSuccess(context.Background(), 1, "u@x", "2.2.2.2", "Mozilla/5.0", "en")
	if mailer.kind != "geo_change" {
		t.Fatalf("expected geo_change alert, got %q", mailer.kind)
	}
	if users.updated != "JP" {
		t.Fatalf("expected last country JP, got %q", users.updated)
	}
	devices, _ := dev.ListByUser(context.Background(), 1)
	if len(devices) != 1 || devices[0].Country != "JP" {
		t.Fatalf("device upsert failed: %+v", devices)
	}
}

func TestRegisterLoginSuccessClearsFail(t *testing.T) {
	lo := newFakeLockout()
	s := New(Deps{Cfg: config.RiskConfig{Enabled: true, LoginLockThreshold: 3, LoginLockDuration: time.Minute}, Lockout: lo})
	s.RegisterLoginFailure(context.Background(), "u@x")
	s.RegisterLoginFailure(context.Background(), "u@x")
	s.RegisterLoginSuccess(context.Background(), 1, "u@x", "1.1.1.1", "ua", "en")
	if lo.fails[failKey("u@x")] != 0 {
		t.Fatalf("fail counter should be reset")
	}
}

func TestSubGuardRevoke(t *testing.T) {
	sub := &fakeSubIPs{uniq: 9}
	users := &fakeUsers{email: "u@x"}
	mailer := &fakeMailer{}
	s := New(Deps{
		Cfg:    config.RiskConfig{Enabled: true, SubRevokeThreshold: 10, SubWindow: time.Hour},
		SubIPs: sub, Users: users, Mailer: mailer,
	})
	ok, err := s.SubGuard(context.Background(), 1, "abc", "1.1.1.1")
	if ok || !errors.Is(err, ErrSubTokenRevoked) {
		t.Fatalf("expected revoke at threshold, got ok=%v err=%v", ok, err)
	}
	if users.rotated == "" {
		t.Fatal("expected token rotation")
	}
	if mailer.kind != "sub_revoked" {
		t.Fatal("expected sub_revoked alert")
	}
}

func TestSubGuardBelowThreshold(t *testing.T) {
	sub := &fakeSubIPs{uniq: 2}
	s := New(Deps{
		Cfg:    config.RiskConfig{Enabled: true, SubRevokeThreshold: 10, SubWindow: time.Hour},
		SubIPs: sub,
	})
	ok, err := s.SubGuard(context.Background(), 1, "abc", "1.1.1.1")
	if !ok || err != nil {
		t.Fatalf("below threshold should allow; ok=%v err=%v", ok, err)
	}
}

func TestRotateSubscriptionToken(t *testing.T) {
	users := &fakeUsers{}
	s := New(Deps{Users: users})
	tok, err := s.RotateSubscriptionToken(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(tok, users.rotated) || tok == "" {
		t.Fatalf("rotate mismatch: tok=%q updated=%q", tok, users.rotated)
	}
}
