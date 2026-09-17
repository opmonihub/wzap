package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/instance"
	"wzap/internal/session"
)

// statusServer builds the API server with the instance fake, the idempotency
// store and the media cap the status routes run with.
func statusServer(t *testing.T, svc *fakeInstanceService, maxMediaBytes int64) *http.Server {
	t.Helper()
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, MaxMediaBytes: maxMediaBytes},
		zerolog.Nop(),
		Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    svc,
			Idempotency:  newFakeIdempotency(),
		})
}

// serveStatus sends an authenticated status request with an optional body and
// extra headers through the server handler.
func serveStatus(t *testing.T, srv *http.Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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

// serveStatusUpload sends an authenticated multipart status media upload
// through the server handler.
func serveStatusUpload(
	t *testing.T, srv *http.Server, id uuid.UUID,
	fields [][2]string, filename, contentType string, content []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	body, formType := multipartBody(t, fields, filename, contentType, content)
	req := httptest.NewRequest(http.MethodPost, "/instances/"+id.String()+"/status/updates/media", bytes.NewReader(body))
	req.Header.Set("apikey", testToken)
	req.Header.Set("Content-Type", formType)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

// statusAcceptedPayload is the decoded data of a 202 publish response.
type statusAcceptedPayload struct {
	Data struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
	} `json:"data"`
}

func TestStatusPublishText(t *testing.T) {
	t.Run("valid text answers accepted with the upstream id", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			publishStatusFn: func(_ context.Context, instanceID uuid.UUID, input session.StatusInput) (string, error) {
				if instanceID != id {
					t.Errorf("PublishStatus instance = %s, want %s", instanceID, id)
				}
				if input.Text != "bom dia" || len(input.MediaData) != 0 {
					t.Errorf("PublishStatus input = %+v, want the text only", input)
				}
				return "wamid.status1", nil
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodPost,
			"/instances/"+id.String()+"/status/updates", `{"type":"text","text":"bom dia"}`, nil)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusAccepted, rec.Body.String())
		}
		var payload statusAcceptedPayload
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.MessageID != "wamid.status1" {
			t.Errorf("data.message_id = %q, want the upstream id", payload.Data.MessageID)
		}
		if payload.Data.Status != "published" {
			t.Errorf("data.status = %q, want published", payload.Data.Status)
		}
		if len(svc.publishStatusCalls) != 1 {
			t.Errorf("PublishStatus calls = %d, want 1", len(svc.publishStatusCalls))
		}
	})

	t.Run("empty text answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodPost,
			"/instances/"+uuid.NewString()+"/status/updates", `{"type":"text","text":"  "}`, nil)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.publishStatusCalls) != 0 {
			t.Errorf("PublishStatus calls = %d, want none on empty text", len(svc.publishStatusCalls))
		}
	})

	t.Run("unknown type answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodPost,
			"/instances/"+uuid.NewString()+"/status/updates", `{"type":"audio","text":"oi"}`, nil)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.publishStatusCalls) != 0 {
			t.Errorf("PublishStatus calls = %d, want none on unknown type", len(svc.publishStatusCalls))
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			publishStatusFn: func(context.Context, uuid.UUID, session.StatusInput) (string, error) {
				return "", instance.ErrNotConnected
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodPost,
			"/instances/"+uuid.NewString()+"/status/updates", `{"type":"text","text":"oi"}`, nil)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}

func TestStatusPublishMedia(t *testing.T) {
	t.Run("image answers accepted with the upstream id", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			publishStatusFn: func(_ context.Context, instanceID uuid.UUID, input session.StatusInput) (string, error) {
				if string(input.MediaData) != "bytes da foto" {
					t.Errorf("PublishStatus media = %q, want the uploaded bytes", input.MediaData)
				}
				if input.MediaMime != "image/jpeg" || input.Caption != "olha" {
					t.Errorf("PublishStatus input = %+v, want jpeg with caption", input)
				}
				return "wamid.status2", nil
			},
		}
		rec := serveStatusUpload(t, statusServer(t, svc, testMaxMediaBytes), id,
			[][2]string{{"type", "image"}, {"caption", "olha"}},
			"foto.jpg", "image/jpeg", []byte("bytes da foto"))

		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusAccepted, rec.Body.String())
		}
		var payload statusAcceptedPayload
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.MessageID != "wamid.status2" {
			t.Errorf("data.message_id = %q, want the upstream id", payload.Data.MessageID)
		}
	})

	t.Run("file above the limit answers unprocessable and publishes nothing", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveStatusUpload(t, statusServer(t, svc, testMaxMediaBytes), id,
			[][2]string{{"type", "video"}},
			"video.mp4", "video/mp4", bytes.Repeat([]byte("x"), testMaxMediaBytes+1))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
			t.Errorf("error code = %q, want unprocessable_entity", code)
		}
		if len(svc.publishStatusCalls) != 0 {
			t.Errorf("PublishStatus calls = %d, want none above the limit", len(svc.publishStatusCalls))
		}
	})

	t.Run("mismatched file type answers unprocessable without touching the session", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveStatusUpload(t, statusServer(t, svc, testMaxMediaBytes), id,
			[][2]string{{"type", "image"}},
			"video.mp4", "video/mp4", []byte("bytes"))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.publishStatusCalls) != 0 {
			t.Errorf("PublishStatus calls = %d, want none on mismatched type", len(svc.publishStatusCalls))
		}
	})
}

func TestStatusList(t *testing.T) {
	t.Run("published statuses answer their snapshot", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			listStatusesFn: func(_ context.Context, instanceID uuid.UUID) ([]session.StatusInfo, error) {
				if instanceID != id {
					t.Errorf("ListStatuses instance = %s, want %s", instanceID, id)
				}
				return []session.StatusInfo{{ID: "wamid.s1", Kind: session.StatusKindText, Text: "oi"}}, nil
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodGet,
			"/instances/"+id.String()+"/status/updates", "", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data struct {
				Items []ownStatusResponse `json:"items"`
			} `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "wamid.s1" {
			t.Errorf("data.items = %+v, want the published status", payload.Data.Items)
		}
	})

	t.Run("no statuses answer an empty array", func(t *testing.T) {
		svc := &fakeInstanceService{
			listStatusesFn: func(context.Context, uuid.UUID) ([]session.StatusInfo, error) {
				return nil, nil
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodGet,
			"/instances/"+uuid.NewString()+"/status/updates", "", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data struct {
				Items []ownStatusResponse `json:"items"`
			} `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Items == nil {
			t.Error("data.items is nil, want an empty array")
		}
	})
}

func TestStatusDelete(t *testing.T) {
	t.Run("own status answers deleted true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			deleteStatusFn: func(_ context.Context, instanceID uuid.UUID, statusID string) error {
				if instanceID != id || statusID != "wamid.s1" {
					t.Errorf("DeleteStatus = (%s, %q), want (%s, wamid.s1)", instanceID, statusID, id)
				}
				return nil
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodDelete,
			"/instances/"+id.String()+"/status/updates/wamid.s1", "", nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data struct {
				Deleted bool `json:"deleted"`
			} `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Deleted {
			t.Error("data.deleted = false, want true")
		}
	})

	t.Run("unknown status answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			deleteStatusFn: func(context.Context, uuid.UUID, string) error {
				return instance.ErrStatusNotFound
			},
		}
		rec := serveStatus(t, statusServer(t, svc, testMaxMediaBytes), http.MethodDelete,
			"/instances/"+uuid.NewString()+"/status/updates/wamid.gone", "", nil)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
			t.Errorf("error code = %q, want not_found", code)
		}
	})
}
