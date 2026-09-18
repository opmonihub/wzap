package session

import (
	"testing"
	"time"
)

func TestParityConstants(t *testing.T) {
	if Disappearing24h != 24*time.Hour {
		t.Fatalf("Disappearing24h = %v, want 24h", Disappearing24h)
	}
	if Disappearing7d != 7*24*time.Hour {
		t.Fatalf("Disappearing7d = %v, want 168h", Disappearing7d)
	}
	if Disappearing90d != 90*24*time.Hour {
		t.Fatalf("Disappearing90d = %v, want 2160h", Disappearing90d)
	}
	var _ = NewsletterMessage{ServerID: "1"}
	var _ = ContactCheckResult{Phone: "5511999999999"}
}
