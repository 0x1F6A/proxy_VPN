// Package gormrepo provides GORM-backed implementations of node-context
// repositories.
package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/node/domain"
	"github.com/0x1F6A/proxy_VPN/internal/node/ports"
)

// ----- nodes -----------------------------------------------------------

type nodeRow struct {
	ID              uint64          `gorm:"primaryKey"`
	Name            string          `gorm:"size:64"`
	Region          string          `gorm:"size:8"`
	Tags            string          `gorm:"size:255"`
	NodeGroupID     uint64
	Protocol        string          `gorm:"size:32"`
	Address         string          `gorm:"size:255"`
	Port            uint32
	TLSConfig       json.RawMessage `gorm:"column:tls_config;type:json"`
	Transport       string          `gorm:"size:32"`
	TransportConfig json.RawMessage `gorm:"column:transport_config;type:json"`
	RateMultiplier  string          `gorm:"column:rate_multiplier"`
	NodeTokenHash   string          `gorm:"column:node_token_hash;size:64"`
	Online          bool
	LastHeartbeatAt *time.Time
	CPUPercent      *string `gorm:"column:cpu_percent"`
	MemPercent      *string `gorm:"column:mem_percent"`
	BandwidthInBps  *uint64 `gorm:"column:bandwidth_in_bps"`
	BandwidthOutBps *uint64 `gorm:"column:bandwidth_out_bps"`
	OnlineUsers     uint32  `gorm:"column:online_users"`
	Sort            int
	Status          int8
	Engine          string          `gorm:"size:16;default:xray"`
	Inbounds        json.RawMessage `gorm:"column:inbounds;type:json"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (nodeRow) TableName() string { return "nodes" }

func toNode(r *nodeRow) *domain.Node {
	if r == nil {
		return nil
	}
	return &domain.Node{
		ID: r.ID, Name: r.Name, Region: r.Region, Tags: r.Tags,
		NodeGroupID: r.NodeGroupID, Protocol: r.Protocol, Address: r.Address, Port: r.Port,
		TLSConfig: r.TLSConfig, Transport: r.Transport, TransportConfig: r.TransportConfig,
		RateMultiplier: r.RateMultiplier, NodeTokenHash: r.NodeTokenHash,
		Online: r.Online, LastHeartbeatAt: r.LastHeartbeatAt,
		CPUPercent: r.CPUPercent, MemPercent: r.MemPercent,
		BandwidthInBps: r.BandwidthInBps, BandwidthOutBps: r.BandwidthOutBps,
		OnlineUsers: r.OnlineUsers, Sort: r.Sort, Status: r.Status,
		Engine: r.Engine, Inbounds: r.Inbounds,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromNode(n *domain.Node) *nodeRow {
	return &nodeRow{
		ID: n.ID, Name: n.Name, Region: n.Region, Tags: n.Tags,
		NodeGroupID: n.NodeGroupID, Protocol: n.Protocol, Address: n.Address, Port: n.Port,
		TLSConfig: n.TLSConfig, Transport: n.Transport, TransportConfig: n.TransportConfig,
		RateMultiplier: n.RateMultiplier, NodeTokenHash: n.NodeTokenHash,
		Sort: n.Sort, Status: n.Status,
		Engine: n.Engine, Inbounds: n.Inbounds,
	}
}

type NodeRepo struct{ db *gorm.DB }

func NewNodeRepo(db *gorm.DB) *NodeRepo { return &NodeRepo{db: db} }

func (r *NodeRepo) List(ctx context.Context, onlyEnabled bool) ([]domain.Node, error) {
	q := r.db.WithContext(ctx).Model(&nodeRow{})
	if onlyEnabled {
		q = q.Where("status = ?", domain.NodeStatusEnabled)
	}
	var rows []nodeRow
	if err := q.Order("sort DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Node, 0, len(rows))
	for i := range rows {
		out = append(out, *toNode(&rows[i]))
	}
	return out, nil
}

func (r *NodeRepo) ListByGroups(ctx context.Context, groupIDs []uint64, onlyServiceable bool) ([]domain.Node, error) {
	if len(groupIDs) == 0 {
		return []domain.Node{}, nil
	}
	q := r.db.WithContext(ctx).Model(&nodeRow{}).Where("node_group_id IN ?", groupIDs)
	if onlyServiceable {
		q = q.Where("status = ? AND online = ?", domain.NodeStatusEnabled, true)
	}
	var rows []nodeRow
	if err := q.Order("sort DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Node, 0, len(rows))
	for i := range rows {
		out = append(out, *toNode(&rows[i]))
	}
	return out, nil
}

func (r *NodeRepo) Get(ctx context.Context, id uint64) (*domain.Node, error) {
	var row nodeRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toNode(&row), nil
}

func (r *NodeRepo) FindByTokenHash(ctx context.Context, hash string) (*domain.Node, error) {
	var row nodeRow
	if err := r.db.WithContext(ctx).Where("node_token_hash = ?", hash).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toNode(&row), nil
}

func (r *NodeRepo) Create(ctx context.Context, n *domain.Node) error {
	row := fromNode(n)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	n.ID = row.ID
	n.CreatedAt = row.CreatedAt
	n.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *NodeRepo) Update(ctx context.Context, n *domain.Node) error {
	return r.db.WithContext(ctx).Model(&nodeRow{}).Where("id = ?", n.ID).Updates(fromNode(n)).Error
}

func (r *NodeRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&nodeRow{}, id).Error
}

func (r *NodeRepo) UpsertHeartbeat(ctx context.Context, id uint64, hb ports.Heartbeat, now time.Time) error {
	return r.db.WithContext(ctx).Model(&nodeRow{}).Where("id = ?", id).Updates(map[string]any{
		"online":            true,
		"last_heartbeat_at": now,
		"cpu_percent":       hb.CPUPercent,
		"mem_percent":       hb.MemPercent,
		"bandwidth_in_bps":  hb.BandwidthInBps,
		"bandwidth_out_bps": hb.BandwidthOutBps,
		"online_users":      hb.OnlineUsers,
	}).Error
}

func (r *NodeRepo) MarkStale(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&nodeRow{}).
		Where("online = ? AND (last_heartbeat_at IS NULL OR last_heartbeat_at < ?)", true, cutoff).
		Update("online", false)
	return res.RowsAffected, res.Error
}

// ----- node groups -----------------------------------------------------

type groupRow struct {
	ID        uint64    `gorm:"primaryKey"`
	Name      string    `gorm:"size:64"`
	Level     int
	Remark    string    `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (groupRow) TableName() string { return "node_groups" }

type GroupRepo struct{ db *gorm.DB }

func NewGroupRepo(db *gorm.DB) *GroupRepo { return &GroupRepo{db: db} }

func (r *GroupRepo) List(ctx context.Context) ([]domain.NodeGroup, error) {
	var rows []groupRow
	if err := r.db.WithContext(ctx).Order("level DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.NodeGroup, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.NodeGroup{
			ID: r.ID, Name: r.Name, Level: r.Level, Remark: r.Remark,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (r *GroupRepo) Get(ctx context.Context, id uint64) (*domain.NodeGroup, error) {
	var row groupRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.NodeGroup{
		ID: row.ID, Name: row.Name, Level: row.Level, Remark: row.Remark,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *GroupRepo) Create(ctx context.Context, g *domain.NodeGroup) error {
	row := &groupRow{Name: g.Name, Level: g.Level, Remark: g.Remark}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	g.ID = row.ID
	g.CreatedAt = row.CreatedAt
	g.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *GroupRepo) Update(ctx context.Context, g *domain.NodeGroup) error {
	return r.db.WithContext(ctx).Model(&groupRow{}).Where("id = ?", g.ID).Updates(map[string]any{
		"name": g.Name, "level": g.Level, "remark": g.Remark,
	}).Error
}

func (r *GroupRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&groupRow{}, id).Error
}

// PlanGroups reads plan_node_groups (composite primary key).
func (r *GroupRepo) PlanGroups(ctx context.Context, planID uint64) ([]uint64, error) {
	type row struct {
		NodeGroupID uint64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("plan_node_groups").
		Select("node_group_id").
		Where("plan_id = ?", planID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.NodeGroupID)
	}
	return out, nil
}
