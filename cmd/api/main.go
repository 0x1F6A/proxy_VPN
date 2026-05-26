// Command api is the main HTTP API server for proxy_VPN.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/httpx"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/storage"
	billingrepo "github.com/0x1F6A/proxy_VPN/internal/billing/infra/gormrepo"
	billingsvc "github.com/0x1F6A/proxy_VPN/internal/billing/service"
	billinghttp "github.com/0x1F6A/proxy_VPN/internal/billing/transport/httpapi"
	noderepo "github.com/0x1F6A/proxy_VPN/internal/node/infra/gormrepo"
	nodesvc "github.com/0x1F6A/proxy_VPN/internal/node/service"
	nodehttp "github.com/0x1F6A/proxy_VPN/internal/node/transport/httpapi"
	paydomain "github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	payrepo "github.com/0x1F6A/proxy_VPN/internal/payment/infra/gormrepo"
	payports "github.com/0x1F6A/proxy_VPN/internal/payment/ports"
	alipayprov "github.com/0x1F6A/proxy_VPN/internal/payment/provider/alipay"
	"github.com/0x1F6A/proxy_VPN/internal/payment/provider/mockprov"
	usdtprov "github.com/0x1F6A/proxy_VPN/internal/payment/provider/usdt"
	wechatprov "github.com/0x1F6A/proxy_VPN/internal/payment/provider/wechat"
	paysvc "github.com/0x1F6A/proxy_VPN/internal/payment/service"
	payhttp "github.com/0x1F6A/proxy_VPN/internal/payment/transport/httpapi"
	trafficchsink "github.com/0x1F6A/proxy_VPN/internal/traffic/infra/chsink"
	trafficrepo "github.com/0x1F6A/proxy_VPN/internal/traffic/infra/gormrepo"
	trafficban "github.com/0x1F6A/proxy_VPN/internal/traffic/infra/redisban"
	trafficsvc "github.com/0x1F6A/proxy_VPN/internal/traffic/service"
	traffichttp "github.com/0x1F6A/proxy_VPN/internal/traffic/transport/httpapi"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/rediskv"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/smtpmail"
	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
	"github.com/0x1F6A/proxy_VPN/internal/user/service"
	"github.com/0x1F6A/proxy_VPN/internal/user/transport/httpapi"

	"github.com/gin-gonic/gin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	log.Info("starting proxy_VPN api",
		"version", version, "commit", commit, "build_date", date,
		"addr", cfg.HTTP.Addr)

	checks := []httpx.ReadinessCheck{}

	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		log.Warn("mysql not available at startup, /readyz will fail until reachable", "err", err)
	} else {
		defer func() { _ = db.Close() }()
		checks = append(checks, httpx.ReadinessCheck{Name: "mysql", Check: db.Ping})
		log.Info("mysql connected")
	}

	rdb, err := storage.NewRedis(cfg.Redis)
	if err != nil {
		log.Warn("redis not available at startup, /readyz will fail until reachable", "err", err)
	} else {
		defer func() { _ = rdb.Close() }()
		checks = append(checks, httpx.ReadinessCheck{Name: "redis", Check: rdb.Ping})
		log.Info("redis connected")
	}

	router := httpx.NewRouter(httpx.Options{
		Version:         version,
		Logger:          log,
		ReadinessChecks: checks,
	})

	if db != nil && rdb != nil {
		mountUserAPI(router, cfg, db, rdb)
		log.Info("user / billing / node API mounted at /api/v1")
	} else {
		log.Warn("user API not mounted: requires both MySQL and Redis")
	}

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down api gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown", "err", err)
	}
}

// mountUserAPI wires the user-bounded-context HTTP routes onto the engine.
// Kept separate to keep main() readable.
func mountUserAPI(r *gin.Engine, cfg *config.Config, db *storage.MySQL, rdb *storage.Redis) {
jwtSigner := auth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.Issuer, cfg.JWT.AllowedClockSkew)
blacklist := rediskv.NewBlacklist(rdb.Client)
limiter := rediskv.NewLimiter(rdb.Client)

userRepo := gormrepo.NewUserRepo(db.DB)
deps := service.Deps{
Users:     userRepo,
Refresh:   gormrepo.NewRefreshRepo(db.DB),
Codes:     gormrepo.NewEmailCodeRepo(db.DB),
Logs:      gormrepo.NewLoginLogRepo(db.DB),
Mailer:    smtpmail.New(cfg.SMTP),
Blacklist: blacklist,
Rate:      limiter,
Admin:     gormrepo.NewAdminUserRepo(db.DB),
JWT:       jwtSigner,
Cfg:       cfg,
}
var _ ports.UserRepo = deps.Users
userHandler := httpapi.New(service.New(deps), jwtSigner, blacklist)
v1 := r.Group("/api/v1")
userHandler.Register(v1)

// billing bounded context shares the same auth middleware + claims extractor.
billingSvc := billingsvc.New(billingsvc.Deps{
Plans:     billingrepo.NewPlanRepo(db.DB),
Packs:     billingrepo.NewDataPackRepo(db.DB),
Coupons:   billingrepo.NewCouponRepo(db.DB),
Orders:    billingrepo.NewOrderRepo(db.DB),
UserApply: gormrepo.NewBillingApplyRepo(db.DB),
})
billingH := billinghttp.New(billingSvc, userHandler.AuthRequired(), httpapi.ClaimsFrom)
billingH.Register(v1)

