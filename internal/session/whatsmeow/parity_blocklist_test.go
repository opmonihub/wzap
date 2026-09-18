package whatsmeow

import (
	"context"
	"testing"
)

func TestUpdateBlocklistRejectsUnknownAction(t *testing.T) {
	s := newParityTestSession(t)
	if err := s.UpdateBlocklist(context.Background(), "5511999999999@s.whatsapp.net", "freeze"); err == nil {
		t.Fatalf("UpdateBlocklist freeze err = nil, want error")
	}
}
