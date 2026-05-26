package domain

import "errors"

var (
	ErrNodeNotFound       = errors.New("node not found")
	ErrNodeGroupNotFound  = errors.New("node group not found")
	ErrUnsupportedProto   = errors.New("unsupported protocol")
	ErrBootstrapForbidden = errors.New("node bootstrap secret mismatch")
	ErrNodeAuth           = errors.New("invalid node token")
	ErrSubTokenInvalid    = errors.New("invalid subscription token")
	ErrSubFormatUnknown   = errors.New("unknown subscription format")
	ErrNoPlanGranted      = errors.New("user has no active plan")
)