// node bounded context: subscriber lookup is fulfilled by the user infra
// layer so node does not depend on user.
nodeSvc := nodesvc.New(nodesvc.Deps{
Nodes:            noderepo.NewNodeRepo(db.DB),
Groups:           noderepo.NewGroupRepo(db.DB),
Subs:             gormrepo.NewSubscriberLookupRepo(db.DB),
BootstrapSecret:  cfg.Node.BootstrapSecret,
HeartbeatTimeout: cfg.Node.HeartbeatTimeout,
})
planLoader := func(c *gin.Context, uid uint64) (*uint64, error) {
u, err := userRepo.FindByID(c.Request.Context(), uid)
if err != nil || u == nil {
	return nil, err
}
return u.PlanID, nil
}
nodeH := nodehttp.New(nodeSvc, userHandler.AuthRequired(), httpapi.ClaimsFrom, planLoader)
nodeH.Register(v1, r)

// payment bounded context. We always register the mock provider so dev
// environments work out of the box; alipay / wechat / usdt are only
// registered if credentials are configured.
providers := buildPaymentProviders(context.Background(), cfg, db)
paySvc := paysvc.New(paysvc.Deps{
	Payments:   payrepo.NewPaymentRepo(db.DB),
	Pool:       payrepo.NewAddressPoolRepo(db.DB),
	Cursor:     payrepo.NewChainScanCursor(db.DB),
	Billing:    billingSvc,
	Providers:  providers,
	NotifyBase: cfg.Payment.NotifyBase,
	ReturnBase: cfg.Payment.ReturnBase,
})
payH := payhttp.New(paySvc, userHandler.AuthRequired(), httpapi.ClaimsFrom)
payH.Register(v1, r)

// traffic bounded context. ClickHouse is optional; when disabled events
// fall back to the MySQL usage_event_fallback table (still durable).
fallback := trafficrepo.NewUsageFallbackSink(db.DB)
sink, _ := trafficchsink.New(trafficchsink.Config{
	Enabled:       cfg.ClickHouse.Enabled,
	Database:      cfg.ClickHouse.Database,
	FlushSize:     cfg.ClickHouse.FlushSize,
	FlushTimeout:  cfg.ClickHouse.FlushInterval,
	Fallback:      fallback,
}, nil)
trafficSvc := trafficsvc.New(trafficsvc.Deps{
	Sink:   sink,
	Quota:  trafficrepo.NewQuotaRepo(db.DB),
	Bans:   trafficban.New(rdb.Client),
	Subs:   gormrepo.NewTrafficSubscriberResolver(db.DB),
	BanTTL: cfg.Traffic.BanCacheTTL,
})
trafficH := traffichttp.New(trafficSvc, cfg.Node.BootstrapSecret, userHandler.AuthRequired(), httpapi.ClaimsFrom)
trafficH.Register(v1)

// background workers — kept here as in-process fallback for single-node
// deployments. When cmd/worker is also running, these become redundant
// but remain safe (both call idempotent operations).
go billingSvc.RunAutoCancelLoop(context.Background(), time.Minute, func(msg string, kv ...any) {
_ = msg
_ = kv
})
go nodeSvc.RunStaleMarker(context.Background(), 30*time.Second, func(msg string, kv ...any) {
_ = msg
_ = kv
})
}

// buildPaymentProviders constructs the channel→provider map based on what
// credentials are present in the config. Channels with empty credentials
// are silently skipped (the channel becomes unavailable, surface as
// ErrChannelUnsupported on /orders/:no/pay).
func buildPaymentProviders(ctx context.Context, cfg *config.Config, db *storage.MySQL) map[paydomain.Channel]payports.PaymentProvider {
	out := map[paydomain.Channel]payports.PaymentProvider{}
	// mock is always available, under both ChannelMock and (in mock mode)
	// the alipay/wechat slots — useful for local dev.
	mock := mockprov.New(mockprov.Config{Channel: paydomain.ChannelMock, Secret: cfg.Payment.MockSecret})
	out[paydomain.ChannelMock] = mock
	if cfg.Payment.Mode == "mock" {
		out[paydomain.ChannelAlipay] = mockprov.New(mockprov.Config{Channel: paydomain.ChannelAlipay, Secret: cfg.Payment.MockSecret})
		out[paydomain.ChannelWechat] = mockprov.New(mockprov.Config{Channel: paydomain.ChannelWechat, Secret: cfg.Payment.MockSecret})
	}
	if cfg.Payment.Alipay.AppID != "" && cfg.Payment.Alipay.PrivateKey != "" {
		if p, err := alipayprov.New(alipayprov.Config{
			AppID:           cfg.Payment.Alipay.AppID,
			PrivateKey:      cfg.Payment.Alipay.PrivateKey,
			AliPayPublicKey: cfg.Payment.Alipay.AliPayPublicKey,
			Production:      cfg.Payment.Alipay.Production,
		}); err == nil {
			out[paydomain.ChannelAlipay] = p
		}
	}
	if cfg.Payment.Wechat.MchID != "" && cfg.Payment.Wechat.PrivateKey != "" {
		if p, err := wechatprov.New(ctx, wechatprov.Config{
			MchID:         cfg.Payment.Wechat.MchID,
			AppID:         cfg.Payment.Wechat.AppID,
			SerialNo:      cfg.Payment.Wechat.SerialNo,
			PrivateKeyPEM: cfg.Payment.Wechat.PrivateKey,
			APIv3Key:      cfg.Payment.Wechat.APIv3Key,
		}); err == nil {
			out[paydomain.ChannelWechat] = p
		}
	}
	if p, err := usdtprov.New(usdtprov.Config{
		Pool:       payrepo.NewAddressPoolRepo(db.DB),
		CNYPerUSDT: cfg.Payment.USDT.CNYPerUSDT,
	}); err == nil {
		out[paydomain.ChannelUSDT] = p
	}
	return out
}
