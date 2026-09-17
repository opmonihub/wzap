package whatsmeow

import (
	"testing"

	"wzap/internal/session"
)

// TestBuildMessageRichTypes pins the outbound construction of every rich
// type: poll, reaction (emoji and empty removal), list and buttons.
func TestBuildMessageRichTypes(t *testing.T) {
	chat := "5547988359190@s.whatsapp.net"
	tests := []struct {
		name string
		msg  session.OutboundMessage
	}{
		{
			name: "poll",
			msg: session.OutboundMessage{
				Type:         "poll",
				RecipientJID: chat,
				Payload:      []byte(`{"question":"Q?","options":["a","b"],"selectable_count":1}`),
			},
		},
		{
			name: "reaction",
			msg: session.OutboundMessage{
				Type:         "reaction",
				RecipientJID: chat,
				Payload:      []byte(`{"target":"WA-1","emoji":"👍"}`),
			},
		},
		{
			name: "reaction removal",
			msg: session.OutboundMessage{
				Type:         "reaction",
				RecipientJID: chat,
				Payload:      []byte(`{"target":"WA-1","emoji":""}`),
			},
		},
		{
			name: "list",
			msg: session.OutboundMessage{
				Type:         "list",
				RecipientJID: chat,
				Payload:      []byte(`{"button_text":"Ver","sections":[{"title":"S","rows":[{"id":"r1","title":"R1"}]}]}`),
			},
		},
		{
			name: "buttons",
			msg: session.OutboundMessage{
				Type:         "buttons",
				RecipientJID: chat,
				Payload:      []byte(`{"text":"Escolha","buttons":[{"id":"a","title":"A"}]}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built, err := buildMessage(tt.msg)
			if err != nil {
				t.Fatalf("buildMessage %s: %v", tt.name, err)
			}
			if built == nil {
				t.Fatalf("buildMessage %s returned nil", tt.name)
			}
		})
	}
}

// TestBuildMessageRichShapes pins the wire shapes: the poll carries the
// question and options, the reaction keys the target as from-me (removal is
// the same shape with an empty text), the list carries sections and the
// buttons carry quick replies.
func TestBuildMessageRichShapes(t *testing.T) {
	chat := "5547988359190@s.whatsapp.net"

	poll, err := buildMessage(session.OutboundMessage{
		Type: "poll", RecipientJID: chat,
		Payload: []byte(`{"question":"Melhor dia?","options":["seg","ter"],"selectable_count":1}`),
	})
	if err != nil {
		t.Fatalf("buildMessage poll: %v", err)
	}
	creation := poll.GetPollCreationMessage()
	if creation.GetName() != "Melhor dia?" {
		t.Errorf("poll name = %q, want the question", creation.GetName())
	}
	if len(creation.GetOptions()) != 2 || creation.GetOptions()[0].GetOptionName() != "seg" {
		t.Errorf("poll options = %v, want the two options in order", creation.GetOptions())
	}
	if creation.GetSelectableOptionsCount() != 1 {
		t.Errorf("poll selectable count = %d, want 1", creation.GetSelectableOptionsCount())
	}
	if len(poll.GetMessageContextInfo().GetMessageSecret()) != 32 {
		t.Errorf("poll secret length = %d, want 32 random bytes", len(poll.GetMessageContextInfo().GetMessageSecret()))
	}

	react, err := buildMessage(session.OutboundMessage{
		Type: "reaction", RecipientJID: chat,
		Payload: []byte(`{"target":"WA-9","emoji":"❤️"}`),
	})
	if err != nil {
		t.Fatalf("buildMessage reaction: %v", err)
	}
	if react.GetReactionMessage().GetKey().GetID() != "WA-9" {
		t.Errorf("reaction target = %q, want WA-9", react.GetReactionMessage().GetKey().GetID())
	}
	if react.GetReactionMessage().GetText() != "❤️" {
		t.Errorf("reaction text = %q, want the emoji", react.GetReactionMessage().GetText())
	}
	removal, err := buildMessage(session.OutboundMessage{
		Type: "reaction", RecipientJID: chat,
		Payload: []byte(`{"target":"WA-9","emoji":""}`),
	})
	if err != nil {
		t.Fatalf("buildMessage reaction removal: %v", err)
	}
	if removal.GetReactionMessage() == nil || removal.GetReactionMessage().GetText() != "" {
		t.Errorf("removal = %v, want the same endpoint with an empty text", removal)
	}

	list, err := buildMessage(session.OutboundMessage{
		Type: "list", RecipientJID: chat,
		Payload: []byte(`{"title":"T","description":"D","button_text":"Ver","sections":[{"title":"S","rows":[{"id":"r1","title":"R1","description":"desc"}]}],"footer":"F"}`),
	})
	if err != nil {
		t.Fatalf("buildMessage list: %v", err)
	}
	got := list.GetListMessage()
	if got.GetButtonText() != "Ver" || got.GetTitle() != "T" || got.GetFooterText() != "F" {
		t.Errorf("list envelope = %+v, want title, button and footer", got)
	}
	rows := got.GetSections()[0].GetRows()
	if len(rows) != 1 || rows[0].GetRowID() != "r1" || rows[0].GetTitle() != "R1" || rows[0].GetDescription() != "desc" {
		t.Errorf("list rows = %v, want the single row with id, title and description", rows)
	}

	buttons, err := buildMessage(session.OutboundMessage{
		Type: "buttons", RecipientJID: chat,
		Payload: []byte(`{"text":"Pague com PIX (chave loja@example.com)","footer":"F","buttons":[{"id":"pix","title":"Copiar chave PIX"}]}`),
	})
	if err != nil {
		t.Fatalf("buildMessage buttons: %v", err)
	}
	btns := buttons.GetButtonsMessage().GetButtons()
	if len(btns) != 1 || btns[0].GetButtonID() != "pix" || btns[0].GetButtonText().GetDisplayText() != "Copiar chave PIX" {
		t.Errorf("buttons = %v, want the PIX pass-through button", btns)
	}
	if buttons.GetButtonsMessage().GetContentText() == "" {
		t.Errorf("buttons content text is empty, want the body")
	}
}

// TestBuildMessageRichRejectsMalformed pins that malformed rich payloads fail
// before any network round-trip.
func TestBuildMessageRichRejectsMalformed(t *testing.T) {
	chat := "5547988359190@s.whatsapp.net"
	tests := []struct {
		name    string
		msgType string
		payload string
	}{
		{name: "poll without options", msgType: "poll", payload: `{"question":"Q?","options":["a"]}`},
		{name: "reaction without target", msgType: "reaction", payload: `{"emoji":"👍"}`},
		{name: "list without sections", msgType: "list", payload: `{"button_text":"Ver"}`},
		{name: "buttons without text", msgType: "buttons", payload: `{"buttons":[{"id":"a","title":"A"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := buildMessage(session.OutboundMessage{Type: tt.msgType, RecipientJID: chat, Payload: []byte(tt.payload)}); err == nil {
				t.Errorf("buildMessage %s error = nil, want rejection", tt.name)
			}
		})
	}
}
