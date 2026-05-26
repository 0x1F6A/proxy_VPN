package proxyvpn

import (
	"context"
	"fmt"
	"time"
)

// Plan is a subscription product.
type Plan struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	Price         int64  `json:"price"`
	DurationDays  int    `json:"duration_days"`
	TrafficGB     int    `json:"traffic_gb"`
	DeviceLimit   int    `json:"device_limit"`
	Status        string `json:"status"`
}

// DataPack is a top-up traffic package.
type DataPack struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Price     int64  `json:"price"`
	TrafficGB int    `json:"traffic_gb"`
	Status    string `json:"status"`
}

// Order represents a created billing order.
type Order struct {
	OrderNo    string    `json:"order_no"`
	Type       string    `json:"type"`
	TargetID   uint64    `json:"target_id"`
	Amount     int64     `json:"amount"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// CreateOrderRequest captures the inputs to /api/v1/orders.
type CreateOrderRequest struct {
	Type           string `json:"type"`
	TargetID       uint64 `json:"target_id"`
	CouponCode     string `json:"coupon_code,omitempty"`
	IdempotencyKey string `json:"-"`
}

// ListPlans returns all enabled plans.
func (c *Client) ListPlans(ctx context.Context) ([]Plan, error) {
	var out struct {
		List []Plan `json:"list"`
	}
	if err := c.do(ctx, "GET", "/api/v1/plans", nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// ListDataPacks returns all enabled data packs.
func (c *Client) ListDataPacks(ctx context.Context) ([]DataPack, error) {
	var out struct {
		List []DataPack `json:"list"`
	}
	if err := c.do(ctx, "GET", "/api/v1/data-packs", nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// CreateOrder creates an order for a plan or data pack purchase.
func (c *Client) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	var out Order
	if err := c.do(ctx, "POST", "/api/v1/orders", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListOrders returns the user's orders, optionally filtered by status.
func (c *Client) ListOrders(ctx context.Context, status string, page, pageSize int) ([]Order, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	path := fmt.Sprintf("/api/v1/orders?page=%d&page_size=%d", page, pageSize)
	if status != "" {
		path += "&status=" + status
	}
	var out struct {
		List []Order `json:"list"`
	}
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// CancelOrder marks an unpaid order as cancelled.
func (c *Client) CancelOrder(ctx context.Context, orderNo string) error {
	return c.do(ctx, "POST", "/api/v1/orders/"+orderNo+"/cancel", nil, nil)
}
