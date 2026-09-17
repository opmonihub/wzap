package message

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session/sessiontest"
)

// TestRichSendersForwardPerType pins that every rich type accepted by Enqueue
// has a sender reaching the session: poll, reaction (emoji and empty removal),
// list and buttons.
func TestRichSendersForwardPerType(t *testing.T) {
	senders := RichSenders(nil)
	for _, msgType := range []string{TypePoll, TypeReaction, TypeList, TypeButtons} {
		sender, ok := senders[msgType]
		if !ok {
			t.Fatalf("senders[%q] missing, want a sender for every rich type", msgType)
		}
		sess := sessiontest.NewSession(uuid.New(), nil)
		var payload string
		switch msgType {
		case TypePoll:
			payload = `{"question":"Q?","options":["a","b"],"selectable_count":1}`
		case TypeReaction:
			payload = `{"target":"wamid.1","emoji":"👍"}`
		case TypeList:
			payload = `{"button_text":"Ver","sections":[{"title":"S","rows":[{"id":"r1","title":"R1"}]}]}`
		case TypeButtons:
			payload = `{"text":"Escolha","buttons":[{"id":"a","title":"A"}]}`
		}
		if _, err := sender.Send(context.Background(), sess, model.OutboundMessage{
			Type:         msgType,
			RecipientJID: "5547988359190@s.whatsapp.net",
			Payload:      []byte(payload),
		}); err != nil {
			t.Errorf("Send %s: %v", msgType, err)
		}
		if len(sess.SendCalls()) != 1 {
			t.Errorf("Send %s session sends = %d, want 1", msgType, len(sess.SendCalls()))
		}
	}
}

// TestReactionSenderRemovalPinsEmptyEmoji pins that an empty emoji (removal)
// still reaches the session instead of being rejected as invalid.
func TestReactionSenderRemovalPinsEmptyEmoji(t *testing.T) {
	senders := RichSenders(nil)
	sender, ok := senders[TypeReaction]
	if !ok {
		t.Fatalf("senders[%q] missing", TypeReaction)
	}
	sess := sessiontest.NewSession(uuid.New(), nil)
	if _, err := sender.Send(context.Background(), sess, model.OutboundMessage{
		Type:         TypeReaction,
		RecipientJID: "5547988359190@s.whatsapp.net",
		Payload:      []byte(`{"target":"wamid.1","emoji":""}`),
	}); err != nil {
		t.Fatalf("Send reaction removal: %v", err)
	}
}

// TestRichSendersRejectInvalidPayloads pins that malformed stored payloads
// never reach the session.
func TestRichSendersRejectInvalidPayloads(t *testing.T) {
	senders := RichSenders(nil)
	tests := []struct {
		name    string
		msgType string
		payload string
	}{
		{name: "poll malformed", msgType: TypePoll, payload: `{`},
		{name: "poll no options", msgType: TypePoll, payload: `{"question":"Q?","options":["a"]}`},
		{name: "reaction without target", msgType: TypeReaction, payload: `{"emoji":"👍"}`},
		{name: "list malformed", msgType: TypeList, payload: `{`},
		{name: "buttons malformed", msgType: TypeButtons, payload: `{`},
		{name: "buttons without buttons", msgType: TypeButtons, payload: `{"text":"Escolha"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, ok := senders[tt.msgType]
			if !ok {
				t.Fatalf("senders[%q] missing", tt.msgType)
			}
			sess := sessiontest.NewSession(uuid.New(), nil)
			if _, err := sender.Send(context.Background(), sess, model.OutboundMessage{
				Type:         tt.msgType,
				RecipientJID: "5547988359190@s.whatsapp.net",
				Payload:      []byte(tt.payload),
			}); err == nil {
				t.Error("Send error = nil, want the invalid payload rejected")
			}
			if len(sess.SendCalls()) != 0 {
				t.Errorf("session sends = %d, want none", len(sess.SendCalls()))
			}
		})
	}
}
