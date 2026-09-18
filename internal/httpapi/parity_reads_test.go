package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
	"wzap/internal/session"
)

// parityReadsServer wires every Fase-1 read handler at its future route behind
// the production auth chain so the tests exercise the same boundary Task 9
// will register in server.go. Registering them together also surfaces any Go
// ServeMux pattern conflict before the wiring lands.
func parityReadsServer(t *testing.T, svc InstanceService) *http.Server {
	t.Helper()
	log := zerolog.Nop()
	mux := http.NewServeMux()
	mux.Handle("GET /instances/{id}/groups", handleListJoinedGroups(svc, log))
	mux.Handle("GET /instances/{id}/groups/invite-preview", handleInvitePreview(svc, log))
	mux.Handle("POST /instances/{id}/contacts/check", handleCheckContacts(svc, log))
	mux.Handle("GET /instances/{id}/contacts/{jid}/devices", handleContactDevices(svc, log))
	mux.Handle("GET /instances/{id}/contacts/{jid}/photo", handleContactPhoto(svc, log))
	mux.Handle("GET /instances/{id}/contacts/{jid}/business", handleContactBusiness(svc, log))
	mux.Handle("GET /instances/{id}/blocklist", handleGetBlocklist(svc, log))
	mux.Handle("GET /instances/{id}/status/privacy", handleGetStatusPrivacy(svc, log))
	mux.Handle("GET /instances/{id}/chats/{chat}/disappearing", handleGetDisappearing(svc, log))
	mux.Handle("GET /instances/{id}/newsletters/{channel}/messages", handleGetNewsletterMessages(svc, log))
	mux.Handle("GET /instances/{id}/newsletters/{channel}/updates", handleGetNewsletterUpdates(svc, log))
	return &http.Server{Handler: RequestID(Authenticate(testToken, nil, "")(mux))}
}

