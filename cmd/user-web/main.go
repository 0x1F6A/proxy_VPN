// Command user-web is the end-user web portal binary.
//
// It binds the same set of bounded contexts as cmd/api (so users can
// register / login / browse plans / pay orders / fetch subscription)
// and additionally serves the React user SPA via go:embed. Default
// listen address is :8082 so it can be reverse-proxied behind a public
// hostname (e.g. https://my.example.com) independently of the admin
// console (:8081) and the raw API (:8080).
//
// No audit middleware is mounted — that is reserved for cmd/admin.
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

	"github.com/gin-gonic/gin"

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
	"github.com/0x1F6A/proxy_VPN/internal/pkg/auth"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/config"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/httpx"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/logger"
	"github.com/0x1F6A/proxy_VPN/internal/pkg/storage"
	reportrepo "github.com/0x1F6A/proxy_VPN/internal/report/infra/gormrepo"
	reportsvc "github.com/0x1F6A/proxy_VPN/internal/report/service"
	reporthttp "github.com/0x1F6A/proxy_VPN/internal/report/transport/httpapi"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/gormrepo"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/rediskv"
	"github.com/0x1F6A/proxy_VPN/internal/user/infra/smtpmail"
	"github.com/0x1F6A/proxy_VPN/internal/user/ports"
	"github.com/0x1F6A/proxy_VPN/internal/user/service"
	"github.com/0x1F6A/proxy_VPN/internal/user/transport/httpapi"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// defaultUserWebAddr is used when http.addr is unset or still pointing at the
// public-API default ":8080" — the user-facing web binary should listen on a separate
// port so it can be firewalled independently.
const defaultUserWebAddr = ":8082"

// newMailer mirrors cmd/api's factory so e2e tests can swap it.
var newMailer = func(cfg config.SMTPConfig) ports.Mailer { return smtpmail.New(cfg) }

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if cfg.HTTP.Addr == "" || cfg.HTTP.Addr == ":8080" {
		cfg.HTTP.Addr = defaultUserWebAddr
	}

	lg := logger.New(cfg.Log.Level, cfg.Log.Format)
	lg.Info("starting proxy_VPN user-web",
		"version", version, "commit", commit, "build_date", date,
		"addr", cfg.HTTP.Addr)

	checks := []httpx.ReadinessCheck{}

	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		lg.Warn("mysql not available at startup, /readyz will fail until reachable", "err", err)
	} else {
		defer func() { _ = db.Close() }()
		checks = append(checks, httpx.ReadinessCheck{Name: "mysql", Check: db.Ping})
		lg.Info("mysql connected")
	}

	rdb, err := storage.NewRedis(cfg.Redis)
	if err != nil {
		lg.Warn("redis not available at startup, /readyz will fail until reachable", "err", err)
	} else {
		defer func() { _ = rdb.Close() }()
		checks = append(checks, httpx.ReadinessCheck{Name: "redis", Check: rdb.Ping})
		lg.Info("redis connected")
	}

	router := httpx.NewRouter(httpx.Options{
		Version:         version,
		Logger:          lg,
		ReadinessChecks: checks,
	})

	if db != nil && rdb != nil {
		mountAPI(router, cfg, db, rdb)
		registerWebUI(router)
		lg.Info("user-web API mounted at /api/v1")
	} else {
		lg.Warn("user-web API not mounted: requires both MySQL and Redis")
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
			lg.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	lg.Info("shutting down admin gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		lg.Error("graceful shutdown", "err", err)
	}
}

// mountAPI wires every bounded context exactly like cmd/api / cmd/admin.
// Admin routes still mount (same handler set) but stay 403-gated behind
// the existing requireAdmin / requireRole middlewares — only the SPA
// shipped here is user-facing.
func mountAPI(r *gin.Engine, cfg *config.Config, db *storage.MySQL, rdb *storage.Redis) {
	jwtSigner := auth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.Issuer, cfg.JWT.AllowedClockSkew)
	blacklist := rediskv.NewBlacklist(rdb.Client)
	limiter := rediskv.NewLimiter(rdb.Client)

	userRepo := gormrepo.NewUserRepo(db.DB)
	deps := service.Deps{
		Users:     userRepo,
		Refresh:   gormrepo.NewRefreshRepo(db.DB),
		Codes:     gormrepo.NewEmailCodeRepo(db.DB),
		Logs:      gormrepo.NewLoginLogRepo(db.DB),
		Mailer:    newMailer(cfg.SMTP),
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

	billingSvc := billingsvc.New(billingsvc.Deps{
		Plans:     billingrepo.NewPlanRepo(db.DB),
		Packs:     billingrepo.NewDataPackRepo(db.DB),
		Coupons:   billingrepo.NewCouponRepo(db.DB),
		Orders:    billingrepo.NewOrderRepo(db.DB),
		UserApply: gormrepo.NewBillingApplyRepo(db.DB),
	})
	billingH := billinghttp.New(billingSvc, userHandler.AuthRequired(), httpapi.ClaimsFrom)
	billingH.Register(v1)

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

	reportH := reporthttp.New(
		reportsvc.New(reportrepo.New(db.DB)),
		userHandler.AuthRequired(),
		func(c *gin.Context) string {
			if cl := httpapi.ClaimsFrom(c); cl != nil {
				return cl.Role
			}
			return ""
		},
	)
	reportH.Register(v1)
}

// buildPaymentProviders mirrors cmd/api's helper. We need it for the
// payment context bootstrap even though the user-facing web binary is primarily
// reading payments — instantiating providers keeps the wiring identical
// and avoids divergent code paths.
func buildPaymentProviders(ctx context.Context, cfg *config.Config, db *storage.MySQL) map[paydomain.Channel]payports.PaymentProvider {
	out := map[paydomain.Channel]payports.PaymentProvider{}
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
