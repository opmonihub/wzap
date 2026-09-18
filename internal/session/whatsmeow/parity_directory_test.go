package whatsmeow

import (
	"context"
	"testing"
)

func TestCheckContactsRejectsOversizeBatch(t *testing.T) {
	s := newParityTestSession(t)
	phones := make([]string, 51)
	for i := range phones {
		phones[i] = "5511999999999"
	}
	if _, err := s.CheckContacts(context.Background(), phones); err == nil {
		t.Fatalf("CheckContacts 51 phones err = nil, want error")
	}
}
