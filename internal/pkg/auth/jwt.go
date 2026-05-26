package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload signed for access tokens. UID is the primary user
// id; Role distinguishes user/admin/ops/finance for coarse-grained auth checks
// in middleware.
type Claims struct {
	UID  uint64 `json:"uid"`
	Role string `json:"role"`
	JTI  string `json:"jti"`
	jwt.RegisteredClaims
}

// JWT issues and parses HS256 access tokens. The Signer is intentionally tiny
// so it can be constructed once at startup and shared safely.
type JWT struct {
	secret    []byte
	ttl       time.Duration
	issuer    string
	clockSkew time.Duration
}

// NewJWT builds a signer from raw configuration values.
func NewJWT(secret string, ttl time.Duration, issuer string, skew time.Duration) *JWT {
	return &JWT{secret: []byte(secret), ttl: ttl, issuer: issuer, clockSkew: skew}
}

// Issue returns a signed JWT and its jti. The caller stores jti only if it
// needs revocation (we keep a Redis blacklist on logout).
func (j *JWT) Issue(uid uint64, role, jti string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(j.ttl)
	claims := Claims{
		UID:  uid,
		Role: role,
		JTI:  jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-j.clockSkew)),
			ExpiresAt: jwt.NewNumericDate(exp),
			Subject:   fmt.Sprintf("%d", uid),
			ID:        jti,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(j.secret)
	return s, exp, err
}

// Parse validates the signature and returns the claims if valid.
func (j *JWT) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	}, jwt.WithLeeway(j.clockSkew), jwt.WithIssuer(j.issuer))
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
