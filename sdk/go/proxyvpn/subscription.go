package proxyvpn

import (
	"context"
)

// Node is a public node listing.
type Node struct {
	ID       uint64   `json:"id"`
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	Protocol string   `json:"protocol"`
	Status   string   `json:"status"`
	Tags     []string `json:"tags"`
	LoadPct  int      `json:"load_pct"`
}

// NodeGroup groups nodes (e.g. by region or tier).
type NodeGroup struct {
	ID    uint64   `json:"id"`
	Name  string   `json:"name"`
	Tags  []string `json:"tags"`
	Nodes []uint64 `json:"nodes"`
}

// ListNodes returns nodes visible to the current user.
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	var out struct {
		List []Node `json:"list"`
	}
	if err := c.do(ctx, "GET", "/api/v1/nodes", nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// ListNodeGroups returns node groups.
func (c *Client) ListNodeGroups(ctx context.Context) ([]NodeGroup, error) {
	var out struct {
		List []NodeGroup `json:"list"`
	}
	if err := c.do(ctx, "GET", "/api/v1/node-groups", nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// SubscriptionFormat is one of the supported subscription serialisations.
type SubscriptionFormat string

const (
	SubFormatClash      SubscriptionFormat = "clash"
	SubFormatSingBox    SubscriptionFormat = "sing-box"
	SubFormatV2RayBase64 SubscriptionFormat = "v2ray"
)

// Subscription downloads the raw subscription payload for the given token.
//
// The endpoint is unauthenticated (token in the URL is the auth material) and
// returns the format-specific body verbatim (yaml / json / base64 text).
func (c *Client) Subscription(ctx context.Context, token string, format SubscriptionFormat) ([]byte, error) {
	path := "/sub/" + token
	if format != "" {
		path += "?format=" + string(format)
	}
	return c.doBytes(ctx, "GET", path)
}
