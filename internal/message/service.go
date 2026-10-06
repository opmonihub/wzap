package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

// Message types accepted by Enqueue and persisted in message_queue.type.
const (
	TypeText     = "text"
	TypeLocation = "location"
	TypeContact  = "contact"
	TypeMedia    = "media"
	// TypePoll is a poll creation message (question + options).
	TypePoll = "poll"
	// TypeReaction is a reaction to a previous message; an empty emoji
	// removes the reaction on the same target.
	TypeReaction = "reaction"
	// TypeSticker is a webp sticker uploaded through /messages/media with
	// type=sticker. It is stored apart from TypeMedia so senders upload it
	// as a sticker instead of an image.
	TypeSticker = "sticker"
	// TypeList is an interactive list message (sections of rows).
	TypeList = "list"
	// TypeButtons is an interactive buttons message (up to three quick
	// replies). A PIX key travels as button content pass-through: there is
	// no distinct PIX button type, the {id,title} pair carries the key as
	// documented by the API.
	TypeButtons = "buttons"
)

// Message statuses persisted in message_queue.status.
const (
	// StatusQueued is the initial state of an accepted message.
	StatusQueued = "queued"
	// StatusSending means a worker claimed the message for delivery.
	StatusSending = "sending"
	// StatusSent means WhatsApp accepted the message.
	StatusSent = "sent"
	// StatusFailed means the message was not delivered and will not be retried.
	StatusFailed = "failed"
)

// Errors reported by the service and mapped to HTTP status codes by the handler
// layer.
var (
	// ErrInstanceNotFound reports that the enqueue target does not exist.
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrInstanceNotConnected reports that the instance cannot send because its
	// session is not connected.
	ErrInstanceNotConnected = errors.New("instance not connected")
	// ErrMessageNotFound reports that the queried message does not exist or
	// belongs to another instance.
	ErrMessageNotFound = errors.New("message not found")
	// ErrInvalidCursor reports that a list cursor is not a valid identifier.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrInvalidInput reports that the message content is malformed or
	// unsupported.
	ErrInvalidInput = errors.New("invalid message input")
)

// Rich message limits, enforced by Enqueue before anything is persisted.
// Anything outside them is ErrInvalidInput (HTTP 422).
const (
	// MaxPollQuestion is the longest poll question in characters.
	MaxPollQuestion = 300
	// MinPollOptions and MaxPollOptions bound the poll option count.
	MinPollOptions = 2
	MaxPollOptions = 12
	// MaxPollOption is the longest poll option in characters.
	MaxPollOption = 100
	// MaxReactionEmoji is the longest reaction content in characters; zero
	// (empty) removes the reaction on the same target.
	MaxReactionEmoji = 32
	// MinListSections and MaxListSections bound the list section count.
	MinListSections = 1
	MaxListSections = 10
	// MinListRows and MaxListRows bound the row count of one list section.
	MinListRows = 1
	MaxListRows = 10
	// MaxListText is the longest list text (button, title, description,
	// footer, section title, row id/title/description) in characters.
	MaxListText = 300
	// MinButtons and MaxButtons bound the button count of a buttons message.
	MinButtons = 1
	MaxButtons = 3
	// MaxButtonID and MaxButtonTitle bound the button id and title in
	// characters.
	MaxButtonID    = 64
	MaxButtonTitle = 64
	// MaxButtonsText is the longest buttons body in characters.
	MaxButtonsText = 300
	// MaxButtonsFooter is the longest buttons footer in characters.
	MaxButtonsFooter = 300
)

// InstanceReader reads the instance a message is enqueued for.
type InstanceReader interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
}

// Resolver resolves a recipient phone number to its canonical WhatsApp JID.
type Resolver interface {
	Resolve(ctx context.Context, instanceID uuid.UUID, phone string) (jid string, err error)
}

// MessageStore persists and queries the outbound message queue.
type MessageStore interface {
	Create(ctx context.Context, message model.OutboundMessage) (*model.OutboundMessage, error)
	Get(ctx context.Context, id uuid.UUID) (*model.OutboundMessage, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error)
}

// EnqueueInput is the content accepted by Enqueue. Only the fields of the
// chosen Type are used; the rest are ignored. QuotedID carries the WhatsApp
// message id being replied to (a quote); empty means no quote.
// PollSelectableCount is 0 or 1 (the HTTP layer defaults an omitted value to
// 1); ReactionTarget is the WhatsApp id of the reacted message and an empty
// ReactionEmoji removes the reaction.
type EnqueueInput struct {
	Type        string
	To          string
	Text        string
	Caption     string
	Filename    string
	PTT         bool
	Latitude    float64
	Longitude   float64
	DisplayName string
	VCard       string
	MediaID     *uuid.UUID
	QuotedID    string

	PollQuestion        string
	PollOptions         []string
	PollSelectableCount int
	ReactionTarget      string
	ReactionEmoji       string
	ListTitle           string
	ListDescription     string
	ListButton          string
	ListSections        []ListSection
	ListFooter          string
	ButtonsText         string
	ButtonsFooter       string
	Buttons             []Button
}

