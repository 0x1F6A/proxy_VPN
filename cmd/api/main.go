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
		log.Info("user API mounted at /api/v1")
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

// background workers
go billingSvc.RunAutoCancelLoop(context.Background(), time.Minute, func(msg string, kv ...any) {
// best-effort, no logger captured here to keep this helper standalone
_ = msg
_ = kv
})
}
