// Package tasks defines the asynq task types and their handlers.
// Tasks are intentionally thin: they delegate to bounded-context services
// so all business logic stays in service packages and is unit-testable
// without asynq involvement.
package tasks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
)

// Task type names — keep stable across releases.
const (
	TypeAutoCancelOrders   = "billing:auto_cancel_orders"
	TypeMarkStaleNodes     = "node:mark_stale"
	TypeReconcilePayments  = "payment:reconcile_channel"
	TypeExpirePayments     = "payment:expire_pending"
	TypeScanUSDTBlock      = "payment:scan_usdt_block"
)

// Payloads ---------------------------------------------------------------

type ReconcileChannelPayload struct {
	Channel domain.Channel `json:"channel"`
	Limit   int            `json:"limit"`
}

// Constructors -----------------------------------------------------------

func NewAutoCancelOrders() *asynq.Task {
	return asynq.NewTask(TypeAutoCancelOrders, nil)
}
func NewMarkStaleNodes() *asynq.Task {
	return asynq.NewTask(TypeMarkStaleNodes, nil)
}
func NewExpirePayments() *asynq.Task {
	return asynq.NewTask(TypeExpirePayments, nil)
}
func NewReconcileChannel(channel domain.Channel, limit int) *asynq.Task {
	b, _ := json.Marshal(ReconcileChannelPayload{Channel: channel, Limit: limit})
	return asynq.NewTask(TypeReconcilePayments, b)
}
func NewScanUSDTBlock() *asynq.Task {
	return asynq.NewTask(TypeScanUSDTBlock, nil)
}

// Handlers ---------------------------------------------------------------

// Deps wires the bounded-context services that tasks need to call.
type Deps struct {
	BillingExpire    func(ctx context.Context) (int64, error)
	NodeMarkStale    func(ctx context.Context, cutoff time.Time) (int64, error)
	NodeStaleCutoff  func() time.Time
	PaymentReconcile func(ctx context.Context, channel domain.Channel, limit int) (int, error)
	PaymentExpire    func(ctx context.Context) (int64, error)
	USDTStep         func(ctx context.Context) (int, error)
	Log              func(string, ...any)
}

// Mount registers all task handlers on the given mux.
func Mount(mux *asynq.ServeMux, d Deps) {
	if d.Log == nil {
		d.Log = func(string, ...any) {}
	}
	mux.HandleFunc(TypeAutoCancelOrders, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.BillingExpire(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("billing.auto_cancel", "count", n)
		}
		return nil
	})
	mux.HandleFunc(TypeMarkStaleNodes, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.NodeMarkStale(ctx, d.NodeStaleCutoff())
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("node.mark_stale", "count", n)
		}
		return nil
	})
	mux.HandleFunc(TypeExpirePayments, func(ctx context.Context, _ *asynq.Task) error {
		n, err := d.PaymentExpire(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("payment.expire_pending", "count", n)
		}
		return nil
	})
	mux.HandleFunc(TypeReconcilePayments, func(ctx context.Context, t *asynq.Task) error {
		var p ReconcileChannelPayload
		if len(t.Payload()) > 0 {
			_ = json.Unmarshal(t.Payload(), &p)
		}
		if p.Limit <= 0 {
			p.Limit = 50
		}
		n, err := d.PaymentReconcile(ctx, p.Channel, p.Limit)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("payment.reconcile", "channel", string(p.Channel), "count", n)
		}
		return nil
	})
	mux.HandleFunc(TypeScanUSDTBlock, func(ctx context.Context, _ *asynq.Task) error {
		if d.USDTStep == nil {
			return nil
		}
		n, err := d.USDTStep(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("payment.usdt_scan", "matched", n)
		}
		return nil
	})
}
