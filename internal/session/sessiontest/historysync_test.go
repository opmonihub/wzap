package sessiontest

import (
	"context"
	"testing"
	"wzap/internal/model"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// TestFakeSessionHistorySyncSnapshotAccumulates pins the fake side of the feed
// the Import plan consumes: observed chunks surface in the snapshot.
func TestFakeSessionHistorySyncSnapshotAccumulates(t *testing.T) {
	sess := NewSession(uuid.New(), nil, WithHistorySync(true))

	sess.ObserveHistorySync(session.HistorySyncChunk{
		SyncType: "RECENT",
		Progress: 50,
		Conversations: []session.HistorySyncConversation{{
			ChatJID:  "5511999999999@s.whatsapp.net",
			Messages: []session.HistorySyncMessage{{MessageID: "H-1"}},
		}},
		Contacts: []session.HistorySyncContact{{JID: "5511888888888@s.whatsapp.net", Name: "Beltrano"}},
	})

	snap := sess.HistorySyncSnapshot()
	if snap.Chunks != 1 || snap.Progress != 50 || snap.SyncType != "RECENT" {
		t.Errorf("snapshot = %+v, want the observed chunk", snap)
	}
	if len(snap.Conversations) != 1 || len(snap.Contacts) != 1 {
		t.Errorf("snapshot = %+v, want batches and contacts", snap)
	}
}

func TestFakeSessionHistorySyncDisabledDropsChunks(t *testing.T) {
	sess := NewSession(uuid.New(), nil)
	sess.ObserveHistorySync(session.HistorySyncChunk{Contacts: []session.HistorySyncContact{{JID: "synthetic@s.whatsapp.net", Name: "Synthetic"}}})
	snap := sess.HistorySyncSnapshot()
	if snap.Chunks != 0 || len(snap.Contacts) != 0 || len(snap.Conversations) != 0 || !snap.UpdatedAt.IsZero() {
		t.Errorf("disabled fake retained history: %+v", snap)
	}
}

func TestFakeManagerHistoryOptionAndAcknowledgement(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		manager := New(nil, WithHistorySync(enabled))
		sessionID := uuid.New()
		created, err := manager.Create(&model.Instance{ID: sessionID})
		if err != nil {
			t.Fatal(err)
		}
		fake := created.(*FakeSession)
		fake.ObserveHistorySync(session.HistorySyncChunk{Contacts: []session.HistorySyncContact{{JID: "old"}}})
		snapshot := fake.HistorySyncSnapshot()
		fake.ObserveHistorySync(session.HistorySyncChunk{Contacts: []session.HistorySyncContact{{JID: "new"}}})
		fake.AckHistorySync(snapshot)
		pending := fake.HistorySyncSnapshot()
		if enabled {
			if pending.Chunks != 1 || len(pending.Contacts) != 1 || pending.Contacts[0].JID != "new" {
				t.Errorf("fake lost pending history: %+v", pending)
			}
		} else if pending.Chunks != 0 || len(pending.Contacts) != 0 {
			t.Errorf("disabled manager retained history: %+v", pending)
		}
		if err := manager.Remove(context.Background(), sessionID); err != nil {
			t.Fatal(err)
		}
	}
}
