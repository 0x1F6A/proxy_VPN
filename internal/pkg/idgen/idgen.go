// Package idgen provides deterministic random identifiers shared across the
// codebase (invite codes, subscription tokens, jti, etc).
package idgen

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

// UUID returns a RFC 4122 v4 UUID.
func UUID() string { return uuid.NewString() }

// HexN returns a uniformly random hex string of length n (n must be even).
func HexN(n int) string {
	b := make([]byte, n/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// InviteCode returns an 8-char uppercase alphanumeric invite code. Excludes
// visually ambiguous characters (0/O, 1/I/L).
func InviteCode() string {
	const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// SubscriptionToken returns a 40-char hex token used in subscription URLs.
func SubscriptionToken() string { return HexN(40) }
