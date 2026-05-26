// Package gormrepo provides GORM-backed implementations of the billing
// repositories defined in internal/billing/ports.
package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/0x1F6A/proxy_VPN/internal/billing/domain"
	"github.com/0x1F6A/proxy_VPN/internal/billing/ports"
)

// ----- Plan ------------------------------------------------------------

type planRow struct {
	ID             uint64 `gorm:"primaryKey"`
	Name           string
	Description    string
	PriceCNY       string `gorm:"column:price_cny;type:decimal(12,2)"`
	DurationDays   uint32
	TrafficGB      uint32 `gorm:"column:traffic_gb"`
	DeviceLimit    uint32
	SpeedLimitMbps uint32 `gorm:"column:speed_limit_mbps"`
	NodeGroupID    uint64
	Tags           string
	Sort           int
	Status         int8
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (planRow) TableName() string { return "plans" }

func plToDomain(r *planRow) *domain.Plan {
	return &domain.Plan{
		ID: r.ID, Name: r.Name, Description: r.Description, PriceCNY: r.PriceCNY,
		DurationDays: r.DurationDays, TrafficGB: r.TrafficGB,
		DeviceLimit: r.DeviceLimit, SpeedLimitMbps: r.SpeedLimitMbps,
		NodeGroupID: r.NodeGroupID, Tags: r.Tags, Sort: r.Sort, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
func plToRow(p *domain.Plan) *planRow {
	return &planRow{
		ID: p.ID, Name: p.Name, Description: p.Description, PriceCNY: p.PriceCNY,
		DurationDays: p.DurationDays, TrafficGB: p.TrafficGB,
		DeviceLimit: p.DeviceLimit, SpeedLimitMbps: p.SpeedLimitMbps,
		NodeGroupID: p.NodeGroupID, Tags: p.Tags, Sort: p.Sort, Status: p.Status,
	}
}

type PlanRepo struct{ db *gorm.DB }

func NewPlanRepo(db *gorm.DB) *PlanRepo { return &PlanRepo{db: db} }

func (r *PlanRepo) List(ctx context.Context, onlyActive bool) ([]domain.Plan, error) {
	q := r.db.WithContext(ctx).Model(&planRow{}).Order("sort ASC, id ASC")
	if onlyActive {
		q = q.Where("status = 1")
	}
	var rows []planRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Plan, len(rows))
	for i := range rows {
		out[i] = *plToDomain(&rows[i])
	}
	return out, nil
}
func (r *PlanRepo) Get(ctx context.Context, id uint64) (*domain.Plan, error) {
	var row planRow
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return plToDomain(&row), nil
}
func (r *PlanRepo) Create(ctx context.Context, p *domain.Plan) error {
	row := plToRow(p)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	p.ID = row.ID
	p.CreatedAt = row.CreatedAt
	p.UpdatedAt = row.UpdatedAt
	return nil
}
func (r *PlanRepo) Update(ctx context.Context, p *domain.Plan) error {
	return r.db.WithContext(ctx).Model(&planRow{}).Where("id = ?", p.ID).Updates(plToRow(p)).Error
}
func (r *PlanRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&planRow{}, id).Error
}

// ----- DataPack --------------------------------------------------------

type packRow struct {
	ID         uint64 `gorm:"primaryKey"`
	Name       string
	PriceCNY   string `gorm:"column:price_cny;type:decimal(12,2)"`
	TrafficGB  uint32 `gorm:"column:traffic_gb"`
	ValidDays  uint32
	AttachMode int8
	Sort       int
	Status     int8
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (packRow) TableName() string { return "data_packs" }

func pkToDomain(r *packRow) *domain.DataPack {
	return &domain.DataPack{
		ID: r.ID, Name: r.Name, PriceCNY: r.PriceCNY, TrafficGB: r.TrafficGB,
		ValidDays: r.ValidDays, AttachMode: r.AttachMode, Sort: r.Sort, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
func pkToRow(p *domain.DataPack) *packRow {
	return &packRow{
		ID: p.ID, Name: p.Name, PriceCNY: p.PriceCNY, TrafficGB: p.TrafficGB,
		ValidDays: p.ValidDays, AttachMode: p.AttachMode, Sort: p.Sort, Status: p.Status,
	}
}

type DataPackRepo struct{ db *gorm.DB }

func NewDataPackRepo(db *gorm.DB) *DataPackRepo { return &DataPackRepo{db: db} }

func (r *DataPackRepo) List(ctx context.Context, onlyActive bool) ([]domain.DataPack, error) {
	q := r.db.WithContext(ctx).Model(&packRow{}).Order("sort ASC, id ASC")
	if onlyActive {
		q = q.Where("status = 1")
	}
	var rows []packRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.DataPack, len(rows))
	for i := range rows {
		out[i] = *pkToDomain(&rows[i])
	}
	return out, nil
}
func (r *DataPackRepo) Get(ctx context.Context, id uint64) (*domain.DataPack, error) {
	var row packRow
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPackNotFound
	}
	if err != nil {
		return nil, err
	}
	return pkToDomain(&row), nil
}
func (r *DataPackRepo) Create(ctx context.Context, p *domain.DataPack) error {
	row := pkToRow(p)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	p.ID = row.ID
	return nil
}
func (r *DataPackRepo) Update(ctx context.Context, p *domain.DataPack) error {
	return r.db.WithContext(ctx).Model(&packRow{}).Where("id = ?", p.ID).Updates(pkToRow(p)).Error
}
func (r *DataPackRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&packRow{}, id).Error
}

// ----- Coupon ----------------------------------------------------------

type couponRow struct {
	ID            uint64 `gorm:"primaryKey"`
	Code          string
	DiscountType  int8
	DiscountValue string `gorm:"column:discount_value;type:decimal(12,2)"`
	MinAmount     string `gorm:"column:min_amount;type:decimal(12,2)"`
	Applicable    string
	TotalQuota    uint32
	UsedCount     uint32
	PerUserLimit  uint32
	StartsAt      *time.Time
	ExpiresAt     *time.Time
	Status        int8
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (couponRow) TableName() string { return "coupons" }

type CouponRepo struct{ db *gorm.DB }

func NewCouponRepo(db *gorm.DB) *CouponRepo { return &CouponRepo{db: db} }

func (r *CouponRepo) FindByCode(ctx context.Context, code string) (*domain.Coupon, error) {
	var row couponRow
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	return &domain.Coupon{
		ID: row.ID, Code: row.Code, DiscountType: row.DiscountType,
		DiscountValue: row.DiscountValue, MinAmount: row.MinAmount,
		Applicable: row.Applicable, TotalQuota: row.TotalQuota,
		UsedCount: row.UsedCount, PerUserLimit: row.PerUserLimit,
		StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt, Status: row.Status,
	}, nil
}

func (r *CouponRepo) IncrementUsage(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Model(&couponRow{}).
		Where("id = ? AND (total_quota = 0 OR used_count < total_quota)", id).
		UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCouponExhausted
	}
	return nil
}

func (r *CouponRepo) CountUsedByUser(ctx context.Context, code string, userID uint64) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("orders").
		Where("coupon_code = ? AND user_id = ? AND status IN ?", code, userID,
			[]string{domain.OrderStatusPaid}).Count(&n).Error
	return int(n), err
}

