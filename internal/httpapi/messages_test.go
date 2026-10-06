package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/media"
	"wzap/internal/message"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// fakeMessageService is an in-memory MessageService: the function fields
// configure each outcome and the recorded fields expose the calls the handlers
// made.
type fakeMessageService struct {
	enqueueFn func(ctx context.Context, instanceID uuid.UUID, input message.EnqueueInput) (uuid.UUID, error)
	getFn     func(ctx context.Context, instanceID, messageID uuid.UUID) (*model.OutboundMessage, error)
	listFn    func(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error)

	enqueueCalls []enqueueCall
	getCalls     []getCall
	listCalls    []listCall
}

// enqueueCall records one Enqueue invocation.
type enqueueCall struct {
	instanceID uuid.UUID
	input      message.EnqueueInput
}

// getCall records one Get invocation.
type getCall struct {
	instanceID uuid.UUID
	messageID  uuid.UUID
}

// listCall records one List invocation.
type listCall struct {
	instanceID uuid.UUID
	limit      int
	cursor     string
}

// Enqueue records the input and returns the configured id, defaulting to a
// fresh one.
func (f *fakeMessageService) Enqueue(ctx context.Context, instanceID uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
	f.enqueueCalls = append(f.enqueueCalls, enqueueCall{instanceID: instanceID, input: input})
	if f.enqueueFn != nil {
		return f.enqueueFn(ctx, instanceID, input)
	}
	return uuid.New(), nil
}

// Get records the ids and returns the configured message, defaulting to
// message.ErrMessageNotFound.
func (f *fakeMessageService) Get(ctx context.Context, instanceID, messageID uuid.UUID) (*model.OutboundMessage, error) {
	f.getCalls = append(f.getCalls, getCall{instanceID: instanceID, messageID: messageID})
	if f.getFn != nil {
		return f.getFn(ctx, instanceID, messageID)
	}
	return nil, message.ErrMessageNotFound
}

// List records the pagination and returns the configured page, defaulting to an
// empty one.
func (f *fakeMessageService) List(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error) {
	f.listCalls = append(f.listCalls, listCall{instanceID: instanceID, limit: limit, cursor: cursor})
	if f.listFn != nil {
		return f.listFn(ctx, instanceID, limit, cursor)
	}
	return nil, "", nil
}

// testMaxMediaBytes is the upload cap the handler tests configure.
const testMaxMediaBytes = 1 << 10

// messagesServer builds the server under test with the given message service
// and idempotency repository.
func messagesServer(t *testing.T, svc MessageService, repo storage.IdempotencyRepository) *http.Server {
	t.Helper()
	return mediaUploadServer(t, svc, &fakeMediaStore{}, repo)
}

// mediaUploadServer builds the server under test with the given message
// service, media store and idempotency repository. Instances default to a
// fake answering every id so global-scope tests exercise the operation
// behind the ownership gate.
func mediaUploadServer(t *testing.T, svc MessageService, store MediaStore, repo storage.IdempotencyRepository) *http.Server {
	t.Helper()
	if svc == nil {
		svc = &fakeMessageService{}
	}
	if store == nil {
		store = &fakeMediaStore{}
	}
	if repo == nil {
		repo = newFakeIdempotency()
	}
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, MaxMediaBytes: testMaxMediaBytes},
		zerolog.Nop(),
		Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    &fakeInstanceService{},
			Messages:     svc,
			Media:        store,
			Idempotency:  repo,
		})
}

