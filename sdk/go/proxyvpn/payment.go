package proxyvpn

import "context"

// PayResponse holds payment-channel-specific instructions returned by Pay.
type PayResponse struct {
	Channel      string `json:"channel"`
	QRCodeURL    string `json:"qrcode_url"`
	PayURL       string `json:"pay_url"`
	USDTAddress  string `json:"usdt_address"`
	USDTAmount   string `json:"usdt_amount"`
	ExpiresAt    string `json:"expires_at"`
}

// OrderStatusResponse is the order status polling shape.
type OrderStatusResponse struct {
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
	PaidAt  string `json:"paid_at"`
}

// Pay initiates payment for orderNo on the given channel (alipay|wechat|usdt).
func (c *Client) Pay(ctx context.Context, orderNo, channel string) (*PayResponse, error) {
	var out PayResponse
	if err := c.do(ctx, "POST", "/api/v1/pay/"+orderNo, map[string]string{"channel": channel}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderStatus polls a single order.
func (c *Client) OrderStatus(ctx context.Context, orderNo string) (*OrderStatusResponse, error) {
	var out OrderStatusResponse
	if err := c.do(ctx, "GET", "/api/v1/orders/"+orderNo, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MockPay simulates a successful payment in dev environments.
func (c *Client) MockPay(ctx context.Context, orderNo string) error {
	return c.do(ctx, "POST", "/api/v1/orders/"+orderNo+"/mock-pay", nil, nil)
}
