// Package geoip 封装 MaxMind GeoLite2 国家查询，提供"未配置 / 找不到库 / 查询
// 失败"三种降级路径。默认 NoopLookup 返回空字符串，保证业务在没有 mmdb 时也能
// 跑（仅地理风控失效）。
package geoip

import (
	"net"
	"sync"
)

// Lookup 把 IP 字符串映射为 ISO 国家码（如 "US"、"JP"、"CN"），找不到返 ""。
type Lookup interface {
	Country(ip string) string
	Close() error
}

// NoopLookup 是地理风控未启用时的占位实现。
type NoopLookup struct{}

func (NoopLookup) Country(string) string { return "" }
func (NoopLookup) Close() error          { return nil }

// New 根据 mmdb 路径返回 Lookup。空路径或打开失败返回 NoopLookup + 对应 error，
// 调用方决定是否记录告警。
func New(path string) (Lookup, error) {
	if path == "" {
		return NoopLookup{}, nil
	}
	r, err := openReader(path)
	if err != nil {
		return NoopLookup{}, err
	}
	return &mmdbLookup{r: r}, nil
}

type mmdbLookup struct {
	mu sync.RWMutex
	r  mmdbReader
}

func (l *mmdbLookup) Country(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.r.lookupCountry(parsed)
}

func (l *mmdbLookup) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.close()
}
