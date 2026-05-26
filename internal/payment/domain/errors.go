package domain

import "errors"

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrChannelUnsupported   = errors.New("payment channel not supported")
	ErrPaymentExpired       = errors.New("payment expired")
	ErrSignatureInvalid     = errors.New("callback signature invalid")
	ErrAmountMismatch       = errors.New("callback amount mismatch")
	ErrPaymentAlreadyPaid   = errors.New("payment already paid")
	ErrNoAddressAvailable   = errors.New("no usdt address available in pool")
	ErrUnknownTradeNo       = errors.New("unknown channel trade no")
	ErrProviderUnavailable  = errors.New("payment provider unavailable")
)
