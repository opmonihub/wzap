package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// errVoteSecrets stands in for a vote whose message secrets are unavailable.
var errVoteSecrets = errors.New("no message secrets for the vote")

// richInfo builds the message source of a rich inbound event.
func richInfo(id string) types.MessageInfo {
	return types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:   types.NewJID("5511999999999", types.DefaultUserServer),
			Sender: types.NewJID("5511888888888", types.DefaultUserServer),
		},
		ID:        id,
		Timestamp: time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
	}
}

func TestDispatchPollVoteCallsOnPollVote(t *testing.T) {
	chat := types.NewJID("5511999999999", types.DefaultUserServer)
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)
	sess.decryptVoteFn = func(_ context.Context, _ *events.Message) ([][]byte, error) {
		return [][]byte{{0xab, 0x12}}, nil
	}

	sess.dispatch(&events.Message{
		Info: richInfo("STANZA-VOTE"),
		Message: &waE2E.Message{
			PollUpdateMessage: &waE2E.PollUpdateMessage{
				PollCreationMessageKey: &waCommon.MessageKey{
					RemoteJID: proto.String(chat.String()),
					ID:        proto.String("POLL-1"),
				},
			},
		},
	})

	if got := sink.pollVoteCount(); got != 1 {
		t.Fatalf("poll votes recorded = %d, want 1", got)
	}
	vote := sink.lastPollVote(t)
	if vote.PollMessageID != "POLL-1" {
		t.Errorf("vote poll_message_id = %q, want the poll key", vote.PollMessageID)
	}
	if vote.ChatJID != chat.String() {
		t.Errorf("vote chat_jid = %q, want %q", vote.ChatJID, chat.String())
	}
	if len(vote.SelectedOptionIDs) != 1 || vote.SelectedOptionIDs[0] != "ab12" {
		t.Errorf("vote option ids = %v, want the hex of the selected hash", vote.SelectedOptionIDs)
	}
	if sink.messageCount() != 0 {
		t.Errorf("plain messages = %d, want none: votes fork off the message path", sink.messageCount())
	}
}

func TestDispatchPollVoteUndecryptableStillEmits(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)
	sess.decryptVoteFn = func(_ context.Context, _ *events.Message) ([][]byte, error) {
		return nil, errVoteSecrets
	}

	sess.dispatch(&events.Message{
		Info: richInfo("STANZA-VOTE"),
		Message: &waE2E.Message{
			PollUpdateMessage: &waE2E.PollUpdateMessage{
				PollCreationMessageKey: &waCommon.MessageKey{
					ID: proto.String("POLL-1"),
				},
			},
		},
	})

	if got := sink.pollVoteCount(); got != 1 {
		t.Fatalf("poll votes recorded = %d, want the undecryptable vote too", got)
	}
	if vote := sink.lastPollVote(t); vote.PollMessageID != "POLL-1" {
		t.Errorf("vote poll_message_id = %q, want the poll key", vote.PollMessageID)
	}
}

func TestDispatchReactionCallsOnReaction(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)

	for _, emoji := range []string{"👍", ""} {
		sess.dispatch(&events.Message{
			Info: richInfo("STANZA-REACT"),
			Message: &waE2E.Message{
				ReactionMessage: &waE2E.ReactionMessage{
					Key: &waCommon.MessageKey{
						RemoteJID: proto.String(types.NewJID("5511999999999", types.DefaultUserServer).String()),
						ID:        proto.String("ORIGINAL-1"),
					},
					Text: proto.String(emoji),
				},
			},
		})
	}

	if got := sink.reactionCount(); got != 2 {
		t.Fatalf("reactions recorded = %d, want reaction plus removal", got)
	}
	first := sink.allReactions(t)[0]
	if first.MessageID != "ORIGINAL-1" || first.Emoji != "👍" {
		t.Errorf("reaction = %+v, want the target and the emoji", first)
	}
	removal := sink.allReactions(t)[1]
	if removal.MessageID != "ORIGINAL-1" || removal.Emoji != "" {
		t.Errorf("removal = %+v, want the target with an empty emoji", removal)
	}
	if sink.messageCount() != 0 {
		t.Errorf("plain messages = %d, want none: reactions fork off the message path", sink.messageCount())
	}
}

func TestDispatchButtonsResponseCallsOnInteractiveResponse(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)

	sess.dispatch(&events.Message{
		Info: richInfo("STANZA-BTN"),
		Message: &waE2E.Message{
			ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{
				Response: &waE2E.ButtonsResponseMessage_SelectedDisplayText{
					SelectedDisplayText: "Copiar chave PIX",
				},
				SelectedButtonID: proto.String("pix-copia"),
			},
		},
	})

	if got := sink.interactiveCount(); got != 1 {
		t.Fatalf("interactive responses recorded = %d, want 1", got)
	}
	resp := sink.lastInteractive(t)
	if resp.Source != "buttons" {
		t.Errorf("response source = %q, want %q", resp.Source, "buttons")
	}
	if resp.SelectedID != "pix-copia" || resp.Title != "Copiar chave PIX" {
		t.Errorf("response = %+v, want the selected id and title", resp)
	}
	if sink.messageCount() != 0 {
		t.Errorf("plain messages = %d, want none", sink.messageCount())
	}
}

func TestDispatchListResponseCallsOnInteractiveResponse(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)

	sess.dispatch(&events.Message{
		Info: richInfo("STANZA-LIST"),
		Message: &waE2E.Message{
			ListResponseMessage: &waE2E.ListResponseMessage{
				Title: proto.String("X-Burger"),
				SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{
					SelectedRowID: proto.String("x-burger"),
				},
			},
		},
	})

	if got := sink.interactiveCount(); got != 1 {
		t.Fatalf("interactive responses recorded = %d, want 1", got)
	}
	resp := sink.lastInteractive(t)
	if resp.Source != "list" {
		t.Errorf("response source = %q, want %q", resp.Source, "list")
	}
	if resp.SelectedID != "x-burger" || resp.Title != "X-Burger" {
		t.Errorf("response = %+v, want the selected row and title", resp)
	}
}

func TestDispatchNativeFlowResponseCallsOnInteractiveResponse(t *testing.T) {
	sink := &recordingSink{}
	sess := dispatchSession(t, sink)

	sess.dispatch(&events.Message{
		Info: richInfo("STANZA-FLOW"),
		Message: &waE2E.Message{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
				InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
					NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
						Name:       proto.String("payment"),
						ParamsJSON: proto.String(`{"pix":"loja@example.com"}`),
					},
				},
				Body: &waE2E.InteractiveResponseMessage_Body{
					Text: proto.String("Pagar com PIX"),
				},
			},
		},
	})

	if got := sink.interactiveCount(); got != 1 {
		t.Fatalf("interactive responses recorded = %d, want 1", got)
	}
	resp := sink.lastInteractive(t)
	if resp.Source != "native_flow" {
		t.Errorf("response source = %q, want %q", resp.Source, "native_flow")
	}
	if resp.SelectedID != "payment" || resp.Title != "Pagar com PIX" {
		t.Errorf("response = %+v, want the flow name and body", resp)
	}
}