// serveMediaUpload sends an authenticated multipart upload through the server
// handler. An empty filename sends the fields without a file part.
func serveMediaUpload(
	t *testing.T, srv *http.Server, id uuid.UUID,
	fields [][2]string, filename, contentType string, content []byte, headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	body, formType := multipartBody(t, fields, filename, contentType, content)
	req := httptest.NewRequest(http.MethodPost, "/instances/"+id.String()+"/messages/media", bytes.NewReader(body))
	req.Header.Set("apikey", testToken)
	req.Header.Set("Content-Type", formType)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

// serveMessages sends an authenticated request with an optional body and extra
// headers through the server handler.
func serveMessages(t *testing.T, srv *http.Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("apikey", testToken)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

// messageAcceptedPayload is the decoded data of a 202 send response: the
// queue message carries its id and the queued send_status under data.message.
type messageAcceptedPayload struct {
	Data struct {
		Message struct {
			ID         string `json:"id"`
			SendStatus string `json:"send_status"`
		} `json:"message"`
	} `json:"data"`
}

func TestSendTextAccepted(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, instanceID uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if instanceID != id {
			t.Errorf("Enqueue instance = %s, want %s", instanceID, id)
		}
		if input.Type != message.TypeText || input.To != "5547988359190" || input.Text != "olá" {
			t.Errorf("Enqueue input = %+v, want type text, recipient and text", input)
		}
		return messageID, nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+id.String()+"/messages/text", `{"to":"5547988359190","text":"olá"}`, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	var payload messageAcceptedPayload
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Message.ID != messageID.String() {
		t.Errorf("data.message.id = %q, want %q", payload.Data.Message.ID, messageID)
	}
	if payload.Data.Message.SendStatus != message.StatusQueued {
		t.Errorf("data.message.send_status = %q, want %q", payload.Data.Message.SendStatus, message.StatusQueued)
	}
}

func TestSendLocationAccepted(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if input.Type != message.TypeLocation {
			t.Errorf("type = %q, want %q", input.Type, message.TypeLocation)
		}
		if input.Latitude != -23.55 || input.Longitude != -46.63 {
			t.Errorf("coordinates = (%v, %v), want (-23.55, -46.63)", input.Latitude, input.Longitude)
		}
		return uuid.New(), nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+id.String()+"/messages/location",
		`{"to":"5547988359190","latitude":-23.55,"longitude":-46.63}`, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestSendLocationRequiresCoordinates(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages/location", `{"to":"5547988359190","latitude":-23.55}`, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
		t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}

func TestSendContactAccepted(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if input.Type != message.TypeContact || input.DisplayName != "Fulano" || input.VCard != "BEGIN:VCARD" {
			t.Errorf("Enqueue input = %+v, want type contact, display name and vcard", input)
		}
		return uuid.New(), nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+id.String()+"/messages/contact",
		`{"to":"5547988359190","display_name":"Fulano","vcard":"BEGIN:VCARD"}`, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestSendRejectsMalformedBody(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages/text", `{"to":`, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "invalid_request" {
		t.Errorf("error code = %q, want %q", code, "invalid_request")
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}

func TestSendRejectsMalformedInstanceID(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
		"/instances/bad.name/messages/text", `{"to":"5547","text":"olá"}`, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}

func TestSendErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "instance not found", err: message.ErrInstanceNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "instance not connected", err: message.ErrInstanceNotConnected, wantStatus: http.StatusConflict, wantCode: "conflict"},
		{name: "number not found", err: message.ErrNumberNotFound, wantStatus: http.StatusUnprocessableEntity, wantCode: "unprocessable_entity"},
		{name: "invalid input", err: message.ErrInvalidInput, wantStatus: http.StatusUnprocessableEntity, wantCode: "unprocessable_entity"},
		{name: "resolver unavailable", err: message.ErrResolverUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: "unavailable"},
		{name: "internal error", err: context.DeadlineExceeded, wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
				return uuid.Nil, tt.err
			}}

			rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodPost,
				"/instances/"+uuid.NewString()+"/messages/text", `{"to":"5547","text":"olá"}`, nil)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if code := errorCode(t, rec.Body.Bytes()); code != tt.wantCode {
				t.Errorf("error code = %q, want %q", code, tt.wantCode)
			}
		})
	}
}

func TestSendTextReplayThroughServer(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
		return messageID, nil
	}}
	srv := messagesServer(t, svc, newFakeIdempotency())
	path := "/instances/" + id.String() + "/messages/text"
	headers := map[string]string{idempotencyKeyHeader: "key-1"}

	first := serveMessages(t, srv, http.MethodPost, path, `{"to":"5547","text":"olá"}`, headers)
	second := serveMessages(t, srv, http.MethodPost, path, `{"to":"5547","text":"olá"}`, headers)

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

func TestSendTextWithoutKeyEnqueuesTwice(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{}
	srv := messagesServer(t, svc, newFakeIdempotency())
	path := "/instances/" + id.String() + "/messages/text"

	serveMessages(t, srv, http.MethodPost, path, `{"to":"5547","text":"olá"}`, nil)
	serveMessages(t, srv, http.MethodPost, path, `{"to":"5547","text":"olá"}`, nil)

	if len(svc.enqueueCalls) != 2 {
		t.Errorf("Enqueue calls = %d, want 2 without an idempotency key", len(svc.enqueueCalls))
	}
}

