package auth

import (
	"bytes"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// EnrollTOTP provisions a new TOTP secret + otpauth URL + PNG QR-code bytes.
// account is shown in the authenticator app (typically the user's email).
func EnrollTOTP(issuer, account string) (secret, url string, qrPNG []byte, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", nil, err
	}
	img, err := key.Image(200, 200)
	if err != nil {
		return "", "", nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", nil, err
	}
	return key.Secret(), key.URL(), buf.Bytes(), nil
}

// ValidateTOTP returns true when the 6-digit code matches the secret within
// the standard ±1 step (≈30s) tolerance.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}
