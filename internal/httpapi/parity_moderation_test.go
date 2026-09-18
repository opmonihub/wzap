package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
)

// parityModerationServer wires every Fase-2 moderation handler at its future
// route behind the production auth chain so the tests exercise the same
// boundary Task 9 will register in server.go. Registering them together also
// surfaces any Go ServeMux pattern conflict before the wiring lands.
func parityModerationServer(t *testing.T, svc InstanceService) *http.Server {
	t.Helper()
	log := zerolog.Nop()
	mux := http.NewServeMux()
	mux.Handle("GET /instances/{id}/groups/{group_id}/requests", handleGroupRequests(svc, log))
	mux.Handle("POST /instances/{id}/groups/{group_id}/requests", handleUpdateGroupRequests(svc, log))
	mux.Handle("PATCH /instances/{id}/groups/{group_id}/settings", handleUpdateGroupSettings(svc, log))
	return &http.Server{Handler: RequestID(Authenticate(testToken, nil, "")(mux))}
}

func TestGroupRequests(t *testing.T) {
	id := uuid.New()
	group := escapeJID("12036300000001@g.us")
	svc := &fakeInstanceService{
		getGroupRequestsFn: func(context.Context, uuid.UUID, string) ([]instance.GroupParticipant, error) {
			return []instance.GroupParticipant{{JID: "5511999999999@s.whatsapp.net"}}, nil
		},
	}
	rec := serveJSON(t, parityModerationServer(t, svc), http.MethodGet, "/instances/"+id.String()+"/groups/"+group+"/requests", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data groupRequestsResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Participants) != 1 || payload.Data.Participants[0].JID != "5511999999999@s.whatsapp.net" {
		t.Errorf("data.participants = %+v, want the single pending request", payload.Data.Participants)
	}
}

func TestGroupRequestsUnknownGroup(t *testing.T) {
	svc := &fakeInstanceService{
		getGroupRequestsFn: func(context.Context, uuid.UUID, string) ([]instance.GroupParticipant, error) {
			return nil, instance.ErrGroupNotFound
		},
	}
	rec := serveJSON(t, parityModerationServer(t, svc), http.MethodGet,
		"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036309999999@g.us")+"/requests", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestUpdateGroupRequests(t *testing.T) {
	t.Run("approve answers updated", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/groups/"+escapeJID("12036300000001@g.us")+"/requests",
			`{"action":"approve","participants":["5511999999999@s.whatsapp.net"]}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.updateGroupRequestsCalls) != 1 || svc.updateGroupRequestsCalls[0].Action != "approve" {
			t.Errorf("UpdateGroupRequests calls = %+v, want one approve", svc.updateGroupRequestsCalls)
		}
	})

	t.Run("bad action answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036300000001@g.us")+"/requests",
			`{"action":"freeze","participants":["5511999999999@s.whatsapp.net"]}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateGroupRequestsCalls) != 0 {
			t.Errorf("UpdateGroupRequests calls = %d, want none on a bad action", len(svc.updateGroupRequestsCalls))
		}
	})

	t.Run("empty batch answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036300000001@g.us")+"/requests",
			`{"action":"decline","participants":[]}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateGroupRequestsCalls) != 0 {
			t.Errorf("UpdateGroupRequests calls = %d, want none on an empty batch", len(svc.updateGroupRequestsCalls))
		}
	})

	t.Run("forbidden answers forbidden", func(t *testing.T) {
		svc := &fakeInstanceService{
			updateGroupRequestsFn: func(context.Context, uuid.UUID, string, string, []string) error {
				return instance.ErrForbidden
			},
		}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036300000001@g.us")+"/requests",
			`{"action":"approve","participants":["5511999999999@s.whatsapp.net"]}`)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})
}

func TestUpdateGroupSettings(t *testing.T) {
	t.Run("announce answers the group", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			updateGroupSettingsFn: func(_ context.Context, instanceID uuid.UUID, groupJID string, announce, _ *bool, _, _ *string) (instance.Group, error) {
				if instanceID != id {
					t.Errorf("UpdateGroupSettings instance = %s, want %s", instanceID, id)
				}
				if announce == nil || !*announce {
					t.Errorf("UpdateGroupSettings announce = %v, want true", announce)
				}
				return instance.Group{JID: groupJID, Name: "Time do churrasco"}, nil
			},
		}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPatch,
			"/instances/"+id.String()+"/groups/"+escapeJID("12036300000001@g.us")+"/settings",
			`{"announce":true}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.JID != "12036300000001@g.us" {
			t.Errorf("data.jid = %q, want the updated group", payload.Data.JID)
		}
	})

	t.Run("no fields answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036300000001@g.us")+"/settings",
			`{}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateGroupSettingsCalls) != 0 {
			t.Errorf("UpdateGroupSettings calls = %d, want none without fields", len(svc.updateGroupSettingsCalls))
		}
	})

	t.Run("mode outside the allowlist answers unprocessable before the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, parityModerationServer(t, svc), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID("12036300000001@g.us")+"/settings",
			`{"member_add_mode":"moderators"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateGroupSettingsCalls) != 0 {
			t.Errorf("UpdateGroupSettings calls = %d, want none outside the allowlist", len(svc.updateGroupSettingsCalls))
		}
	})
}
