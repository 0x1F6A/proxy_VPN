package usdtprov_test

import (
	"context"
	"sync"
	"testing"

	usdtprov "github.com/0x1F6A/proxy_VPN/internal/payment/provider/usdt"
)

type fakeClient struct {
	now  int64
	logs map[[2]int64][]usdtprov.TRC20Transfer
}

func (f *fakeClient) NowBlock(_ context.Context) (int64, error) { return f.now, nil }
func (f *fakeClient) ListTRC20Transfers(_ context.Context, _ string, from, to int64) ([]usdtprov.TRC20Transfer, error) {
	return f.logs[[2]int64{from, to}], nil
}

type fakeCursor struct {
	mu sync.Mutex
	v  int64
}

func (c *fakeCursor) Get(_ context.Context, _ string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v, nil
}
func (c *fakeCursor) Set(_ context.Context, _ string, v int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.v = v
	return nil
}

type fakeObserver struct{ called int }

func (o *fakeObserver) USDTReceived(_ context.Context, _, _, _ string) error {
	o.called++
	return nil
}

func TestScannerStepAdvancesCursor(t *testing.T) {
	cli := &fakeClient{now: 110, logs: map[[2]int64][]usdtprov.TRC20Transfer{
		{101, 110}: {
			{TxHash: "h1", To: "TA1", AmountToken: "10.0", Block: 105},
			{TxHash: "h2", To: "TA2", AmountToken: "20.0", Block: 108},
		},
	}}
	cur := &fakeCursor{v: 100}
	obs := &fakeObserver{}
	s := &usdtprov.Scanner{
		Client:        cli,
		Cursor:        cur,
		Confirmations: 0,
		ContractAddr:  "T-USDT",
		Observer:      obs,
		BatchBlocks:   50,
	}
	n, err := s.Step(context.Background())
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if n != 2 {
		t.Fatalf("matched=%d want 2", n)
	}
	if obs.called != 2 {
		t.Fatalf("observer called %d times, want 2", obs.called)
	}
	if cur.v != 110 {
		t.Fatalf("cursor=%d want 110", cur.v)
	}
}

func TestScannerStepNoNewBlocks(t *testing.T) {
	cli := &fakeClient{now: 100}
	cur := &fakeCursor{v: 100}
	obs := &fakeObserver{}
	s := &usdtprov.Scanner{Client: cli, Cursor: cur, Observer: obs, Confirmations: 5}
	n, err := s.Step(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("expected no-op, got n=%d err=%v", n, err)
	}
}