func TestGetMessage(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	deliveredAt := time.Now().UTC().Add(-time.Minute)
	svc := &fakeMessageService{getFn: func(_ context.Context, instanceID, gotMessageID uuid.UUID) (*model.OutboundMessage, error) {
		if instanceID != id || gotMessageID != messageID {
			t.Errorf("Get(%s, %s), want (%s, %s)", instanceID, gotMessageID, id, messageID)
		}
		return &model.OutboundMessage{
			ID:                messageID,
			InstanceID:        id,
			Type:              message.TypeText,
			RecipientJID:      "5547988359190@s.whatsapp.net",
			Status:            "sent",
			WhatsAppMessageID: "wamid-1",
			// A pre-remodel row carries free text only: the public
			// last_error surfaces it as legacy_error with no invented
			// occurrence time.
			LastError:   "boom",
			Attempts:    2,
			DeliveredAt: &deliveredAt,
			CreatedAt:   deliveredAt,
			UpdatedAt:   deliveredAt,
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
	msg := payload.Data.Message
	if msg.ID != messageID.String() || msg.InstanceID != id.String() {
		t.Errorf("data.message ids = (%q, %q), want (%q, %q)", msg.ID, msg.InstanceID, messageID, id)
	}
	if msg.MessageType != message.TypeText {
		t.Errorf("data.message.message_type = %q, want %q", msg.MessageType, message.TypeText)
	}
	if msg.RecipientJID != "5547988359190@s.whatsapp.net" {
		t.Errorf("data.message.recipient_jid = %q, want the stored JID", msg.RecipientJID)
	}
	if msg.SendStatus != "sent" {
		t.Errorf("data.message.send_status = %q, want %q", msg.SendStatus, "sent")
	}
	if msg.WAID == nil || *msg.WAID != "wamid-1" {
		t.Errorf("data.message.wa_id = %v, want %q", msg.WAID, "wamid-1")
	}
	if msg.LastError == nil || msg.LastError.Code != "legacy_error" || msg.LastError.Message != "boom" || msg.LastError.OccurredAt != nil {
		t.Errorf("data.message.last_error = %+v, want legacy_error boom with null occurred_at", msg.LastError)
	}
	if msg.RetryCount != 2 {
		t.Errorf("data.message.retry_count = %d, want 2", msg.RetryCount)
	}
	if msg.DeliveredAt == nil || !msg.DeliveredAt.Equal(deliveredAt) {
		t.Errorf("data.message.delivered_at = %v, want %v", msg.DeliveredAt, deliveredAt)
	}
	if msg.ReadAt != nil {
		t.Errorf("data.message.read_at = %v, want nil", msg.ReadAt)
	}
	if strings.Contains(rec.Body.String(), `"whatsapp_message_id"`) || strings.Contains(rec.Body.String(), `"attempts"`) {
		t.Errorf("body %q leaks the pre-remodel field names", rec.Body.String())
	}
}

func TestGetMessageNotFound(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+uuid.NewString()+"/messages/"+uuid.NewString(), "", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
		t.Errorf("error code = %q, want %q", code, "not_found")
	}
}

func TestGetMessageRejectsMalformedID(t *testing.T) {
	svc := &fakeMessageService{}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+uuid.NewString()+"/messages/not-a-uuid", "", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(svc.getCalls) != 0 {
		t.Errorf("Get calls = %d, want none", len(svc.getCalls))
	}
}

func TestListMessages(t *testing.T) {
	id := uuid.New()
	svc := &fakeMessageService{listFn: func(_ context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error) {
		if instanceID != id {
			t.Errorf("List instance = %s, want %s", instanceID, id)
		}
		return []model.OutboundMessage{{ID: uuid.New(), InstanceID: id, Type: message.TypeText, Status: "sent"}}, "next-1", nil
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+id.String()+"/messages?limit=10&cursor=start", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data messageListResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(payload.Data.Items))
	}
	if payload.Data.NextCursor != "next-1" {
		t.Errorf("next_cursor = %q, want %q", payload.Data.NextCursor, "next-1")
	}
	if len(svc.listCalls) != 1 || svc.listCalls[0].limit != 10 || svc.listCalls[0].cursor != "start" {
		t.Errorf("List calls = %+v, want limit 10 and cursor start", svc.listCalls)
	}
}

func TestListMessagesDefaultsLimit(t *testing.T) {
	svc := &fakeMessageService{}

	serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+uuid.NewString()+"/messages", "", nil)

	if len(svc.listCalls) != 1 || svc.listCalls[0].limit != defaultMessagesLimit {
		t.Errorf("List calls = %+v, want the default limit %d", svc.listCalls, defaultMessagesLimit)
	}
}

func TestListMessagesInvalidCursor(t *testing.T) {
	svc := &fakeMessageService{listFn: func(context.Context, uuid.UUID, int, string) ([]model.OutboundMessage, string, error) {
		return nil, "", message.ErrInvalidCursor
	}}

	rec := serveMessages(t, messagesServer(t, svc, nil), http.MethodGet,
		"/instances/"+uuid.NewString()+"/messages?cursor=bad", "", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "invalid_request" {
		t.Errorf("error code = %q, want %q", code, "invalid_request")
	}
}

func TestListMessagesEmptyPageIsArray(t *testing.T) {
	rec := serveMessages(t, messagesServer(t, &fakeMessageService{}, nil), http.MethodGet,
		"/instances/"+uuid.NewString()+"/messages", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("body = %q, want an empty items array instead of null", rec.Body.String())
	}
}

func TestSendMediaAccepted(t *testing.T) {
	id := uuid.New()
	storedID := uuid.New()
	messageID := uuid.New()
	store := &fakeMediaStore{saveFn: func(
		_ context.Context, instanceID uuid.UUID, direction, messageID, mimetype, filename string, data []byte,
	) (*model.Media, error) {
		if instanceID != id {
			t.Errorf("Save instance = %s, want %s", instanceID, id)
		}
		if direction != "outbound" {
			t.Errorf("Save direction = %q, want outbound", direction)
		}
		if messageID != "" {
			t.Errorf("Save message id = %q, want empty before the message exists", messageID)
		}
		if mimetype != "image/jpeg" {
			t.Errorf("Save mimetype = %q, want the parsed image/jpeg", mimetype)
		}
		if filename != "foto.jpg" {
			t.Errorf("Save filename = %q, want foto.jpg", filename)
		}
		if string(data) != "bytes da foto" {
			t.Errorf("Save data = %q, want the uploaded bytes", data)
		}
		return &model.Media{ID: storedID, Filename: "foto.jpg", Mimetype: "image/jpeg", SizeBytes: int64(len(data))}, nil
	}}
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, instanceID uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if instanceID != id {
			t.Errorf("Enqueue instance = %s, want %s", instanceID, id)
		}
		if input.Type != message.TypeMedia || input.To != "5547988359190" || input.Caption != "olha" {
			t.Errorf("Enqueue input = %+v, want media, recipient and caption", input)
		}
		if input.Filename != "foto.jpg" {
			t.Errorf("Enqueue filename = %q, want the stored foto.jpg", input.Filename)
		}
		if input.MediaID == nil || *input.MediaID != storedID {
			t.Errorf("Enqueue media id = %v, want %s", input.MediaID, storedID)
		}
		if input.PTT {
			t.Error("Enqueue PTT = true, want false for an image")
		}
		return messageID, nil
	}}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), id,
		[][2]string{{"to", "5547988359190"}, {"type", "image"}, {"caption", "olha"}},
		"foto.jpg", "image/jpeg; charset=binary", []byte("bytes da foto"), nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	var payload messageAcceptedPayload
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Message.ID != messageID.String() {
		t.Errorf("data.message.id = %q, want %q", payload.Data.Message.ID, messageID)
	}
	if payload.Data.Message.SendStatus != message.StatusQueued {
		t.Errorf("data.message.send_status = %q, want %q", payload.Data.Message.SendStatus, message.StatusQueued)
	}
	if len(store.saveCalls) != 1 {
		t.Errorf("Save calls = %d, want 1", len(store.saveCalls))
	}
	if len(svc.enqueueCalls) != 1 {
		t.Errorf("Enqueue calls = %d, want 1", len(svc.enqueueCalls))
	}
}

