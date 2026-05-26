// Package service implements Phase 15-A risk-control use cases. The service
// is intentionally written to be optional: any nil dependency turns the
// corresponding rule into a no-op, so existing code paths keep working when
// risk is disabled in config.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/idgen"
	"github.com/0x1F6A/proxy_VPN/internal/risk/domain"
	"github.com/0x1F6A/proxy_VPN/internal/risk/ports"
)

// Deps wires every collaborator. All fields may be nil; methods degrade
// gracefully when a dependency is missing.
type Deps struct {
	Cfg      config.RiskConfig
	Devices  ports.DeviceRepo
	GeoIP    ports.CountryLookup
	Lockout  ports.LockoutStore
	SubIPs   ports.SubIPTracker
	Mailer   ports.AlertMailer
	Users    ports.UserLookup
}

// Service is the entry point for transport/handlers.
type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// ErrSubTokenRevoked 由 SubGuard 返回，表示订阅 token 已被风控自动吊销。
var ErrSubTokenRevoked = errors.New("subscription token revoked by risk control")

// ErrAccountLocked 由 PreLogin 返回；user-service 应停止后续密码校验。
var ErrAccountLocked = errors.New("account temporarily locked")

// Fingerprint 计算稳定的设备指纹：sha256(ua + accept-language + ip/24)。
// IP 截断到 /24 是为了让同一家 NAT 反复登录不会被算成新设备。
func Fingerprint(ua, acceptLang, ip string) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(ua))))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(strings.TrimSpace(acceptLang))))
	h.Write([]byte{0})
	h.Write([]byte(slashTwentyFour(ip)))
	return hex.EncodeToString(h.Sum(nil))
}

func slashTwentyFour(ip string) string {
	// IPv4 only — strip last octet. For IPv6 keep first 64 bits via simple
	// colon-split (best-effort).
	if i := strings.LastIndex(ip, "."); i > 0 && strings.Count(ip, ".") == 3 {
		return ip[:i] + ".0"
	}
	parts := strings.Split(ip, ":")
	if len(parts) > 4 {
		return strings.Join(parts[:4], ":") + "::"
	}
	return ip
}

// PreLogin 在用户输密前调用：返回 ErrAccountLocked 时 caller 应停止后续步骤。
// 当 Cfg.Enabled=false 或 Lockout=nil 时永远返回 nil。
func (s *Service) PreLogin(ctx context.Context, email string) error {
	if !s.d.Cfg.Enabled || s.d.Lockout == nil {
		return nil
	}
	locked, err := s.d.Lockout.IsLocked(ctx, lockKey(email))
	if err != nil || !locked {
		return nil
	}
	return ErrAccountLocked
}

// RegisterLoginFailure 在密码错误 / 2FA 错误后调用。达到阈值后写锁定 key。
func (s *Service) RegisterLoginFailure(ctx context.Context, email string) {
	if !s.d.Cfg.Enabled || s.d.Lockout == nil {
		return
	}
	n, _ := s.d.Lockout.IncrFail(ctx, failKey(email), s.d.Cfg.LoginLockDuration)
	if s.d.Cfg.LoginLockThreshold > 0 && n >= s.d.Cfg.LoginLockThreshold {
		_ = s.d.Lockout.Lock(ctx, lockKey(email), s.d.Cfg.LoginLockDuration)
	}
}

// RegisterLoginSuccess 登录成功后调用：清失败计数、登记设备、地理风控告警。
// 任何 sub-step 出错只记录不返回，避免影响正常登录流程。
func (s *Service) RegisterLoginSuccess(ctx context.Context, userID uint64, email, ip, ua, acceptLang string) {
	if !s.d.Cfg.Enabled {
		return
	}
	if s.d.Lockout != nil {
		_ = s.d.Lockout.ResetFail(ctx, failKey(email))
	}
	country := ""
	if s.d.GeoIP != nil {
		country = s.d.GeoIP.Country(ip)
	}
	if s.d.Devices != nil {
		now := time.Now()
		_ = s.d.Devices.Upsert(ctx, &domain.LoginDevice{
			UserID:      userID,
			FPHash:      Fingerprint(ua, acceptLang, ip),
			IP:          ip,
			UserAgent:   ua,
			Country:     country,
			FirstSeenAt: now,
			LastSeenAt:  now,
		})
	}
	// Geo alert: notify on country change vs last login.
	if s.d.Users != nil && country != "" {
		_, locale, prev, err := s.d.Users.EmailAndCountry(ctx, userID)
		if err == nil && prev != "" && !strings.EqualFold(prev, country) && s.d.Mailer != nil {
			_ = s.d.Mailer.SendRiskAlert(ctx, email, locale, "geo_change", map[string]string{
				"prev_country": prev, "new_country": country, "ip": ip,
			})
		}
		_ = s.d.Users.UpdateLastCountry(ctx, userID, country)
	}
}

// ListDevices / RevokeDevice 暴露给 admin handler。
func (s *Service) ListDevices(ctx context.Context, userID uint64) ([]domain.LoginDevice, error) {
	if s.d.Devices == nil {
		return nil, nil
	}
	return s.d.Devices.ListByUser(ctx, userID)
}

func (s *Service) RevokeDevice(ctx context.Context, userID uint64, fpHash string) error {
	if s.d.Devices == nil {
		return nil
	}
	return s.d.Devices.Revoke(ctx, userID, fpHash, time.Now())
}

// RotateSubscriptionToken 用户主动 / admin 强制轮换订阅 token；返回新 token。
func (s *Service) RotateSubscriptionToken(ctx context.Context, userID uint64) (string, error) {
	if s.d.Users == nil {
		return "", errors.New("user lookup not configured")
	}
	newTok := idgen.SubscriptionToken()
	if err := s.d.Users.RotateSubscriptionToken(ctx, userID, newTok, time.Now()); err != nil {
		return "", err
	}
	return newTok, nil
}

// SubGuard 订阅请求处理前调用：window 内 unique IP 达 SubRevokeThreshold 时
// 自动 rotate token 并发邮件告警；达 SubMaxIPs 时仅标记返回 false（处理路径
// 仍正常返回订阅，让用户感知"被分享了"而非粗暴拒绝服务）。返回 true=允许，
// false=应拒绝（token 已 revoke）。
func (s *Service) SubGuard(ctx context.Context, userID uint64, token, ip string) (bool, error) {
	if !s.d.Cfg.Enabled || s.d.SubIPs == nil {
		return true, nil
	}
	window := s.d.Cfg.SubWindow
	if window <= 0 {
		window = time.Hour
	}
	uniq, err := s.d.SubIPs.Touch(ctx, token, ip, window)
	if err != nil {
		// fail open
		return true, nil
	}
	if s.d.Cfg.SubRevokeThreshold > 0 && uniq >= s.d.Cfg.SubRevokeThreshold && s.d.Users != nil {
		_, _ = s.RotateSubscriptionToken(ctx, userID)
		if s.d.Mailer != nil {
			email, locale, _, lerr := s.d.Users.EmailAndCountry(ctx, userID)
			if lerr == nil {
				_ = s.d.Mailer.SendRiskAlert(ctx, email, locale, "sub_revoked", map[string]string{
					"unique_ips": itoa(uniq),
				})
			}
		}
		return false, ErrSubTokenRevoked
	}
	return true, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func failKey(email string) string { return "risk:fail:" + strings.ToLower(email) }
func lockKey(email string) string { return "risk:lock:" + strings.ToLower(email) }
