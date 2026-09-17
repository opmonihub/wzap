package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"wzap/internal/model"
	"wzap/internal/session"
)

// The rich senders satisfy the contract; the assertions catch signature drift
// at build time.
var (
	_ Sender = pollSender{}
	_ Sender = reactionSender{}
	_ Sender = stickerSender{}
	_ Sender = listSender{}
	_ Sender = buttonsSender{}
)

// RichSenders maps every rich message type accepted by Enqueue to its sender.
// A nil media resolver leaves sticker messages unsupported.
func RichSenders(media MediaPathResolver) map[string]Sender {
	return map[string]Sender{
		TypePoll:     pollSender{},
		TypeReaction: reactionSender{},
		TypeSticker:  stickerSender{media: media},
		TypeList:     listSender{},
		TypeButtons:  buttonsSender{},
	}
}

// pollSender validates the stored poll body before handing it to the session.
type pollSender struct{}

// Send rejects a poll without a question or with fewer than MinPollOptions
// options and forwards the message.
func (pollSender) Send(ctx context.Context, sess session.Session, msg model.OutboundMessage) (string, error) {
	var payload pollPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", fmt.Errorf("poll payload: %w", err)
	}
	if strings.TrimSpace(payload.Question) == "" {
		return "", errors.New("poll payload: question is required")
	}
	if len(payload.Options) < MinPollOptions {
		return "", fmt.Errorf("poll payload: at least %d options are required", MinPollOptions)
	}
	return sess.Send(ctx, session.OutboundMessage{
		Type:         msg.Type,
		RecipientJID: msg.RecipientJID,
		Payload:      msg.Payload,
	})
}

// reactionSender validates the stored reaction body before handing it to the
// session. An empty emoji is a removal, not an error.
type reactionSender struct{}

// Send rejects a reaction without a target and forwards the message; the
// emoji travels verbatim, including empty for removals.
func (reactionSender) Send(ctx context.Context, sess session.Session, msg model.OutboundMessage) (string, error) {
	var payload reactionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", fmt.Errorf("reaction payload: %w", err)
	}
	if strings.TrimSpace(payload.Target) == "" {
		return "", errors.New("reaction payload: target is required")
	}
	return sess.Send(ctx, session.OutboundMessage{
		Type:         msg.Type,
		RecipientJID: msg.RecipientJID,
		Payload:      msg.Payload,
	})
}

// stickerSender loads the stored webp of a sticker message and hands the
// session the file to upload as a sticker. A missing or expired file is a
// definitive failure: retrying cannot bring it back.
type stickerSender struct {
	media MediaPathResolver
}

// Send resolves the sticker of msg and forwards it to the session.
func (s stickerSender) Send(ctx context.Context, sess session.Session, msg model.OutboundMessage) (string, error) {
	if s.media == nil {
		return "", errors.New("sticker payload: media storage is not configured")
	}
	if msg.MediaID == nil {
		return "", errors.New("sticker payload: media_id is required")
	}

	path, record, err := s.media.Path(ctx, *msg.MediaID)
	if err != nil {
		return "", fmt.Errorf("sticker message %s: %w", msg.ID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(record.Mimetype), "image/webp") {
		return "", fmt.Errorf("sticker payload: unsupported mimetype %q", record.Mimetype)
	}

	var payload stickerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", fmt.Errorf("sticker payload: %w", err)
	}

	filename := payload.Filename
	if filename == "" {
		filename = record.Filename
	}
	body, err := json.Marshal(mediaOutboundPayload{
		Caption:  payload.Caption,
		Filename: filename,
		MimeType: record.Mimetype,
		QuotedID: payload.QuotedID,
	})
	if err != nil {
		return "", fmt.Errorf("sticker payload: %w", err)
	}

	return sess.Send(ctx, session.OutboundMessage{
		Type:         TypeSticker,
		RecipientJID: msg.RecipientJID,
		Payload:      body,
		MediaPath:    path,
	})
}

// listSender validates the stored list body before handing it to the session.
type listSender struct{}

// Send rejects a list without a button text, sections or rows and forwards
// the message.
func (listSender) Send(ctx context.Context, sess session.Session, msg model.OutboundMessage) (string, error) {
	var payload listPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", fmt.Errorf("list payload: %w", err)
	}
	if strings.TrimSpace(payload.Button) == "" {
		return "", errors.New("list payload: button_text is required")
	}
	if len(payload.Sections) == 0 {
		return "", errors.New("list payload: at least one section is required")
	}
	for _, section := range payload.Sections {
		if len(section.Rows) == 0 {
			return "", errors.New("list payload: every section needs at least one row")
		}
	}
	return sess.Send(ctx, session.OutboundMessage{
		Type:         msg.Type,
		RecipientJID: msg.RecipientJID,
		Payload:      msg.Payload,
	})
}

// buttonsSender validates the stored buttons body before handing it to the
// session.
type buttonsSender struct{}

// Send rejects a buttons message without a text or buttons and forwards the
// message.
func (buttonsSender) Send(ctx context.Context, sess session.Session, msg model.OutboundMessage) (string, error) {
	var payload buttonsPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", fmt.Errorf("buttons payload: %w", err)
	}
	if strings.TrimSpace(payload.Text) == "" {
		return "", errors.New("buttons payload: text is required")
	}
	if len(payload.Buttons) == 0 {
		return "", errors.New("buttons payload: at least one button is required")
	}
	return sess.Send(ctx, session.OutboundMessage{
		Type:         msg.Type,
		RecipientJID: msg.RecipientJID,
		Payload:      msg.Payload,
	})
}

// stickerMimetype is the only content type accepted for a sticker upload.
const stickerMimetype = "image/webp"

// ValidStickerMime reports whether mimetype is accepted for a sticker upload.
func ValidStickerMime(mimetype string) bool {
	return strings.EqualFold(strings.TrimSpace(mimetype), stickerMimetype)
}
