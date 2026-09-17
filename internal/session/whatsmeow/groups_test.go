package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

func TestCreateGroupRejectsInvalidName(t *testing.T) {
	sess := actionSession(t)

	// The upstream limit counts characters, not bytes: a 21-rune multibyte
	// name passes the guard (and fails later on the disconnected client),
	// while blank and 26-character names never reach the session.
	if _, err := sess.CreateGroup(context.Background(), "çãõéíúâêôàäöüßñýÿžšćčđ", nil); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("CreateGroup(multibyte) error = %v, want %v (guard passed, client offline)", err, session.ErrNotConnected)
	}
	for _, name := range []string{"", "   ", string(make([]byte, maxGroupNameLen+1))} {
		if _, err := sess.CreateGroup(context.Background(), name, nil); !errors.Is(err, session.ErrInvalidRecipient) {
			t.Errorf("CreateGroup(%q) error = %v, want %v", name, err, session.ErrInvalidRecipient)
		}
	}
}

func TestCreateGroupRejectsInvalidParticipant(t *testing.T) {
	sess := actionSession(t)

	if _, err := sess.CreateGroup(context.Background(), "Time", []string{""}); !errors.Is(err, session.ErrInvalidRecipient) {
		t.Errorf("CreateGroup error = %v, want %v", err, session.ErrInvalidRecipient)
	}
}

func TestGroupMethodsNeedConnection(t *testing.T) {
	sess := actionSession(t)
	ctx := context.Background()

	if _, err := sess.GetGroup(ctx, "120363000000000000@g.us"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("GetGroup error = %v, want %v", err, session.ErrNotConnected)
	}
	if err := sess.SetGroupName(ctx, "120363000000000000@g.us", "Novo"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("SetGroupName error = %v, want %v", err, session.ErrNotConnected)
	}
	if err := sess.UpdateGroupParticipants(ctx, "120363000000000000@g.us", "add", []string{"5511888888888@s.whatsapp.net"}); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("UpdateGroupParticipants error = %v, want %v", err, session.ErrNotConnected)
	}
	if _, err := sess.GetGroupInvite(ctx, "120363000000000000@g.us"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("GetGroupInvite error = %v, want %v", err, session.ErrNotConnected)
	}
	if _, err := sess.JoinGroup(ctx, "invite-code-1"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("JoinGroup error = %v, want %v", err, session.ErrNotConnected)
	}
	if err := sess.LeaveGroup(ctx, "120363000000000000@g.us"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("LeaveGroup error = %v, want %v", err, session.ErrNotConnected)
	}
	if err := sess.FollowNewsletter(ctx, "12345@newsletter"); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("FollowNewsletter error = %v, want %v", err, session.ErrNotConnected)
	}
	if _, err := sess.ListNewsletters(ctx); !errors.Is(err, session.ErrNotConnected) {
		t.Errorf("ListNewsletters error = %v, want %v", err, session.ErrNotConnected)
	}
}

func TestUpdateGroupParticipantsRejectsUnknownAction(t *testing.T) {
	sess := actionSession(t)

	err := sess.UpdateGroupParticipants(context.Background(), "120363000000000000@g.us", "crown", []string{"5511888888888@s.whatsapp.net"})
	if !errors.Is(err, session.ErrInvalidRecipient) {
		t.Errorf("UpdateGroupParticipants error = %v, want %v", err, session.ErrInvalidRecipient)
	}
}

func TestJoinGroupRejectsEmptyCode(t *testing.T) {
	sess := actionSession(t)

	for _, code := range []string{"", "   ", "https://chat.whatsapp.com/"} {
		if _, err := sess.JoinGroup(context.Background(), code); !errors.Is(err, session.ErrInvalidRecipient) {
			t.Errorf("JoinGroup(%q) error = %v, want %v", code, err, session.ErrInvalidRecipient)
		}
	}
}

func TestInviteCodeFromLink(t *testing.T) {
	for code, want := range map[string]string{
		"invite-code-1":                       "invite-code-1",
		"https://chat.whatsapp.com/invite-2":  "invite-2",
		whatsmeow.InviteLinkPrefix + "abc123": "abc123",
	} {
		if got := inviteCodeFromLink(code); got != want {
			t.Errorf("inviteCodeFromLink(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestClassifyRemoteError(t *testing.T) {
	for err, want := range map[error]error{
		whatsmeow.ErrGroupNotFound:               session.ErrNotFound,
		whatsmeow.ErrIQNotFound:                  session.ErrNotFound,
		whatsmeow.ErrNotInGroup:                  session.ErrForbidden,
		whatsmeow.ErrIQForbidden:                 session.ErrForbidden,
		whatsmeow.ErrIQNotAuthorized:             session.ErrForbidden,
		whatsmeow.ErrGroupInviteLinkUnauthorized: session.ErrForbidden,
		whatsmeow.ErrNotConnected:                session.ErrNotConnected,
		errors.New("boom"):                       session.ErrTransient,
	} {
		if got := classifyRemoteError(err); !errors.Is(got, want) {
			t.Errorf("classifyRemoteError(%v) = %v, want %v", err, got, want)
		}
	}
}

func TestGroupInfoFromTypes(t *testing.T) {
	jid := types.NewJID("120363000000000000", types.GroupServer)
	member := types.NewJID("5511888888888", types.DefaultUserServer)
	info := &types.GroupInfo{
		GroupName:        types.GroupName{Name: "Time do churrasco"},
		GroupTopic:       types.GroupTopic{Topic: "Só coisa séria", TopicID: "topic-1"},
		Participants:     []types.GroupParticipant{{JID: member, PhoneNumber: member, IsAdmin: true}},
		ParticipantCount: 1,
		GroupCreated:     time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
	}
	info.JID = jid

	got := groupInfoFromTypes(info)
	if got.JID != jid.String() {
		t.Errorf("JID = %q, want %q", got.JID, jid.String())
	}
	if got.Name != "Time do churrasco" || got.Description != "Só coisa séria" || got.DescriptionID != "topic-1" {
		t.Errorf("info = %+v, want the subject and topic", got)
	}
	if len(got.Participants) != 1 || got.Participants[0].JID != member.String() || !got.Participants[0].IsAdmin {
		t.Errorf("participants = %+v, want the single admin", got.Participants)
	}
	if got.ParticipantCount != 1 {
		t.Errorf("participant count = %d, want 1", got.ParticipantCount)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created at is zero, want the upstream timestamp")
	}
	if got := groupInfoFromTypes(nil); got.JID != "" {
		t.Errorf("nil info = %+v, want empty", got)
	}
}

func TestNewsletterFromMeta(t *testing.T) {
	jid := types.NewJID("12345", types.NewsletterServer)
	meta := &types.NewsletterMetadata{ID: jid}
	meta.ThreadMeta.Name.Text = "Canal da loja"
	meta.ThreadMeta.Description.Text = "Ofertas"
	meta.ThreadMeta.SubscriberCount = 41

	got := newsletterFromMeta(meta)
	if got.ChannelJID != jid.String() {
		t.Errorf("channel = %q, want %q", got.ChannelJID, jid.String())
	}
	if got.Title != "Canal da loja" || got.Description != "Ofertas" || got.FollowerCount != 41 {
		t.Errorf("info = %+v, want title, description and followers", got)
	}
	if got := newsletterFromMeta(nil); got.ChannelJID != "" {
		t.Errorf("nil meta = %+v, want empty", got)
	}
}
