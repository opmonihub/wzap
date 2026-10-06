package whatsmeow

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

func TestGetProfile(t *testing.T) {
	t.Run("uses user jid without device suffix", func(t *testing.T) {
		jid := types.NewJID("5511999999999", types.DefaultUserServer)
		jid.Device = 3
		device := &store.Device{ID: &jid, PushName: "Loja"}

		sess, err := newSession(uuid.New(), device, zerolog.Nop(), nil, testMediaLimit)
		if err != nil {
			t.Fatalf("newSession: %v", err)
		}
		sess.isConnectedFn = func() bool { return true }

		var queried types.JID
		sess.getUserInfoFn = func(_ context.Context, jids []types.JID) (map[types.JID]types.UserInfo, error) {
			if len(jids) != 1 {
				t.Fatalf("GetUserInfo jids = %v, want one jid", jids)
			}
			queried = jids[0]
			return map[types.JID]types.UserInfo{
				jids[0]: {Status: "aberto"},
			}, nil
		}
		sess.getProfilePictureInfoFn = func(_ context.Context, jid types.JID, _ *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
			if jid.Device != 0 {
				t.Fatalf("GetProfilePictureInfo jid = %v, want user jid without device", jid)
			}
			return &types.ProfilePictureInfo{URL: "https://example.com/me.jpg"}, nil
		}

		profile, err := sess.GetProfile(context.Background())
		if err != nil {
			t.Fatalf("GetProfile: %v", err)
		}
		if queried.Device != 0 {
			t.Fatalf("GetUserInfo jid = %v, want user jid without device", queried)
		}
		if profile.Name != "Loja" || profile.StatusText != "aberto" || profile.PhotoURL != "https://example.com/me.jpg" {
			t.Fatalf("profile = %+v, want push name, recado and photo", profile)
		}
	})

	t.Run("upstream failures still return push name", func(t *testing.T) {
		jid := types.NewJID("5511999999999", types.DefaultUserServer)
		jid.Device = 1
		device := &store.Device{ID: &jid, PushName: "Loja"}

		sess, err := newSession(uuid.New(), device, zerolog.Nop(), nil, testMediaLimit)
		if err != nil {
			t.Fatalf("newSession: %v", err)
		}
		sess.isConnectedFn = func() bool { return true }
		sess.getUserInfoFn = func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
			return nil, errors.New("bad-request")
		}
		sess.getProfilePictureInfoFn = func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
			return nil, errors.New("item-not-found")
		}

		profile, err := sess.GetProfile(context.Background())
		if err != nil {
			t.Fatalf("GetProfile: %v", err)
		}
		if profile.Name != "Loja" || profile.StatusText != "" || profile.PhotoURL != "" {
			t.Fatalf("profile = %+v, want push name only", profile)
		}
	})

	t.Run("disconnected stays not connected", func(t *testing.T) {
		sess := actionSession(t)
		if _, err := sess.GetProfile(context.Background()); !errors.Is(err, session.ErrNotConnected) {
			t.Fatalf("GetProfile error = %v, want %v", err, session.ErrNotConnected)
		}
	})
}
