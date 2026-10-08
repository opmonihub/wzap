package session

import (
	"testing"
	"time"
)

// TestHistorySyncAccumulatorMergesChunks pins the merge the Import plan reads:
// chunks add up, progress keeps the max, conversations merge by chat and
// messages dedupe by id.
func TestHistorySyncAccumulatorMergesChunks(t *testing.T) {
	var acc HistorySyncAccumulator
	at := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

	acc.Observe(HistorySyncChunk{
		SyncType: "INITIAL_BOOTSTRAP",
		Progress: 30,
		Conversations: []HistorySyncConversation{{
			ChatJID: "5511999999999@s.whatsapp.net",
			Name:    "Fulano",
			Messages: []HistorySyncMessage{
				{MessageID: "H-1", ChatJID: "5511999999999@s.whatsapp.net", Timestamp: at, Text: "oi"},
			},
		}},
		Contacts: []HistorySyncContact{{JID: "5511888888888@s.whatsapp.net", Name: "Beltrano"}},
	})
	acc.Observe(HistorySyncChunk{
		SyncType: "INITIAL_BOOTSTRAP",
		Progress: 60,
		Conversations: []HistorySyncConversation{{
			ChatJID: "5511999999999@s.whatsapp.net",
			Messages: []HistorySyncMessage{
				{MessageID: "H-1", ChatJID: "5511999999999@s.whatsapp.net", Timestamp: at, Text: "oi"},
				{MessageID: "H-2", ChatJID: "5511999999999@s.whatsapp.net", Timestamp: at.Add(time.Minute), Text: "tudo bem?"},
			},
		}},
	})

	snap := acc.Snapshot()
	if snap.Chunks != 2 {
		t.Errorf("snapshot chunks = %d, want 2", snap.Chunks)
	}
	if snap.Progress != 60 {
		t.Errorf("snapshot progress = %d, want the max 60", snap.Progress)
	}
	if snap.SyncType != "INITIAL_BOOTSTRAP" {
		t.Errorf("snapshot sync_type = %q, want INITIAL_BOOTSTRAP", snap.SyncType)
	}
	if len(snap.Conversations) != 1 {
		t.Fatalf("snapshot conversations = %+v, want the single merged chat", snap.Conversations)
	}
	conv := snap.Conversations[0]
	if conv.Name != "Fulano" {
		t.Errorf("conversation name = %q, want Fulano kept from the first chunk", conv.Name)
	}
	if len(conv.Messages) != 2 || conv.Messages[0].MessageID != "H-1" || conv.Messages[1].MessageID != "H-2" {
		t.Errorf("conversation messages = %+v, want H-1/H-2 deduped in order", conv.Messages)
	}
	if len(snap.Contacts) != 1 || snap.Contacts[0].Name != "Beltrano" {
		t.Errorf("snapshot contacts = %+v, want Beltrano kept", snap.Contacts)
	}
	if snap.UpdatedAt.IsZero() {
		t.Error("snapshot updated_at is zero, want the last observe time")
	}
}

// TestHistorySyncAccumulatorMergesContactsByJID pins contact merging: the
// first non-empty name wins, so later chunks fill gaps without clobbering.
func TestHistorySyncAccumulatorMergesContactsByJID(t *testing.T) {
	var acc HistorySyncAccumulator

	acc.Observe(HistorySyncChunk{Contacts: []HistorySyncContact{
		{JID: "5511777777777@s.whatsapp.net"},
		{JID: "5511666666666@s.whatsapp.net", Name: "Primeiro"},
	}})
	acc.Observe(HistorySyncChunk{Contacts: []HistorySyncContact{
		{JID: "5511777777777@s.whatsapp.net", Name: "Ciclana"},
		{JID: "5511666666666@s.whatsapp.net", Name: "Segundo"},
	}})

	snap := acc.Snapshot()
	if len(snap.Contacts) != 2 {
		t.Fatalf("snapshot contacts = %+v, want 2 merged by JID", snap.Contacts)
	}
	byJID := map[string]string{}
	for _, c := range snap.Contacts {
		byJID[c.JID] = c.Name
	}
	if byJID["5511777777777@s.whatsapp.net"] != "Ciclana" {
		t.Errorf("contacts = %+v, want the gap filled with Ciclana", snap.Contacts)
	}
	if byJID["5511666666666@s.whatsapp.net"] != "Primeiro" {
		t.Errorf("contacts = %+v, want Primeiro kept over Segundo", snap.Contacts)
	}
}

