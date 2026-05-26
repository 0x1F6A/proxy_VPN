package proxyvpn

import "fmt"

// APIError represents a non-zero business code returned by the proxy_VPN API.
type APIError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("proxyvpn: api error code=%d http=%d msg=%s req=%s", e.Code, e.HTTPStatus, e.Message, e.RequestID)
}

// Known error codes (mirrors docs/api.md §1.4). Use errors.Is for matching:
//
//	if errors.Is(err, proxyvpn.ErrAccessTokenInvalid) { ... }
var (
	ErrBadRequest          = &APIError{Code: 40001, Message: "bad request"}
	ErrAccessTokenInvalid  = &APIError{Code: 40101, Message: "access token invalid"}
	ErrRefreshTokenInvalid = &APIError{Code: 40102, Message: "refresh token invalid"}
	ErrForbidden           = &APIError{Code: 40301, Message: "forbidden"}
	ErrAccountDisabled     = &APIError{Code: 40302, Message: "account disabled"}
	ErrNotFound            = &APIError{Code: 40401, Message: "not found"}
	ErrConflict            = &APIError{Code: 40901, Message: "conflict"}
	ErrRateLimited         = &APIError{Code: 42901, Message: "rate limited"}
	ErrInternal            = &APIError{Code: 50001, Message: "internal error"}
	ErrNodeUnavailable     = &APIError{Code: 50301, Message: "node unavailable"}
	ErrInsufficientBalance = &APIError{Code: 60001, Message: "insufficient balance"}
	ErrPlanExpired         = &APIError{Code: 60002, Message: "plan expired"}
	ErrQuotaExhausted      = &APIError{Code: 60003, Message: "quota exhausted"}
	ErrOrderTimeout        = &APIError{Code: 60101, Message: "order timeout"}
	ErrAmountMismatch      = &APIError{Code: 60102, Message: "amount mismatch"}
)

// Is supports errors.Is by comparing on Code only (Message/RequestID vary per call).
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
