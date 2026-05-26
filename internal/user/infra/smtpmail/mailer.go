// Package smtpmail sends transactional email via SMTP. In dev we point at
// MailHog (127.0.0.1:1025) which does not require auth or TLS.
package smtpmail

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
)

type Mailer struct{ cfg config.SMTPConfig }

func New(cfg config.SMTPConfig) *Mailer { return &Mailer{cfg: cfg} }

func (m *Mailer) SendCode(ctx context.Context, to, scene, code string) error {
	subject := "[proxy_VPN] 验证码"
	body := fmt.Sprintf("您的验证码为：%s（场景：%s，10 分钟内有效）。", code, scene)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, to, subject, body))

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, msg)
}

// LogMailer is a development fallback that prints codes to stdout instead of
// dispatching SMTP. Useful when running tests without MailHog.
type LogMailer struct{}

func (LogMailer) SendCode(_ context.Context, to, scene, code string) error {
	fmt.Printf("[mail] to=%s scene=%s code=%s\n", to, scene, code)
	return nil
}
