package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	chatwootcfg "wzap/internal/chatwoot/config"
	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/storage/postgres/postgrestest"
)

// testChatwootTokenKey returns a valid 32-byte AES-256 key for repository
// tests: tokens are always sealed at rest, so any test persisting a token
// needs a real key.
func testChatwootTokenKey(t *testing.T) []byte {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef")
	if len(key) != 32 {
		t.Fatalf("test key length = %d, want 32", len(key))
	}
	return key
}

func TestChatwootConfigPutAndGet(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-cfg",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	cfgRepo, _ := NewChatwootRepositories(pool, testChatwootTokenKey(t))
	want := model.ChatwootConfig{
		InstanceID:          instance.ID,
		Enabled:             true,
		URL:                 "https://chatwoot.example.com",
		AccountID:           "42",
		Token:               "secret-token",
		NameInbox:           "wzap-inbox",
		SignMsg:             true,
		SignDelimiter:       "\n",
		ReopenConversation:  true,
		ConversationPending: false,
		MergeBrazilContacts: true,
		ImportContacts:      true,
		ImportMessages:      false,
		DaysLimit:           30,
		AutoCreate:          true,
		Organization:        "acme",
		Logo:                "https://example.com/logo.png",
		IgnoreJIDs:          []string{"123@s.whatsapp.net"},
	}

	stored, err := cfgRepo.Put(ctx, want)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.InstanceID != want.InstanceID {
		t.Errorf("Put InstanceID = %s, want %s", stored.InstanceID, want.InstanceID)
	}

	got, err := cfgRepo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.InstanceID != want.InstanceID {
		t.Errorf("Get InstanceID = %s, want %s", got.InstanceID, want.InstanceID)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Get Enabled = %v, want %v", got.Enabled, want.Enabled)
	}
	if got.URL != want.URL {
		t.Errorf("Get URL = %q, want %q", got.URL, want.URL)
	}
	if got.AccountID != want.AccountID {
		t.Errorf("Get AccountID = %q, want %q", got.AccountID, want.AccountID)
	}
	if got.Token != want.Token {
		t.Errorf("Get Token = %q, want %q", got.Token, want.Token)
	}
	if got.NameInbox != want.NameInbox {
		t.Errorf("Get NameInbox = %q, want %q", got.NameInbox, want.NameInbox)
	}
	if got.SignMsg != want.SignMsg {
		t.Errorf("Get SignMsg = %v, want %v", got.SignMsg, want.SignMsg)
	}
	if got.SignDelimiter != want.SignDelimiter {
		t.Errorf("Get SignDelimiter = %q, want %q", got.SignDelimiter, want.SignDelimiter)
	}
	if got.ReopenConversation != want.ReopenConversation {
		t.Errorf("Get ReopenConversation = %v, want %v", got.ReopenConversation, want.ReopenConversation)
	}
	if got.ConversationPending != want.ConversationPending {
		t.Errorf("Get ConversationPending = %v, want %v", got.ConversationPending, want.ConversationPending)
	}
	if got.MergeBrazilContacts != want.MergeBrazilContacts {
		t.Errorf("Get MergeBrazilContacts = %v, want %v", got.MergeBrazilContacts, want.MergeBrazilContacts)
	}
	if got.ImportContacts != want.ImportContacts {
		t.Errorf("Get ImportContacts = %v, want %v", got.ImportContacts, want.ImportContacts)
	}
	if got.ImportMessages != want.ImportMessages {
		t.Errorf("Get ImportMessages = %v, want %v", got.ImportMessages, want.ImportMessages)
	}
	if got.DaysLimit != want.DaysLimit {
		t.Errorf("Get DaysLimit = %d, want %d", got.DaysLimit, want.DaysLimit)
	}
	if got.AutoCreate != want.AutoCreate {
		t.Errorf("Get AutoCreate = %v, want %v", got.AutoCreate, want.AutoCreate)
	}
	if got.Organization != want.Organization {
		t.Errorf("Get Organization = %q, want %q", got.Organization, want.Organization)
	}
	if got.Logo != want.Logo {
		t.Errorf("Get Logo = %q, want %q", got.Logo, want.Logo)
	}
	if len(got.IgnoreJIDs) != 1 || got.IgnoreJIDs[0] != want.IgnoreJIDs[0] {
		t.Errorf("Get IgnoreJIDs = %v, want %v", got.IgnoreJIDs, want.IgnoreJIDs)
	}
	if got.CreatedAt.IsZero() {
		t.Error("Get CreatedAt is zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("Get UpdatedAt is zero")
	}

	if _, err := cfgRepo.Get(ctx, uuid.New()); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get(unknown) err = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestChatwootConfigPutNilAndEmptyIgnoreJIDs(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	cfgRepo, _ := NewChatwootRepositories(pool, nil)

	for _, tc := range []struct {
		name       string
		ignoreJIDs []string
	}{
		{name: "nil", ignoreJIDs: nil},
		{name: "empty", ignoreJIDs: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance, err := instances.Create(ctx, model.Instance{
				ID:          uuid.New(),
				Name:        "chatwoot-ignore-" + tc.name,
				OwnerUserID: &owner.ID,
				Connection:  model.InstanceConnection{Status: "disconnected"},
			})
			if err != nil {
				t.Fatalf("create instance: %v", err)
			}

			stored, err := cfgRepo.Put(ctx, model.ChatwootConfig{
				InstanceID: instance.ID,
				IgnoreJIDs: tc.ignoreJIDs,
			})
			if err != nil {
				t.Fatalf("Put(IgnoreJIDs=%v): %v", tc.ignoreJIDs, err)
			}
			if len(stored.IgnoreJIDs) != 0 {
				t.Errorf("Put IgnoreJIDs = %v, want empty", stored.IgnoreJIDs)
			}

			got, err := cfgRepo.Get(ctx, instance.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if len(got.IgnoreJIDs) != 0 {
				t.Errorf("Get IgnoreJIDs = %v, want empty", got.IgnoreJIDs)
			}
		})
	}
}

func TestChatwootMessagePutGetDeleteByInstance(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-msg",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	want := model.ChatwootMessage{
		InstanceID:        instance.ID,
		WAKey:             "WAID:ABC123",
		ChatwootMessageID: 101,
		ConversationID:    202,
		InboxID:           303,
		ContactSourceID:   "source-1",
		IsRead:            false,
	}

	if _, err := msgRepo.Put(ctx, want); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := msgRepo.GetByWAKey(ctx, instance.ID, want.WAKey)
	if err != nil {
		t.Fatalf("GetByWAKey: %v", err)
	}
	if got.InstanceID != want.InstanceID {
		t.Errorf("GetByWAKey InstanceID = %s, want %s", got.InstanceID, want.InstanceID)
	}
	if got.WAKey != want.WAKey {
		t.Errorf("GetByWAKey WAKey = %q, want %q", got.WAKey, want.WAKey)
	}
	if got.ChatwootMessageID != want.ChatwootMessageID {
		t.Errorf("GetByWAKey ChatwootMessageID = %d, want %d", got.ChatwootMessageID, want.ChatwootMessageID)
	}
	if got.ConversationID != want.ConversationID {
		t.Errorf("GetByWAKey ConversationID = %d, want %d", got.ConversationID, want.ConversationID)
	}
	if got.InboxID != want.InboxID {
		t.Errorf("GetByWAKey InboxID = %d, want %d", got.InboxID, want.InboxID)
	}
	if got.ContactSourceID != want.ContactSourceID {
		t.Errorf("GetByWAKey ContactSourceID = %q, want %q", got.ContactSourceID, want.ContactSourceID)
	}
	if got.IsRead != want.IsRead {
		t.Errorf("GetByWAKey IsRead = %v, want %v", got.IsRead, want.IsRead)
	}
	if got.CreatedAt.IsZero() {
		t.Error("GetByWAKey CreatedAt is zero")
	}

	if _, err := msgRepo.GetByWAKey(ctx, instance.ID, "WAID:missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByWAKey(missing) err = %v, want %v", err, storage.ErrNotFound)
	}

	removed, err := msgRepo.DeleteByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("DeleteByInstance: %v", err)
	}
	if removed != 1 {
		t.Errorf("DeleteByInstance = %d, want 1", removed)
	}
	if _, err := msgRepo.GetByWAKey(ctx, instance.ID, want.WAKey); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByWAKey(after delete) err = %v, want %v", err, storage.ErrNotFound)
	}
}

