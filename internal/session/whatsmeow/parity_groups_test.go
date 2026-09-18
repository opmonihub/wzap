package whatsmeow

import (
	"context"
	"testing"
)

func TestUpdateGroupRequestsValidatesAction(t *testing.T) {
	s := newParityTestSession(t)
	err := s.UpdateGroupRequestParticipants(context.Background(), "12036300000001@g.us", "maybe", []string{"5511999999999@s.whatsapp.net"})
	if err == nil {
		t.Fatalf("UpdateGroupRequestParticipants bad action err = nil, want error")
	}
}
