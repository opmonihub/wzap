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

// profileServer builds the API server with the instance fake for the
// profile and privacy routes.
func profileServer(t *testing.T, svc *fakeInstanceService, maxMediaBytes int64) *http.Server {
	t.Helper()
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, MaxMediaBytes: maxMediaBytes},
		zerolog.Nop(),
		Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    svc,
		})
}

// serveProfile sends an authenticated profile request through the server
// handler.
func serveProfile(t *testing.T, srv *http.Server, method, path, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("apikey", testToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

func TestGetProfile(t *testing.T) {
	t.Run("connected instance answers profile", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			getProfileFn: func(_ context.Context, instanceID uuid.UUID) (session.Profile, error) {
				if instanceID != id {
					t.Errorf("GetProfile instance = %s, want %s", instanceID, id)
				}
				return session.Profile{Name: "Loja", StatusText: "aberto"}, nil
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodGet,
			"/instances/"+id.String()+"/profile", "", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("disconnected instance answers conflict", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			getProfileFn: func(context.Context, uuid.UUID) (session.Profile, error) {
				return session.Profile{}, instance.ErrNotConnected
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodGet,
			"/instances/"+id.String()+"/profile", "", "")

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}

func TestProfileGet(t *testing.T) {
	t.Run("own profile answers its fields", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			getProfileFn: func(_ context.Context, instanceID uuid.UUID) (session.Profile, error) {
				if instanceID != id {
					t.Errorf("GetProfile instance = %s, want %s", instanceID, id)
				}
				return session.Profile{Name: "Loja", StatusText: "aberto", PhotoURL: "https://example.com/foto.jpg"}, nil
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodGet,
			"/instances/"+id.String()+"/profile", "", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data profileResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Name != "Loja" || payload.Data.StatusText != "aberto" || payload.Data.PhotoURL != "https://example.com/foto.jpg" {
			t.Errorf("data = %+v, want the own profile", payload.Data)
		}
	})
}

func TestProfileUpdate(t *testing.T) {
	t.Run("valid name and recado answer the refreshed profile", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			setProfileNameFn: func(_ context.Context, instanceID uuid.UUID, name string) error {
				if name != "Loja nova" {
					t.Errorf("SetProfileName name = %q, want the trimmed Loja nova", name)
				}
				return nil
			},
			setProfileStatusFn: func(_ context.Context, _ uuid.UUID, text string) error {
				if text != "aberto até 18h" {
					t.Errorf("SetProfileStatusText text = %q, want the recado", text)
				}
				return nil
			},
			getProfileFn: func(context.Context, uuid.UUID) (session.Profile, error) {
				return session.Profile{Name: "Loja nova", StatusText: "aberto até 18h"}, nil
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPatch,
			"/instances/"+id.String()+"/profile",
			`{"name":"  Loja nova  ","status_text":"aberto até 18h"}`, "application/json")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data profileResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Name != "Loja nova" || payload.Data.StatusText != "aberto até 18h" {
			t.Errorf("data = %+v, want the refreshed profile", payload.Data)
		}
		if len(svc.setProfileNameCalls) != 1 || len(svc.setProfileStatusCalls) != 1 {
			t.Errorf("set calls = (%d, %d), want one name and one recado",
				len(svc.setProfileNameCalls), len(svc.setProfileStatusCalls))
		}
	})

	t.Run("name above 100 characters answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/profile",
			`{"name":"`+strings.Repeat("a", 101)+`"}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setProfileNameCalls) != 0 || len(svc.setProfileStatusCalls) != 0 {
			t.Errorf("set calls = (%d, %d), want none on invalid name",
				len(svc.setProfileNameCalls), len(svc.setProfileStatusCalls))
		}
	})

	t.Run("recado above 500 characters answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/profile",
			`{"status_text":"`+strings.Repeat("b", 501)+`"}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setProfileNameCalls) != 0 || len(svc.setProfileStatusCalls) != 0 {
			t.Errorf("set calls = (%d, %d), want none on invalid recado",
				len(svc.setProfileNameCalls), len(svc.setProfileStatusCalls))
		}
	})

	t.Run("empty name answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/profile", `{"name":"   "}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
	})

	t.Run("unsupported name change answers not implemented", func(t *testing.T) {
		svc := &fakeInstanceService{
			setProfileNameFn: func(context.Context, uuid.UUID, string) error {
				return instance.ErrUnsupported
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/profile", `{"name":"Loja"}`, "application/json")

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotImplemented, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_supported" {
			t.Errorf("error code = %q, want not_supported", code)
		}
	})
}

func TestProfilePhoto(t *testing.T) {
	t.Run("image body answers updated true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			setProfilePhotoFn: func(_ context.Context, instanceID uuid.UUID, image []byte) error {
				if string(image) != "bytes da foto" {
					t.Errorf("SetProfilePhoto image = %q, want the uploaded bytes", image)
				}
				return nil
			},
		}
		req := httptest.NewRequest(http.MethodPut, "/instances/"+id.String()+"/profile/photo",
			bytes.NewReader([]byte("bytes da foto")))
		req.Header.Set("apikey", testToken)
		req.Header.Set("Content-Type", "image/jpeg")
		rec := httptest.NewRecorder()
		profileServer(t, svc, 0).Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data struct {
				Updated bool `json:"updated"`
			} `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Updated {
			t.Error("data.updated = false, want true")
		}
	})

	t.Run("non image body answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+uuid.NewString()+"/profile/photo", `{"photo":"x"}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setProfilePhotoCalls) != 0 {
			t.Errorf("SetProfilePhoto calls = %d, want none on non image body", len(svc.setProfilePhotoCalls))
		}
	})

	t.Run("unsupported photo change answers not implemented", func(t *testing.T) {
		svc := &fakeInstanceService{
			setProfilePhotoFn: func(context.Context, uuid.UUID, []byte) error {
				return instance.ErrUnsupported
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+uuid.NewString()+"/profile/photo", "bytes", "image/png")

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotImplemented, rec.Body.String())
		}
	})
}

