// Package domain holds aggregate types for the ticket bounded context.
package domain

import (
	"errors"
	"time"
)

type Status string

const (
	StatusOpen     Status = "open"
	StatusPending  Status = "pending"
	StatusResolved Status = "resolved"
	StatusClosed   Status = "closed"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

type SenderType string

const (
	SenderUser  SenderType = "user"
	SenderAdmin SenderType = "admin"
)

type Ticket struct {
	ID         uint64
	UserID     uint64
	Subject    string
	Category   string
	Priority   Priority
	Status     Status
	AssigneeID *uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Message struct {
	ID          uint64
	TicketID    uint64
	SenderID    uint64
	SenderType  SenderType
	Body        string
	Attachments []string
	CreatedAt   time.Time
}

var (
	ErrNotFound  = errors.New("ticket not found")
	ErrForbidden = errors.New("not allowed")
	ErrClosed    = errors.New("ticket closed")
)
