package domain

import "errors"

// Sentinel domain errors. Transport layer maps these to HTTP status codes.
var (
	ErrEmailTaken       = errors.New("email already registered")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserDisabled     = errors.New("user disabled")
	ErrUserPending      = errors.New("user pending activation")

	ErrCodeNotFound     = errors.New("verification code not found")
	ErrCodeExpired      = errors.New("verification code expired")
	ErrCodeMismatch     = errors.New("verification code mismatch")
	ErrCodeRateLimited  = errors.New("verification code requested too frequently")
	ErrCodeMaxAttempts  = errors.New("verification code attempts exceeded")

	ErrRefreshInvalid   = errors.New("refresh token invalid or revoked")

	ErrTOTPRequired     = errors.New("totp code required")
	ErrTOTPInvalid      = errors.New("totp code invalid")
	ErrTOTPNotEnrolled  = errors.New("totp not enrolled")
	ErrTOTPAlreadyEnrolled = errors.New("totp already enrolled")

	ErrPasswordWeak     = errors.New("password too weak (min 8 chars)")
	ErrEmailInvalid     = errors.New("email format invalid")

	ErrForbidden        = errors.New("forbidden")
)
