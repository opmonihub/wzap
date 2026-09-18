package whatsmeow

import (
	"context"
	"testing"
)

func newParityTestSession(t *testing.T) *instanceSession {
	t.Helper()
	return actionSession(t)
}

func TestEditMessageValidatesText(t *testing.T) {
	s := newParityTestSession(t)
	if _, err := s.EditMessage(context.Background(), "5511999999999@s.whatsapp.net", "WAID-1", ""); err == nil {
		t.Fatalf("EditMessage empty text err = nil, want invalid recipient")
	}
	long := make([]byte, 4097)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := s.EditMessage(context.Background(), "5511999999999@s.whatsapp.net", "WAID-1", string(long)); err == nil {
		t.Fatalf("EditMessage 4097 chars err = nil, want invalid recipient")
	}
}
