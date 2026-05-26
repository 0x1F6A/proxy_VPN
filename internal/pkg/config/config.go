// Package config loads runtime configuration from YAML files and environment
// variables. Environment variables take precedence and use the prefix
// PROXYVPN_ with double underscores to express nesting (e.g.
// PROXYVPN_HTTP__ADDR).
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig     `mapstructure:"app"`
	HTTP    HTTPConfig    `mapstructure:"http"`
	Log     LogConfig     `mapstructure:"log"`
	MySQL   MySQLConfig   `mapstructure:"mysql"`
	Redis   RedisConfig   `mapstructure:"redis"`
	JWT     JWTConfig     `mapstructure:"jwt"`
	SMTP    SMTPConfig    `mapstructure:"smtp"`
	Rate    RateConfig    `mapstructure:"rate"`
	Node    NodeConfig    `mapstructure:"node"`
	Payment    PaymentConfig    `mapstructure:"payment"`
	Asynq      AsynqConfig      `mapstructure:"asynq"`
	Traffic    TrafficConfig    `mapstructure:"traffic"`
	ClickHouse ClickHouseConfig `mapstructure:"clickhouse"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Addr string `mapstructure:"addr"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type MySQLConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	// ReadReplicas is an optional list of read-only DSNs. When non-empty
	// the storage layer attaches gorm.io/plugin/dbresolver and routes
	// SELECT queries across them. Writes always go to the primary DSN.
	ReadReplicas []string `mapstructure:"read_replicas"`
	// ResolverPolicy picks the load-balancing strategy across replicas:
	// "random" (default) or "round_robin".
	ResolverPolicy string `mapstructure:"resolver_policy"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	// Mode selects the client topology: "standalone" (default) or
	// "sentinel". In sentinel mode the storage layer uses
	// redis.NewFailoverClient(MasterName, SentinelAddrs).
	Mode          string   `mapstructure:"mode"`
	MasterName    string   `mapstructure:"master_name"`
	SentinelAddrs []string `mapstructure:"sentinel_addrs"`
}

type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	AccessTTL       time.Duration `mapstructure:"access_ttl"`
	RefreshTTL      time.Duration `mapstructure:"refresh_ttl"`
	Issuer          string        `mapstructure:"issuer"`
	AllowedClockSkew time.Duration `mapstructure:"allowed_clock_skew"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	UseTLS   bool   `mapstructure:"use_tls"`
}

type RateConfig struct {
	SendCodePerEmailMin int `mapstructure:"send_code_per_email_min"`
	LoginFailPerIPMin   int `mapstructure:"login_fail_per_ip_min"`
}

// NodeConfig governs node-agent bootstrap & subscription rendering.
type NodeConfig struct {
	BootstrapSecret  string        `mapstructure:"bootstrap_secret"`  // shared secret used by node-agent to register
	HeartbeatTimeout time.Duration `mapstructure:"heartbeat_timeout"` // mark offline if no HB within
	SubscriptionBase string        `mapstructure:"subscription_base"` // e.g. https://api.example.com
}

// PaymentConfig holds per-channel credentials. An empty channel block
// disables that provider (the channel becomes unavailable for new orders).
// Mode="mock" enables the in-memory mockprov even when live keys are
// configured — used in dev/test.
type PaymentConfig struct {
	Mode       string                `mapstructure:"mode"`        // "live" or "mock"
	NotifyBase string                `mapstructure:"notify_base"` // e.g. https://api.example.com
	ReturnBase string                `mapstructure:"return_base"`
	Alipay     AlipayConfig          `mapstructure:"alipay"`
	Wechat     WechatPayConfig       `mapstructure:"wechat"`
	USDT       USDTConfig            `mapstructure:"usdt"`
	MockSecret string                `mapstructure:"mock_secret"` // hmac secret for mockprov signing
}

type AlipayConfig struct {
	AppID              string `mapstructure:"app_id"`
	PrivateKey         string `mapstructure:"private_key"`         // PEM string
	AliPayPublicKey    string `mapstructure:"alipay_public_key"`   // PEM string
	Production         bool   `mapstructure:"production"`
}

type WechatPayConfig struct {
	MchID         string `mapstructure:"mch_id"`
	AppID         string `mapstructure:"app_id"`
	SerialNo      string `mapstructure:"serial_no"`
	PrivateKey    string `mapstructure:"private_key"` // PEM string
	APIv3Key      string `mapstructure:"api_v3_key"`
}

