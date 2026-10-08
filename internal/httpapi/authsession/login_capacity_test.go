package authsession

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginRateLimiterCapacityPreservesActiveBudgets(t *testing.T) {
	limiter := NewLoginRateLimiter(2, time.Hour)
	limiter.now = func() time.Time { return time.Unix(1000, 0) }
	for i := 0; i < MaxLoginBuckets; i++ {
		if !limiter.Allow(fmt.Sprintf("ip-%d", i)) {
			t.Fatalf("IP %d denied before capacity", i)
		}
	}
	calls := 0
	handler := LoginLimit(limiter, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = fmt.Sprintf("new-ip-%d:1234", i)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("overflow status = %d, want 429", rec.Code)
		}
	}
	if calls != 0 {
		t.Errorf("overflow handler calls = %d, want 0", calls)
	}
	if len(limiter.hits) != MaxLoginBuckets {
		t.Errorf("active buckets = %d, want %d", len(limiter.hits), MaxLoginBuckets)
	}
	if !limiter.Allow("ip-0") || limiter.Allow("ip-0") {
		t.Fatal("capacity overflow changed an existing IP's remaining budget")
	}
}

func TestLoginRateLimiterExpiredBucketFreesCapacity(t *testing.T) {
	limiter := NewLoginRateLimiter(1, time.Hour)
	now := time.Unix(1000, 0)
	limiter.now = func() time.Time { return now }
	limiter.Allow("ip-0")
	now = now.Add(time.Minute)
	for i := 1; i < MaxLoginBuckets; i++ {
		limiter.Allow(fmt.Sprintf("ip-%d", i))
	}
	now = now.Add(59 * time.Minute)
	if !limiter.Allow("new-ip") {
		t.Fatal("expired bucket did not free capacity for a new IP")
	}
	if len(limiter.hits) != MaxLoginBuckets {
		t.Fatalf("active buckets = %d, want %d", len(limiter.hits), MaxLoginBuckets)
	}
	if limiter.Allow("another-new-ip") {
		t.Fatal("admitted another IP after the freed capacity was consumed")
	}
	if limiter.Allow("ip-1") {
		t.Fatal("cleanup reset the budget of an active IP")
	}
}
