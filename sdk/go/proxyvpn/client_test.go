package proxyvpn

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(envelope{Code: 0, Message: "ok", Data: mustJSON(data), RequestID: "req_test"})
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Message: msg, RequestID: "req_test"})
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestClient_LoginAndMe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, TokenPair{AccessToken: "at1", RefreshToken: "rt1", ExpiresIn: 3600})
	})
	mux.HandleFunc("/api/v1/user/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at1" {
			writeErr(w, ErrAccessTokenInvalid.Code, "no auth")
			return
		}
		writeOK(w, User{ID: 1, Email: "a@b.com", Role: "user"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL})
	if _, err := c.LoginPassword(context.Background(), "a@b.com", "pw", ""); err != nil {
		t.Fatalf("login: %v", err)
	}
	u, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if u.Email != "a@b.com" {
		t.Fatalf("user: %+v", u)
	}
}

func TestClient_AutoRefreshOn401(t *testing.T) {
	var meCalls, refreshCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCalls, 1)
		writeOK(w, TokenPair{AccessToken: "at2", RefreshToken: "rt2", ExpiresIn: 3600})
	})
	mux.HandleFunc("/api/v1/user/me", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&meCalls, 1)
		if n == 1 {
			writeErr(w, ErrAccessTokenInvalid.Code, "expired")
			return
		}
		writeOK(w, User{ID: 1, Email: "a@b.com"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL})
	c.WithTokens("at1", "rt1")
	u, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if u.ID != 1 {
		t.Fatalf("user: %+v", u)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("expected 1 refresh, got %d", got)
	}
	at, rt := c.Tokens()
	if at != "at2" || rt != "rt2" {
		t.Fatalf("tokens not rotated: %s %s", at, rt)
	}
}

func TestClient_ErrorCodeMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, ErrInsufficientBalance.Code, "no money")
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL})
	c.WithTokens("at", "rt")
	_, err := c.CreateOrder(context.Background(), CreateOrderRequest{Type: "plan", TargetID: 1})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestClient_Subscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxies: []"))
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL})
	body, err := c.Subscription(context.Background(), "tok", SubFormatClash)
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	if string(body) != "proxies: []" {
		t.Fatalf("body: %s", string(body))
	}
}

func TestClient_ListPlans(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, map[string]any{"list": []Plan{{ID: 1, Name: "basic", Price: 1000}}})
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL})
	c.WithTokens("a", "b")
	ps, err := c.ListPlans(context.Background())
	if err != nil || len(ps) != 1 || ps[0].Name != "basic" {
		t.Fatalf("plans: %v %+v", err, ps)
	}
}