func TestSendMediaVoiceNote(t *testing.T) {
	id := uuid.New()
	store := &fakeMediaStore{}
	svc := &fakeMessageService{enqueueFn: func(_ context.Context, _ uuid.UUID, input message.EnqueueInput) (uuid.UUID, error) {
		if !input.PTT {
			t.Error("Enqueue PTT = false, want true for a voice note")
		}
		return uuid.New(), nil
	}}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), id,
		[][2]string{{"to", "5547988359190"}, {"type", "audio"}, {"ptt", "true"}},
		"voice.ogg", "audio/ogg", []byte("ogg bytes"), nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestSendMediaDefaultsFilenameFromUpload(t *testing.T) {
	id := uuid.New()
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), id,
		[][2]string{{"to", "5547988359190"}, {"type", "document"}},
		"nota.pdf", "application/pdf", []byte("%PDF"), nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if len(store.saveCalls) != 1 || store.saveCalls[0].filename != "nota.pdf" {
		t.Fatalf("Save calls = %+v, want the uploaded file name", store.saveCalls)
	}
	if len(svc.enqueueCalls) != 1 || svc.enqueueCalls[0].input.Filename != "nota.pdf" {
		t.Fatalf("Enqueue calls = %+v, want the uploaded file name", svc.enqueueCalls)
	}
}

