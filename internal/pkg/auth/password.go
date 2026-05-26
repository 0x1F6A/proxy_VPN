// Package auth groups primitives that protect user credentials and sessions:
// password hashing (bcrypt), JWT issuance/verification, and TOTP helpers.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash with cost 12 — a balanced default for
// servers as of 2024.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword verifies a plain-text password against a stored bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
