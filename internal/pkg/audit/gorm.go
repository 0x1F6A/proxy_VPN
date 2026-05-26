// GORM-backed Writer for admin_audit_logs.
package audit

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

type auditRow struct {
	ID        uint64 `gorm:"primaryKey"`
	AdminID   uint64 `gorm:"column:admin_id"`
	Action    string
	Target    *string
	Before    *string `gorm:"column:before;type:json"`
	After     *string `gorm:"column:after;type:json"`
	IP        *string `gorm:"column:ip"`
	UserAgent *string `gorm:"column:user_agent"`
	CreatedAt time.Time
}

func (auditRow) TableName() string { return "admin_audit_logs" }

// GormWriter persists audit Records to the admin_audit_logs table. Failures
// are logged via slog at warn level — we never block the caller's request
// on an audit write failure.
type GormWriter struct {
	db  *gorm.DB
	log *slog.Logger
}

func NewGormWriter(db *gorm.DB, log *slog.Logger) *GormWriter {
	if log == nil {
		log = slog.Default()
	}
	return &GormWriter{db: db, log: log}
}

func (g *GormWriter) Write(ctx context.Context, rec Record) {
	var target, ip, ua *string
	if rec.Target != "" {
		target = &rec.Target
	}
	if rec.IP != "" {
		ip = &rec.IP
	}
	if rec.UserAgent != "" {
		ua = &rec.UserAgent
	}
	var after *string
	if rec.Payload != "" {
		// Best-effort: stash payload in the `after` JSON column wrapped as a
		// string. Avoids extra ALTER TABLE for a dedicated payload column.
		s := `{"payload":` + jsonString(rec.Payload) + `}`
		after = &s
	}
	row := &auditRow{
		AdminID:   rec.AdminID,
		Action:    rec.Action,
		Target:    target,
		After:     after,
		IP:        ip,
		UserAgent: ua,
		CreatedAt: rec.CreatedAt,
	}
	if err := g.db.WithContext(ctx).Create(row).Error; err != nil {
		g.log.Warn("audit write failed",
			"admin_id", rec.AdminID,
			"action", rec.Action,
			"err", err)
	}
}

// jsonString returns a minimal JSON-encoded string literal of s.
func jsonString(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				out = append(out, '\\', 'u', '0', '0',
					hex[c>>4], hex[c&0x0F])
			} else {
				out = append(out, c)
			}
		}
	}
	out = append(out, '"')
	return string(out)
}

const hex = "0123456789abcdef"
