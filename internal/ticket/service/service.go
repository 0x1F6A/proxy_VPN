// Package service implements ticket use cases. The state machine is:
//   open -> pending (user awaits reply) -> resolved -> closed.
// Replies from either party automatically reopen a resolved ticket.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/ticket/domain"
	"github.com/0x1F6A/proxy_VPN/internal/ticket/ports"
)

type Deps struct {
	Repo ports.Repo
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

type CreateInput struct {
	UserID      uint64
	Subject     string
	Category    string
	Priority    domain.Priority
	Body        string
	Attachments []string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Ticket, error) {
	if strings.TrimSpace(in.Subject) == "" || strings.TrimSpace(in.Body) == "" {
		return nil, domain.ErrForbidden
	}
	if !in.Priority.Valid() {
		in.Priority = domain.PriorityNormal
	}
	if in.Category == "" {
		in.Category = "general"
	}
	now := time.Now()
	t := &domain.Ticket{
		UserID:    in.UserID,
		Subject:   in.Subject,
		Category:  in.Category,
		Priority:  in.Priority,
		Status:    domain.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	msg := &domain.Message{
		SenderID:    in.UserID,
		SenderType:  domain.SenderUser,
		Body:        in.Body,
		Attachments: in.Attachments,
		CreatedAt:   now,
	}
	if err := s.d.Repo.Create(ctx, t, msg); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Get(ctx context.Context, id, callerID uint64, isAdmin bool) (*domain.Ticket, []domain.Message, error) {
	t, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if !isAdmin && t.UserID != callerID {
		return nil, nil, domain.ErrForbidden
	}
	msgs, err := s.d.Repo.ListMessages(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, msgs, nil
}

func (s *Service) ListForUser(ctx context.Context, userID uint64, limit, offset int) ([]domain.Ticket, int64, error) {
	return s.d.Repo.List(ctx, ports.ListFilter{UserID: userID, Limit: limit, Offset: offset})
}

func (s *Service) ListForAdmin(ctx context.Context, f ports.ListFilter) ([]domain.Ticket, int64, error) {
	f.UserID = 0
	return s.d.Repo.List(ctx, f)
}

type ReplyInput struct {
	TicketID    uint64
	SenderID    uint64
	SenderType  domain.SenderType
	Body        string
	Attachments []string
}

func (s *Service) Reply(ctx context.Context, in ReplyInput, isAdmin bool) (*domain.Ticket, *domain.Message, error) {
	t, err := s.d.Repo.Get(ctx, in.TicketID)
	if err != nil {
		return nil, nil, err
	}
	if !isAdmin && t.UserID != in.SenderID {
		return nil, nil, domain.ErrForbidden
	}
	if t.Status == domain.StatusClosed {
		return nil, nil, domain.ErrClosed
	}
	now := time.Now()
	m := &domain.Message{
		TicketID:    t.ID,
		SenderID:    in.SenderID,
		SenderType:  in.SenderType,
		Body:        in.Body,
		Attachments: in.Attachments,
		CreatedAt:   now,
	}
	if err := s.d.Repo.AppendMessage(ctx, m); err != nil {
		return nil, nil, err
	}
	// Reply transitions: admin reply -> pending (awaiting user); user reply
	// reopens a resolved ticket back to open.
	switch in.SenderType {
	case domain.SenderAdmin:
		t.Status = domain.StatusPending
	case domain.SenderUser:
		if t.Status == domain.StatusResolved {
			t.Status = domain.StatusOpen
		}
	}
	t.UpdatedAt = now
	if err := s.d.Repo.Update(ctx, t); err != nil {
		return nil, nil, err
	}
	return t, m, nil
}

func (s *Service) Close(ctx context.Context, id, callerID uint64, isAdmin bool) error {
	t, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if !isAdmin && t.UserID != callerID {
		return domain.ErrForbidden
	}
	t.Status = domain.StatusClosed
	t.UpdatedAt = time.Now()
	return s.d.Repo.Update(ctx, t)
}

type PatchInput struct {
	Status     *domain.Status
	Priority   *domain.Priority
	AssigneeID *uint64
}

func (s *Service) AdminPatch(ctx context.Context, id uint64, in PatchInput) (*domain.Ticket, error) {
	t, err := s.d.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Status != nil {
		t.Status = *in.Status
	}
	if in.Priority != nil && in.Priority.Valid() {
		t.Priority = *in.Priority
	}
	if in.AssigneeID != nil {
		t.AssigneeID = in.AssigneeID
	}
	t.UpdatedAt = time.Now()
	if err := s.d.Repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Assign(ctx context.Context, id, assigneeID uint64) (*domain.Ticket, error) {
	a := assigneeID
	return s.AdminPatch(ctx, id, PatchInput{AssigneeID: &a})
}
