// Package gormrepo provides GORM-backed implementations of the payment
// repositories.
package gormrepo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

// ----- Payment ---------------------------------------------------------

type paymentRow struct {
	ID             uint64 `gorm:"primaryKey"`
	OrderNo        string `gorm:"column:order_no"`
	UserID         uint64
	Channel        string
	ChannelTradeNo string `gorm:"column:channel_trade_no"`
	AmountCNY      string `gorm:"column:amount_cny;type:decimal(12,2)"`
	AmountToken    string `gorm:"column:amount_token;type:decimal(20,6)"`
	Status         string
	QRorURL        string `gorm:"column:qr_or_url"`
	Address        string
	RawNotify      string `gorm:"column:raw_notify"`
	PaidAt         *time.Time
	ExpiredAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (paymentRow) TableName() string { return "payments" }

func pmToDomain(r *paymentRow) *domain.Payment {
	return &domain.Payment{
		ID: r.ID, OrderNo: r.OrderNo, UserID: r.UserID,
		Channel: domain.Channel(r.Channel), ChannelTradeNo: r.ChannelTradeNo,
		AmountCNY: r.AmountCNY, AmountToken: r.AmountToken,
		Status: domain.Status(r.Status), QRorURL: r.QRorURL, Address: r.Address,
		RawNotify: r.RawNotify, PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
func pmToRow(p *domain.Payment) *paymentRow {
	return &paymentRow{
		ID: p.ID, OrderNo: p.OrderNo, UserID: p.UserID,
		Channel: string(p.Channel), ChannelTradeNo: p.ChannelTradeNo,
		AmountCNY: p.AmountCNY, AmountToken: p.AmountToken,
		Status: string(p.Status), QRorURL: p.QRorURL, Address: p.Address,
		RawNotify: p.RawNotify, PaidAt: p.PaidAt, ExpiredAt: p.ExpiredAt,
	}
}

type PaymentRepo struct{ db *gorm.DB }

func NewPaymentRepo(db *gorm.DB) *PaymentRepo { return &PaymentRepo{db: db} }

func (r *PaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	row := pmToRow(p)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	p.ID = row.ID
	p.CreatedAt = row.CreatedAt
	p.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *PaymentRepo) FindByID(ctx context.Context, id uint64) (*domain.Payment, error) {
	var row paymentRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return pmToDomain(&row), nil
}

func (r *PaymentRepo) FindByOrder(ctx context.Context, orderNo string) ([]domain.Payment, error) {
	var rows []paymentRow
	if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Payment, 0, len(rows))
	for i := range rows {
		out = append(out, *pmToDomain(&rows[i]))
	}
	return out, nil
}

func (r *PaymentRepo) FindActiveByOrder(ctx context.Context, orderNo string, channel domain.Channel) (*domain.Payment, error) {
	var row paymentRow
	err := r.db.WithContext(ctx).
		Where("order_no = ? AND channel = ? AND status = ? AND expired_at > ?",
			orderNo, string(channel), string(domain.StatusPending), time.Now()).
		Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return pmToDomain(&row), nil
}

func (r *PaymentRepo) FindByChannelTradeNo(ctx context.Context, channel domain.Channel, tradeNo string) (*domain.Payment, error) {
	if tradeNo == "" {
		return nil, nil
	}
	var row paymentRow
	err := r.db.WithContext(ctx).
		Where("channel = ? AND channel_trade_no = ?", string(channel), tradeNo).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return pmToDomain(&row), nil
}

func (r *PaymentRepo) FindByAddress(ctx context.Context, channel domain.Channel, address, amountToken string) (*domain.Payment, error) {
	var row paymentRow
	q := r.db.WithContext(ctx).
		Where("channel = ? AND address = ? AND status = ?",
			string(channel), address, string(domain.StatusPending))
	if amountToken != "" {
		q = q.Where("amount_token = ?", amountToken)
	}
	err := q.Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return pmToDomain(&row), nil
}

func (r *PaymentRepo) MarkPaid(ctx context.Context, id uint64, tradeNo string, raw string, paidAt time.Time) error {
	res := r.db.WithContext(ctx).Model(&paymentRow{}).
		Where("id = ? AND status = ?", id, string(domain.StatusPending)).
		Updates(map[string]any{
			"status":           string(domain.StatusPaid),
			"channel_trade_no": tradeNo,
			"raw_notify":       raw,
			"paid_at":          paidAt,
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PaymentRepo) UpdateTradeNo(ctx context.Context, id uint64, tradeNo string) error {
	return r.db.WithContext(ctx).Model(&paymentRow{}).
		Where("id = ?", id).
		Update("channel_trade_no", tradeNo).Error
}

func (r *PaymentRepo) ExpirePending(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&paymentRow{}).
		Where("status = ? AND expired_at <= ?", string(domain.StatusPending), before).
		Update("status", string(domain.StatusExpired))
	return res.RowsAffected, res.Error
}

func (r *PaymentRepo) ListPendingByChannel(ctx context.Context, channel domain.Channel, limit int) ([]domain.Payment, error) {
	var rows []paymentRow
	err := r.db.WithContext(ctx).
		Where("channel = ? AND status = ? AND channel_trade_no <> ''",
			string(channel), string(domain.StatusPending)).
		Order("id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Payment, 0, len(rows))
	for i := range rows {
		out = append(out, *pmToDomain(&rows[i]))
	}
	return out, nil
}

func (r *PaymentRepo) AdminList(ctx context.Context, f ports.PaymentFilter, limit, offset int) ([]domain.Payment, int64, error) {
	tx := r.db.WithContext(ctx).Model(&paymentRow{})
	if f.Status != "" {
		tx = tx.Where("status = ?", string(f.Status))
	}
	if f.Channel != "" {
		tx = tx.Where("channel = ?", string(f.Channel))
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
	var rows []paymentRow
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.Payment, len(rows))
	for i := range rows {
		out[i] = *pmToDomain(&rows[i])
	}
	return out, total, nil
}

// ----- Address pool ----------------------------------------------------

type addrRow struct {
	ID          uint64 `gorm:"primaryKey"`
	Channel     string
	Address     string
	Status      string
	OrderNo     string
	AllocatedAt *time.Time
	ReleasedAt  *time.Time
	CreatedAt   time.Time
}

func (addrRow) TableName() string { return "payment_addresses" }

type AddressPoolRepo struct{ db *gorm.DB }

func NewAddressPoolRepo(db *gorm.DB) *AddressPoolRepo { return &AddressPoolRepo{db: db} }

// Allocate atomically claims the next free address in the channel pool.
func (r *AddressPoolRepo) Allocate(ctx context.Context, channel domain.Channel, orderNo string) (*domain.AddressLease, error) {
	var lease *domain.AddressLease
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row addrRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("channel = ? AND status = ?", string(channel), "free").
			Order("id ASC").Limit(1).First(&row).Error; err != nil {
			return err
		}
		now := time.Now()
		row.Status = "allocated"
		row.OrderNo = orderNo
		row.AllocatedAt = &now
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		lease = &domain.AddressLease{
			ID: row.ID, Channel: domain.Channel(row.Channel), Address: row.Address,
			Status: row.Status, OrderNo: row.OrderNo, AllocatedAt: row.AllocatedAt,
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNoAddressAvailable
	}
	return lease, err
}

func (r *AddressPoolRepo) Release(ctx context.Context, channel domain.Channel, address string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&addrRow{}).
		Where("channel = ? AND address = ? AND status = ?", string(channel), address, "allocated").
		Updates(map[string]any{
			"status":      "free",
			"order_no":    "",
			"released_at": &now,
		}).Error
}

func (r *AddressPoolRepo) MarkUsed(ctx context.Context, channel domain.Channel, address string) error {
	return r.db.WithContext(ctx).Model(&addrRow{}).
		Where("channel = ? AND address = ?", string(channel), address).
		Update("status", "used").Error
}

func (r *AddressPoolRepo) Seed(ctx context.Context, channel domain.Channel, addrs []string) error {
	if len(addrs) == 0 {
		return nil
	}
	rows := make([]addrRow, 0, len(addrs))
	for _, a := range addrs {
		rows = append(rows, addrRow{Channel: string(channel), Address: a, Status: "free"})
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}

func (r *AddressPoolRepo) CountFree(ctx context.Context, channel domain.Channel) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&addrRow{}).
		Where("channel = ? AND status = ?", string(channel), "free").
		Count(&n).Error
	return n, err
}

// ----- Chain scan cursor ----------------------------------------------

type cursorRow struct {
	Chain     string `gorm:"primaryKey;column:chain"`
	LastBlock int64  `gorm:"column:last_block"`
	UpdatedAt time.Time
}

func (cursorRow) TableName() string { return "chain_scan_cursor" }

type ChainScanCursor struct{ db *gorm.DB }

func NewChainScanCursor(db *gorm.DB) *ChainScanCursor { return &ChainScanCursor{db: db} }

func (r *ChainScanCursor) Get(ctx context.Context, chain string) (int64, error) {
	var row cursorRow
	if err := r.db.WithContext(ctx).Where("chain = ?", chain).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return row.LastBlock, nil
}

func (r *ChainScanCursor) Set(ctx context.Context, chain string, block int64) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_block"}),
	}).Create(&cursorRow{Chain: chain, LastBlock: block}).Error
}
