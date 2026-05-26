package auth

import (
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	h, err := HashPassword("hunter22")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "hunter22") {
		t.Fatal("expected match")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("expected mismatch")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	j := NewJWT("very-secret", time.Hour, "test", 5*time.Second)
	tok, _, err := j.Issue(42, "user", "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	c, err := j.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UID != 42 || c.Role != "user" || c.JTI != "jti-1" {
		t.Fatalf("bad claims: %+v", c)
	}
}

func TestJWTInvalidSignature(t *testing.T) {
	j1 := NewJWT("secret-a", time.Hour, "test", 0)
	j2 := NewJWT("secret-b", time.Hour, "test", 0)
	tok, _, _ := j1.Issue(1, "user", "x")
	if _, err := j2.Parse(tok); err == nil {
		t.Fatal("expected error on bad secret")
	}
}

func TestSHA256HexDeterministic(t *testing.T) {
	if SHA256Hex("a") != SHA256Hex("a") {
		t.Fatal("hash not deterministic")
	}
	if SHA256Hex("a") == SHA256Hex("b") {
		t.Fatal("different inputs collide")
	}
}
