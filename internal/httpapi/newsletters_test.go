package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/httpapi/channels"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
)

const testChannel = "12345@newsletter"

func TestNewsletterFollow(t *testing.T) {
	t.Run("existing channel answers followed true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/newsletters/follow",
			`{"channel":"`+testChannel+`"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data channels.NewsletterFollowResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Followed {
			t.Errorf("data.followed = false, want true")
		}
		if len(svc.followNewsletterCalls) != 1 || svc.followNewsletterCalls[0].ChannelJID != testChannel {
			t.Errorf("FollowNewsletter calls = %+v, want the channel", svc.followNewsletterCalls)
		}
	})

	t.Run("unknown channel answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			followNewsletterFn: func(context.Context, uuid.UUID, string) error {
				return instance.ErrNewsletterNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/follow",
			`{"channel":"99999@newsletter"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
			t.Errorf("error code = %q, want %q", code, "not_found")
		}
	})

	t.Run("empty channel answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/follow",
			`{"channel":""}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.followNewsletterCalls) != 0 {
			t.Errorf("FollowNewsletter calls = %d, want none on empty channel", len(svc.followNewsletterCalls))
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			followNewsletterFn: func(context.Context, uuid.UUID, string) error {
				return instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/follow",
			`{"channel":"`+testChannel+`"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("unfollow answers followed false", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/newsletters/unfollow",
			`{"channel":"`+testChannel+`"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data channels.NewsletterFollowResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Followed {
			t.Errorf("data.followed = true, want false after unfollow")
		}
		if len(svc.unfollowNewsletterCalls) != 1 {
			t.Errorf("UnfollowNewsletter calls = %d, want 1", len(svc.unfollowNewsletterCalls))
		}
	})
}
func TestNewsletterGet(t *testing.T) {
	t.Run("known channel answers its metadata", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			getNewsletterFn: func(_ context.Context, _ uuid.UUID, channel string) (instance.Newsletter, error) {
				if channel != testChannel {
					t.Errorf("GetNewsletter channel = %q, want %q", channel, testChannel)
				}
				return instance.Newsletter{
					ChannelJID:    testChannel,
					Title:         "Canal da loja",
					Description:   "Ofertas",
					FollowerCount: 41,
					UpdatedAt:     now,
				}, nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID(testChannel), "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data representation.ChannelEnvelope `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Channel.Title != "Canal da loja" || payload.Data.Channel.FollowerCount != 41 {
			t.Errorf("data = %+v, want title and followers under data.channel", payload.Data)
		}
		if !payload.Data.Channel.UpdatedAt.Equal(now) {
			t.Errorf("data.channel.updated_at = %v, want %v", payload.Data.Channel.UpdatedAt, now)
		}
	})

	t.Run("unknown channel answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getNewsletterFn: func(context.Context, uuid.UUID, string) (instance.Newsletter, error) {
				return instance.Newsletter{}, instance.ErrNewsletterNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters/"+escapeJID("99999@newsletter"), "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})
}
func TestNewsletterList(t *testing.T) {
	t.Run("page carries items and the next cursor", func(t *testing.T) {
		svc := &fakeInstanceService{
			listNewslettersFn: func(_ context.Context, _ uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error) {
				if limit != 50 {
					t.Errorf("ListNewsletters limit = %d, want the default 50", limit)
				}
				if cursor != "" {
					t.Errorf("ListNewsletters cursor = %q, want empty", cursor)
				}
				return []instance.Newsletter{
					{ChannelJID: "111@newsletter", Title: "Um"},
					{ChannelJID: "222@newsletter", Title: "Dois"},
				}, "222@newsletter", nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data channels.NewsletterListResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if len(payload.Data.Channels) != 2 {
			t.Fatalf("data.items length = %d, want 2", len(payload.Data.Channels))
		}
		if payload.Data.NextCursor != "222@newsletter" {
			t.Errorf("data.next_cursor = %q, want the cursor", payload.Data.NextCursor)
		}
	})

	t.Run("limit is capped like the other listings", func(t *testing.T) {
		svc := &fakeInstanceService{
			listNewslettersFn: func(_ context.Context, _ uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error) {
				if limit != 100 {
					t.Errorf("ListNewsletters limit = %d, want the cap 100", limit)
				}
				if cursor != "111@newsletter" {
					t.Errorf("ListNewsletters cursor = %q, want 111@newsletter", cursor)
				}
				return nil, "", nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodGet,
			"/instances/"+uuid.NewString()+"/newsletters?limit=1000&cursor="+escapeJID("111@newsletter"), "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data channels.NewsletterListResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.Channels == nil {
			t.Error("data.items = null, want an empty array")
		}
	})
}
