// Package prober runs HTTP self-probes against configured targets and
// records their outcomes via the SLA service.
package prober

import (
	"context"
	"net/http"
	"time"

	"github.com/0x1F6A/proxy_VPN/internal/sla/domain"
	"github.com/0x1F6A/proxy_VPN/internal/sla/service"
)

type Target struct {
	Name string
	URL  string
}

type Prober struct {
	svc     *service.Service
	region  string
	targets []Target
	client  *http.Client
}

func New(svc *service.Service, region string, targets []Target, timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Prober{
		svc: svc, region: region, targets: targets,
		client: &http.Client{Timeout: timeout},
	}
}

// RunOnce probes every target sequentially and records results.
func (p *Prober) RunOnce(ctx context.Context) {
	for _, t := range p.targets {
		p.probe(ctx, t)
	}
}

func (p *Prober) probe(ctx context.Context, t Target) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		_ = p.svc.Record(ctx, domain.Probe{
			TS: start, Region: p.region, Target: t.Name,
			Success: false, Err: err.Error(),
		})
		return
	}
	resp, err := p.client.Do(req)
	latency := uint32(time.Since(start).Milliseconds())
	out := domain.Probe{TS: start, Region: p.region, Target: t.Name, LatencyMs: latency}
	if err != nil {
		out.Success = false
		out.Err = err.Error()
	} else {
		_ = resp.Body.Close()
		out.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
		if !out.Success {
			out.Err = resp.Status
		}
	}
	_ = p.svc.Record(ctx, out)
}