func TestPrivacyGet(t *testing.T) {
	t.Run("privacy answers one value per field", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			getPrivacyFn: func(_ context.Context, instanceID uuid.UUID) (session.Privacy, error) {
				if instanceID != id {
					t.Errorf("GetPrivacy instance = %s, want %s", instanceID, id)
				}
				return session.Privacy{
					LastSeen:     "contacts",
					ProfilePhoto: "all",
					Status:       "contacts",
					ReadReceipts: "all",
					GroupsAdd:    "contact_blacklist",
				}, nil
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodGet,
			"/instances/"+id.String()+"/privacy", "", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data privacyResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.LastSeen != "contacts" || payload.Data.ReadReceipts != "all" || payload.Data.GroupsAdd != "contact_blacklist" {
			t.Errorf("data = %+v, want the field values", payload.Data)
		}
	})
}

func TestPrivacyUpdate(t *testing.T) {
	t.Run("valid values answer the applied settings", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			setPrivacyFn: func(_ context.Context, instanceID uuid.UUID, input session.Privacy) (session.Privacy, error) {
				if input.LastSeen != "contacts" || input.ReadReceipts != "none" {
					t.Errorf("SetPrivacy input = %+v, want contacts last_seen and none receipts", input)
				}
				if input.ProfilePhoto != "" || input.Status != "" || input.GroupsAdd != "" {
					t.Errorf("SetPrivacy input = %+v, want absent fields empty", input)
				}
				return session.Privacy{LastSeen: "contacts", ReadReceipts: "none"}, nil
			},
		}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+id.String()+"/privacy",
			`{"last_seen":"contacts","read_receipts":"none"}`, "application/json")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data privacyResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.LastSeen != "contacts" || payload.Data.ReadReceipts != "none" {
			t.Errorf("data = %+v, want the applied settings", payload.Data)
		}
	})

	t.Run("value outside the allowlist answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+uuid.NewString()+"/privacy", `{"last_seen":"everyone"}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setPrivacyCalls) != 0 {
			t.Errorf("SetPrivacy calls = %d, want none outside the allowlist", len(svc.setPrivacyCalls))
		}
	})

	t.Run("contacts read receipts answers unprocessable: receipts take all or none", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+uuid.NewString()+"/privacy", `{"read_receipts":"contacts"}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setPrivacyCalls) != 0 {
			t.Errorf("SetPrivacy calls = %d, want none on receipts contacts", len(svc.setPrivacyCalls))
		}
	})

	t.Run("empty patch answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveProfile(t, profileServer(t, svc, 0), http.MethodPut,
			"/instances/"+uuid.NewString()+"/privacy", `{}`, "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
	})
}