func TestListJoinedGroups(t *testing.T) {
	t.Run("connected answers a page with next cursor", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			getJoinedGroupsFn: func(_ context.Context, instanceID uuid.UUID, limit int, cursor string) ([]instance.Group, string, error) {
				if instanceID != id {
					t.Errorf("GetJoinedGroups instance = %s, want %s", instanceID, id)
				}
				if limit != 50 {
					t.Errorf("GetJoinedGroups limit = %d, want the default 50", limit)
				}
				return []instance.Group{
					{JID: "120363000000000001@g.us", Name: "Time do churrasco", ParticipantCount: 3},
				}, "120363000000000001@g.us", nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+id.String()+"/groups", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data joinedGroupsResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Items) != 1 || payload.Data.Items[0].JID != "120363000000000001@g.us" {
			t.Errorf("data.items = %+v, want the single group", payload.Data.Items)
		}
		if payload.Data.NextCursor == "" {
			t.Error("data.next_cursor is empty, want the page cursor")
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			getJoinedGroupsFn: func(context.Context, uuid.UUID, int, string) ([]instance.Group, string, error) {
				return nil, "", instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups", "")

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}

func TestCheckContacts(t *testing.T) {
	t.Run("valid batch answers one result per phone", func(t *testing.T) {
		id := uuid.New()
		seen := time.Now().UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			checkContactsFn: func(_ context.Context, instanceID uuid.UUID, phones []string) ([]session.ContactCheckResult, error) {
				if len(phones) != 2 {
					t.Errorf("CheckContacts phones = %v, want 2 numbers", phones)
				}
				return []session.ContactCheckResult{
					{Phone: phones[0], JID: "5511999999999@s.whatsapp.net", IsOnWhatsApp: true, LastSeen: &seen},
					{Phone: phones[1], JID: "5511888888888@s.whatsapp.net", IsOnWhatsApp: false},
				}, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/contacts/check",
			`{"phones":["5511999999999","5511888888888"]}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data checkContactsResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Items) != 2 {
			t.Fatalf("data.items length = %d, want 2", len(payload.Data.Items))
		}
		if payload.Data.Items[0].JID != "5511999999999@s.whatsapp.net" || !payload.Data.Items[0].IsOnWhatsApp {
			t.Errorf("data.items[0] = %+v, want the registered contact", payload.Data.Items[0])
		}
		if payload.Data.Items[1].IsOnWhatsApp {
			t.Errorf("data.items[1] = %+v, want is_on_whatsapp false", payload.Data.Items[1])
		}
	})
}

func TestCheckContactsRejects51(t *testing.T) {
	svc := &fakeInstanceService{}
	phones := make([]string, 0, 51)
	for i := 0; i < 51; i++ {
		phones = append(phones, `"55119`+strings.Repeat("0", 2)+`000`+strings.Repeat("1", 4)+`"`)
	}
	rec := serveJSON(t, parityReadsServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/contacts/check",
		`{"phones":[`+strings.Join(phones, ",")+`]}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.checkContactsCalls) != 0 {
		t.Errorf("CheckContacts calls = %d, want none above the 50 cap", len(svc.checkContactsCalls))
	}
}

func TestCheckContactsRejectsEmpty(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityReadsServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/contacts/check",
		`{"phones":[]}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.checkContactsCalls) != 0 {
		t.Errorf("CheckContacts calls = %d, want none on an empty batch", len(svc.checkContactsCalls))
	}
}

func TestInvitePreview(t *testing.T) {
	t.Run("valid code answers the preview without joining", func(t *testing.T) {
		svc := &fakeInstanceService{
			getGroupInvitePreviewFn: func(_ context.Context, _ uuid.UUID, code string) (instance.Group, error) {
				if code != "invite-code-1" {
					t.Errorf("GetGroupInvitePreview code = %q, want invite-code-1", code)
				}
				return instance.Group{JID: "120363000000000001@g.us", Name: "Time do churrasco"}, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups/invite-preview?code=invite-code-1", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.JID != "120363000000000001@g.us" {
			t.Errorf("data.jid = %q, want the previewed group", payload.Data.JID)
		}
	})

	t.Run("blank code answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups/invite-preview?code=", "")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.getGroupInvitePreviewCalls) != 0 {
			t.Errorf("GetGroupInvitePreview calls = %d, want none on a blank code", len(svc.getGroupInvitePreviewCalls))
		}
	})

	t.Run("unknown code answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{
			getGroupInvitePreviewFn: func(context.Context, uuid.UUID, string) (instance.Group, error) {
				return instance.Group{}, instance.ErrInvalidInput
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups/invite-preview?code=expired", "")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
	})
}

func TestContactDirectoryReads(t *testing.T) {
	const contact = "5511999999999@s.whatsapp.net"

	t.Run("devices answers the list", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactDevicesFn: func(_ context.Context, _ uuid.UUID, jid string) ([]string, error) {
				if jid != contact {
					t.Errorf("GetContactDevices jid = %q, want %q", jid, contact)
				}
				return []string{contact, "5511999999999:12@s.whatsapp.net"}, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contacts/"+escapeJID(contact)+"/devices", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data contactDevicesResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Devices) != 2 {
			t.Errorf("data.devices = %v, want 2 entries", payload.Data.Devices)
		}
	})

	t.Run("photo answers url and version", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactPhotoFn: func(context.Context, uuid.UUID, string) (session.ProfilePictureInfo, error) {
				return session.ProfilePictureInfo{URL: "https://example.com/photo.jpg", Version: "v3"}, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contacts/"+escapeJID(contact)+"/photo", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data contactPhotoResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.URL != "https://example.com/photo.jpg" || payload.Data.Version != "v3" {
			t.Errorf("data = %+v, want the photo url and version", payload.Data)
		}
	})

	t.Run("business answers the profile", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactBusinessFn: func(context.Context, uuid.UUID, string) (session.BusinessProfile, error) {
				return session.BusinessProfile{Name: "Loja", Description: "Ofertas", VerifiedName: "Loja"}, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contacts/"+escapeJID(contact)+"/business", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data contactBusinessResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Name != "Loja" {
			t.Errorf("data = %+v, want the business profile", payload.Data)
		}
	})

	t.Run("unknown contact answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactDevicesFn: func(context.Context, uuid.UUID, string) ([]string, error) {
				return nil, instance.ErrContactNotFound
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contacts/"+escapeJID(contact)+"/devices", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
			t.Errorf("error code = %q, want not_found", code)
		}
	})
}

func TestGetBlocklist(t *testing.T) {
	svc := &fakeInstanceService{
		getBlocklistFn: func(context.Context, uuid.UUID) ([]string, error) {
			return []string{"5511888888888@s.whatsapp.net"}, nil
		},
	}
	rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
		"/instances/"+uuid.NewString()+"/blocklist", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data blocklistResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Items) != 1 {
		t.Errorf("data.items = %v, want the blocked JID", payload.Data.Items)
	}
}

func TestGetStatusPrivacy(t *testing.T) {
	svc := &fakeInstanceService{
		getStatusPrivacyFn: func(context.Context, uuid.UUID) (session.StatusPrivacy, error) {
			return session.StatusPrivacy{Mode: "contacts", JIDs: []string{}}, nil
		},
	}
	rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
		"/instances/"+uuid.NewString()+"/status/privacy", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data statusPrivacyResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Mode != "contacts" {
		t.Errorf("data.mode = %q, want contacts", payload.Data.Mode)
	}
	if payload.Data.JIDs == nil {
		t.Error("data.jids is null, want an empty array")
	}
}

func TestGetDisappearing(t *testing.T) {
	const chat = "5511999999999@s.whatsapp.net"

	t.Run("chat with timer answers duration", func(t *testing.T) {
		svc := &fakeInstanceService{
			getDisappearingTimerFn: func(_ context.Context, _ uuid.UUID, chatJID string) (time.Duration, bool, error) {
				if chatJID != chat {
					t.Errorf("GetDisappearingTimer chat = %q, want %q", chatJID, chat)
				}
				return 24 * time.Hour, true, nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/chats/"+escapeJID(chat)+"/disappearing", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data disappearingResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Found || payload.Data.DurationSeconds != 86400 {
			t.Errorf("data = %+v, want found with 86400s", payload.Data)
		}
	})
}

func TestGetNewsletterMessages(t *testing.T) {
	t.Run("channel page answers items with next cursor", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			getNewsletterMessagesFn: func(_ context.Context, _ uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error) {
				if channel != testChannel {
					t.Errorf("GetNewsletterMessages channel = %q, want %q", channel, testChannel)
				}
				if limit != 50 {
					t.Errorf("GetNewsletterMessages limit = %d, want the default 50", limit)
				}
				return []session.NewsletterMessage{
					{ServerID: "srv-1", Content: "Oferta", Timestamp: now},
				}, "cursor-1", nil
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/messages", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data newsletterMessagesResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Items) != 1 || payload.Data.Items[0].ServerID != "srv-1" {
			t.Errorf("data.items = %+v, want the message", payload.Data.Items)
		}
		if payload.Data.NextCursor != "cursor-1" {
			t.Errorf("data.next_cursor = %q, want cursor-1", payload.Data.NextCursor)
		}
	})

	t.Run("unknown channel answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getNewsletterMessagesFn: func(context.Context, uuid.UUID, string, string, int) ([]session.NewsletterMessage, string, error) {
				return nil, "", instance.ErrNewsletterNotFound
			},
		}
		rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/messages", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestGetNewsletterUpdates(t *testing.T) {
	svc := &fakeInstanceService{
		getNewsletterUpdatesFn: func(_ context.Context, _ uuid.UUID, channel string) ([]session.NewsletterMessage, error) {
			if channel != testChannel {
				t.Errorf("GetNewsletterUpdates channel = %q, want %q", channel, testChannel)
			}
			return []session.NewsletterMessage{{ServerID: "srv-9", Content: "Nova"}}, nil
		},
	}
	rec := serveJSON(t, parityReadsServer(t, svc), http.MethodGet,
		"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/updates", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data newsletterUpdatesResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ServerID != "srv-9" {
		t.Errorf("data.items = %+v, want the update", payload.Data.Items)
	}
}