// TestLatestByConversationTiebreaksDeterministically pins migration 00004:
// Migrate applies the covering index on a clean DB, and ties on created_at
// resolve to the larger cw_id instead of an arbitrary row.
func TestLatestByConversationTiebreaksDeterministically(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	var indexRegclass *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('chatwoot_messages_conversation_idx')::text`).Scan(&indexRegclass); err != nil {
		t.Fatalf("lookup covering index: %v", err)
	}
	if indexRegclass == nil {
		t.Fatal("covering index chatwoot_messages_conversation_idx missing after Migrate")
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-latest",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// Two rows sharing the exact same created_at: Put stamps now() per row,
	// so the tie is seeded with explicit SQL.
	stamp := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := pool.Exec(ctx,
		`INSERT INTO chatwoot_messages (instance_id, wa_key, cw_id, conversation_id, inbox_id, chat_jid, created_at)
		 VALUES ($1, 'WA-TIE-1', 11, 55, 7, 'source', $2), ($1, 'WA-TIE-2', 22, 55, 7, 'source', $2)`,
		instance.ID, stamp); err != nil {
		t.Fatalf("seed tied rows: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	got, err := msgRepo.LatestByConversation(ctx, instance.ID, 55)
	if err != nil {
		t.Fatalf("LatestByConversation: %v", err)
	}
	if got.ChatwootMessageID != 22 {
		t.Errorf("LatestByConversation ChatwootMessageID = %d, want 22 (deterministic tiebreak)", got.ChatwootMessageID)
	}
	if got.WAKey != "WA-TIE-2" {
		t.Errorf("LatestByConversation WAKey = %q, want WA-TIE-2", got.WAKey)
	}

	if _, err := msgRepo.LatestByConversation(ctx, instance.ID, 999); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("LatestByConversation(unknown) err = %v, want %v", err, storage.ErrNotFound)
	}
}

// TestLatestByConversationSkipsPendingRows pins the pending guard: the
// provisional "pending:{uuid}" row of a queued send is never returned as the
// latest correlation, so read markers resolve the last real row instead of a
// synthetic key. A conversation holding only pending rows reports
// ErrNotFound.
func TestLatestByConversationSkipsPendingRows(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-latest-pending",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// The real row is older; the pending row is the newest of the
	// conversation. Timestamps are seeded explicitly because Put stamps
	// now() per row and two rows can share the instant.
	realStamp := time.Now().UTC().Add(-time.Hour)
	pendingStamp := time.Now().UTC()
	pendingKey := "pending:44444444-4444-4444-4444-444444444444"
	if _, err := pool.Exec(ctx,
		`INSERT INTO chatwoot_messages (instance_id, wa_key, cw_id, conversation_id, inbox_id, chat_jid, created_at)
		 VALUES ($1, 'WA-REAL-9', 41, 56, 7, '5511999999999@s.whatsapp.net', $2),
		        ($1, $3, 42, 56, 7, '5511999999999@s.whatsapp.net', $4)`,
		instance.ID, realStamp, pendingKey, pendingStamp); err != nil {
		t.Fatalf("seed rows: %v", err)
	}
	// A second conversation holds only the pending row.
	if _, err := pool.Exec(ctx,
		`INSERT INTO chatwoot_messages (instance_id, wa_key, cw_id, conversation_id, inbox_id, chat_jid, created_at)
		 VALUES ($1, 'pending:55555555-5555-5555-5555-555555555555', 43, 57, 7, '5511999999999@s.whatsapp.net', $2)`,
		instance.ID, pendingStamp); err != nil {
		t.Fatalf("seed pending-only conversation: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	got, err := msgRepo.LatestByConversation(ctx, instance.ID, 56)
	if err != nil {
		t.Fatalf("LatestByConversation: %v", err)
	}
	if got.WAKey != "WA-REAL-9" {
		t.Errorf("LatestByConversation WAKey = %q, want WA-REAL-9 (pending row ignored)", got.WAKey)
	}
	if got.ChatwootMessageID != 41 {
		t.Errorf("LatestByConversation ChatwootMessageID = %d, want 41", got.ChatwootMessageID)
	}

	// A conversation with no real row yet reports ErrNotFound instead of the
	// pending key.
	if _, err := msgRepo.LatestByConversation(ctx, instance.ID, 57); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("LatestByConversation(pending-only) err = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestChatwootConfigTokenSealedAtRest(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-sealed",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	key := testChatwootTokenKey(t)
	cfgRepo, _ := NewChatwootRepositories(pool, key)
	stored, err := cfgRepo.Put(ctx, model.ChatwootConfig{
		InstanceID: instance.ID,
		Enabled:    true,
		URL:        "https://chatwoot.example.com",
		AccountID:  "42",
		Token:      "super-secret-token",
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.Token != "super-secret-token" {
		t.Errorf("Put Token = %q, want plaintext back on the in-memory row", stored.Token)
	}

	var raw string
	if err := pool.QueryRow(ctx, `SELECT token FROM chatwoot_configs WHERE instance_id = $1`, instance.ID).Scan(&raw); err != nil {
		t.Fatalf("read raw token: %v", err)
	}
	if raw == "super-secret-token" {
		t.Error("stored token is plaintext, want the sealed envelope")
	}

	got, err := cfgRepo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Token != "super-secret-token" {
		t.Errorf("Get Token = %q, want the opened plaintext", got.Token)
	}

	wrongRepo, _ := NewChatwootRepositories(pool, []byte("fedcba9876543210fedcba9876543210"))
	if _, err := wrongRepo.Get(ctx, instance.ID); err == nil {
		t.Error("Get with wrong key = nil, want authentication failure")
	}

	// Tampering with the stored ciphertext must fail closed: reads
	// authenticate before decrypting, so a corrupted envelope never opens.
	if _, err := pool.Exec(ctx, `UPDATE chatwoot_configs SET token = $1 WHERE instance_id = $2`,
		chatwootcfg.SealedTokenPrefix+"AAAA", instance.ID); err != nil {
		t.Fatalf("tamper stored token: %v", err)
	}
	if _, err := cfgRepo.Get(ctx, instance.ID); err == nil {
		t.Error("Get tampered ciphertext = nil, want authentication failure")
	}
}

// TestChatwootConfigPutRejectsNonEmptyTokenWithoutKey pins the mandatory
// cipher contract at the write boundary: a non-empty token without a
// repository key is refused instead of persisting plaintext.
func TestChatwootConfigPutRejectsNonEmptyTokenWithoutKey(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-nokey",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	plainRepo, _ := NewChatwootRepositories(pool, nil)
	if _, err := plainRepo.Put(ctx, model.ChatwootConfig{
		InstanceID: instance.ID,
		Enabled:    true,
		URL:        "https://chatwoot.example.com",
		AccountID:  "42",
		Token:      "secret-token",
	}); err == nil {
		t.Error("Put non-empty token without key = nil, want refusal")
	}

	var raw string
	if err := pool.QueryRow(ctx, `SELECT token FROM chatwoot_configs WHERE instance_id = $1`, instance.ID).Scan(&raw); err == nil && raw != "" {
		t.Errorf("refused Put persisted token %q, want no row", raw)
	}
}

// TestChatwootConfigPutEmptyTokenWithoutKeySucceeds pins the other side of
// the contract: a disabled config carries no token, so it persists without
// a key and reads back empty.
func TestChatwootConfigPutEmptyTokenWithoutKeySucceeds(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-empty",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	plainRepo, _ := NewChatwootRepositories(pool, nil)
	stored, err := plainRepo.Put(ctx, model.ChatwootConfig{
		InstanceID: instance.ID,
		URL:        "https://chatwoot.example.com",
		AccountID:  "42",
	})
	if err != nil {
		t.Fatalf("Put disabled config without key: %v", err)
	}
	if stored.Token != "" {
		t.Errorf("Put Token = %q, want empty", stored.Token)
	}

	got, err := plainRepo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Token != "" {
		t.Errorf("Get Token = %q, want empty (no key required without a token)", got.Token)
	}
}

// TestChatwootConfigGetRejectsLegacyPlaintextRow pins that plaintext
// storage no longer reads through: a row seeded with an unsealed token
// fails to open with or without a repository key.
func TestChatwootConfigGetRejectsLegacyPlaintextRow(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-legacy",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO chatwoot_configs (instance_id, is_enabled, url, account_id, token, ignored_jids)
		 VALUES ($1, true, 'https://chatwoot.example.com', '42', 'legacy-plaintext-token', '{}')`,
		instance.ID); err != nil {
		t.Fatalf("seed legacy plaintext row: %v", err)
	}

	keyedRepo, _ := NewChatwootRepositories(pool, testChatwootTokenKey(t))
	if _, err := keyedRepo.Get(ctx, instance.ID); err == nil {
		t.Error("Get legacy plaintext row with key = nil, want failure (plaintext storage removed)")
	}

	plainRepo, _ := NewChatwootRepositories(pool, nil)
	if _, err := plainRepo.Get(ctx, instance.ID); err == nil {
		t.Error("Get legacy plaintext row without key = nil, want failure (plaintext storage removed)")
	}
}