// ListSection is one section of a list message: a titled group of rows.
type ListSection struct {
	Title string    `json:"title"`
	Rows  []ListRow `json:"rows"`
}

// ListRow is one selectable row of a list section.
type ListRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Button is one quick-reply button of a buttons message. A PIX key travels
// as pass-through content (for example in the title), not as a distinct
// button type.
type Button struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Service accepts outbound messages: it validates the target instance, resolves
// the recipient JID and persists the message as queued for the outbox workers.
type Service struct {
	instances InstanceReader
	resolver  Resolver
	messages  MessageStore
}

// The concrete resolver satisfies the service contract; the assertion catches
// signature drift at build time.
var _ Resolver = (*JIDResolver)(nil)

// NewService builds the service over its dependencies.
func NewService(instances InstanceReader, resolver Resolver, messages MessageStore) *Service {
	return &Service{instances: instances, resolver: resolver, messages: messages}
}

// Enqueue validates and stores one message, returning its identifier. A
// disconnected instance is a conflict; an unresolved recipient or an invalid
// payload is rejected without storing anything. The stored recipient is the
// canonical JID returned by the resolver.
func (s *Service) Enqueue(ctx context.Context, instanceID uuid.UUID, input EnqueueInput) (uuid.UUID, error) {
	instance, err := s.instances.Get(ctx, instanceID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return uuid.Nil, fmt.Errorf("enqueue message: %w", ErrInstanceNotFound)
		}
		return uuid.Nil, fmt.Errorf("enqueue message: get instance: %w", err)
	}
	if instance.Connection.Status != string(session.StatusConnected) {
		return uuid.Nil, fmt.Errorf("enqueue message: instance %s is %s: %w", instanceID, instance.Connection.Status, ErrInstanceNotConnected)
	}

	payload, err := buildPayload(input)
	if err != nil {
		return uuid.Nil, err
	}

	jid, err := s.resolver.Resolve(ctx, instanceID, input.To)
	if err != nil {
		return uuid.Nil, fmt.Errorf("enqueue message: %w", err)
	}

	created, err := s.messages.Create(ctx, model.OutboundMessage{
		ID:           uuid.New(),
		InstanceID:   instanceID,
		Type:         input.Type,
		RecipientJID: jid,
		Payload:      payload,
		MediaID:      input.MediaID,
		Status:       StatusQueued,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("enqueue message: create: %w", err)
	}
	return created.ID, nil
}

// Get returns a message of instanceID or ErrMessageNotFound. A message that
// belongs to another instance is hidden so the query cannot cross instances.
func (s *Service) Get(ctx context.Context, instanceID, messageID uuid.UUID) (*model.OutboundMessage, error) {
	message, err := s.messages.Get(ctx, messageID)
	if err != nil {
		return nil, mapMessageError("get message", err)
	}
	if message.InstanceID != instanceID {
		return nil, fmt.Errorf("get message: %w", ErrMessageNotFound)
	}
	return message, nil
}

// List returns a page of instanceID messages and the cursor of the next page,
// empty on the last page.
func (s *Service) List(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error) {
	messages, next, err := s.messages.ListByInstance(ctx, instanceID, limit, cursor)
	if err != nil {
		return nil, "", mapMessageError("list messages", err)
	}
	return messages, next, nil
}

