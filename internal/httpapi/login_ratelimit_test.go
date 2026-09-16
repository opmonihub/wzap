package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/config"
)

func TestLoginRateLimiterAllowsUnderLimit(t *testing.T) {
	limiter := NewLoginRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("Allow = false on attempt %d, want true under limit 3", i+1)
		}
	}
}

func TestLoginRateLimiterBlocksOverLimit(t *testing.T) {
	limiter := NewLoginRateLimiter(2, time.Minute)

	limiter.Allow("192.0.2.1")
	limiter.Allow("192.0.2.1")
	if limiter.Allow("192.0.2.1") {
		t.Error("Allow = true over limit 2, want false")
	}
}

func TestLoginRateLimiterScopesByIP(t *testing.T) {
	limiter := NewLoginRateLimiter(1, time.Minute)

	if !limiter.Allow("192.0.2.1") {
		t.Fatal("Allow = false for first IP, want true")
	}
	if limiter.Allow("192.0.2.1") {
		t.Error("Allow = true for exhausted IP, want false")
	}
	if !limiter.Allow("192.0.2.2") {
		t.Error("Allow = false for fresh IP, want true (limits are per IP)")
	}
}

func TestLoginRateLimiterDisabledWhenLimitNonPositive(t *testing.T) {
	limiter := NewLoginRateLimiter(0, time.Minute)

	for i := 0; i < 50; i++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("Allow = false with limit 0 on attempt %d, want always true", i+1)
		}
	}
}

func TestLoginRateLimiterNilAllows(t *testing.T) {
	var limiter *LoginRateLimiter
	if !limiter.Allow("192.0.2.1") {
		t.Error("nil limiter Allow = false, want true")
	}
}

func TestClientIPStripsPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.7:54321"
	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.7")
	}
}

func TestAuthLoginRateLimitedAnswers429(t *testing.T) {
	user := seedAuthUser(t, "admin@example.com", "s3cret-password", "admin")
	srv := New(
		config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, JWTSecret: testJWTSecret},
		zerolog.Nop(),
		Deps{
			ReadyChecker: checkFunc(func(ctx context.Context) error { return nil }),
			Users:        newFakeUserRepository(user),
			JWTSecret:    testJWTSecret,
			LoginLimiter: NewLoginRateLimiter(2, time.Minute),
		},
	)
	_ = srv

	for i := 0; i < 2; i++ {
		rec := serveAuth(t, srv, http.MethodPost, "/auth/login",
			`{"email":"admin@example.com","password":"wrong-password"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, http.StatusUnauthorized)
		}
	}
	rec := serveAuth(t, srv, http.MethodPost, "/auth/login",
		`{"email":"admin@example.com","password":"wrong-password"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "rate_limited" {
		t.Errorf("error code = %q, want %q", code, "rate_limited")
	}
}
