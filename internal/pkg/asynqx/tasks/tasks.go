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
	TypeTrafficFlushCH     = "traffic:flush_ch_buffer"
	TypeTrafficRollupDaily = "traffic:rollup_daily"
	TypeTrafficRecomputeBan = "traffic:recompute_bans"
	TypeSLAProbe            = "sla:probe"
	TypeSLARollupDaily      = "sla:rollup_daily"
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
func NewTrafficFlushCH() *asynq.Task    { return asynq.NewTask(TypeTrafficFlushCH, nil) }
func NewTrafficRollupDaily() *asynq.Task { return asynq.NewTask(TypeTrafficRollupDaily, nil) }
func NewTrafficRecomputeBans() *asynq.Task {
	return asynq.NewTask(TypeTrafficRecomputeBan, nil)
}
func NewSLAProbe() *asynq.Task       { return asynq.NewTask(TypeSLAProbe, nil) }
func NewSLARollupDaily() *asynq.Task { return asynq.NewTask(TypeSLARollupDaily, nil) }

// Handlers ---------------------------------------------------------------

// Deps wires the bounded-context services that tasks need to call.
type Deps struct {
	BillingExpire    func(ctx context.Context) (int64, error)
	NodeMarkStale    func(ctx context.Context, cutoff time.Time) (int64, error)
	NodeStaleCutoff  func() time.Time
	PaymentReconcile func(ctx context.Context, channel domain.Channel, limit int) (int, error)
	PaymentExpire    func(ctx context.Context) (int64, error)
	USDTStep         func(ctx context.Context) (int, error)
	TrafficFlushCH   func(ctx context.Context) error
	TrafficRollup    func(ctx context.Context) (int64, error)
	TrafficRecompute func(ctx context.Context) (int, int, error)
	SLAProbe         func(ctx context.Context) error
	SLARollupDaily   func(ctx context.Context) (int, error)
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
	mux.HandleFunc(TypeTrafficFlushCH, func(ctx context.Context, _ *asynq.Task) error {
		if d.TrafficFlushCH == nil {
			return nil
		}
		return d.TrafficFlushCH(ctx)
	})
	mux.HandleFunc(TypeTrafficRollupDaily, func(ctx context.Context, _ *asynq.Task) error {
		if d.TrafficRollup == nil {
			return nil
		}
		n, err := d.TrafficRollup(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("traffic.rollup", "rows", n)
		}
		return nil
	})
	mux.HandleFunc(TypeTrafficRecomputeBan, func(ctx context.Context, _ *asynq.Task) error {
		if d.TrafficRecompute == nil {
			return nil
		}
		banned, unbanned, err := d.TrafficRecompute(ctx)
		if err != nil {
			return err
		}
		if banned > 0 || unbanned > 0 {
			d.Log("traffic.recompute_bans", "banned", banned, "unbanned", unbanned)
		}
		return nil
	})
	mux.HandleFunc(TypeSLAProbe, func(ctx context.Context, _ *asynq.Task) error {
		if d.SLAProbe == nil {
			return nil
		}
		return d.SLAProbe(ctx)
	})
	mux.HandleFunc(TypeSLARollupDaily, func(ctx context.Context, _ *asynq.Task) error {
		if d.SLARollupDaily == nil {
			return nil
		}
		n, err := d.SLARollupDaily(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			d.Log("sla.rollup_daily", "rows", n)
		}
		return nil
	})
}