// buildPayload validates the content of input and serializes the type-specific
// JSON body stored in message_queue.payload. An unsupported type or missing
// required content is ErrInvalidInput.
func buildPayload(input EnqueueInput) ([]byte, error) {
	switch input.Type {
	case TypeText:
		if strings.TrimSpace(input.Text) == "" {
			return nil, fmt.Errorf("%w: text is required", ErrInvalidInput)
		}
		return json.Marshal(textPayload{Text: input.Text, QuotedID: input.QuotedID})
	case TypeLocation:
		if math.IsNaN(input.Latitude) || input.Latitude < -90 || input.Latitude > 90 {
			return nil, fmt.Errorf("%w: latitude out of range", ErrInvalidInput)
		}
		if math.IsNaN(input.Longitude) || input.Longitude < -180 || input.Longitude > 180 {
			return nil, fmt.Errorf("%w: longitude out of range", ErrInvalidInput)
		}
		return json.Marshal(locationPayload{Latitude: input.Latitude, Longitude: input.Longitude})
	case TypeContact:
		if strings.TrimSpace(input.DisplayName) == "" {
			return nil, fmt.Errorf("%w: display_name is required", ErrInvalidInput)
		}
		if strings.TrimSpace(input.VCard) == "" {
			return nil, fmt.Errorf("%w: vcard is required", ErrInvalidInput)
		}
		return json.Marshal(contactPayload{DisplayName: input.DisplayName, VCard: input.VCard})
	case TypeMedia:
		if input.MediaID == nil {
			return nil, fmt.Errorf("%w: media_id is required", ErrInvalidInput)
		}
		return json.Marshal(mediaPayload{Caption: input.Caption, Filename: input.Filename, PTT: input.PTT, QuotedID: input.QuotedID})
	case TypePoll:
		if err := validatePoll(input); err != nil {
			return nil, err
		}
		return json.Marshal(pollPayload{Question: input.PollQuestion, Options: input.PollOptions, SelectableCount: input.PollSelectableCount})
	case TypeReaction:
		if strings.TrimSpace(input.ReactionTarget) == "" {
			return nil, fmt.Errorf("%w: reaction target is required", ErrInvalidInput)
		}
		if len([]rune(input.ReactionEmoji)) > MaxReactionEmoji {
			return nil, fmt.Errorf("%w: reaction emoji exceeds %d characters", ErrInvalidInput, MaxReactionEmoji)
		}
		return json.Marshal(reactionPayload{Target: input.ReactionTarget, Emoji: input.ReactionEmoji})
	case TypeSticker:
		if input.MediaID == nil {
			return nil, fmt.Errorf("%w: sticker rides /messages/media with type=sticker, not the generic endpoint", ErrInvalidInput)
		}
		return json.Marshal(stickerPayload{Caption: input.Caption, Filename: input.Filename, QuotedID: input.QuotedID})
	case TypeList:
		if err := validateList(input); err != nil {
			return nil, err
		}
		return json.Marshal(listPayload{
			Title:       input.ListTitle,
			Description: input.ListDescription,
			Button:      input.ListButton,
			Sections:    input.ListSections,
			Footer:      input.ListFooter,
		})
	case TypeButtons:
		if err := validateButtons(input); err != nil {
			return nil, err
		}
		return json.Marshal(buttonsPayload{Text: input.ButtonsText, Footer: input.ButtonsFooter, Buttons: input.Buttons})
	default:
		return nil, fmt.Errorf("%w: unsupported type %q", ErrInvalidInput, input.Type)
	}
}

// validatePoll rejects a poll outside the documented limits: a question of
// 1..MaxPollQuestion characters, MinPollOptions..MaxPollOptions options of
// 1..MaxPollOption characters each and a selectable count of 0 or 1.
func validatePoll(input EnqueueInput) error {
	if n := len([]rune(input.PollQuestion)); n == 0 || n > MaxPollQuestion {
		return fmt.Errorf("%w: poll question must be 1..%d characters", ErrInvalidInput, MaxPollQuestion)
	}
	if len(input.PollOptions) < MinPollOptions || len(input.PollOptions) > MaxPollOptions {
		return fmt.Errorf("%w: poll must carry %d..%d options", ErrInvalidInput, MinPollOptions, MaxPollOptions)
	}
	for _, option := range input.PollOptions {
		if n := len([]rune(option)); n == 0 || n > MaxPollOption {
			return fmt.Errorf("%w: poll options must be 1..%d characters", ErrInvalidInput, MaxPollOption)
		}
	}
	if input.PollSelectableCount < 0 || input.PollSelectableCount > 1 {
		return fmt.Errorf("%w: poll selectable count must be 0 or 1", ErrInvalidInput)
	}
	return nil
}

// validateList rejects a list outside the documented limits: 1..MaxListSections
// sections of 1..MaxListRows rows each, a required button text and every text
// within 1..MaxListText characters where required.
func validateList(input EnqueueInput) error {
	if n := len([]rune(input.ListButton)); n == 0 || n > MaxListText {
		return fmt.Errorf("%w: list button text must be 1..%d characters", ErrInvalidInput, MaxListText)
	}
	for _, text := range []string{input.ListTitle, input.ListDescription, input.ListFooter} {
		if len([]rune(text)) > MaxListText {
			return fmt.Errorf("%w: list texts must be at most %d characters", ErrInvalidInput, MaxListText)
		}
	}
	if len(input.ListSections) < MinListSections || len(input.ListSections) > MaxListSections {
		return fmt.Errorf("%w: list must carry %d..%d sections", ErrInvalidInput, MinListSections, MaxListSections)
	}
	for _, section := range input.ListSections {
		if n := len([]rune(section.Title)); n == 0 || n > MaxListText {
			return fmt.Errorf("%w: list section titles must be 1..%d characters", ErrInvalidInput, MaxListText)
		}
		if len(section.Rows) < MinListRows || len(section.Rows) > MaxListRows {
			return fmt.Errorf("%w: list sections must carry %d..%d rows", ErrInvalidInput, MinListRows, MaxListRows)
		}
		for _, row := range section.Rows {
			if n := len([]rune(row.ID)); n == 0 || n > MaxListText {
				return fmt.Errorf("%w: list row ids must be 1..%d characters", ErrInvalidInput, MaxListText)
			}
			if n := len([]rune(row.Title)); n == 0 || n > MaxListText {
				return fmt.Errorf("%w: list row titles must be 1..%d characters", ErrInvalidInput, MaxListText)
			}
			if len([]rune(row.Description)) > MaxListText {
				return fmt.Errorf("%w: list row descriptions must be at most %d characters", ErrInvalidInput, MaxListText)
			}
		}
	}
	return nil
}

