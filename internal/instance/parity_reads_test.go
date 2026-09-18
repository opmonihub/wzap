package instance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

func TestParityReadsJoinedGroupsPagination(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()

	for _, jid := range []string{"120363000000000003@g.us", "120363000000000001@g.us", "120363000000000002@g.us"} {
		sess.PutGroup(session.GroupInfo{JID: jid, Name: "G " + jid})
	}

	first, next, err := svc.GetJoinedGroups(ctx, inst.ID, 2, "")
	if err != nil {
		t.Fatalf("GetJoinedGroups: %v", err)
	}
	if len(first) != 2 || first[0].JID != "120363000000000001@g.us" || first[1].JID != "120363000000000002@g.us" {
		t.Fatalf("first page = %+v, want 001 then 002 sorted", first)
	}
	if next != "120363000000000002@g.us" {
		t.Fatalf("next = %q, want last JID of the page", next)
	}

	second, next, err := svc.GetJoinedGroups(ctx, inst.ID, 2, next)
	if err != nil {
		t.Fatalf("GetJoinedGroups second: %v", err)
	}
	if len(second) != 1 || second[0].JID != "120363000000000003@g.us" {
		t.Errorf("second page = %+v, want 003 only", second)
	}
	if next != "" {
		t.Errorf("next = %q, want empty on the last page", next)
	}

	all, _, err := svc.GetJoinedGroups(ctx, inst.ID, 0, "")
	if err != nil {
		t.Fatalf("GetJoinedGroups default limit: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("default-limit items = %d, want 3", len(all))
	}
}

func TestParityReadsJoinedGroupsMapsNotConnected(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())

	sess.GroupErr = session.ErrNotConnected
	if _, _, err := svc.GetJoinedGroups(context.Background(), inst.ID, 50, ""); !errors.Is(err, ErrNotConnected) {
		t.Errorf("GetJoinedGroups err = %v, want ErrNotConnected", err)
	}
}

func TestParityReadsInvitePreview(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()

	code := sess.PutGroup(session.GroupInfo{JID: "120363000000000007@g.us", Name: "Time"})

	got, err := svc.GetGroupInvitePreview(ctx, inst.ID, code)
	if err != nil {
		t.Fatalf("GetGroupInvitePreview: %v", err)
	}
	if got.JID != "120363000000000007@g.us" || got.Name != "Time" {
		t.Errorf("preview = %+v, want the seeded group", got)
	}

	if _, err := svc.GetGroupInvitePreview(ctx, inst.ID, "   "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank code err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetGroupInvitePreview(ctx, inst.ID, "unknown-code"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown code err = %v, want ErrInvalidInput (422, not 404)", err)
	}
}

func TestParityReadsCheckContacts(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()

	sess.PutContact(session.ContactCheckResult{Phone: "5511999999999", JID: "5511999999999@s.whatsapp.net", IsOnWhatsApp: true})

	results, err := svc.CheckContacts(ctx, inst.ID, []string{"5511999999999", "5511888888888"})
	if err != nil {
		t.Fatalf("CheckContacts: %v", err)
	}
	if len(results) != 2 || results[0].Phone != "5511999999999" || !results[0].IsOnWhatsApp {
		t.Errorf("results = %+v, want seeded hit first", results)
	}

	if _, err := svc.CheckContacts(ctx, inst.ID, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty batch err = %v, want ErrInvalidInput", err)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = "5511000000000"
	}
	if _, err := svc.CheckContacts(ctx, inst.ID, many); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("oversize batch err = %v, want ErrInvalidInput", err)
	}
	if got := sess.DirectoryCalls(); len(got) != 1 {
		t.Errorf("DirectoryCalls = %d, want 1 (validation before session)", len(got))
	}

	sess.DirectoryErr = session.ErrNotConnected
	if _, err := svc.CheckContacts(ctx, inst.ID, []string{"5511999999999"}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("disconnected err = %v, want ErrNotConnected", err)
	}
}

