package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/channels"
	"wzap/internal/httpapi/chats"
	"wzap/internal/httpapi/contacts"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
)

// The test server exercises resource operations through the production router.

func parityWritesServer(t *testing.T, svc httpapi.InstanceService) *http.Server {
	t.Helper()
	return httpapi.New(config.Config{APIKey: testToken}, zerolog.Nop(), httpapi.Deps{Instances: svc})
}
func TestUpdateBlocklist(t *testing.T) {
	t.Run("block answers updated", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/blocklist",
			`{"action":"block","jid":"5511888888888@s.whatsapp.net"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.updateBlocklistCalls) != 1 || svc.updateBlocklistCalls[0].Action != "block" {
			t.Errorf("UpdateBlocklist calls = %+v, want one block", svc.updateBlocklistCalls)
		}
	})

	t.Run("bad action answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/blocklist",
			`{"action":"mute","jid":"5511888888888@s.whatsapp.net"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateBlocklistCalls) != 0 {
			t.Errorf("UpdateBlocklist calls = %d, want none on a bad action", len(svc.updateBlocklistCalls))
		}
	})
}
func TestDisappearingRejectsBadDuration(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPut, "/instances/"+id.String()+"/chats/"+escapeJID("5511999999999@s.whatsapp.net")+"/disappearing", `{"duration":"5h"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %q)", rec.Code, rec.Body.String())
	}
	if len(svc.setDisappearingTimerCalls) != 0 {
		t.Errorf("SetDisappearingTimer calls = %d, want none outside the allowlist", len(svc.setDisappearingTimerCalls))
	}
}
func TestSetDisappearing(t *testing.T) {
	t.Run("24h acknowledges the updated timer", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPut,
			"/instances/"+id.String()+"/chats/"+escapeJID("5511999999999@s.whatsapp.net")+"/disappearing",
			`{"duration":"24h"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data chats.DisappearingUpdatedResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Updated {
			t.Errorf("data = %+v, want updated", payload.Data)
		}
		if len(svc.setDisappearingTimerCalls) != 1 {
			t.Fatalf("SetDisappearingTimer calls = %d, want 1", len(svc.setDisappearingTimerCalls))
		}
	})

	t.Run("default acknowledges the updated timer", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPut,
			"/instances/"+uuid.NewString()+"/chats/default-disappearing",
			`{"duration":"168h"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.setDefaultDisappearingCalls) != 1 {
			t.Errorf("SetDefaultDisappearingTimer calls = %d, want 1", len(svc.setDefaultDisappearingCalls))
		}
	})
}
func TestSubscribePresence(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/contacts/"+escapeJID("5511999999999@s.whatsapp.net")+"/subscribe", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data contacts.SubscribePresenceResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if !payload.Data.Subscribed {
		t.Errorf("data.subscribed = false, want the single presence signal")
	}
	if len(svc.subscribePresenceCalls) != 1 {
		t.Errorf("SubscribePresence calls = %d, want exactly 1 (no heartbeat)", len(svc.subscribePresenceCalls))
	}
}
func TestContactLink(t *testing.T) {
	t.Run("plain answers the link", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactQRLinkFn: func(_ context.Context, _ uuid.UUID, revoke bool) (string, error) {
				if revoke {
					t.Error("GetContactQRLink revoke = true, want false without the query flag")
				}
				return "https://wa.me/qr/link-1", nil
			},
		}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contact-link", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data contacts.ContactLinkResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Link != "https://wa.me/qr/link-1" {
			t.Errorf("data.link = %q, want the own contact link", payload.Data.Link)
		}
	})

	t.Run("revoke passes through", func(t *testing.T) {
		svc := &fakeInstanceService{
			getContactQRLinkFn: func(context.Context, uuid.UUID, bool) (string, error) {
				return "https://wa.me/qr/link-2", nil
			},
		}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/contact-link?revoke=true", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.getContactQRLinkCalls) != 1 || !svc.getContactQRLinkCalls[0].Revoke {
			t.Errorf("GetContactQRLink calls = %+v, want one revoke", svc.getContactQRLinkCalls)
		}
	})
}
func TestCreateNewsletter(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{
		createNewsletterFn: func(_ context.Context, _ uuid.UUID, title, desc string) (instance.Newsletter, error) {
			if title == "" {
				return instance.Newsletter{}, instance.ErrInvalidInput
			}
			return instance.Newsletter{ChannelJID: "123@newsletter", Title: title, Description: desc}, nil
		},
	}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost, "/instances/"+id.String()+"/newsletters", `{"title":"Comunidade","description":"avisos"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data representation.ChannelEnvelope `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Channel.Channel != "123@newsletter" || payload.Data.Channel.Title != "Comunidade" {
		t.Errorf("data = %+v, want the created channel under data.channel", payload.Data)
	}
}
func TestCreateNewsletterRejectsBlankTitle(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/newsletters", `{"title":"   "}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.createNewsletterCalls) != 0 {
		t.Errorf("CreateNewsletter calls = %d, want none on a blank title", len(svc.createNewsletterCalls))
	}
}
func TestCreateNewsletterRejectsLongDescription(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/newsletters",
		`{"title":"Comunidade","description":"`+strings.Repeat("a", 501)+`"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.createNewsletterCalls) != 0 {
		t.Errorf("CreateNewsletter calls = %d, want none above 500 runes", len(svc.createNewsletterCalls))
	}
}
func TestMuteNewsletter(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/mute",
		`{"muted":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data channels.MuteNewsletterResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if !payload.Data.Muted {
		t.Errorf("data.muted = false, want the applied mute")
	}
	if len(svc.muteNewsletterCalls) != 1 || !svc.muteNewsletterCalls[0].Muted {
		t.Errorf("MuteNewsletter calls = %+v, want one mute", svc.muteNewsletterCalls)
	}
}
func TestMarkNewsletterViewed(t *testing.T) {
	t.Run("batch answers viewed", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/viewed",
			`{"server_ids":["srv-1","srv-2"]}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.markNewsletterViewedCalls) != 1 {
			t.Fatalf("MarkNewsletterViewed calls = %d, want 1", len(svc.markNewsletterViewedCalls))
		}
	})

	t.Run("empty batch answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/viewed",
			`{"server_ids":[]}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.markNewsletterViewedCalls) != 0 {
			t.Errorf("MarkNewsletterViewed calls = %d, want none on an empty batch", len(svc.markNewsletterViewedCalls))
		}
	})
}
func TestReactNewsletter(t *testing.T) {
	t.Run("reaction answers reacted", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/reactions",
			`{"server_id":"srv-1","reaction":"👍"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.reactNewsletterCalls) != 1 || svc.reactNewsletterCalls[0].Reaction != "👍" {
			t.Errorf("ReactNewsletter calls = %+v, want the reaction", svc.reactNewsletterCalls)
		}
	})

	t.Run("missing server id answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/reactions",
			`{"reaction":"👍"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.reactNewsletterCalls) != 0 {
			t.Errorf("ReactNewsletter calls = %d, want none without a server id", len(svc.reactNewsletterCalls))
		}
	})

	t.Run("unknown message answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			reactNewsletterFn: func(context.Context, uuid.UUID, string, string, string) error {
				return instance.ErrNewsletterNotFound
			},
		}
		rec := serveJSON(t, parityWritesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel)+"/reactions",
			`{"server_id":"srv-9","reaction":"👍"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}
