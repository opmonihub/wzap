package authsession_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wzap/internal/httpapi/authsession"
)

func TestLoginRateLimiterAllowsUnderLimit(t *testing.T) {
	limiter := authsession.NewLoginRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("Allow = false on attempt %d, want true under limit 3", i+1)
		}
	}
}
func TestLoginRateLimiterBlocksOverLimit(t *testing.T) {
	limiter := authsession.NewLoginRateLimiter(2, time.Minute)

	limiter.Allow("192.0.2.1")
	limiter.Allow("192.0.2.1")
	if limiter.Allow("192.0.2.1") {
		t.Error("Allow = true over limit 2, want false")
	}
}
func TestLoginRateLimiterScopesByIP(t *testing.T) {
	limiter := authsession.NewLoginRateLimiter(1, time.Minute)

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
	limiter := authsession.NewLoginRateLimiter(0, time.Minute)

	for i := 0; i < 50; i++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("Allow = false with limit 0 on attempt %d, want always true", i+1)
		}
	}
}
func TestLoginRateLimiterNilAllows(t *testing.T) {
	var limiter *authsession.LoginRateLimiter
	if !limiter.Allow("192.0.2.1") {
		t.Error("nil limiter Allow = false, want true")
	}
}
func TestClientIPStripsPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.7:54321"
	if got := authsession.ClientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.7")
	}
}
