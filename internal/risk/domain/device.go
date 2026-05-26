// Package domain 定义风控聚合根。这里只放纯数据/不变式，不依赖框架。
package domain

import "time"

// LoginDevice 记录一次登录命中的设备指纹（user_id, fp_hash 唯一）。
type LoginDevice struct {
	ID          uint64
	UserID      uint64
	FPHash      string
	IP          string
	UserAgent   string
	Country     string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
}

// IsRevoked 返回设备是否已被 admin / 用户手工下线。
func (d LoginDevice) IsRevoked() bool { return d.RevokedAt != nil }
