package chatimport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/chatwoot/client"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
	"wzap/internal/storage/postgres/postgrestest"
)

type snapshotBarrierFeed struct {
	*sessiontest.FakeSession
	snapshotted chan struct{}
	proceed     chan struct{}
	once        sync.Once
}

func (f *snapshotBarrierFeed) HistorySyncSnapshot() session.HistorySyncSnapshot {
	snap := f.FakeSession.HistorySyncSnapshot()
	f.once.Do(func() { close(f.snapshotted); <-f.proceed })
	return snap
}

func TestRunImportAcknowledgesOnlyProcessedSnapshot(t *testing.T) {
	for _, name := range []string{"success", "invalid-account", "message-write-failure"} {
		failed := name != "success"
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := postgrestest.NewPool(t)
			if _, err := pool.Exec(ctx, minimalChatwootSchema+extendedChatwootSchema); err != nil {
				t.Fatalf("Chatwoot schema: %v", err)
			}
			if name == "message-write-failure" {
				if _, err := pool.Exec(ctx, `CREATE FUNCTION reject_history_message() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'history write rejected'; END $$; CREATE TRIGGER reject_history_message BEFORE INSERT ON messages FOR EACH ROW EXECUTE FUNCTION reject_history_message()`); err != nil {
					t.Fatalf("install failure trigger: %v", err)
				}
			}
			feed := &snapshotBarrierFeed{FakeSession: sessiontest.NewSession(uuid.New(), nil, sessiontest.WithHistorySync(true)), snapshotted: make(chan struct{}), proceed: make(chan struct{})}
			now := time.Now().UTC().Truncate(time.Second)
			feed.ObserveHistorySync(session.HistorySyncChunk{
				Conversations: []session.HistorySyncConversation{{ChatJID: "5511999887766@s.whatsapp.net", Name: "Original chat", Messages: []session.HistorySyncMessage{{MessageID: "processed", SenderJID: "5511999887766@s.whatsapp.net", Timestamp: now, Text: "original message"}}}},
				Contacts:      []session.HistorySyncContact{{JID: "5511999887766@s.whatsapp.net", Name: "Original contact"}},
			})
			account := "1"
			if name == "invalid-account" {
				account = "invalid-account"
			}
			deps := RunDeps{Pool: pool, Config: model.ChatwootConfig{AccountID: account, NameInbox: "Test Inbox", ImportContacts: true, ImportMessages: true}, Inboxes: &fakeInboxes{inboxes: []client.Inbox{{ID: 7, Name: "Test Inbox"}}}, Feed: feed}
			done := make(chan error, 1)
			go func() { _, err := RunImport(ctx, deps); done <- err }()
			<-feed.snapshotted
			// Same identities with fresh data must remain in the next batch;
			// acknowledging by message/contact IDs alone loses these updates.
			feed.ObserveHistorySync(session.HistorySyncChunk{
				Conversations: []session.HistorySyncConversation{{ChatJID: "5511999887766@s.whatsapp.net", Name: "New chat", Messages: []session.HistorySyncMessage{
					{MessageID: "processed", SenderJID: "5511999887766@s.whatsapp.net", Timestamp: now, Text: "new observation"},
					{MessageID: "arrived-during-import", SenderJID: "5511999887766@s.whatsapp.net", Timestamp: now.Add(time.Minute), Text: "new message"},
				}}},
				Contacts: []session.HistorySyncContact{{JID: "5511999887766@s.whatsapp.net", Name: "New contact"}},
			})
			close(feed.proceed)
			err := <-done
			if (err != nil) != failed {
				t.Fatalf("import error = %v, failure requested = %v", err, failed)
			}
			pending := feed.FakeSession.HistorySyncSnapshot()
			wantChunks := uint64(1)
			wantName, wantChat, wantText := "New contact", "New chat", "new observation"
			if failed {
				wantChunks = 2
				wantName, wantChat, wantText = "Original contact", "Original chat", "original message"
			}
			if pending.Chunks != wantChunks || len(pending.Contacts) != 1 || len(pending.Conversations) != 1 {
				t.Fatalf("pending feed = %+v; want %d chunks with original identities", pending, wantChunks)
			}
			conv := pending.Conversations[0]
			if pending.Contacts[0].Name != wantName || conv.Name != wantChat || len(conv.Messages) != 2 || conv.Messages[0].Text != wantText || conv.Messages[1].MessageID != "arrived-during-import" {
				t.Errorf("pending chunk lost data: %+v", pending)
			}
		})
	}
}