func TestSendMediaRejections(t *testing.T) {
	tests := []struct {
		name        string
		fields      [][2]string
		filename    string
		contentType string
		content     []byte
	}{
		{
			name:        "invalid mime",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "document"}},
			filename:    "malicioso.html",
			contentType: "text/html",
			content:     []byte("<html>"),
		},
		{
			name:        "empty file part content type",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "document"}},
			filename:    "arquivo",
			contentType: "",
			content:     []byte("data"),
		},
		{
			name:        "type does not match the mime",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "image"}},
			filename:    "nota.pdf",
			contentType: "application/pdf",
			content:     []byte("%PDF"),
		},
		{
			name:        "sticker rejects non-webp",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "sticker"}},
			filename:    "foto.jpg",
			contentType: "image/jpeg",
			content:     []byte("jpeg"),
		},
		{
			name:        "missing type",
			fields:      [][2]string{{"to", "5547988359190"}},
			filename:    "foto.jpg",
			contentType: "image/jpeg",
			content:     []byte("jpeg"),
		},
		{
			name:        "missing recipient",
			fields:      [][2]string{{"type", "image"}},
			filename:    "foto.jpg",
			contentType: "image/jpeg",
			content:     []byte("jpeg"),
		},
		{
			name:        "invalid ptt",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "audio"}, {"ptt", "maybe"}},
			filename:    "voice.ogg",
			contentType: "audio/ogg",
			content:     []byte("ogg"),
		},
		{
			name:        "empty file",
			fields:      [][2]string{{"to", "5547988359190"}, {"type", "image"}},
			filename:    "vazia.jpg",
			contentType: "image/jpeg",
			content:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeMediaStore{}
			svc := &fakeMessageService{}

			rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
				tt.fields, tt.filename, tt.contentType, tt.content, nil)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
				t.Errorf("error code = %q, want unprocessable_entity", code)
			}
			if len(store.saveCalls) != 0 {
				t.Errorf("Save calls = %d, want none", len(store.saveCalls))
			}
			if len(svc.enqueueCalls) != 0 {
				t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
			}
		})
	}
}

func TestSendMediaWithoutFileRejected(t *testing.T) {
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "image"}}, "", "", nil, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(store.saveCalls) != 0 || len(svc.enqueueCalls) != 0 {
		t.Errorf("Save calls = %d, Enqueue calls = %d, want none of either",
			len(store.saveCalls), len(svc.enqueueCalls))
	}
}

func TestSendMediaRejectsOversize(t *testing.T) {
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "document"}},
		"grande.pdf", "application/pdf", bytes.Repeat([]byte("a"), testMaxMediaBytes+1), nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
		t.Errorf("error code = %q, want unprocessable_entity", code)
	}
	if len(store.saveCalls) != 0 || len(svc.enqueueCalls) != 0 {
		t.Errorf("Save calls = %d, Enqueue calls = %d, want none of either",
			len(store.saveCalls), len(svc.enqueueCalls))
	}
}

func TestSendMediaRejectsBodyAboveRequestCap(t *testing.T) {
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}
	content := bytes.Repeat([]byte("a"), int(testMaxMediaBytes+mediaFormOverhead)+1)

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "document"}},
		"grande.pdf", "application/pdf", content, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(store.saveCalls) != 0 || len(svc.enqueueCalls) != 0 {
		t.Errorf("Save calls = %d, Enqueue calls = %d, want none of either",
			len(store.saveCalls), len(svc.enqueueCalls))
	}
}

