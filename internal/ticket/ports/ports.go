// Package ports defines the storage contract for the ticket service. The
// gormrepo package is one concrete implementation; tests use an in-memory
// fake.
package ports

import (
	"context"

	"github.com/0x1F6A/proxy_VPN/internal/ticket/domain"
)

type ListFilter struct {
	UserID     uint64 // 0 = no filter (admin)
	Status     string
	Priority   string
	AssigneeID uint64
	Keyword    string
	Limit      int
	Offset     int
}

type Repo interface {
	Create(ctx context.Context, t *domain.Ticket, firstMsg *domain.Message) error
	Get(ctx context.Context, id uint64) (*domain.Ticket, error)
	List(ctx context.Context, f ListFilter) ([]domain.Ticket, int64, error)
	Update(ctx context.Context, t *domain.Ticket) error
	AppendMessage(ctx context.Context, m *domain.Message) error
	ListMessages(ctx context.Context, ticketID uint64) ([]domain.Message, error)
}