// TestHistorySyncAccumulatorPreviewCompleteReset pins the lifecycle the Import
// plan drives: preview totals, completion flag and reset between syncs.
func TestHistorySyncAccumulatorPreviewCompleteReset(t *testing.T) {
	var acc HistorySyncAccumulator

	acc.ObservePreview(12)
	acc.Observe(HistorySyncChunk{Progress: 100})
	acc.MarkComplete()

	snap := acc.Snapshot()
	if snap.Total != 12 {
		t.Errorf("snapshot total = %d, want the preview 12", snap.Total)
	}
	if !snap.Completed {
		t.Error("snapshot completed = false, want true after MarkComplete")
	}

	acc.Reset()
	snap = acc.Snapshot()
	if snap.Chunks != 0 || snap.Progress != 0 || snap.Completed || len(snap.Conversations) != 0 || len(snap.Contacts) != 0 {
		t.Errorf("snapshot after reset = %+v, want the zero feed", snap)
	}
}

// TestHistorySyncAccumulatorsAreIndependent pins per-instance isolation: two
// accumulators never share conversations.
func TestHistorySyncAccumulatorsAreIndependent(t *testing.T) {
	var first, second HistorySyncAccumulator

	first.Observe(HistorySyncChunk{Conversations: []HistorySyncConversation{{ChatJID: "aaa@s.whatsapp.net"}}})
	second.Observe(HistorySyncChunk{Conversations: []HistorySyncConversation{{ChatJID: "bbb@s.whatsapp.net"}}})

	if got := first.Snapshot().Conversations; len(got) != 1 || got[0].ChatJID != "aaa@s.whatsapp.net" {
		t.Errorf("first conversations = %+v, want only aaa", got)
	}
	if got := second.Snapshot().Conversations; len(got) != 1 || got[0].ChatJID != "bbb@s.whatsapp.net" {
		t.Errorf("second conversations = %+v, want only bbb", got)
	}
}

func TestHistoryAcknowledgementKeepsLaterObservationsAndMetadata(t *testing.T) {
	var acc HistorySyncAccumulator
	acc.ObservePreview(10)
	acc.Observe(HistorySyncChunk{Progress: 100, Conversations: []HistorySyncConversation{{ChatJID: "same-chat", Name: "Old chat", Messages: []HistorySyncMessage{{MessageID: "same-message", Text: "Old text"}}}}, Contacts: []HistorySyncContact{{JID: "same-contact", Name: "Old contact"}}})
	acc.MarkComplete()
	processed := acc.Snapshot()
	chunk := HistorySyncChunk{SyncType: "RECENT", Progress: 20, Conversations: []HistorySyncConversation{{ChatJID: "same-chat", Name: "New chat", Messages: []HistorySyncMessage{{MessageID: "same-message", Text: "New text"}}}}, Contacts: []HistorySyncContact{{JID: "same-contact", Name: "New contact"}}}
	acc.Observe(chunk)
	// The caller owns its input: later mutations must not change the journal
	// that acknowledgement uses to rebuild the pending snapshot.
	chunk.Conversations[0].Messages[0].Text = "caller mutation"
	chunk.Contacts[0].Name = "caller mutation"
	acc.ObservePreview(2)
	acc.MarkComplete()
	acc.Acknowledge(processed)
	remaining := acc.Snapshot()
	if remaining.Chunks != 1 || remaining.Progress != 20 || remaining.Total != 2 || remaining.SyncType != "RECENT" || !remaining.Completed {
		t.Fatalf("later metadata lost: %+v", remaining)
	}
	if len(remaining.Conversations) != 1 || len(remaining.Conversations[0].Messages) != 1 || remaining.Conversations[0].Name != "New chat" || remaining.Conversations[0].Messages[0].Text != "New text" || len(remaining.Contacts) != 1 || remaining.Contacts[0].Name != "New contact" {
		t.Errorf("later observation lost: %+v", remaining)
	}
	acc.Acknowledge(processed)
	if got := acc.Snapshot(); got.Chunks != 1 {
		t.Errorf("repeated acknowledgement erased pending chunks: %+v", got)
	}
	acc.Acknowledge(remaining)
	if got := acc.Snapshot(); got.Chunks != 0 || got.Completed || got.Total != 0 || !got.UpdatedAt.IsZero() {
		t.Errorf("acknowledged feed still pending: %+v", got)
	}
}

func TestHistoryAcknowledgementCannotEraseAnotherFeedOrAfterReset(t *testing.T) {
	var first, other HistorySyncAccumulator
	first.Observe(HistorySyncChunk{Contacts: []HistorySyncContact{{JID: "original"}}})
	old := first.Snapshot()
	other.Observe(HistorySyncChunk{Contacts: []HistorySyncContact{{JID: "other-instance"}}})
	other.Acknowledge(old)
	if got := other.Snapshot(); got.Chunks != 1 || got.Contacts[0].JID != "other-instance" {
		t.Errorf("foreign acknowledgement erased another instance: %+v", got)
	}
	first.Reset()
	first.Observe(HistorySyncChunk{Contacts: []HistorySyncContact{{JID: "after-reset"}}})
	first.Acknowledge(old)
	if got := first.Snapshot(); got.Chunks != 1 || got.Contacts[0].JID != "after-reset" {
		t.Errorf("stale acknowledgement erased reset feed: %+v", got)
	}
}