func couponFromRow(r *couponRow) *domain.Coupon {
	return &domain.Coupon{
		ID: r.ID, Code: r.Code, DiscountType: r.DiscountType,
		DiscountValue: r.DiscountValue, MinAmount: r.MinAmount,
		Applicable: r.Applicable, TotalQuota: r.TotalQuota,
		UsedCount: r.UsedCount, PerUserLimit: r.PerUserLimit,
		StartsAt: r.StartsAt, ExpiresAt: r.ExpiresAt, Status: r.Status,
	}
}

func couponToRow(c *domain.Coupon) *couponRow {
	return &couponRow{
		ID: c.ID, Code: c.Code, DiscountType: c.DiscountType,
		DiscountValue: c.DiscountValue, MinAmount: c.MinAmount,
		Applicable: c.Applicable, TotalQuota: c.TotalQuota,
		UsedCount: c.UsedCount, PerUserLimit: c.PerUserLimit,
		StartsAt: c.StartsAt, ExpiresAt: c.ExpiresAt, Status: c.Status,
	}
}

func (r *CouponRepo) List(ctx context.Context, q string, limit, offset int) ([]domain.Coupon, int64, error) {
	tx := r.db.WithContext(ctx).Model(&couponRow{})
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("code LIKE ?", like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []couponRow
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.Coupon, len(rows))
	for i := range rows {
		out[i] = *couponFromRow(&rows[i])
	}
	return out, total, nil
}

func (r *CouponRepo) Get(ctx context.Context, id uint64) (*domain.Coupon, error) {
	var row couponRow
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	return couponFromRow(&row), nil
}

func (r *CouponRepo) Create(ctx context.Context, c *domain.Coupon) error {
	row := couponToRow(c)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	c.ID = row.ID
	return nil
}

func (r *CouponRepo) Update(ctx context.Context, c *domain.Coupon) error {
	return r.db.WithContext(ctx).Model(&couponRow{}).Where("id = ?", c.ID).Updates(map[string]any{
		"code":           c.Code,
		"discount_type":  c.DiscountType,
		"discount_value": c.DiscountValue,
		"min_amount":     c.MinAmount,
		"applicable":     c.Applicable,
		"total_quota":    c.TotalQuota,
		"per_user_limit": c.PerUserLimit,
		"starts_at":      c.StartsAt,
		"expires_at":     c.ExpiresAt,
		"status":         c.Status,
	}).Error
}

func (r *CouponRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&couponRow{}, id).Error
}

