// Command node-agent runs on each proxy VPS. It registers with the control
// plane using a shared bootstrap secret + per-node token, then sends periodic
// heartbeats (cpu/mem/bandwidth/online_users). Real Xray config push lands
// in a later phase; for now this validates the control-plane contract.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var version = "dev"

type registerReq struct {
	Bootstrap string `json:"bootstrap"`
	NodeToken string `json:"node_token"`
}
type heartbeatReq struct {
	NodeToken       string `json:"node_token"`
	CPUPercent      string `json:"cpu_percent"`
	MemPercent      string `json:"mem_percent"`
	BandwidthInBps  uint64 `json:"bandwidth_in_bps"`
	BandwidthOutBps uint64 `json:"bandwidth_out_bps"`
	OnlineUsers     uint32 `json:"online_users"`
}

func main() {
	endpoint := flag.String("api", envOr("PROXYVPN_API", "http://127.0.0.1:8080"), "control-plane base URL")
	bootstrap := flag.String("bootstrap", os.Getenv("PROXYVPN_BOOTSTRAP"), "bootstrap secret")
	nodeToken := flag.String("token", os.Getenv("PROXYVPN_NODE_TOKEN"), "per-node bootstrap token")
	interval := flag.Duration("interval", 30*time.Second, "heartbeat interval")
	nodeID := flag.Uint64("node-id", uint64Env("PROXYVPN_NODE_ID"), "control-plane assigned node id (for traffic reports)")
	flag.Parse()

	if *bootstrap == "" || *nodeToken == "" {
		log.Fatal("--bootstrap and --token are required (or set PROXYVPN_BOOTSTRAP / PROXYVPN_NODE_TOKEN)")
	}

	log.Printf("proxy_VPN node-agent %s (go %s) starting; api=%s interval=%s",
		version, runtime.Version(), *endpoint, interval.String())

	if err := register(*endpoint, *bootstrap, *nodeToken); err != nil {
		log.Fatalf("register failed: %v", err)
	}
	log.Println("registered successfully; entering heartbeat loop")

	ctx, cancel := context.WithCancel(context.Background())
	if *nodeID > 0 {
		_ = runTrafficLoops(ctx, *endpoint, *bootstrap, *nodeID, 60*time.Second, 30*time.Second)
		log.Printf("traffic loops started (node_id=%d)", *nodeID)
	} else {
		log.Println("traffic loops disabled (no --node-id provided)")
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("shutting down node-agent")
		cancel()
	}()

	t := time.NewTicker(*interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := heartbeat(*endpoint, *nodeToken); err != nil {
				log.Printf("heartbeat error: %v (will retry next tick)", err)
			}
		}
	}
}

func register(api, bootstrap, token string) error {
	body, _ := json.Marshal(registerReq{Bootstrap: bootstrap, NodeToken: token})
	return postJSON(api+"/api/v1/node-agent/register", body)
}

func heartbeat(api, token string) error {
	hb := heartbeatReq{
		NodeToken:       token,
		CPUPercent:      fmt.Sprintf("%.2f", 5+rand.Float64()*20),
		MemPercent:      fmt.Sprintf("%.2f", 15+rand.Float64()*30),
		BandwidthInBps:  uint64(rand.Intn(10_000_000)),
		BandwidthOutBps: uint64(rand.Intn(50_000_000)),
		OnlineUsers:     uint32(rand.Intn(50)),
	}
	body, _ := json.Marshal(hb)
	return postJSON(api+"/api/v1/node-agent/heartbeat", body)
}

func postJSON(url string, body []byte) error {
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func uint64Env(k string) uint64 {
	v := os.Getenv(k)
	if v == "" {
		return 0
	}
	var n uint64
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint64(c-'0')
	}
	return n
}