func TestSendMediaRejectsMalformedMultipart(t *testing.T) {
	id := uuid.New()
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}
	srv := mediaUploadServer(t, svc, store, nil)

	req := httptest.NewRequest(http.MethodPost, "/instances/"+id.String()+"/messages/media",
		strings.NewReader("não é multipart"))
	req.Header.Set("apikey", testToken)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "invalid_request" {
		t.Errorf("error code = %q, want invalid_request", code)
	}
	if len(store.saveCalls) != 0 || len(svc.enqueueCalls) != 0 {
		t.Errorf("Save calls = %d, Enqueue calls = %d, want none of either",
			len(store.saveCalls), len(svc.enqueueCalls))
	}
}

func TestSendMediaInstanceNotFound(t *testing.T) {
	store := &fakeMediaStore{saveFn: func(
		context.Context, uuid.UUID, string, string, string, string, []byte,
	) (*model.Media, error) {
		return nil, media.ErrNotFound
	}}
	svc := &fakeMessageService{}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "image"}},
		"foto.jpg", "image/jpeg", []byte("jpeg"), nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(svc.enqueueCalls) != 0 {
		t.Errorf("Enqueue calls = %d, want none", len(svc.enqueueCalls))
	}
}

func TestSendMediaEnqueueErrorMaps(t *testing.T) {
	store := &fakeMediaStore{}
	svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
		return uuid.Nil, message.ErrInstanceNotConnected
	}}

	rec := serveMediaUpload(t, mediaUploadServer(t, svc, store, nil), uuid.New(),
		[][2]string{{"to", "5547988359190"}, {"type", "image"}},
		"foto.jpg", "image/jpeg", []byte("jpeg"), nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "conflict" {
		t.Errorf("error code = %q, want conflict", code)
	}
}

func TestSendMediaReplayThroughServer(t *testing.T) {
	id := uuid.New()
	messageID := uuid.New()
	store := &fakeMediaStore{}
	svc := &fakeMessageService{enqueueFn: func(context.Context, uuid.UUID, message.EnqueueInput) (uuid.UUID, error) {
		return messageID, nil
	}}
	srv := mediaUploadServer(t, svc, store, newFakeIdempotency())
	fields := [][2]string{{"to", "5547988359190"}, {"type", "image"}, {"caption", "olha"}}
	headers := map[string]string{idempotencyKeyHeader: "key-1"}

	first := serveMediaUpload(t, srv, id, fields, "foto.jpg", "image/jpeg", []byte("bytes"), headers)
	second := serveMediaUpload(t, srv, id, fields, "foto.jpg", "image/jpeg", []byte("bytes"), headers)

	if first.Code != http.StatusAccepted || second.Code != http.StatusAccepted {
		t.Fatalf("statuses = %d and %d, want %d for both", first.Code, second.Code, http.StatusAccepted)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("replay body = %q, want the original %q", second.Body.String(), first.Body.String())
	}
	if got := second.Header().Get(idempotentReplayHeader); got != "true" {
		t.Errorf("%s = %q, want true", idempotentReplayHeader, got)
	}
	if len(store.saveCalls) != 1 {
		t.Errorf("Save calls = %d, want 1 for a replayed send", len(store.saveCalls))
	}
	if len(svc.enqueueCalls) != 1 {
		t.Errorf("Enqueue calls = %d, want 1 for a replayed send", len(svc.enqueueCalls))
	}
}

func TestSendMediaSameKeyDifferentFileRejected(t *testing.T) {
	id := uuid.New()
	store := &fakeMediaStore{}
	svc := &fakeMessageService{}
	srv := mediaUploadServer(t, svc, store, newFakeIdempotency())
	fields := [][2]string{{"to", "5547988359190"}, {"type", "image"}, {"caption", "olha"}}
	headers := map[string]string{idempotencyKeyHeader: "key-1"}

	first := serveMediaUpload(t, srv, id, fields, "foto.jpg", "image/jpeg", []byte("primeira"), headers)
	second := serveMediaUpload(t, srv, id, fields, "foto.jpg", "image/jpeg", []byte("segunda"), headers)

	if first.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusAccepted)
	}
	if second.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d for a reused key with a different file", second.Code, http.StatusUnprocessableEntity)
	}
	if len(store.saveCalls) != 1 || len(svc.enqueueCalls) != 1 {
		t.Errorf("Save calls = %d, Enqueue calls = %d, want only the first send",
			len(store.saveCalls), len(svc.enqueueCalls))
	}
}