type USDTConfig struct {
	TronGRPC      string        `mapstructure:"tron_grpc"`      // e.g. grpc.trongrid.io:50051
	ContractAddr  string        `mapstructure:"contract_addr"`  // USDT TRC20: TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t
	Confirmations int64         `mapstructure:"confirmations"`
	ScanInterval  time.Duration `mapstructure:"scan_interval"`
	CNYPerUSDT    string        `mapstructure:"cny_per_usdt"`
	AddressPool   []string      `mapstructure:"address_pool"`   // seed pool on startup
}

// AsynqConfig configures the asynq worker / scheduler runtime.
type AsynqConfig struct {
	Concurrency  int           `mapstructure:"concurrency"`
	// re-uses redis from RedisConfig
}

// TrafficConfig governs node-agent reporting cadence and ban-cache TTL.
type TrafficConfig struct {
	ReportInterval     time.Duration `mapstructure:"report_interval"`
	BanCacheTTL        time.Duration `mapstructure:"ban_cache_ttl"`
	RateDefaultUpMbps   uint64       `mapstructure:"rate_default_up_mbps"`
	RateDefaultDownMbps uint64       `mapstructure:"rate_default_down_mbps"`
}

// ClickHouseConfig configures the optional traffic-event sink.
type ClickHouseConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Addr          string        `mapstructure:"addr"`
	Database      string        `mapstructure:"database"`
	User          string        `mapstructure:"user"`
	Password      string        `mapstructure:"password"`
	FlushSize     int           `mapstructure:"flush_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

// Load reads configuration from (in priority order): env vars, ./config.yaml,
// ./deploy/config.yaml, and a built-in default.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./deploy")
	v.AddConfigPath("/etc/proxy_VPN")

	v.SetEnvPrefix("PROXYVPN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !asConfigNotFound(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "proxy_VPN")
	v.SetDefault("app.env", envOr("APP_ENV", "dev"))

	v.SetDefault("http.addr", ":8080")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("mysql.max_open_conns", 50)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", time.Hour)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)

	v.SetDefault("jwt.access_ttl", 2*time.Hour)
	v.SetDefault("jwt.refresh_ttl", 7*24*time.Hour)
	v.SetDefault("jwt.issuer", "proxy_VPN")
	v.SetDefault("jwt.allowed_clock_skew", 30*time.Second)

	v.SetDefault("smtp.host", "127.0.0.1")
	v.SetDefault("smtp.port", 1025)
	v.SetDefault("smtp.from", "no-reply@proxy-vpn.local")
	v.SetDefault("smtp.use_tls", false)

	v.SetDefault("rate.send_code_per_email_min", 1)
	v.SetDefault("rate.login_fail_per_ip_min", 10)

	v.SetDefault("node.bootstrap_secret", "change-me-bootstrap")
	v.SetDefault("node.heartbeat_timeout", 90*time.Second)
	v.SetDefault("node.subscription_base", "http://127.0.0.1:8080")

	v.SetDefault("payment.mode", "mock")
	v.SetDefault("payment.notify_base", "http://127.0.0.1:8080")
	v.SetDefault("payment.mock_secret", "change-me-mock")
	v.SetDefault("payment.usdt.confirmations", int64(19))
	v.SetDefault("payment.usdt.scan_interval", 15*time.Second)
	v.SetDefault("payment.usdt.cny_per_usdt", "7.30")
	v.SetDefault("payment.usdt.contract_addr", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")

	v.SetDefault("asynq.concurrency", 8)

	v.SetDefault("traffic.report_interval", 60*time.Second)
	v.SetDefault("traffic.ban_cache_ttl", 24*time.Hour)
	v.SetDefault("traffic.rate_default_up_mbps", 0)
	v.SetDefault("traffic.rate_default_down_mbps", 0)

	v.SetDefault("clickhouse.enabled", false)
	v.SetDefault("clickhouse.database", "proxy_vpn")
	v.SetDefault("clickhouse.flush_size", 500)
	v.SetDefault("clickhouse.flush_interval", 15*time.Second)
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func asConfigNotFound(err error, target *viper.ConfigFileNotFoundError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(viper.ConfigFileNotFoundError); ok {
		*target = e
		return true
	}
	return false
}
