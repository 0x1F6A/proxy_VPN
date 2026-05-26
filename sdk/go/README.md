# proxy_VPN Go SDK

Typed Go client for the [proxy_VPN](https://github.com/0x1F6A/proxy_VPN) platform.

## Install

```bash
go get github.com/0x1F6A/proxy_VPN/sdk/go/proxyvpn@latest
```

## Quick start

```go
package main

import (
	"context"
	"fmt"

	"github.com/0x1F6A/proxy_VPN/sdk/go/proxyvpn"
)

func main() {
	c := proxyvpn.New(proxyvpn.Config{BaseURL: "https://api.example.com"})

	if _, err := c.LoginPassword(context.Background(), "alice@example.com", "secret", ""); err != nil {
		panic(err)
	}

	me, err := c.Me(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("welcome", me.Email)

	plans, _ := c.ListPlans(context.Background())
	for _, p := range plans {
		fmt.Println(p.ID, p.Name, p.Price)
	}
}
```

## Features

- Typed coverage of every public REST endpoint documented in `docs/api.md`
- Automatic 401 -> refresh -> retry once (with mutex + token version to avoid
  concurrent refresh storms)
- Unified error mapping: use `errors.Is(err, proxyvpn.ErrInsufficientBalance)`
- Zero non-stdlib dependencies

## Error handling

```go
_, err := c.CreateOrder(ctx, proxyvpn.CreateOrderRequest{Type: "plan", TargetID: 1})
if errors.Is(err, proxyvpn.ErrInsufficientBalance) {
	// prompt user to top up
}

var apiErr *proxyvpn.APIError
if errors.As(err, &apiErr) {
	log.Printf("code=%d req=%s", apiErr.Code, apiErr.RequestID)
}
```

## Reference CLI

A reference CLI built on this SDK lives in `sdk/cli/proxyvpnctl/`. Build with:

```bash
go build -o proxyvpnctl ./sdk/cli/proxyvpnctl
PROXYVPN_BASE_URL=https://api.example.com ./proxyvpnctl login --email a@b.com --password ***
./proxyvpnctl me
./proxyvpnctl plans
./proxyvpnctl buy --plan 1
./proxyvpnctl pay <order_no> --channel alipay --watch
./proxyvpnctl sub --format clash --out clash.yaml
```

## Versioning

The SDK is a separate Go module (`github.com/0x1F6A/proxy_VPN/sdk/go`) but
follows the main repository's tags.
