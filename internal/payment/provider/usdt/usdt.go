// Package usdtprov implements the USDT-TRC20 PaymentProvider. Unlike
// alipay/wechat (which talk to a payment gateway), USDT receives payments
// passively — we allocate a one-time receiving address from a pre-seeded
// pool and a separate on-chain Scanner reports incoming transfers back
// through Service.USDTReceived.
//
// QueryStatus is therefore a no-op (returns pending) — reconciliation
// happens via the Scanner, not by polling per-payment.
//
// The Scanner type is exported so callers (e.g. cmd/worker) can run it.
// It uses the gotron-sdk gRPC client and a per-payment address index
// (fed from the database).
package usdtprov

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
	"github.com/0x1F6A/proxy_VPN/internal/payment/ports"
)

// USDTRate is CNY per 1 USDT. Replace with a live oracle in production.
// We expose it as a field so admin can tune without redeploy (e.g. via
// env / config reload).
type Config struct {
	Pool      ports.AddressPoolRepo
	CNYPerUSDT string // e.g. "7.30"
}

type Provider struct{ cfg Config }

func New(cfg Config) (*Provider, error) {
	if cfg.Pool == nil {
		return nil, errors.New("usdt: address pool required")
	}
	if cfg.CNYPerUSDT == "" {
		cfg.CNYPerUSDT = "7.30"
	}
	return &Provider{cfg: cfg}, nil
}

func (p *Provider) Channel() domain.Channel { return domain.ChannelUSDT }

func (p *Provider) Create(ctx context.Context, req ports.CreateRequest) (*ports.CreateResult, error) {
	lease, err := p.cfg.Pool.Allocate(ctx, domain.ChannelUSDT, req.OrderNo)
	if err != nil {
		return nil, err
	}
	tokenAmt := convertCNYtoUSDT(req.AmountCNY, p.cfg.CNYPerUSDT)
	return &ports.CreateResult{
		ChannelTradeNo: "", // assigned when scanner observes the tx
		Address:        lease.Address,
		AmountToken:    tokenAmt,
	}, nil
}

func (p *Provider) VerifyNotify(ctx context.Context, headers map[string]string, body []byte) (*ports.VerifiedNotify, error) {
	return nil, errors.New("usdt: webhook not supported; use on-chain scanner")
}

func (p *Provider) QueryStatus(ctx context.Context, channelTradeNo string) (domain.Status, error) {
	// No active polling — Scanner is the source of truth.
	return domain.StatusPending, nil
}

// convertCNYtoUSDT divides CNY/rate keeping 6 decimal precision.
func convertCNYtoUSDT(cny, rate string) string {
	a, _ := new(big.Rat).SetString(cny)
	b, _ := new(big.Rat).SetString(rate)
	if a == nil || b == nil || b.Sign() == 0 {
		return cny
	}
	r := new(big.Rat).Quo(a, b)
	s := r.FloatString(6)
	// trim trailing zeros after the decimal point for nicer display
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		if s == "" {
			s = "0"
		}
	}
	return s
}

// ----- on-chain scanner ------------------------------------------------

// TronClient is the minimal subset of gotron-sdk we need; defined as an
// interface so the scanner can be unit-tested with a fake client.
type TronClient interface {
	NowBlock(ctx context.Context) (int64, error)
	ListTRC20Transfers(ctx context.Context, contractAddr string, fromBlock, toBlock int64) ([]TRC20Transfer, error)
}

// TRC20Transfer represents a single observed transfer event.
type TRC20Transfer struct {
	TxHash   string
	From     string
	To       string
	AmountToken string // USDT has 6 decimals; we keep as string
	Block    int64
}

// Scanner walks new blocks looking for transfers into addresses we have
// allocated. When found it calls Observer.USDTReceived.
type Scanner struct {
	Client       TronClient
	Cursor       ports.ChainScanCursor
	Confirmations int64
	ContractAddr string // TRC20 USDT contract address (TR7... on mainnet)
	Observer     UsdtReceiver
	Pool         ports.AddressPoolRepo // used only to optionally validate address belongs to pool
	BatchBlocks  int64                 // max blocks per scan call
}

type UsdtReceiver interface {
	USDTReceived(ctx context.Context, address string, amountToken string, txHash string) error
}

// Step advances the scanner by one batch. Returns the number of payments
// matched. Designed to be invoked periodically by an asynq task.
func (s *Scanner) Step(ctx context.Context) (int, error) {
	if s.Client == nil || s.Cursor == nil || s.Observer == nil {
		return 0, errors.New("scanner: missing deps")
	}
	now, err := s.Client.NowBlock(ctx)
	if err != nil {
		return 0, err
	}
	tip := now - s.Confirmations
	if tip <= 0 {
		return 0, nil
	}
	from, err := s.Cursor.Get(ctx, "tron")
	if err != nil {
		return 0, err
	}
	if from == 0 {
		from = tip
	}
	from++
	if from > tip {
		return 0, nil
	}
	batch := s.BatchBlocks
	if batch <= 0 {
		batch = 50
	}
	to := from + batch - 1
	if to > tip {
		to = tip
	}
	txs, err := s.Client.ListTRC20Transfers(ctx, s.ContractAddr, from, to)
	if err != nil {
		return 0, err
	}
	matched := 0
	for _, t := range txs {
		// Address-pool lookup is implicit: Observer.USDTReceived returns
		// nil only when an active payment matches that (address, amount).
		if err := s.Observer.USDTReceived(ctx, t.To, t.AmountToken, t.TxHash); err == nil {
			matched++
		}
	}
	if err := s.Cursor.Set(ctx, "tron", to); err != nil {
		return matched, err
	}
	return matched, nil
}

// RunLoop runs Step every interval until ctx is done. Errors are reported
// via log func and the loop keeps going (transient gRPC failures should
// not crash the worker).
func (s *Scanner) RunLoop(ctx context.Context, interval time.Duration, log func(string, ...any)) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.Step(ctx); err != nil {
				log("usdt.scanner.error", "err", err)
			} else if n > 0 {
				log("usdt.scanner.matched", "count", n)
			}
		}
	}
}
