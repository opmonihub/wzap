package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
)

// escapeJID escapes a JID for embedding in a route path.
func escapeJID(jid string) string {
	return url.PathEscape(jid)
}

func serveGroupPhoto(t *testing.T, srv *http.Server, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
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

func TestGroupCreate(t *testing.T) {
	t.Run("connected answers created with group and invite", func(t *testing.T) {
		id := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			createGroupFn: func(_ context.Context, instanceID uuid.UUID, input instance.CreateGroupInput) (instance.Group, error) {
				if instanceID != id {
					t.Errorf("CreateGroup instance = %s, want %s", instanceID, id)
				}
				if input.Name != "Time do churrasco" {
					t.Errorf("CreateGroup name = %q, want the requested subject", input.Name)
				}
				return instance.Group{JID: testChatGroup, Name: input.Name, UpdatedAt: now}, nil
			},
			getGroupInviteFn: func(context.Context, uuid.UUID, string) (string, error) {
				return "invite-code-1", nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/groups",
			`{"name":"Time do churrasco","participants":["`+testSender+`"]}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
		}
		var payload struct {
			Data groupResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.JID != testChatGroup {
			t.Errorf("data.jid = %q, want %q", payload.Data.JID, testChatGroup)
		}
		if payload.Data.InviteCode != "invite-code-1" {
			t.Errorf("data.invite_code = %q, want the group invite", payload.Data.InviteCode)
		}
		if len(svc.createGroupCalls) != 1 {
			t.Fatalf("CreateGroup calls = %d, want 1", len(svc.createGroupCalls))
		}
	})

	t.Run("empty name answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups",
			`{"name":"","participants":[]}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.createGroupCalls) != 0 {
			t.Errorf("CreateGroup calls = %d, want none on empty name", len(svc.createGroupCalls))
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			createGroupFn: func(context.Context, uuid.UUID, instance.CreateGroupInput) (instance.Group, error) {
				return instance.Group{}, instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups",
			`{"name":"Time do churrasco"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("unknown instance answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
				return nil, instance.ErrNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups",
			`{"name":"Time do churrasco"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}

func TestGroupGet(t *testing.T) {
	t.Run("known group answers the group", func(t *testing.T) {
		id := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			getGroupFn: func(_ context.Context, instanceID uuid.UUID, groupJID string) (instance.Group, error) {
				if groupJID != testChatGroup {
					t.Errorf("GetGroup group = %q, want %q", groupJID, testChatGroup)
				}
				return instance.Group{
					JID:         testChatGroup,
					Name:        "Time do churrasco",
					Description: "Só coisa séria",
					Participants: []instance.GroupParticipant{
						{JID: testSender, IsAdmin: true},
					},
					ParticipantCount: 1,
					UpdatedAt:        now,
				}, nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+id.String()+"/groups/"+escapeJID(testChatGroup), "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Name != "Time do churrasco" {
			t.Errorf("data.name = %q, want the subject", payload.Data.Name)
		}
		if len(payload.Data.Participants) != 1 || !payload.Data.Participants[0].IsAdmin {
			t.Errorf("data.participants = %+v, want the single admin", payload.Data.Participants)
		}
		if !payload.Data.UpdatedAt.Equal(now) {
			t.Errorf("data.updated_at = %v, want %v", payload.Data.UpdatedAt, now)
		}
	})

	t.Run("unknown group answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getGroupFn: func(context.Context, uuid.UUID, string) (instance.Group, error) {
				return instance.Group{}, instance.ErrGroupNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup), "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
			t.Errorf("error code = %q, want %q", code, "not_found")
		}
	})
}

func TestGroupUpdate(t *testing.T) {
	t.Run("subject and description update answers the group", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			updateGroupFn: func(_ context.Context, instanceID uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error) {
				if input.Name == nil || *input.Name != "Novo assunto" {
					t.Errorf("UpdateGroup name = %+v, want Novo assunto", input.Name)
				}
				if input.Description == nil || *input.Description != "Nova descrição" {
					t.Errorf("UpdateGroup description = %+v, want Nova descrição", input.Description)
				}
				return instance.Group{JID: groupJID, Name: *input.Name, Description: *input.Description}, nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPatch,
			"/instances/"+id.String()+"/groups/"+escapeJID(testChatGroup),
			`{"name":"Novo assunto","description":"Nova descrição"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Name != "Novo assunto" || payload.Data.Description != "Nova descrição" {
			t.Errorf("data = %+v, want the updated subject and description", payload.Data)
		}
	})

	t.Run("empty patch answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPatch,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup),
			`{}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateGroupCalls) != 0 {
			t.Errorf("UpdateGroup calls = %d, want none on empty patch", len(svc.updateGroupCalls))
		}
	})
}

func TestGroupParticipants(t *testing.T) {
	for _, action := range []string{"add", "remove", "promote", "demote"} {
		t.Run(action+" applies", func(t *testing.T) {
			id := uuid.New()
			svc := &fakeInstanceService{}
			rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
				"/instances/"+id.String()+"/groups/"+escapeJID(testChatGroup)+"/participants",
				`{"action":"`+action+`","participants":["`+testSender+`"]}`)

			if rec.Code != http.StatusOK {
				t.Fatalf("action %q: status = %d, want %d (body %q)", action, rec.Code, http.StatusOK, rec.Body.String())
			}
			if len(svc.updateParticipantsCalls) != 1 {
				t.Fatalf("action %q: UpdateGroupParticipants calls = %d, want 1", action, len(svc.updateParticipantsCalls))
			}
			call := svc.updateParticipantsCalls[0]
			if call.Action != action || len(call.Participants) != 1 || call.Participants[0] != testSender {
				t.Errorf("action %q: call = %+v, want the single participant", action, call)
			}
		})
	}

	t.Run("unknown action answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup)+"/participants",
			`{"action":"crown","participants":["`+testSender+`"]}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.updateParticipantsCalls) != 0 {
			t.Errorf("UpdateGroupParticipants calls = %d, want none on unknown action", len(svc.updateParticipantsCalls))
		}
	})

	t.Run("without group permission answers forbidden and changes nothing", func(t *testing.T) {
		svc := &fakeInstanceService{
			updateParticipantsFn: func(context.Context, uuid.UUID, string, string, []string) error {
				return instance.ErrForbidden
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup)+"/participants",
			`{"action":"promote","participants":["`+testSender+`"]}`)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "forbidden" {
			t.Errorf("error code = %q, want %q", code, "forbidden")
		}
	})
}

func TestGroupInvite(t *testing.T) {
	t.Run("invite consult answers the code", func(t *testing.T) {
		svc := &fakeInstanceService{
			getGroupInviteFn: func(context.Context, uuid.UUID, string) (string, error) {
				return "invite-code-1", nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup)+"/invite", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupInviteResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.InviteCode != "invite-code-1" {
			t.Errorf("data.invite_code = %q, want the code", payload.Data.InviteCode)
		}
	})

	t.Run("reset answers a new code", func(t *testing.T) {
		svc := &fakeInstanceService{
			resetGroupInviteFn: func(context.Context, uuid.UUID, string) (string, error) {
				return "invite-code-2", nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup)+"/invite/reset", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupInviteResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.InviteCode != "invite-code-2" {
			t.Errorf("data.invite_code = %q, want the reset code", payload.Data.InviteCode)
		}
	})

	t.Run("join with valid code answers the group", func(t *testing.T) {
		svc := &fakeInstanceService{
			joinGroupFn: func(_ context.Context, _ uuid.UUID, code string) (string, error) {
				if code != "invite-code-1" {
					t.Errorf("JoinGroup code = %q, want invite-code-1", code)
				}
				return testChatGroup, nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/join",
			`{"invite_code":"invite-code-1"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupJoinResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.JID != testChatGroup {
			t.Errorf("data.jid = %q, want %q", payload.Data.JID, testChatGroup)
		}
	})

	t.Run("join with invalid code answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/groups/join",
			`{"invite_code":""}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.joinGroupCalls) != 0 {
			t.Errorf("JoinGroup calls = %d, want none on empty code", len(svc.joinGroupCalls))
		}
	})

	t.Run("leave answers left true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/groups/"+escapeJID(testChatGroup)+"/leave", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data groupLeaveResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Left {
			t.Errorf("data.left = false, want true")
		}
		if len(svc.leaveGroupCalls) != 1 || svc.leaveGroupCalls[0].GroupJID != testChatGroup {
			t.Errorf("LeaveGroup calls = %+v, want the group", svc.leaveGroupCalls)
		}
	})
}

func TestGroupPhoto(t *testing.T) {
	t.Run("image bytes update the photo", func(t *testing.T) {
		id := uuid.New()
		image := []byte{0xff, 0xd8, 0xff, 0x00, 0x01}
		svc := &fakeInstanceService{}
		rec := serveGroupPhoto(t, instancesServer(t, svc), http.MethodPut,
			"/instances/"+id.String()+"/groups/"+escapeJID(testChatGroup)+"/photo",
			image, "image/jpeg")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.setGroupPhotoCalls) != 1 {
			t.Fatalf("SetGroupPhoto calls = %d, want 1", len(svc.setGroupPhotoCalls))
		}
		call := svc.setGroupPhotoCalls[0]
		if string(call.Image) != string(image) {
			t.Errorf("SetGroupPhoto image = %d bytes, want %d", len(call.Image), len(image))
		}
	})

	t.Run("non image answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveGroupPhoto(t, instancesServer(t, svc), http.MethodPut,
			"/instances/"+uuid.NewString()+"/groups/"+escapeJID(testChatGroup)+"/photo",
			[]byte(`{"not":"an image"}`), "application/json")

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.setGroupPhotoCalls) != 0 {
			t.Errorf("SetGroupPhoto calls = %d, want none on non image", len(svc.setGroupPhotoCalls))
		}
	})
}
