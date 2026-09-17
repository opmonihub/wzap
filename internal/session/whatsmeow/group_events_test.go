package whatsmeow

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"wzap/internal/session"
)

func groupJID() types.JID {
	return types.NewJID("120363000000000000", types.GroupServer)
}

func TestDispatchJoinedGroupCallsOnGroupEvent(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)
	inviter := types.NewJID("5511888888888", types.DefaultUserServer)

	sess.dispatch(&events.JoinedGroup{
		Reason: "invite",
		Sender: &inviter,
		GroupInfo: types.GroupInfo{
			JID: groupJID(),
		},
	})

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.groupEvents) != 1 {
		t.Fatalf("group events recorded = %d, want 1", len(sink.groupEvents))
	}
	event := sink.groupEvents[0]
	if event.GroupJID != groupJID().String() {
		t.Errorf("group_jid = %q, want %q", event.GroupJID, groupJID())
	}
	if event.Kind != session.GroupEventParticipants {
		t.Errorf("kind = %q, want %q", event.Kind, session.GroupEventParticipants)
	}
	if event.ActorJID != inviter.String() {
		t.Errorf("actor = %q, want the inviter %q", event.ActorJID, inviter.String())
	}
}

func TestDispatchGroupMemberJoinCarriesActorAndAffected(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)
	actor := types.NewJID("5511888888888", types.DefaultUserServer)
	joined := types.NewJID("5511999999999", types.DefaultUserServer)

	sess.dispatch(&events.GroupInfo{
		JID:       groupJID(),
		Sender:    &actor,
		Timestamp: time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
		Join:      []types.JID{joined},
	})

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.groupEvents) != 1 {
		t.Fatalf("group events recorded = %d, want 1", len(sink.groupEvents))
	}
	event := sink.groupEvents[0]
	if event.Kind != session.GroupEventParticipants {
		t.Errorf("kind = %q, want %q", event.Kind, session.GroupEventParticipants)
	}
	if event.ActorJID != actor.String() {
		t.Errorf("actor = %q, want %q", event.ActorJID, actor.String())
	}
	if len(event.Affected) != 1 || event.Affected[0] != joined.String() {
		t.Errorf("affected = %v, want the joined member", event.Affected)
	}
}

func TestDispatchGroupSubjectChangeCallsInfoEvent(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)
	actor := types.NewJID("5511888888888", types.DefaultUserServer)

	sess.dispatch(&events.GroupInfo{
		JID:       groupJID(),
		Sender:    &actor,
		Timestamp: time.Date(2026, 9, 16, 10, 1, 0, 0, time.UTC),
		Name:      &types.GroupName{Name: "Novo assunto"},
	})

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.groupEvents) != 1 {
		t.Fatalf("group events recorded = %d, want 1", len(sink.groupEvents))
	}
	event := sink.groupEvents[0]
	if event.Kind != session.GroupEventInfo {
		t.Errorf("kind = %q, want %q", event.Kind, session.GroupEventInfo)
	}
	if event.Name != "Novo assunto" {
		t.Errorf("name = %q, want the new subject", event.Name)
	}
	if event.ActorJID != actor.String() {
		t.Errorf("actor = %q, want %q", event.ActorJID, actor.String())
	}
}
