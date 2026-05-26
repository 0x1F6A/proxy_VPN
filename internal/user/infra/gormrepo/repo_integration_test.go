//go:build integration

package gormrepo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/testsupport"
	"github.com/0x1F6A/proxy_VPN/internal/user/domain"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/gormrepo"
)

func TestUserRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	repo := gormrepo.NewUserRepo(db)
	ctx := context.Background()

	u := &domain.User{
		Email: "alice@example.com", PasswordHash: "argon2id$hash",
		UUID: "00000000-0000-0000-0000-000000000001", Role: "user", Status: 1,
		BalanceCNY: "0.00", DeviceLimit: 3,
		SubscriptionToken: "tok-alice-0000000000000000000000000000",
		InviteCode:        "ALICE001",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatalf("expected generated id")
	}

	got, err := repo.FindByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != u.ID || got.Role != "user" {
		t.Fatalf("unexpected user: %+v", got)
	}

	_, err = repo.FindByEmail(ctx, "missing@example.com")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	if err := repo.UpdatePassword(ctx, u.ID, "argon2id$new"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got2, _ := repo.FindByID(ctx, u.ID)
	if got2.PasswordHash != "argon2id$new" {
		t.Fatalf("password not updated: %q", got2.PasswordHash)
	}

	if err := repo.UpdateLogin(ctx, u.ID, time.Now().UTC(), "1.2.3.4"); err != nil {
		t.Fatalf("UpdateLogin: %v", err)
	}
	if err := repo.UpdateTOTP(ctx, u.ID, "JBSWY3DPEHPK3PXP", true); err != nil {
		t.Fatalf("UpdateTOTP: %v", err)
	}
	got3, _ := repo.FindByID(ctx, u.ID)
	if !got3.TOTPEnabled || got3.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("TOTP not updated: %+v", got3)
	}
}

func TestRefreshRepoIntegration(t *testing.T) {
	db := testsupport.StartMySQL(t)
	userRepo := gormrepo.NewUserRepo(db)
	refRepo := gormrepo.NewRefreshRepo(db)
	ctx := context.Background()

	u := &domain.User{
		Email: "bob@example.com", PasswordHash: "h", UUID: "00000000-0000-0000-0000-000000000002",
		Role: "user", Status: 1, BalanceCNY: "0.00", DeviceLimit: 3,
		SubscriptionToken: "tok-bob-00000000000000000000000000000000",
		InviteCode:        "BOB00001",
	}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	rt := &domain.RefreshToken{
		UserID: u.ID, TokenHash: "sha256-hash-1", UserAgent: "test-agent",
		IP: "127.0.0.1", ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := refRepo.Create(ctx, rt); err != nil {
		t.Fatalf("create refresh: %v", err)
	}
	if rt.ID == 0 {
		t.Fatalf("missing id")
	}

	got, err := refRepo.FindByHash(ctx, "sha256-hash-1")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if got.UserID != u.ID || !got.IsActive(time.Now()) {
		t.Fatalf("unexpected refresh: %+v", got)
	}

	if err := refRepo.Revoke(ctx, rt.ID, time.Now()); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	got2, _ := refRepo.FindByHash(ctx, "sha256-hash-1")
	if got2.RevokedAt == nil || got2.IsActive(time.Now()) {
		t.Fatalf("expected revoked, got %+v", got2)
	}
}
