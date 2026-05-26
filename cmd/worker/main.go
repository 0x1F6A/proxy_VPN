// Command worker hosts the asynq worker process: it consumes background
// tasks (auto-cancel orders, mark stale nodes, expire / reconcile payments,
// scan USDT) and runs the asynq Scheduler to enqueue periodic ones.
//
// Run alongside cmd/api in production. Both share the same Redis.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	billingrepo "github.com/0x1F6A/proxy_VPN/internal/billing/infra/gormrepo"
	billingsvc "github.com/0x1F6A/proxy_VPN/internal/billing/service"
	noderepo "github.com/0x1F6A/proxy_VPN/internal/node/infra/gormrepo"
	nodesvc "github.com/0x1F6A/proxy_VPN/internal/node/service"
	paydomain "github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	payrepo "github.com/0x1F6A/proxy_VPN/internal/payment/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
	paysvc "github.com/0x1F6A/proxy_VPN/internal/payment/service"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/asynqx"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/asynqx/tasks"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/storage"
	usergorm "github.com/0x1F6A/proxy_VPN/internal/user/infra/gormrepo"
	trafficchsink "github.com/0x1F6A/proxy_VPN/internal/traffic/infra/chsink"
	"github.com/0x1F6A/proxy_VPN/internal/traffic/infra/chsink/chgo"
	trafficrepo "github.com/0x1F6A/proxy_VPN/internal/traffic/infra/gormrepo"
	trafficsvc "github.com/0x1F6A/proxy_VPN/internal/traffic/service"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	lg := logger.New(cfg.Log.Level, cfg.Log.Format)
	lg.Info("starting proxy_VPN worker", "version", version)

	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		lg.Error("mysql connect", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	billing := billingsvc.New(billingsvc.Deps{
		Plans:     billingrepo.NewPlanRepo(db.DB),
		Packs:     billingrepo.NewDataPackRepo(db.DB),
		Coupons:   billingrepo.NewCouponRepo(db.DB),
		Orders:    billingrepo.NewOrderRepo(db.DB),
		UserApply: usergorm.NewBillingApplyRepo(db.DB),
	})
	node := nodesvc.New(nodesvc.Deps{
		Nodes:            noderepo.NewNodeRepo(db.DB),
		Groups:           noderepo.NewGroupRepo(db.DB),
		Subs:             usergorm.NewSubscriberLookupRepo(db.DB),
		BootstrapSecret:  cfg.Node.BootstrapSecret,
		HeartbeatTimeout: cfg.Node.HeartbeatTimeout,
	})
	pay := paysvc.New(paysvc.Deps{
		Payments:   payrepo.NewPaymentRepo(db.DB),
		Pool:       payrepo.NewAddressPoolRepo(db.DB),
		Cursor:     payrepo.NewChainScanCursor(db.DB),
		Billing:    billing,
		Providers:  map[paydomain.Channel]ports.PaymentProvider{},
		NotifyBase: cfg.Payment.NotifyBase,
	})

	var chDriver trafficchsink.Driver
	if cfg.ClickHouse.Enabled {
		conn, err := chgo.Open(context.Background(), chgo.Options{
			Addr:     cfg.ClickHouse.Addr,
			Database: cfg.ClickHouse.Database,
			User:     cfg.ClickHouse.User,
			Password: cfg.ClickHouse.Password,
		})
		if err != nil {
			log.Fatalf("clickhouse open: %v", err)
		}
		if err := conn.EnsureDatabase(context.Background(), cfg.ClickHouse.Database); err != nil {
			log.Fatalf("clickhouse create db: %v", err)
		}
		chDriver = conn
	}
	trafficSink, err := trafficchsink.New(trafficchsink.Config{
		Enabled:      cfg.ClickHouse.Enabled,
		Database:     cfg.ClickHouse.Database,
		FlushSize:    cfg.ClickHouse.FlushSize,
		FlushTimeout: cfg.ClickHouse.FlushInterval,
		Fallback:     trafficrepo.NewUsageFallbackSink(db.DB),
	}, chDriver)
	if err != nil {
		log.Fatalf("traffic sink: %v", err)
	}
	if cfg.ClickHouse.Enabled {
		if err := trafficSink.Bootstrap(context.Background()); err != nil {
			log.Fatalf("clickhouse bootstrap: %v", err)
		}
	}
	traffic := trafficsvc.New(trafficsvc.Deps{
		Sink:   trafficSink,
		Quota:  trafficrepo.NewQuotaRepo(db.DB),
		Subs:   usergorm.NewTrafficSubscriberResolver(db.DB),
		BanTTL: cfg.Traffic.BanCacheTTL,
		Log:    lg.Info,
	})

	axCfg := asynqx.Config{
		RedisAddr:     cfg.Redis.Addr,
		RedisPassword: cfg.Redis.Password,
		RedisDB:       cfg.Redis.DB,
		Concurrency:   cfg.Asynq.Concurrency,
	}

	mux := asynq.NewServeMux()
	tasks.Mount(mux, tasks.Deps{
		BillingExpire: billing.AutoCancelExpired,
		NodeMarkStale: func(ctx context.Context, _ time.Time) (int64, error) {
			return node.MarkStaleNow(ctx)
		},
		NodeStaleCutoff:  func() time.Time { return time.Now() },
		PaymentExpire:    pay.ExpireOldPending,
		PaymentReconcile: pay.ReconcileChannel,
		USDTStep:         nil, // optional: wire a Scanner here once live providers are configured
		TrafficFlushCH:   func(ctx context.Context) error { return nil }, // chsink writes are synchronous
		TrafficRollup:    func(ctx context.Context) (int64, error) { return 0, nil }, // online upsert covers UI
		TrafficRecompute: func(ctx context.Context) (int, int, error) {
			return traffic.RecomputeBans(ctx, 500)
		},
		Log:              lg.Info,
	})

	srv := asynqx.NewServer(axCfg)
	scheduler := asynqx.NewScheduler(axCfg)
	mustSchedule(scheduler, "@every 1m", tasks.NewAutoCancelOrders())
	mustSchedule(scheduler, "@every 30s", tasks.NewMarkStaleNodes())
	mustSchedule(scheduler, "@every 1m", tasks.NewExpirePayments())
	mustSchedule(scheduler, "@every 5m", tasks.NewReconcileChannel(paydomain.ChannelAlipay, 100))
	mustSchedule(scheduler, "@every 5m", tasks.NewReconcileChannel(paydomain.ChannelWechat, 100))
	mustSchedule(scheduler, "@every 15s", tasks.NewScanUSDTBlock())
	mustSchedule(scheduler, "@every 15s", tasks.NewTrafficFlushCH())
	mustSchedule(scheduler, "@every 1m", tasks.NewTrafficRecomputeBans())
	mustSchedule(scheduler, "30 1 * * *", tasks.NewTrafficRollupDaily())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := srv.Run(mux); err != nil {
			lg.Error("asynq server", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := scheduler.Run(); err != nil {
			lg.Error("asynq scheduler", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	lg.Info("shutting down worker gracefully")
	srv.Shutdown()
	scheduler.Shutdown()
	wg.Wait()
}

func mustSchedule(s *asynq.Scheduler, spec string, t *asynq.Task) {
	if _, err := s.Register(spec, t); err != nil {
		log.Fatalf("register periodic %s: %v", t.Type(), err)
	}
}
