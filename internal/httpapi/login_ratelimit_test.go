package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/authsession"
)

func TestAuthLoginRateLimitedAnswers429(t *testing.T) {
	user := seedAuthUser(t, "admin@example.com", "s3cret-password", "admin")
	srv := httpapi.New(
		config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, JWTSecret: testJWTSecret},
		zerolog.Nop(),
		httpapi.Deps{
			ReadyChecker: checkFunc(func(ctx context.Context) error { return nil }),
			Users:        newFakeUserRepository(user),
			JWTSecret:    testJWTSecret,
			LoginLimiter: authsession.NewLoginRateLimiter(2, time.Minute),
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
