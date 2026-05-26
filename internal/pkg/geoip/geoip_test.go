package geoip

import "testing"

func TestNoopLookup(t *testing.T) {
	l := NoopLookup{}
	if got := l.Country("8.8.8.8"); got != "" {
		t.Fatalf("noop should return empty, got %q", got)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("noop close: %v", err)
	}
}

func TestNewEmptyPath(t *testing.T) {
	l, err := New("")
	if err != nil {
		t.Fatalf("empty path should not error: %v", err)
	}
	if _, ok := l.(NoopLookup); !ok {
		t.Fatalf("expected NoopLookup for empty path, got %T", l)
	}
}

func TestNewMissingFile(t *testing.T) {
	l, err := New("/nonexistent/geolite.mmdb")
	if err == nil {
		t.Fatal("missing file should return error")
	}
	if _, ok := l.(NoopLookup); !ok {
		t.Fatalf("expected fallback NoopLookup, got %T", l)
	}
}
