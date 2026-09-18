package sessiontest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"wzap/internal/session"
)

func TestFakeEditMessage(t *testing.T) {
	sess := NewSession(uuid.New(), nil)
	sess.SetStatus(session.StatusConnected)
	id, err := sess.EditMessage(context.Background(), "5511999999999@s.whatsapp.net", "WAID-1", "novo texto")
	if err != nil {
		t.Fatalf("EditMessage err = %v", err)
	}
	if id == "" {
		t.Fatalf("EditMessage id empty")
	}
	if len(sess.EditCalls()) != 1 {
		t.Fatalf("EditCalls = %d, want 1", len(sess.EditCalls()))
	}
}

func TestFakeBlocklistRoundTrip(t *testing.T) {
	sess := NewSession(uuid.New(), nil)
	sess.PutBlocklist([]string{"5511888888888@s.whatsapp.net"})
	list, err := sess.GetBlocklist(context.Background())
	if err != nil {
		t.Fatalf("GetBlocklist err = %v", err)
	}
	if len(list) != 1 || list[0] != "5511888888888@s.whatsapp.net" {
		t.Fatalf("GetBlocklist = %v, want 1 seeded jid", list)
	}
}
