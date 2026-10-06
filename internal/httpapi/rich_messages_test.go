package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/message"
	"wzap/internal/model"
)

func TestGetMessageReadsRichType(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	at := time.Now().UTC()
	svc := &fakeMessageService{getFn: func(_ context.Context, instanceID, gotMessageID uuid.UUID) (*model.OutboundMessage, error) {
		if instanceID != id || gotMessageID != messageID {
			t.Errorf("Get(%s, %s), want (%s, %s)", instanceID, gotMessageID, id, messageID)
		}
		return &model.OutboundMessage{
			ID:           messageID,
			InstanceID:   id,
			Type:         message.TypePoll,
			RecipientJID: "5547988359190@s.whatsapp.net",
			Status:       message.StatusQueued,
			CreatedAt:    at,
			UpdatedAt:    at,
		}, nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+id.String()+"/messages/"+messageID.String(), "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data messageEnvelope `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Message.MessageType != message.TypePoll {
		t.Errorf("data.message.message_type = %q, want %q", payload.Data.Message.MessageType, message.TypePoll)
	}
}

func TestSendPollAccepted(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if input.Type != message.TypePoll || input.PollQuestion != "Qual o horário?" || len(input.PollOptions) != 2 {
			t.Errorf("Enqueue input = %+v, want type poll with question and options", input)
		}
		return uuid.New(), nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+id.String()+"/messages",
		`{"type":"poll","to":"5547988359190","question":"Qual o horário?","options":["manhã","tarde"]}`, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestSendPollInvalidContentMapsTo422(t *testing.T) {
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if input.Type != message.TypePoll || len(input.PollOptions) != 1 {
			t.Errorf("Enqueue input = %+v, want the single-option poll passed through", input)
		}
		return uuid.Nil, message.ErrInvalidInput
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages",
		`{"type":"poll","to":"5547988359190","question":"Q?","options":["só"]}`, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
		t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
	}
}

func TestSendReactionAcceptedAndRemoved(t *testing.T) {
	for _, body := range []string{
		`{"type":"reaction","to":"5547988359190","target":"wamid.1","emoji":"👍"}`,
		`{"type":"reaction","to":"5547988359190","target":"wamid.1","emoji":""}`,
	} {
		svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
			if input.Type != message.TypeReaction || input.ReactionTarget != "wamid.1" {
				t.Errorf("Enqueue input = %+v, want type reaction with target", input)
			}
			return uuid.New(), nil
		}}

		rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
			"/instances/"+uuid.NewString()+"/messages", body, nil)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("body %s: status = %d, want %d", body, rec.Code, http.StatusAccepted)
		}
	}
}

func TestSendListAndButtonsAccepted(t *testing.T) {
	bodies := []string{
		`{"type":"list","to":"5547988359190","button_text":"Ver","sections":[{"title":"S","rows":[{"id":"r1","title":"R1"}]}]}`,
		`{"type":"buttons","to":"5547988359190","text":"Escolha","buttons":[{"id":"a","title":"A"}]}`,
	}
	for _, body := range bodies {
		svc := &fakeMessageService{}

		rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
			"/instances/"+uuid.NewString()+"/messages", body, nil)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("body %s: status = %d, want %d", body, rec.Code, http.StatusAccepted)
		}
	}
	if len(bodies) == 0 {
		t.Fatal("no bodies")
	}
}

func TestSendRichRejectsUnknownType(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages",
		`{"type":"carrier-pigeon","to":"5547988359190"}`, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}

func TestSendRichRejectsStickerOnGeneric(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages",
		`{"type":"sticker","to":"5547988359190"}`, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none: sticker rides /messages/media only", len(svc.enqueueCalls))
	}
}

func TestSendRichDisconnected(t *testing.T) {
	svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
		return uuid.Nil, message.ErrInstanceNotConnected
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages",
		`{"type":"poll","to":"5547","question":"Q?","options":["a","b"]}`, nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestSendPollReplayThroughServer(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
		return messageID, nil
	}}
	srv := messagesServer(t, svc, newFakeIdempotency())
	path := "/instances/" + id.String() + "/messages"
	headers := map[string]string{idempotencyKeyHeader: "poll-key-1"}
	body := `{"type":"poll","to":"5547","question":"Q?","options":["a","b"]}`

	first := serveMessages(t, srv, http.MethodPost, path, body, headers)
	second := serveMessages(t, srv, http.MethodPost, path, body, headers)

	if first.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusAccepted)
	}
	if second.Code != http.StatusAccepted {
		t.Fatalf("replay status = %d, want %d", second.Code, http.StatusAccepted)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("replay body = %q, want the original %q", second.Body.String(), first.Body.String())
	}
	if got := second.Header().Get(idempotentReplayHeader); got != "true" {
		t.Errorf("%s = %q, want %q", idempotentReplayHeader, got, "true")
	}
	if len(svc.enqueueCalls) != 1 {
		t.Errorf("Enqueue calls = %d, want 1 for a replayed send", len(svc.enqueueCalls))
	}
}

func TestSendStickerViaMediaAccepted(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if input.Type != message.TypeSticker {
			t.Errorf("Enqueue type = %q, want %q", input.Type, message.TypeSticker)
		}
		if input.MediaID == nil {
			t.Error("Enqueue media id is nil, want the stored sticker")
		}
		return uuid.New(), nil
	}}
	srv := mediaUploadServer(t, svc, &fakeMediaStore{}, newFakeIdempotency())
	webp := []byte("RIFF....WEBPVP8 fake-bytes")

	rec := serveMediaUpload(t, srv, id,
		[][2]string{{"to", "5547988359190"}, {"type", "sticker"}}, "sticker.webp", "image/webp", webp, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

func TestSendStickerViaMediaRejectsNonWebp(t *testing.T) {
	svc := &fakeMessageService{}
	srv := mediaUploadServer(t, svc, &fakeMediaStore{}, newFakeIdempotency())

	rec := serveMediaUpload(t, srv, uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "sticker"}}, "photo.jpg", "image/jpeg", []byte("fake-jpeg"), nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}