// TestChatwootMessagesMultipleSendsShareChatwootID proves the N:1 relation
// the remodel kept: two distinct wa_keys of one instance may hold the same
// cw_id (a forwarded message mirrored as two sends). UNIQUE stays on
// (instance_id, wa_key), never on cw_id.
func TestChatwootMessagesMultipleSendsShareChatwootID(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-shared",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	for _, waKey := range []string{"WAID:FIRST", "WAID:SECOND"} {
		if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
			InstanceID:        instance.ID,
			WAKey:             waKey,
			ChatwootMessageID: 4242,
			ConversationID:    7,
			InboxID:           3,
			ContactSourceID:   "src",
		}); err != nil {
			t.Fatalf("Put(%s) with shared cw_id: %v", waKey, err)
		}
	}

	// A third send for a different conversation must also store the same
	// cw_id without conflict.
	if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
		InstanceID:        instance.ID,
		WAKey:             "WAID:THIRD",
		ChatwootMessageID: 4242,
		ConversationID:    8,
		InboxID:           3,
		ContactSourceID:   "src",
	}); err != nil {
		t.Fatalf("Put(third) with shared cw_id: %v", err)
	}
}

// TestChatwootMessageKeepsCorrelationWhenQueueRowDies proves the
// message_id SET NULL policy: deleting the queue row referenced by a
// correlation clears the FK but keeps the wa_key → cw_id mapping (the
// mirror evidence survives the queue purge).
func TestChatwootMessageKeepsCorrelationWhenQueueRowDies(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	messages := NewMessageRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-correl",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	queue, err := messages.Create(ctx, model.OutboundMessage{
		ID:           uuid.New(),
		InstanceID:   instance.ID,
		Type:         "text",
		RecipientJID: "5511999999999@s.whatsapp.net",
		Payload:      []byte(`{"text":"espelhado"}`),
		Status:       "sent",
	})
	if err != nil {
		t.Fatalf("create queue row: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
		InstanceID:        instance.ID,
		WAKey:             "WAID:LINKED",
		ChatwootMessageID: 777,
		ConversationID:    5,
		InboxID:           3,
		ContactSourceID:   "src",
	}); err != nil {
		t.Fatalf("Put correlation: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE chatwoot_messages SET message_id = $1 WHERE instance_id = $2 AND wa_key = 'WAID:LINKED'`,
		queue.ID, instance.ID); err != nil {
		t.Fatalf("link message_id: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM message_queue WHERE id = $1`, queue.ID); err != nil {
		t.Fatalf("delete queue row: %v", err)
	}

	var linked *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT message_id FROM chatwoot_messages WHERE instance_id = $1 AND wa_key = 'WAID:LINKED'`,
		instance.ID).Scan(&linked); err != nil {
		t.Fatalf("read linked message_id: %v", err)
	}
	if linked != nil {
		t.Errorf("message_id = %v, want NULL after queue delete (SET NULL)", *linked)
	}

	got, err := msgRepo.GetByWAKey(ctx, instance.ID, "WAID:LINKED")
	if err != nil {
		t.Fatalf("GetByWAKey after queue delete: %v", err)
	}
	if got.ChatwootMessageID != 777 {
		t.Errorf("cw_id = %d, want the mirror evidence 777 kept", got.ChatwootMessageID)
	}
}

// TestChatwootPromotePendingRewritesWAKey proves the pending:{uuid} window
// end to end on real Postgres: the inbound write lands with the provisional
// key and queue FK, and the sent event promotes it to the real wa_id.
func TestChatwootPromotePendingRewritesWAKey(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	messages := NewMessageRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-pending",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	queue, err := messages.Create(ctx, model.OutboundMessage{
		ID:           uuid.New(),
		InstanceID:   instance.ID,
		Type:         "text",
		RecipientJID: "5511999999999@s.whatsapp.net",
		Payload:      []byte(`{"text":"pendente"}`),
		Status:       "queued",
	})
	if err != nil {
		t.Fatalf("create queue row: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
		InstanceID:        instance.ID,
		MessageID:         &queue.ID,
		WAKey:             "pending:" + queue.ID.String(),
		ChatwootMessageID: 555,
		ConversationID:    12,
		InboxID:           4,
	}); err != nil {
		t.Fatalf("Put pending: %v", err)
	}

	promoted, err := msgRepo.PromotePending(ctx, instance.ID, queue.ID, "WAMID-REAL-9")
	if err != nil {
		t.Fatalf("PromotePending: %v", err)
	}
	if !promoted {
		t.Fatal("PromotePending = false, want the pending row rewritten")
	}

	got, err := msgRepo.GetByWAKey(ctx, instance.ID, "WAMID-REAL-9")
	if err != nil {
		t.Fatalf("GetByWAKey real id: %v", err)
	}
	if got.ChatwootMessageID != 555 {
		t.Errorf("cw_id = %d, want 555", got.ChatwootMessageID)
	}
	if got.MessageID == nil || *got.MessageID != queue.ID {
		t.Errorf("message_id = %v, want %s kept", got.MessageID, queue.ID)
	}
	if _, err := msgRepo.GetByWAKey(ctx, instance.ID, "pending:"+queue.ID.String()); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("pending key still present: %v", err)
	}

	// Idempotent: replaying the same promotion reports no pending row, no error.
	again, err := msgRepo.PromotePending(ctx, instance.ID, queue.ID, "WAMID-REAL-9")
	if err != nil {
		t.Fatalf("replayed PromotePending: %v", err)
	}
	if again {
		t.Error("replayed PromotePending = true, want false (nothing pending)")
	}
}

// TestChatwootPromotePendingConflictKeepsRealKey proves that when the real
// wa_key is already correlated (mirror won the race), the promotion drops
// the stale pending row instead of erroring on the UNIQUE conflict.
func TestChatwootPromotePendingConflictKeepsRealKey(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	messages := NewMessageRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "chatwoot-conflict",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	queue, err := messages.Create(ctx, model.OutboundMessage{
		ID:           uuid.New(),
		InstanceID:   instance.ID,
		Type:         "text",
		RecipientJID: "5511999999999@s.whatsapp.net",
		Payload:      []byte(`{"text":"x"}`),
		Status:       "queued",
	})
	if err != nil {
		t.Fatalf("create queue row: %v", err)
	}

	_, msgRepo := NewChatwootRepositories(pool, nil)
	// The real key is already correlated (mirror/import wrote first).
	if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
		InstanceID:        instance.ID,
		WAKey:             "WAMID-DUP",
		ChatwootMessageID: 900,
		ConversationID:    1,
		InboxID:           2,
	}); err != nil {
		t.Fatalf("Put real key: %v", err)
	}
	if _, err := msgRepo.Put(ctx, model.ChatwootMessage{
		InstanceID:        instance.ID,
		MessageID:         &queue.ID,
		WAKey:             "pending:" + queue.ID.String(),
		ChatwootMessageID: 900,
		ConversationID:    1,
		InboxID:           2,
	}); err != nil {
		t.Fatalf("Put pending: %v", err)
	}

	promoted, err := msgRepo.PromotePending(ctx, instance.ID, queue.ID, "WAMID-DUP")
	if err != nil {
		t.Fatalf("PromotePending conflict: %v", err)
	}
	if !promoted {
		t.Error("conflict PromotePending = false, want true (stale pending dropped)")
	}
	if _, err := msgRepo.GetByWAKey(ctx, instance.ID, "pending:"+queue.ID.String()); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("stale pending row still present: %v", err)
	}
	got, err := msgRepo.GetByWAKey(ctx, instance.ID, "WAMID-DUP")
	if err != nil {
		t.Fatalf("GetByWAKey: %v", err)
	}
	if got.ChatwootMessageID != 900 {
		t.Errorf("cw_id = %d, want the first correlation kept", got.ChatwootMessageID)
	}
}