func TestParityReadsContactDetails(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()
	jid := "5511999999999@s.whatsapp.net"

	devices, err := svc.GetContactDevices(ctx, inst.ID, jid)
	if err != nil {
		t.Fatalf("GetContactDevices: %v", err)
	}
	if len(devices) != 1 || devices[0] != jid {
		t.Errorf("devices = %v, want the single fake device", devices)
	}

	if _, err := svc.GetContactPhoto(ctx, inst.ID, jid); err != nil {
		t.Fatalf("GetContactPhoto: %v", err)
	}
	biz, err := svc.GetContactBusiness(ctx, inst.ID, jid)
	if err != nil {
		t.Fatalf("GetContactBusiness: %v", err)
	}
	if biz.Name == "" {
		t.Errorf("business = %+v, want the fake stub name", biz)
	}

	if _, err := svc.GetContactDevices(ctx, inst.ID, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("devices blank jid err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetContactPhoto(ctx, inst.ID, ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("photo blank jid err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetContactBusiness(ctx, inst.ID, ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("business blank jid err = %v, want ErrInvalidInput", err)
	}

	sess.DirectoryErr = session.ErrNotFound
	if _, err := svc.GetContactDevices(ctx, inst.ID, jid); !errors.Is(err, ErrContactNotFound) {
		t.Errorf("devices unknown err = %v, want ErrContactNotFound", err)
	}
	if _, err := svc.GetContactPhoto(ctx, inst.ID, jid); !errors.Is(err, ErrContactNotFound) {
		t.Errorf("photo unknown err = %v, want ErrContactNotFound", err)
	}
	if _, err := svc.GetContactBusiness(ctx, inst.ID, jid); !errors.Is(err, ErrContactNotFound) {
		t.Errorf("business unknown err = %v, want ErrContactNotFound", err)
	}
	sess.DirectoryErr = nil

	sess.DirectoryErr = session.ErrNotConnected
	if _, err := svc.GetContactDevices(ctx, inst.ID, jid); !errors.Is(err, ErrNotConnected) {
		t.Errorf("devices offline err = %v, want ErrNotConnected", err)
	}
}

func TestParityReadsBlocklistStatusDisappearing(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()

	sess.PutBlocklist([]string{"5511888888888@s.whatsapp.net"})
	list, err := svc.GetBlocklist(ctx, inst.ID)
	if err != nil {
		t.Fatalf("GetBlocklist: %v", err)
	}
	if len(list) != 1 || list[0] != "5511888888888@s.whatsapp.net" {
		t.Errorf("blocklist = %v, want the seeded jid", list)
	}

	privacy, err := svc.GetStatusPrivacy(ctx, inst.ID)
	if err != nil {
		t.Fatalf("GetStatusPrivacy: %v", err)
	}
	if privacy.Mode == "" {
		t.Errorf("privacy = %+v, want the fake default mode", privacy)
	}

	if _, _, err := svc.GetDisappearingTimer(ctx, inst.ID, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank chat err = %v, want ErrInvalidInput", err)
	}
	dur, found, err := svc.GetDisappearingTimer(ctx, inst.ID, "5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetDisappearingTimer: %v", err)
	}
	if found {
		t.Errorf("found = true, want false without a timer (dur=%v)", dur)
	}

	sess.BlocklistErr = session.ErrNotConnected
	if _, err := svc.GetBlocklist(ctx, inst.ID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("blocklist offline err = %v, want ErrNotConnected", err)
	}
	sess.BlocklistErr = nil
	sess.ChatSettingsErr = session.ErrNotConnected
	if _, err := svc.GetStatusPrivacy(ctx, inst.ID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("privacy offline err = %v, want ErrNotConnected", err)
	}
	if _, _, err := svc.GetDisappearingTimer(ctx, inst.ID, "a@s.whatsapp.net"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("timer offline err = %v, want ErrNotConnected", err)
	}
}

func TestParityReadsNewsletterMessages(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()
	channel := "12345@newsletter"

	sess.PutNewsletter(session.NewsletterInfo{ChannelJID: channel, Title: "Canal"})
	for _, id := range []string{"1", "2", "3"} {
		sess.PutNewsletterMessage(channel, session.NewsletterMessage{ServerID: id, Content: "m" + id})
	}

	msgs, next, err := svc.GetNewsletterMessages(ctx, inst.ID, channel, "", 2)
	if err != nil {
		t.Fatalf("GetNewsletterMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].ServerID != "1" || msgs[1].ServerID != "2" {
		t.Fatalf("msgs = %+v, want first two", msgs)
	}
	if next == "" {
		t.Error("next cursor empty, want the truncation cursor")
	}

	updates, err := svc.GetNewsletterUpdates(ctx, inst.ID, channel)
	if err != nil {
		t.Fatalf("GetNewsletterUpdates: %v", err)
	}
	if len(updates) != 3 {
		t.Errorf("updates = %d, want 3 seeded messages", len(updates))
	}

	if _, _, err := svc.GetNewsletterMessages(ctx, inst.ID, "  ", "", 10); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank channel messages err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetNewsletterUpdates(ctx, inst.ID, ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank channel updates err = %v, want ErrInvalidInput", err)
	}
	if _, _, err := svc.GetNewsletterMessages(ctx, inst.ID, "99999@newsletter", "", 10); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("unknown channel messages err = %v, want ErrNewsletterNotFound", err)
	}
	if _, err := svc.GetNewsletterUpdates(ctx, inst.ID, "99999@newsletter"); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("unknown channel updates err = %v, want ErrNewsletterNotFound", err)
	}

	sess.NewsletterOpErr = session.ErrNotConnected
	if _, _, err := svc.GetNewsletterMessages(ctx, inst.ID, channel, "", 10); !errors.Is(err, ErrNotConnected) {
		t.Errorf("messages offline err = %v, want ErrNotConnected", err)
	}
	if _, err := svc.GetNewsletterUpdates(ctx, inst.ID, channel); !errors.Is(err, ErrNotConnected) {
		t.Errorf("updates offline err = %v, want ErrNotConnected", err)
	}
}

func TestParityReadsInstanceNotFound(t *testing.T) {
	_, repo := newParityTestInstance(t)
	sessions := sessiontest.New(nil)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, testLogger())
	ctx := context.Background()

	missing := uuid.New()
	if _, err := svc.GetGroupInvitePreview(ctx, missing, "code"); !errors.Is(err, ErrNotFound) {
		t.Errorf("preview missing instance err = %v, want ErrNotFound", err)
	}
	if _, _, err := svc.GetJoinedGroups(ctx, missing, 10, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("joined missing instance err = %v, want ErrNotFound", err)
	}
}
