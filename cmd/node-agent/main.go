// Command node-agent runs on each proxy VPS. It reports traffic and health
// to the control plane and applies pushed configuration to Xray.
// Phase 0 placeholder — real implementation lands in Phase 5.
package main

import "fmt"

var version = "dev"

func main() {
	fmt.Printf("proxy_VPN node-agent %s — not yet implemented (Phase 5)\n", version)
}
