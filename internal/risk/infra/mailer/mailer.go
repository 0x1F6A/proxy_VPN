// Package mailer adapts the existing smtpmail / log mailer to risk's
// AlertMailer interface, formatting subject+body per locale via i18n.Bundle.
package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/i18n"
)

type RiskMailer struct {
	cfg    config.SMTPConfig
	bundle *i18n.Bundle
}

func New(cfg config.SMTPConfig, bundle *i18n.Bundle) *RiskMailer {
	return &RiskMailer{cfg: cfg, bundle: bundle}
}

// SendRiskAlert 根据 kind 渲染告警邮件。kind 取值：geo_change / sub_revoked。
func (m *RiskMailer) SendRiskAlert(ctx context.Context, to, locale, kind string, args map[string]string) error {
	if to == "" {
		return nil
	}
	ctx = i18n.WithLocale(ctx, locale)
	subject := m.bundle.T(ctx, "email.risk_alert_subject")
	body := renderBody(kind, args)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, to, subject, body))

	if m.cfg.Host == "" || m.cfg.Port == 0 {
		fmt.Printf("[risk-mail] to=%s kind=%s args=%v\n", to, kind, args)
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, msg)
}

func renderBody(kind string, args map[string]string) string {
	var b strings.Builder
	switch kind {
	case "geo_change":
		fmt.Fprintf(&b, "Unusual login detected.\nPrevious country: %s\nNew country: %s\nIP: %s\n",
			args["prev_country"], args["new_country"], args["ip"])
	case "sub_revoked":
		fmt.Fprintf(&b, "Subscription token revoked due to suspected sharing (unique IPs: %s).\nPlease rotate your subscription URL.\n",
			args["unique_ips"])
	default:
		fmt.Fprintf(&b, "Risk alert: %s\n%+v\n", kind, args)
	}
	return b.String()
}
