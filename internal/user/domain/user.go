// Package domain holds the user-aggregate entities. These types are pure
// data + invariants — no framework / DB / HTTP imports allowed.
package domain

import "time"

const (
	StatusDisabled = 0
	StatusActive   = 1
	StatusPending  = 2
)

// User is the aggregate root for the user bounded context.
type User struct {
	ID                uint64
	Email             string
	PasswordHash      string
	UUID              string
	Role              string
	Status            int
	BalanceCNY        string // decimal as string to avoid float
	PlanID            *uint64
	PlanExpireAt      *time.Time
	TrafficTotal      uint64
	TrafficUsed       uint64
	TrafficResetAt    *time.Time
	DeviceLimit       uint32
	SubscriptionToken string
	TOTPSecret        string
	TOTPEnabled       bool
	InvitedBy         *uint64
	InviteCode        string
	LastLoginAt       *time.Time
	LastLoginIP       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	OIDCSubject       string
}

// RefreshToken represents one issued refresh token entry. We persist the
// sha256 hash, never the plaintext.
type RefreshToken struct {
	ID        uint64
	UserID    uint64
	TokenHash string
	UserAgent string
	IP        string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// IsActive reports whether the refresh token is usable right now.
func (r RefreshToken) IsActive(now time.Time) bool {
	return r.RevokedAt == nil && now.Before(r.ExpiresAt)
}

// EmailCode is a one-time numeric code emailed to users for registration,
// password reset, or email change.
type EmailCode struct {
	ID        uint64
	Email     string
	Scene     string
	CodeHash  string
	Attempts  uint8
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// LoginLog records each authentication attempt for audit + threat detection.
type LoginLog struct {
	UserID    *uint64
	Email     string
	Success   bool
	IP        string
	UserAgent string
	Reason    string
}
