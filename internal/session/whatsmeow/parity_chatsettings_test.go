package whatsmeow

import (
	"context"
	"testing"
	"time"
)

func TestSetDisappearingTimerRejectsBadDuration(t *testing.T) {
	s := newParityTestSession(t)
	if err := s.SetDisappearingTimer(context.Background(), "5511999999999@s.whatsapp.net", 5*time.Hour); err == nil {
		t.Fatalf("SetDisappearingTimer 5h err = nil, want error")
	}
}
