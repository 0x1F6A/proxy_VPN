package nodecfg

import (
	"strconv"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
)

// ErrUnsupportedProtocol signals an inbound.Protocol the renderer doesn't know.
type ErrUnsupportedProtocol struct{ Protocol string }

func (e ErrUnsupportedProtocol) Error() string {
	return "nodecfg: unsupported protocol: " + e.Protocol
}

// IsSupportedProtocol reports whether the given protocol can be rendered by
// either engine.
func IsSupportedProtocol(p string) bool {
	switch p {
	case domain.ProtocolVLESSReality, domain.ProtocolTrojan,
		domain.ProtocolHysteria2, domain.ProtocolSS2022:
		return true
	}
	return false
}

func userEmail(id uint64) string { return "u" + strconv.FormatUint(id, 10) }

func emptyOr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