// validateButtons rejects a buttons message outside the documented limits: a
// body of 1..MaxButtonsText characters and MinButtons..MaxButtons buttons
// with ids of 1..MaxButtonID and titles of 1..MaxButtonTitle characters.
func validateButtons(input EnqueueInput) error {
	if n := len([]rune(input.ButtonsText)); n == 0 || n > MaxButtonsText {
		return fmt.Errorf("%w: buttons text must be 1..%d characters", ErrInvalidInput, MaxButtonsText)
	}
	if len([]rune(input.ButtonsFooter)) > MaxButtonsFooter {
		return fmt.Errorf("%w: buttons footer must be at most %d characters", ErrInvalidInput, MaxButtonsFooter)
	}
	if len(input.Buttons) < MinButtons || len(input.Buttons) > MaxButtons {
		return fmt.Errorf("%w: buttons must carry %d..%d buttons", ErrInvalidInput, MinButtons, MaxButtons)
	}
	for _, button := range input.Buttons {
		if n := len([]rune(button.ID)); n == 0 || n > MaxButtonID {
			return fmt.Errorf("%w: button ids must be 1..%d characters", ErrInvalidInput, MaxButtonID)
		}
		if n := len([]rune(button.Title)); n == 0 || n > MaxButtonTitle {
			return fmt.Errorf("%w: button titles must be 1..%d characters", ErrInvalidInput, MaxButtonTitle)
		}
	}
	return nil
}

// textPayload is the stored body of a text message. QuotedID is the WhatsApp
// id being replied to, omitted when the message is not a quote.
type textPayload struct {
	Text     string `json:"text"`
	QuotedID string `json:"quoted_id,omitempty"`
}

// locationPayload is the stored body of a location message.
type locationPayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// contactPayload is the stored body of a contact message.
type contactPayload struct {
	DisplayName string `json:"display_name"`
	VCard       string `json:"vcard"`
}

// mediaPayload is the stored body of a media message. The mimetype is resolved
// from the media row by the sender. QuotedID is the WhatsApp id being replied
// to, omitted when the message is not a quote.
type mediaPayload struct {
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
	PTT      bool   `json:"ptt,omitempty"`
	QuotedID string `json:"quoted_id,omitempty"`
}

// pollPayload is the stored body of a poll message: the question, the option
// names in order and how many options the recipient may select (0 or 1).
type pollPayload struct {
	Question        string   `json:"question"`
	Options         []string `json:"options"`
	SelectableCount int      `json:"selectable_count"`
}

// reactionPayload is the stored body of a reaction message: the WhatsApp id
// of the reacted message and the emoji. An empty emoji removes the reaction
// on the same target; the emoji itself is not validated.
type reactionPayload struct {
	Target string `json:"target"`
	Emoji  string `json:"emoji"`
}

// stickerPayload is the stored body of a sticker message. The webp mimetype
// is resolved from the media row by the sender, like media messages.
type stickerPayload struct {
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
	QuotedID string `json:"quoted_id,omitempty"`
}

// listPayload is the stored body of a list message.
type listPayload struct {
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Button      string        `json:"button_text"`
	Sections    []ListSection `json:"sections"`
	Footer      string        `json:"footer,omitempty"`
}

// buttonsPayload is the stored body of a buttons message. A PIX key travels
// as button content pass-through, not as a distinct button type.
type buttonsPayload struct {
	Text    string   `json:"text"`
	Footer  string   `json:"footer,omitempty"`
	Buttons []Button `json:"buttons"`
}

// mapMessageError translates a storage error into the service sentinel the HTTP
// layer maps to a status code, preserving the operation context for the logs.
func mapMessageError(op string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return fmt.Errorf("%s: %w", op, ErrMessageNotFound)
	case errors.Is(err, storage.ErrInvalidCursor):
		return fmt.Errorf("%s: %w", op, ErrInvalidCursor)
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}
