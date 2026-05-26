// Package gormrepo persists tickets / messages to MySQL via GORM.
package gormrepo

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/ticket/domain"
	"github.com/0x1F6A/proxy_VPN/internal/ticket/ports"
)

type ticketRow struct {
	ID         uint64    `gorm:"primaryKey"`
	UserID     uint64    `gorm:"column:user_id"`
	Subject    string    `gorm:"column:subject"`
	Category   string    `gorm:"column:category"`
	Priority   string    `gorm:"column:priority"`
	Status     string    `gorm:"column:status"`
	AssigneeID *uint64   `gorm:"column:assignee_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (ticketRow) TableName() string { return "tickets" }

type messageRow struct {
	ID          uint64    `gorm:"primaryKey"`
	TicketID    uint64    `gorm:"column:ticket_id"`
	SenderID    uint64    `gorm:"column:sender_id"`
	SenderType  string    `gorm:"column:sender_type"`
	Body        string    `gorm:"column:body"`
	Attachments []byte    `gorm:"column:attachments"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (messageRow) TableName() string { return "ticket_messages" }

type Repo struct{ db *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, t *domain.Ticket, firstMsg *domain.Message) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := toTicketRow(t)
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		t.ID = row.ID
		if firstMsg != nil {
			firstMsg.TicketID = row.ID
			mr, err := toMessageRow(firstMsg)
			if err != nil {
				return err
			}
			if err := tx.Create(&mr).Error; err != nil {
				return err
			}
			firstMsg.ID = mr.ID
		}
		return nil
	})
}

func (r *Repo) Get(ctx context.Context, id uint64) (*domain.Ticket, error) {
	var row ticketRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	t := fromTicketRow(&row)
	return &t, nil
}

func (r *Repo) List(ctx context.Context, f ports.ListFilter) ([]domain.Ticket, int64, error) {
	q := r.db.WithContext(ctx).Model(&ticketRow{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Priority != "" {
		q = q.Where("priority = ?", f.Priority)
	}
	if f.AssigneeID > 0 {
		q = q.Where("assignee_id = ?", f.AssigneeID)
	}
	if k := strings.TrimSpace(f.Keyword); k != "" {
		q = q.Where("subject LIKE ?", "%"+k+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	var rows []ticketRow
	if err := q.Order("updated_at DESC").Limit(f.Limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.Ticket, 0, len(rows))
	for i := range rows {
		out = append(out, fromTicketRow(&rows[i]))
	}
	return out, total, nil
}

func (r *Repo) Update(ctx context.Context, t *domain.Ticket) error {
	row := toTicketRow(t)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *Repo) AppendMessage(ctx context.Context, m *domain.Message) error {
	mr, err := toMessageRow(m)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&mr).Error; err != nil {
		return err
	}
	m.ID = mr.ID
	return nil
}

func (r *Repo) ListMessages(ctx context.Context, ticketID uint64) ([]domain.Message, error) {
	var rows []messageRow
	if err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Message, 0, len(rows))
	for i := range rows {
		m, err := fromMessageRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func toTicketRow(t *domain.Ticket) ticketRow {
	return ticketRow{
		ID:         t.ID,
		UserID:     t.UserID,
		Subject:    t.Subject,
		Category:   t.Category,
		Priority:   string(t.Priority),
		Status:     string(t.Status),
		AssigneeID: t.AssigneeID,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func fromTicketRow(r *ticketRow) domain.Ticket {
	return domain.Ticket{
		ID:         r.ID,
		UserID:     r.UserID,
		Subject:    r.Subject,
		Category:   r.Category,
		Priority:   domain.Priority(r.Priority),
		Status:     domain.Status(r.Status),
		AssigneeID: r.AssigneeID,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func toMessageRow(m *domain.Message) (messageRow, error) {
	var att []byte
	if len(m.Attachments) > 0 {
		b, err := json.Marshal(m.Attachments)
		if err != nil {
			return messageRow{}, err
		}
		att = b
	}
	return messageRow{
		ID:          m.ID,
		TicketID:    m.TicketID,
		SenderID:    m.SenderID,
		SenderType:  string(m.SenderType),
		Body:        m.Body,
		Attachments: att,
		CreatedAt:   m.CreatedAt,
	}, nil
}

func fromMessageRow(r *messageRow) (domain.Message, error) {
	m := domain.Message{
		ID:         r.ID,
		TicketID:   r.TicketID,
		SenderID:   r.SenderID,
		SenderType: domain.SenderType(r.SenderType),
		Body:       r.Body,
		CreatedAt:  r.CreatedAt,
	}
	if len(r.Attachments) > 0 {
		var att []string
		if err := json.Unmarshal(r.Attachments, &att); err == nil {
			m.Attachments = att
		}
	}
	return m, nil
}