// ----- Order -----------------------------------------------------------

type orderRow struct {
	ID             uint64 `gorm:"primaryKey"`
	OrderNo        string
	UserID         uint64
	Type           string
	TargetID       uint64
	TargetSnapshot json.RawMessage `gorm:"column:target_snapshot;type:json"`
	AmountCNY      string          `gorm:"column:amount_cny;type:decimal(12,2)"`
	DiscountCNY    string          `gorm:"column:discount_cny;type:decimal(12,2)"`
	PaidCNY        string          `gorm:"column:paid_cny;type:decimal(12,2)"`
	CouponCode     string
	PayMethod      string
	PayChannelNo   string
	Status         string
	ExpireAt       time.Time
	PaidAt         *time.Time
	IdempotencyKey string
	ClientIP       string
	Remark         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (orderRow) TableName() string { return "orders" }

func ordToDomain(r *orderRow) *domain.Order {
	return &domain.Order{
		ID: r.ID, OrderNo: r.OrderNo, UserID: r.UserID, Type: r.Type,
		TargetID: r.TargetID, TargetSnapshot: []byte(r.TargetSnapshot),
		AmountCNY: r.AmountCNY, DiscountCNY: r.DiscountCNY, PaidCNY: r.PaidCNY,
		CouponCode: r.CouponCode, PayMethod: r.PayMethod,
		PayChannelNo: r.PayChannelNo, Status: r.Status,
		ExpireAt: r.ExpireAt, PaidAt: r.PaidAt,
		IdempotencyKey: r.IdempotencyKey, ClientIP: r.ClientIP, Remark: r.Remark,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type OrderRepo struct{ db *gorm.DB }

func NewOrderRepo(db *gorm.DB) *OrderRepo { return &OrderRepo{db: db} }

func (r *OrderRepo) Create(ctx context.Context, o *domain.Order) error {
	row := &orderRow{
		OrderNo: o.OrderNo, UserID: o.UserID, Type: o.Type,
		TargetID: o.TargetID, TargetSnapshot: json.RawMessage(o.TargetSnapshot),
		AmountCNY: o.AmountCNY, DiscountCNY: o.DiscountCNY, PaidCNY: o.PaidCNY,
		CouponCode: o.CouponCode, PayMethod: o.PayMethod,
		Status: o.Status, ExpireAt: o.ExpireAt,
		IdempotencyKey: o.IdempotencyKey, ClientIP: o.ClientIP,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	o.ID = row.ID
	o.CreatedAt = row.CreatedAt
	o.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *OrderRepo) FindByIdempotency(ctx context.Context, userID uint64, key string) (*domain.Order, error) {
	var row orderRow
	err := r.db.WithContext(ctx).Where("user_id = ? AND idempotency_key = ?", userID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ordToDomain(&row), nil
}

func (r *OrderRepo) FindByOrderNo(ctx context.Context, no string) (*domain.Order, error) {
	var row orderRow
	err := r.db.WithContext(ctx).Where("order_no = ?", no).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return ordToDomain(&row), nil
}

func (r *OrderRepo) ListByUser(ctx context.Context, userID uint64, limit, offset int) ([]domain.Order, error) {
	var rows []orderRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Order, len(rows))
	for i := range rows {
		out[i] = *ordToDomain(&rows[i])
	}
	return out, nil
}

func (r *OrderRepo) AdminList(ctx context.Context, f ports.OrderFilter, limit, offset int) ([]domain.Order, int64, error) {
	tx := r.db.WithContext(ctx).Model(&orderRow{})
	if f.Status != "" {
		tx = tx.Where("status = ?", f.Status)
	}
	if f.Type != "" {
		tx = tx.Where("type = ?", f.Type)
	}
	if f.UserID != 0 {
		tx = tx.Where("user_id = ?", f.UserID)
	}
	if f.OrderNo != "" {
		tx = tx.Where("order_no LIKE ?", "%"+f.OrderNo+"%")
	}
	if f.From != nil {
		tx = tx.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		tx = tx.Where("created_at < ?", *f.To)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []orderRow
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.Order, len(rows))
	for i := range rows {
		out[i] = *ordToDomain(&rows[i])
	}
	return out, total, nil
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, no, status string, paidAt *time.Time, paid, channelNo string) error {
	updates := map[string]any{"status": status, "paid_cny": paid}
	if paidAt != nil {
		updates["paid_at"] = *paidAt
	}
	if channelNo != "" {
		updates["pay_channel_no"] = channelNo
	}
	return r.db.WithContext(ctx).Model(&orderRow{}).Where("order_no = ?", no).Updates(updates).Error
}

func (r *OrderRepo) ExpirePending(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&orderRow{}).
		Where("status = ? AND expire_at < ?", domain.OrderStatusPending, before).
		Update("status", domain.OrderStatusExpired)
	return res.RowsAffected, res.Error
}
