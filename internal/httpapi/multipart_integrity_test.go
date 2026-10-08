package httpapi_test

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/httpapi/core"
)

func serveMultipartPath(t *testing.T, srv *http.Server, path string, body []byte, formType, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("apikey", testToken)
	req.Header.Set("Content-Type", formType)
	if key != "" {
		req.Header.Set(core.IdempotencyKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

func TestMultipartMessageUsesOnlyFormFields(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		t.Run(map[bool]string{false: "without key", true: "with key"}[keyed], func(t *testing.T) {
			svc := &fakeMessageService{}
			store := &fakeMediaStore{}
			srv := mediaUploadServer(t, svc, store, nil)
			path := "/instances/" + uuid.NewString() + "/messages/media"
			body, formType := multipartBody(t, [][2]string{
				{"to", "5547988359190"}, {"type", "image"}, {"caption", "form caption"}, {"filename", "form.jpg"}, {"ptt", "false"},
			}, "upload.jpg", "image/jpeg", []byte("image bytes"))
			key := ""
			if keyed {
				key = "form-query"
			}
			first := serveMultipartPath(t, srv, path+"?to=5547999999999&type=image&caption=query&filename=query.jpg&ptt=true", body, formType, key)
			if first.Code != http.StatusAccepted {
				t.Fatalf("status = %d, body %q", first.Code, first.Body.String())
			}
			if len(svc.enqueueCalls) != 1 || len(store.saveCalls) != 1 {
				t.Fatal("expected one stored upload and message")
			}
			input := svc.enqueueCalls[0].input
			if input.To != "5547988359190" || input.Caption != "form caption" || input.Filename != "form.jpg" || input.PTT {
				t.Errorf("effective message fields came from query: %+v", input)
			}
			if keyed {
				replay := serveMultipartPath(t, srv, path+"?to=5547111111111&type=video&caption=other&filename=other.jpg&ptt=invalid", body, formType, key)
				if replay.Code != first.Code || replay.Body.String() != first.Body.String() || replay.Header().Get(core.IdempotentReplayHeader) != "true" {
					t.Error("query-only change did not replay the same form")
				}
				if len(svc.enqueueCalls) != 1 || len(store.saveCalls) != 1 {
					t.Error("query-only retry repeated an effect")
				}
			}
		})
	}
}

func TestMultipartStatusUsesOnlyFormFields(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		t.Run(map[bool]string{false: "without key", true: "with key"}[keyed], func(t *testing.T) {
			svc := &fakeInstanceService{}
			srv := statusServer(t, svc, testMaxMediaBytes)
			path := "/instances/" + uuid.NewString() + "/status/updates/media"
			body, formType := multipartBody(t, [][2]string{{"type", "image"}, {"caption", "form caption"}}, "upload.jpg", "image/jpeg", []byte("image bytes"))
			key := ""
			if keyed {
				key = "status-form-query"
			}
			first := serveMultipartPath(t, srv, path+"?type=image&caption=query", body, formType, key)
			if first.Code != http.StatusAccepted {
				t.Fatalf("status = %d, body %q", first.Code, first.Body.String())
			}
			if len(svc.publishStatusCalls) != 1 {
				t.Fatal("expected one published status")
			}
			if input := svc.publishStatusCalls[0].Input; input.Caption != "form caption" || input.MediaMime != "image/jpeg" || string(input.MediaData) != "image bytes" {
				t.Errorf("effective status differs from form: %+v", input)
			}
			if keyed {
				replay := serveMultipartPath(t, srv, path+"?type=video&caption=other", body, formType, key)
				if replay.Code != first.Code || replay.Body.String() != first.Body.String() || replay.Header().Get(core.IdempotentReplayHeader) != "true" {
					t.Error("query-only change did not replay the same status form")
				}
				if len(svc.publishStatusCalls) != 1 {
					t.Error("query-only retry repeated status publish")
				}
			}
		})
	}
}

func TestMultipartMediaKindIgnoresQuery(t *testing.T) {
	for _, route := range []string{"message", "status"} {
		t.Run(route, func(t *testing.T) {
			messages := &fakeMessageService{}
			instances := &fakeInstanceService{}
			srv := mediaUploadServer(t, messages, nil, nil)
			path := "/instances/" + uuid.NewString() + "/messages/media"
			if route == "status" {
				srv = statusServer(t, instances, testMaxMediaBytes)
				path = "/instances/" + uuid.NewString() + "/status/updates/media"
			}
			body, formType := multipartBody(t, [][2]string{{"type", "image"}, {"to", "5547988359190"}}, "upload.jpg", "image/jpeg", []byte("image bytes"))
			rec := serveMultipartPath(t, srv, path+"?type=video", body, formType, "")
			if rec.Code != http.StatusAccepted {
				t.Errorf("query kind overrode form kind: status = %d, body %q", rec.Code, rec.Body.String())
			}
			if route == "message" && len(messages.enqueueCalls) != 1 {
				t.Error("form image was not sent")
			}
			if route == "status" && len(instances.publishStatusCalls) != 1 {
				t.Error("form image was not published")
			}
		})
	}
}

func TestMultipartRepeatedFieldsRetainEffectiveOrder(t *testing.T) {
	for _, route := range []string{"message", "status"} {
		t.Run(route, func(t *testing.T) {
			messages := &fakeMessageService{}
			instances := &fakeInstanceService{}
			store := &fakeMediaStore{}
			srv := mediaUploadServer(t, messages, store, nil)
			path := "/instances/" + uuid.NewString() + "/messages/media"
			base := [][2]string{{"type", "image"}}
			field := "to"
			firstValue, secondValue := "5547988359190", "5547999999999"
			if route == "status" {
				srv = statusServer(t, instances, testMaxMediaBytes)
				path = "/instances/" + uuid.NewString() + "/status/updates/media"
				field, firstValue, secondValue = "caption", "first caption", "second caption"
			}
			firstFields := append(append([][2]string{}, base...), [2]string{field, firstValue}, [2]string{field, secondValue})
			secondFields := append(append([][2]string{}, base...), [2]string{field, secondValue}, [2]string{field, firstValue})
			body, formType := multipartBody(t, firstFields, "upload.jpg", "image/jpeg", []byte("image bytes"))
			first := serveMultipartPath(t, srv, path, body, formType, "repeated-values")
			if first.Code != http.StatusAccepted {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			if route == "message" && messages.enqueueCalls[0].input.To != "5547988359190" {
				t.Error("message did not consume the first form value")
			}
			if route == "status" && instances.publishStatusCalls[0].Input.Caption != "first caption" {
				t.Error("status did not consume the first form value")
			}
			body, formType = multipartBody(t, secondFields, "upload.jpg", "image/jpeg", []byte("image bytes"))
			second := serveMultipartPath(t, srv, path, body, formType, "repeated-values")
			if second.Code != http.StatusUnprocessableEntity {
				t.Errorf("reordered repeated fields status = %d, want 422", second.Code)
			}
			if route == "message" && (len(messages.enqueueCalls) != 1 || len(store.saveCalls) != 1) {
				t.Error("divergent form repeated message/upload effect")
			}
			if route == "status" && len(instances.publishStatusCalls) != 1 {
				t.Error("divergent form repeated status publish")
			}
		})
	}
}

func duplicateFileMultipart(t *testing.T, fields [][2]string, contents []string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, content := range contents {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": "upload.jpg"}))
		header.Set("Content-Type", "image/jpeg")
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func TestMultipartRepeatedFilesRetainEffectiveOrder(t *testing.T) {
	for _, route := range []string{"message", "status"} {
		t.Run(route, func(t *testing.T) {
			messages := &fakeMessageService{}
			instances := &fakeInstanceService{}
			store := &fakeMediaStore{}
			srv := mediaUploadServer(t, messages, store, nil)
			path := "/instances/" + uuid.NewString() + "/messages/media"
			fields := [][2]string{{"to", "5547988359190"}, {"type", "image"}}
			if route == "status" {
				srv = statusServer(t, instances, testMaxMediaBytes)
				path = "/instances/" + uuid.NewString() + "/status/updates/media"
			}
			body, formType := duplicateFileMultipart(t, fields, []string{"first image", "second image"})
			first := serveMultipartPath(t, srv, path, body, formType, "repeated-files")
			if first.Code != http.StatusAccepted {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			if route == "message" && string(store.saveCalls[0].data) != "first image" {
				t.Error("message did not store the first file")
			}
			if route == "status" && string(instances.publishStatusCalls[0].Input.MediaData) != "first image" {
				t.Error("status did not publish the first file")
			}
			body, formType = duplicateFileMultipart(t, fields, []string{"second image", "first image"})
			second := serveMultipartPath(t, srv, path, body, formType, "repeated-files")
			if second.Code != http.StatusUnprocessableEntity {
				t.Errorf("reordered repeated files status = %d, want 422", second.Code)
			}
			if route == "message" && (len(messages.enqueueCalls) != 1 || len(store.saveCalls) != 1) {
				t.Error("divergent files repeated message/upload effect")
			}
			if route == "status" && len(instances.publishStatusCalls) != 1 {
				t.Error("divergent files repeated status publish")
			}
		})
	}
}
