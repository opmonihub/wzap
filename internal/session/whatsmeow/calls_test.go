package whatsmeow

import (
	"testing"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"

	"wzap/internal/session"
)

// TestCallOfferTranslation pins the 1:1 offer translation: the caller, the
// upstream call id and the offer state.
func TestCallOfferTranslation(t *testing.T) {
	id := uuid.New()
	from := types.NewJID("5511888888888", "s.whatsapp.net")

	event := callOfferEvent(id, &waEvents.CallOffer{
		BasicCallMeta: types.BasicCallMeta{From: from, CallID: "call-1"},
	})

	if event.InstanceID != id {
		t.Errorf("instance = %s, want %s", event.InstanceID, id)
	}
	if event.CallID != "call-1" {
		t.Errorf("call id = %q, want call-1", event.CallID)
	}
	if event.FromJID != from.String() {
		t.Errorf("from = %q, want %q", event.FromJID, from.String())
	}
	if event.State != session.CallStateOffer {
		t.Errorf("state = %q, want offer", event.State)
	}
	if event.IsVideo {
		t.Error("is_video = true, want false when the wire names no media")
	}
	if event.Timestamp.IsZero() {
		t.Error("timestamp is zero, want the observed moment")
	}
}

// TestCallStateTranslations pins the remaining lifecycle states: accept,
// reject and end travel unified as call.offer with their state field.
func TestCallStateTranslations(t *testing.T) {
	id := uuid.New()
	from := types.NewJID("5511888888888", "s.whatsapp.net")
	meta := types.BasicCallMeta{From: from, CallID: "call-9"}

	if got := callAcceptEvent(id, &waEvents.CallAccept{BasicCallMeta: meta}); got.State != session.CallStateAccept || got.CallID != "call-9" {
		t.Errorf("accept = %+v, want state accept on call-9", got)
	}
	if got := callRejectEvent(id, &waEvents.CallReject{BasicCallMeta: meta}); got.State != session.CallStateReject || got.FromJID != from.String() {
		t.Errorf("reject = %+v, want state reject from the caller", got)
	}
	if got := callTerminateEvent(id, &waEvents.CallTerminate{BasicCallMeta: meta}); got.State != session.CallStateEnd {
		t.Errorf("terminate state = %q, want end", got.State)
	}
}

// TestCallOfferNoticeTranslation pins the group notice translation: the
// creator stands in for the caller and audio/video follows the media.
func TestCallOfferNoticeTranslation(t *testing.T) {
	id := uuid.New()
	creator := types.NewJID("5511888888888", "s.whatsapp.net")

	video := callOfferNoticeEvent(id, &waEvents.CallOfferNotice{
		BasicCallMeta: types.BasicCallMeta{CallCreator: creator, CallID: "call-g"},
		Media:         "video",
	})
	if video.State != session.CallStateOffer {
		t.Errorf("notice state = %q, want offer", video.State)
	}
	if video.FromJID != creator.String() {
		t.Errorf("notice from = %q, want the creator", video.FromJID)
	}
	if !video.IsVideo {
		t.Error("notice is_video = false, want true for video media")
	}

	audio := callOfferNoticeEvent(id, &waEvents.CallOfferNotice{
		BasicCallMeta: types.BasicCallMeta{CallCreator: creator, CallID: "call-g"},
		Media:         "audio",
	})
	if audio.IsVideo {
		t.Error("notice is_video = true, want false for audio media")
	}
}
