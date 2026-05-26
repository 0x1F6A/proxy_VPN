// Config-reload loop for node-agent.
//
// Polls POST /api/v1/node-agent/config with {node_token, known_hash}. On
// version change, writes the new config to disk and (optionally) execs a
// reload command (e.g. `systemctl reload xray`). Designed to be safe to
// run on a bare VPS — no shell injection, exec uses ProcessState exit code
// for surface logging only.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type configPullReq struct {
	NodeToken string `json:"node_token"`
	KnownHash string `json:"known_hash,omitempty"`
}

type configPullResp struct {
	Code int `json:"code"`
	Data struct {
		Version   string          `json:"version"`
		Format    string          `json:"format"`
		Config    json.RawMessage `json:"config"`
		Unchanged bool            `json:"unchanged"`
	} `json:"data"`
	Message string `json:"message"`
}

// runConfigLoop polls the control-plane for a new server-side config and,
// when the version changes, writes it to outFile and runs reloadCmd.
// reloadCmd is parsed by whitespace; empty disables exec.
func runConfigLoop(ctx context.Context, api, nodeToken, outFile, reloadCmd string, every time.Duration) {
	if every <= 0 {
		every = 60 * time.Second
	}
	if outFile == "" {
		log.Println("config-loop disabled (no --config-out)")
		return
	}
	known := ""
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		// Run an immediate pull on startup, then on each tick.
		if err := pullAndApply(ctx, api, nodeToken, outFile, reloadCmd, &known); err != nil {
			log.Printf("config-loop error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func pullAndApply(ctx context.Context, api, nodeToken, outFile, reloadCmd string, known *string) error {
	body, _ := json.Marshal(configPullReq{NodeToken: nodeToken, KnownHash: *known})
	req, _ := http.NewRequestWithContext(ctx, "POST", api+"/api/v1/node-agent/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 15 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return &httpStatusErr{status: resp.StatusCode, body: string(b)}
	}
	var out configPullResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Data.Unchanged {
		return nil
	}
	if len(out.Data.Config) == 0 {
		return nil
	}
	if err := writeAtomic(outFile, out.Data.Config); err != nil {
		return err
	}
	*known = out.Data.Version
	log.Printf("config updated: version=%s bytes=%d → %s", out.Data.Version, len(out.Data.Config), outFile)
	if reloadCmd != "" {
		parts := strings.Fields(reloadCmd)
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) // #nosec G204 — operator-supplied
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("reload command failed: %v", err)
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
