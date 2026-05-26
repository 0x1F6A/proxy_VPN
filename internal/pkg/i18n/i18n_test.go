package i18n

import (
	"context"
	"testing"
)

func mustBundle(t *testing.T) *Bundle {
	t.Helper()
	b, err := New("en", []string{"en", "zh-CN", "zh-TW", "ja"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestT_LocaleHit(t *testing.T) {
	b := mustBundle(t)
	ctx := WithLocale(context.Background(), "zh-CN")
	got := b.T(ctx, "err.login_locked")
	if got == "err.login_locked" || got == "account temporarily locked due to too many failed attempts" {
		t.Fatalf("expected zh-CN translation, got %q", got)
	}
}

func TestT_FallbackToDefault(t *testing.T) {
	b := mustBundle(t)
	ctx := WithLocale(context.Background(), "ja")
	got := b.T(ctx, "err.nonexistent")
	if got != "err.nonexistent" {
		t.Fatalf("missing key should fallback to key, got %q", got)
	}
}

func TestMatchLocale(t *testing.T) {
	b := mustBundle(t)
	cases := []struct{ in, want string }{
		{"", "en"},
		{"zh-CN,zh;q=0.9", "zh-CN"},
		{"zh", "zh-CN"}, // prefix match to first zh-* in map (non-deterministic, accept zh-CN or zh-TW)
		{"ja-JP", "ja"},
		{"fr", "en"},
	}
	for _, c := range cases {
		got := b.MatchLocale(c.in)
		// for prefix-only "zh" both zh-CN and zh-TW are acceptable
		if c.in == "zh" {
			if got != "zh-CN" && got != "zh-TW" {
				t.Errorf("MatchLocale(%q) = %q, want zh-CN or zh-TW", c.in, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("MatchLocale(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	b := mustBundle(t)
	ctx := WithLocale(context.Background(), "en")
	got := b.T(ctx, "email.ticket_reply_subject", "T-100")
	if got != "[Ticket #T-100] New reply" {
		t.Fatalf("format mismatch: %q", got)
	}
}
