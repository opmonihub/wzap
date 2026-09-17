package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"

	"wzap/internal/session"
)

// RejectCall rejects the active call callID from fromJID through the client.
// The service never initiates calls: rejection is the only call control.
func (s *instanceSession) RejectCall(ctx context.Context, fromJID, callID string) error {
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: reject call", session.ErrNotConnected)
	}
	from, err := types.ParseJID(fromJID)
	if err != nil || from.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, fromJID)
	}
	if callID == "" {
		return errors.New("reject call: empty call id")
	}
	if err := s.client.RejectCall(ctx, from, callID); err != nil {
		return classifySessionError(err)
	}
	return nil
}

// callOfferEvent translates an incoming 1:1 call offer.
func callOfferEvent(instanceID uuid.UUID, evt *waEvents.CallOffer) session.CallEvent {
	return baseCallEvent(instanceID, evt.BasicCallMeta, session.CallStateOffer, false, evt)
}

// callOfferNoticeEvent translates a group call notice: the creator stands in
// for the caller and the media names audio or video.
func callOfferNoticeEvent(instanceID uuid.UUID, evt *waEvents.CallOfferNotice) session.CallEvent {
	from := evt.From
	if from.IsEmpty() {
		from = evt.CallCreator
	}
	event := baseCallEvent(instanceID, evt.BasicCallMeta, session.CallStateOffer, evt.Media == "video", evt)
	event.FromJID = from.String()
	return event
}

// callAcceptEvent translates a call accepted on another device.
func callAcceptEvent(instanceID uuid.UUID, evt *waEvents.CallAccept) session.CallEvent {
	return baseCallEvent(instanceID, evt.BasicCallMeta, session.CallStateAccept, false, evt)
}

// callRejectEvent translates a call rejected on another device.
func callRejectEvent(instanceID uuid.UUID, evt *waEvents.CallReject) session.CallEvent {
	return baseCallEvent(instanceID, evt.BasicCallMeta, session.CallStateReject, false, evt)
}

// callTerminateEvent translates a terminated call.
func callTerminateEvent(instanceID uuid.UUID, evt *waEvents.CallTerminate) session.CallEvent {
	return baseCallEvent(instanceID, evt.BasicCallMeta, session.CallStateEnd, false, evt)
}

// baseCallEvent fills the shared call fields: the caller, the upstream call
// id and the observed moment (now when the wire carries none).
func baseCallEvent(instanceID uuid.UUID, meta types.BasicCallMeta, state string, isVideo bool, evt any) session.CallEvent {
	at := meta.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return session.CallEvent{
		InstanceID: instanceID,
		CallID:     meta.CallID,
		FromJID:    meta.From.String(),
		State:      state,
		IsVideo:    isVideo,
		Timestamp:  at,
		Raw:        captureRaw(evt),
	}
}
