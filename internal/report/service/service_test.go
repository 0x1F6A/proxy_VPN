package service

import (
	"context"
	"testing"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/report/ports"
)

type fakeRepo struct {
	rev      []ports.RevenuePoint
	traf     []ports.TrafficPoint
	statuses []ports.OrderStatusCount
	snap     ports.DashboardSnapshot
}

func (f *fakeRepo) RevenueDaily(_ context.Context, _, _ time.Time) ([]ports.RevenuePoint, error) {
	return f.rev, nil
}
func (f *fakeRepo) TrafficDaily(_ context.Context, _, _ time.Time) ([]ports.TrafficPoint, error) {
	return f.traf, nil
}
func (f *fakeRepo) OrderStatusCounts(_ context.Context, _, _ time.Time) ([]ports.OrderStatusCount, error) {
	return f.statuses, nil
}
func (f *fakeRepo) Dashboard(_ context.Context, _ time.Time) (ports.DashboardSnapshot, error) {
	return f.snap, nil
}

func TestService_RangeValidation(t *testing.T) {
	s := New(&fakeRepo{})
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := s.RevenueDaily(ctx, now, now); err != ErrRangeInvalid {
		t.Fatalf("want ErrRangeInvalid, got %v", err)
	}
	if _, err := s.TrafficDaily(ctx, now.Add(-400*24*time.Hour), now); err != ErrRangeTooLong {
		t.Fatalf("want ErrRangeTooLong, got %v", err)
	}
}

func TestService_Passthrough(t *testing.T) {
	want := []ports.RevenuePoint{{Day: time.Now(), OrderCnt: 3, PaidCents: 1299}}
	s := New(&fakeRepo{rev: want})
	got, err := s.RevenueDaily(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PaidCents != 1299 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestService_Dashboard(t *testing.T) {
	s := New(&fakeRepo{snap: ports.DashboardSnapshot{
		UsersTotal: 10, UsersActive: 6, OrdersToday: 2, RevenueTodayCNY: "199.00",
	}})
	got, err := s.Dashboard(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.UsersTotal != 10 || got.RevenueTodayCNY != "199.00" {
		t.Fatalf("unexpected snap: %+v", got)
	}
}
