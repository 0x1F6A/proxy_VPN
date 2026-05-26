// proxyvpnctl is the reference CLI built on top of github.com/0x1F6A/proxy_VPN/sdk/go.
//
// Configuration:
//
//	PROXYVPN_BASE_URL   API base URL (default http://localhost:8082)
//	PROXYVPN_CRED_FILE  credentials file path (default ~/.proxyvpn/credentials.json)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/0x1F6A/proxy_VPN/sdk/go/proxyvpn"
	"github.com/spf13/cobra"
)

type credentials struct {
	BaseURL      string `json:"base_url"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func credFile() string {
	if v := os.Getenv("PROXYVPN_CRED_FILE"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".proxyvpn", "credentials.json")
}

func loadCreds() *credentials {
	b, err := os.ReadFile(credFile())
	if err != nil {
		return nil
	}
	var c credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return nil
	}
	return &c
}

func saveCreds(c *credentials) error {
	path := credFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func newClient() *proxyvpn.Client {
	base := os.Getenv("PROXYVPN_BASE_URL")
	if base == "" {
		base = "http://localhost:8082"
	}
	c := proxyvpn.New(proxyvpn.Config{BaseURL: base, Timeout: 30 * time.Second})
	if creds := loadCreds(); creds != nil {
		c.WithTokens(creds.AccessToken, creds.RefreshToken)
	}
	return c
}

func persistTokens(c *proxyvpn.Client) error {
	at, rt := c.Tokens()
	base := os.Getenv("PROXYVPN_BASE_URL")
	if base == "" {
		base = "http://localhost:8082"
	}
	return saveCreds(&credentials{BaseURL: base, AccessToken: at, RefreshToken: rt})
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func main() {
	root := &cobra.Command{
		Use:   "proxyvpnctl",
		Short: "Reference CLI for the proxy_VPN platform",
	}
	root.AddCommand(loginCmd(), meCmd(), plansCmd(), buyCmd(), payCmd(), ordersCmd(), subCmd(), nodesCmd(), logoutCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loginCmd() *cobra.Command {
	var email, password, totp string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with email + password",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newClient()
			if _, err := c.LoginPassword(cmd.Context(), email, password, totp); err != nil {
				return err
			}
			if err := persistTokens(c); err != nil {
				return err
			}
			fmt.Println("logged in,", credFile())
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "account email (required)")
	cmd.Flags().StringVar(&password, "password", "", "account password (required)")
	cmd.Flags().StringVar(&totp, "totp", "", "TOTP code if 2FA enabled")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke current session",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newClient()
			_ = c.Logout(cmd.Context())
			return os.Remove(credFile())
		},
	}
}

func meCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Print current user",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := newClient().Me(cmd.Context())
			if err != nil {
				return err
			}
			return printJSON(u)
		},
	}
}

func plansCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plans",
		Short: "List plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := newClient().ListPlans(cmd.Context())
			if err != nil {
				return err
			}
			return printJSON(ps)
		},
	}
}

func buyCmd() *cobra.Command {
	var planID uint64
	var packID uint64
	var coupon string
	cmd := &cobra.Command{
		Use:   "buy",
		Short: "Create a plan or data-pack order",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := proxyvpn.CreateOrderRequest{CouponCode: coupon}
			switch {
			case planID > 0:
				req.Type = "plan"
				req.TargetID = planID
			case packID > 0:
				req.Type = "data_pack"
				req.TargetID = packID
			default:
				return fmt.Errorf("either --plan or --pack is required")
			}
			o, err := newClient().CreateOrder(cmd.Context(), req)
			if err != nil {
				return err
			}
			return printJSON(o)
		},
	}
	cmd.Flags().Uint64Var(&planID, "plan", 0, "plan id to purchase")
	cmd.Flags().Uint64Var(&packID, "pack", 0, "data pack id to purchase")
	cmd.Flags().StringVar(&coupon, "coupon", "", "optional coupon code")
	return cmd
}

func payCmd() *cobra.Command {
	var channel string
	var watch bool
	cmd := &cobra.Command{
		Use:   "pay [order_no]",
		Short: "Initiate payment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newClient()
			r, err := c.Pay(cmd.Context(), args[0], channel)
			if err != nil {
				return err
			}
			if err := printJSON(r); err != nil {
				return err
			}
			if !watch {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			tick := time.NewTicker(3 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					s, err := c.OrderStatus(ctx, args[0])
					if err != nil {
						return err
					}
					fmt.Println("status:", s.Status)
					if s.Status == "paid" || s.Status == "cancelled" {
						return nil
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "alipay", "alipay|wechat|usdt")
	cmd.Flags().BoolVar(&watch, "watch", false, "poll order status until paid/cancelled")
	return cmd
}

func ordersCmd() *cobra.Command {
	var status string
	var page, pageSize int
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "List your orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			os_, err := newClient().ListOrders(cmd.Context(), status, page, pageSize)
			if err != nil {
				return err
			}
			return printJSON(os_)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().IntVar(&page, "page", 1, "")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "")
	return cmd
}

func subCmd() *cobra.Command {
	var format, out string
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Download subscription for current user",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := newClient()
			u, err := c.Me(cmd.Context())
			if err != nil {
				return err
			}
			body, err := c.Subscription(cmd.Context(), u.SubscribeToken, proxyvpn.SubscriptionFormat(format))
			if err != nil {
				return err
			}
			if out == "" || out == "-" {
				_, err = os.Stdout.Write(body)
				return err
			}
			return os.WriteFile(out, body, 0o600)
		},
	}
	cmd.Flags().StringVar(&format, "format", "clash", "clash|sing-box|v2ray")
	cmd.Flags().StringVarP(&out, "out", "o", "-", "output file (- for stdout)")
	return cmd
}

func nodesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nodes",
		Short: "List visible nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, err := newClient().ListNodes(cmd.Context())
			if err != nil {
				return err
			}
			return printJSON(ns)
		},
	}
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// allow imports referenced by future commands without unused warnings
var _ = strconv.Itoa
var _ = die
