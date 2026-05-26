// Traffic reporting & ban-list sync for node-agent.
//
// This file adds two background loops to the agent:
//
//   - Periodic POST /api/v1/nodes/usage    (bootstrap-secret auth)
//   - Periodic GET  /api/v1/nodes/banlist  (bootstrap-secret auth)
//
// Until the xray stats API is wired in (a follow-up phase), Items are
// generated as synthetic events so the end-to-end pipeline can be
// validated against a live control-plane.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type usageItem struct {
	SubToken  string `json:"sub_token,omitempty"`
	UserID    uint64 `json:"user_id,omitempty"`
	Protocol  string `json:"protocol"`
	UpBytes   uint64 `json:"up_bytes"`
	DownBytes uint64 `json:"down_bytes"`
}
type usageReportBody struct {
	NodeID uint64      `json:"node_id"`
	Items  []usageItem `json:"items"`
}
type banlistResp struct {
	UserIDs []uint64 `json:"user_ids"`
}

// trafficState holds the latest ban list so the (future) xray controller
// can read it without re-fetching.
type trafficState struct {
	mu      sync.RWMutex
	bans    map[uint64]struct{}
	updated time.Time
}

func (s *trafficState) set(ids []uint64) {
	m := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	s.mu.Lock()
	s.bans = m
	s.updated = time.Now()
	s.mu.Unlock()
}

// runTrafficLoops spawns the reporter + ban-list poller. nodeID is the
// control-plane assigned id (passed via flag or env).
func runTrafficLoops(ctx context.Context, api, bootstrap string, nodeID uint64, reportEvery, banEvery time.Duration) *trafficState {
	st := &trafficState{bans: map[uint64]struct{}{}}
	if reportEvery <= 0 {
		reportEvery = 60 * time.Second
	}
	if banEvery <= 0 {
		banEvery = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(reportEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := reportUsage(ctx, api, bootstrap, nodeID); err != nil {
					log.Printf("traffic.report error: %v", err)
				}
			}
		}
	}()
	go func() {
		t := time.NewTicker(banEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ids, err := fetchBanlist(ctx, api, bootstrap)
				if err != nil {
					log.Printf("traffic.banlist error: %v", err)
					continue
				}
				st.set(ids)
			}
		}
	}()
	return st
}

func reportUsage(ctx context.Context, api, bootstrap string, nodeID uint64) error {
	// Placeholder: generate one synthetic event. Replace with xray stats
	// API pull (queryStats prefix=user>) once that integration lands.
	body := usageReportBody{
		NodeID: nodeID,
		Items: []usageItem{{
			Protocol:  "vmess",
			UpBytes:   uint64(rand.Intn(1024 * 1024)),
			DownBytes: uint64(rand.Intn(4 * 1024 * 1024)),
		}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", api+"/api/v1/nodes/usage", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Secret", bootstrap)
	return doDiscard(req)
}

func fetchBanlist(ctx context.Context, api, bootstrap string) ([]uint64, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", api+"/api/v1/nodes/banlist", nil)
	req.Header.Set("X-Bootstrap-Secret", bootstrap)
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, &httpStatusErr{status: resp.StatusCode, body: string(b)}
	}
	var out banlistResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.UserIDs, nil
}

type httpStatusErr struct {
	status int
	body   string
}

func (e *httpStatusErr) Error() string {
	return "http " + itoa(e.status) + ": " + e.body
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	s := string(buf[pos:])
	if neg {
		s = "-" + s
	}
	return s
}

func doDiscard(req *http.Request) error {
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return &httpStatusErr{status: resp.StatusCode, body: string(b)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
