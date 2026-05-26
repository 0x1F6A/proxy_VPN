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
	App   AppConfig   `mapstructure:"app"`
	HTTP  HTTPConfig  `mapstructure:"http"`
	Log   LogConfig   `mapstructure:"log"`
	MySQL MySQLConfig `mapstructure:"mysql"`
	Redis RedisConfig `mapstructure:"redis"`
	JWT   JWTConfig   `mapstructure:"jwt"`
	SMTP  SMTPConfig  `mapstructure:"smtp"`
	Rate  RateConfig  `mapstructure:"rate"`
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
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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
