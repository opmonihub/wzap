package messages

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/media"
	"wzap/internal/message"
	"wzap/internal/model"
)

// MessageService is the message acceptance and query contract consumed by the
// handlers.
type MessageService interface {
	Enqueue(ctx context.Context, instanceID uuid.UUID, input message.EnqueueInput) (uuid.UUID, error)
	Get(ctx context.Context, instanceID, messageID uuid.UUID) (*model.OutboundMessage, error)
	List(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error)
}

// The service satisfies the handler contract; the assertion catches signature
// drift at build time.
var _ MessageService = (*message.Service)(nil)

// SendTextRequest is the POST /instances/{instance}/messages/text payload.
type SendTextRequest struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// SendLocationRequest is the POST /instances/{instance}/messages/location payload.
// The coordinates are pointers so a missing field is distinct from zero.
type SendLocationRequest struct {
	To        string   `json:"to"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// SendContactRequest is the POST /instances/{instance}/messages/contact payload.
type SendContactRequest struct {
	To          string `json:"to"`
	DisplayName string `json:"display_name"`
	VCard       string `json:"vcard"`
}

// SendListSection is one section of a generic list message request.
type SendListSection struct {
	Title string        `json:"title"`
	Rows  []SendListRow `json:"rows"`
}

// SendListRow is one selectable row of a generic list section request.
type SendListRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// SendButton is one quick-reply button of a generic buttons message request.
// A PIX key travels as pass-through content (for example in the title): there
// is no distinct PIX button type.
type SendButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SendMessageRequest is the POST /instances/{instance}/messages payload. It carries
// the rich types (poll, reaction, list, buttons); the historical text,
// location and contact types keep their dedicated routes, and stickers ride
// /messages/media with type=sticker. Only the fields of the chosen type are
// used. SelectableCount defaults to 1 when omitted; an empty reaction emoji
// removes the reaction on the same target.
type SendMessageRequest struct {
	Type            string            `json:"type"`
	To              string            `json:"to"`
	Question        string            `json:"question,omitempty"`
	Options         []string          `json:"options,omitempty"`
	SelectableCount *int              `json:"selectable_count,omitempty"`
	Target          string            `json:"target,omitempty"`
	Emoji           string            `json:"emoji,omitempty"`
	Title           string            `json:"title,omitempty"`
	Description     string            `json:"description,omitempty"`
	ButtonText      string            `json:"button_text,omitempty"`
	Sections        []SendListSection `json:"sections,omitempty"`
	Footer          string            `json:"footer,omitempty"`
	Text            string            `json:"text,omitempty"`
	Buttons         []SendButton      `json:"buttons,omitempty"`
}

// HandleSendMessage accepts a rich message (poll, reaction, list or buttons)
// and answers 202 with its id. It loads the target instance first (404) and
// authorizes (403) before reading the body or enqueueing anything. Content
// outside the documented limits answers 422 without persisting anything; a
// disconnected instance answers 409. Reactions address messages sent by the
// instance; removal reuses the same endpoint with an empty emoji.
//
// @Summary Send a rich message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body SendMessageRequest true "Rich payload: type poll|reaction|list|buttons plus its fields"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=representation.MessageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid content, unknown number, unsupported type, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages [post]
func HandleSendMessage(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request SendMessageRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		input, ok := RichEnqueueInput(request)
		if !ok {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				`type must be poll, reaction, list or buttons (text, location and contact keep their routes, sticker rides /messages/media)`)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, input)
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusAccepted, NewMessageAcceptedResponse(messageID, id, nil))
	}
}

// RichEnqueueInput maps a generic send request onto the service input. It
// reports false for types the generic endpoint does not carry.
func RichEnqueueInput(request SendMessageRequest) (message.EnqueueInput, bool) {
	switch request.Type {
	case message.TypePoll:
		selectable := 1
		if request.SelectableCount != nil {
			selectable = *request.SelectableCount
		}
		return message.EnqueueInput{
			Type:                message.TypePoll,
			To:                  request.To,
			PollQuestion:        request.Question,
			PollOptions:         request.Options,
			PollSelectableCount: selectable,
		}, true
	case message.TypeReaction:
		return message.EnqueueInput{
			Type:           message.TypeReaction,
			To:             request.To,
			ReactionTarget: request.Target,
			ReactionEmoji:  request.Emoji,
		}, true
	case message.TypeList:
		sections := make([]message.ListSection, 0, len(request.Sections))
		for _, section := range request.Sections {
			rows := make([]message.ListRow, 0, len(section.Rows))
			for _, row := range section.Rows {
				rows = append(rows, message.ListRow{ID: row.ID, Title: row.Title, Description: row.Description})
			}
			sections = append(sections, message.ListSection{Title: section.Title, Rows: rows})
		}
		return message.EnqueueInput{
			Type:            message.TypeList,
			To:              request.To,
			ListTitle:       request.Title,
			ListDescription: request.Description,
			ListButton:      request.ButtonText,
			ListSections:    sections,
			ListFooter:      request.Footer,
		}, true
	case message.TypeButtons:
		buttons := make([]message.Button, 0, len(request.Buttons))
		for _, button := range request.Buttons {
			buttons = append(buttons, message.Button{ID: button.ID, Title: button.Title})
		}
		return message.EnqueueInput{
			Type:          message.TypeButtons,
			To:            request.To,
			ButtonsText:   request.Text,
			ButtonsFooter: request.Footer,
			Buttons:       buttons,
		}, true
	}
	return message.EnqueueInput{}, false
}

// HandleSendText accepts a text message and answers 202 with its id. It loads
// the target instance first (404) and authorizes (403) before reading the
// body or enqueueing anything.
//
// @Summary Send a text message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body SendTextRequest true "Text payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=representation.MessageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/text [post]
func HandleSendText(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request SendTextRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type: message.TypeText,
			To:   request.To,
			Text: request.Text,
		})
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusAccepted, NewMessageAcceptedResponse(messageID, id, nil))
	}
}

// HandleSendLocation accepts a location message and answers 202 with its id.
// It loads the target instance first (404) and authorizes (403) before
// reading the body or enqueueing anything.
//
// @Summary Send a location message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body SendLocationRequest true "Location payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=representation.MessageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/location [post]
func HandleSendLocation(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request SendLocationRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if request.Latitude == nil || request.Longitude == nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "latitude and longitude are required")
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type:      message.TypeLocation,
			To:        request.To,
			Latitude:  *request.Latitude,
			Longitude: *request.Longitude,
		})
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusAccepted, NewMessageAcceptedResponse(messageID, id, nil))
	}
}

// HandleSendContact accepts a contact message and answers 202 with its id. It
// loads the target instance first (404) and authorizes (403) before reading
// the body or enqueueing anything.
//
// @Summary Send a contact message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body SendContactRequest true "Contact payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=representation.MessageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/contact [post]
func HandleSendContact(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request SendContactRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type:        message.TypeContact,
			To:          request.To,
			DisplayName: request.DisplayName,
			VCard:       request.VCard,
		})
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusAccepted, NewMessageAcceptedResponse(messageID, id, nil))
	}
}

// HandleSendMedia accepts a multipart media upload, stores the file and
// enqueues a media message referencing it. The declared type must be one of
// the outbound kinds and must match the uploaded content type; invalid content
// answers 422 before the media is stored or the message enqueued. It loads the
// target instance first (404) and authorizes (403) before reading the upload.
//
// @Summary Send a media message
// @Tags messages
// @Accept multipart/form-data
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param to formData string true "Recipient phone"
// @Param type formData string true "Media kind: image, video, audio, document or sticker (webp only, stored apart from media)"
// @Param caption formData string false "Caption"
// @Param filename formData string false "Override filename"
// @Param ptt formData string false "Push-to-talk flag for audio"
// @Param file formData file true "Media file"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=representation.MessageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Invalid multipart body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid file, mismatched type, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/media [post]
func HandleSendMedia(instances InstanceService, messages MessageService, mediaStore MediaStore, maxBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		target, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, target); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		if maxBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes+core.MediaFormOverhead)
		}
		if err := r.ParseMultipartForm(core.MediaFormMemory); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file exceeds the size limit")
				return
			}
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid multipart body")
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		to := strings.TrimSpace(r.FormValue("to"))
		kind := strings.TrimSpace(r.FormValue("type"))
		caption := r.FormValue("caption")
		filename := strings.TrimSpace(r.FormValue("filename"))
		if to == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "to is required")
			return
		}
		if !ValidMediaKind(kind) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type must be image, video, audio, document or sticker")
			return
		}
		ptt, err := ParseFormBool(r.FormValue("ptt"))
		if err != nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "ptt must be a boolean")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file is required")
			return
		}
		defer func() { _ = file.Close() }()

		mimetype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
		if err != nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file type is not supported")
			return
		}
		if kind == message.TypeSticker {
			if !message.ValidStickerMime(mimetype) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "sticker must be webp")
				return
			}
		} else {
			fileKind, ok := media.Kind(mimetype)
			if !ok {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file type is not supported")
				return
			}
			if fileKind != kind {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"type does not match the file content type")
				return
			}
		}
		if filename == "" {
			filename = header.Filename
		}

		data, err := core.ReadMediaUpload(file, maxBytes)
		if err != nil {
			if errors.Is(err, media.ErrTooLarge) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file exceeds the size limit")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if len(data) == 0 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file is empty")
			return
		}

		stored, err := mediaStore.Save(r.Context(), id, core.MediaDirectionOutbound, "", mimetype, filename, data)
		if err != nil {
			core.WriteMediaUploadError(w, r, err)
			return
		}

		enqueue := message.EnqueueInput{
			Type:     message.TypeMedia,
			To:       to,
			Caption:  caption,
			Filename: stored.Filename,
			PTT:      ptt,
			MediaID:  &stored.ID,
		}
		if kind == message.TypeSticker {
			enqueue = message.EnqueueInput{
				Type:     message.TypeSticker,
				To:       to,
				Caption:  caption,
				Filename: stored.Filename,
				MediaID:  &stored.ID,
			}
		}
		messageID, err := messages.Enqueue(r.Context(), id, enqueue)
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusAccepted, NewMessageAcceptedResponse(messageID, id, &stored.ID))
	}
}

// ValidMediaKind reports whether kind is one of the outbound media kinds,
// including sticker for webp uploads stored apart from media.
func ValidMediaKind(kind string) bool {
	switch kind {
	case media.KindImage, media.KindVideo, media.KindAudio, media.KindDocument, message.TypeSticker:
		return true
	}
	return false
}

// ParseFormBool reads an optional form boolean: "true"/"1" are true, an empty
// value, "false" and "0" are false and anything else is an error.
func ParseFormBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	}
	return false, errors.New("invalid boolean")
}

// HandleGetMessage answers 200 with one message of the instance. It loads the
// target instance first (404) and authorizes (403) before looking the message
// up.
//
// @Summary Get a message
// @Tags messages
// @Produce json
// @Security apikey
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param id path string true "Message ID (UUID)"
// @Success 200 {object} core.Envelope{data=representation.MessageEnvelope} "Message, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or message not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/{id} [get]
func HandleGetMessage(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		messageID, err := uuid.Parse(core.PathParam(r, "id"))
		if err != nil {
			core.Error(w, r, http.StatusNotFound, "not_found", "message not found")
			return
		}

		found, err := messages.Get(r.Context(), id, messageID)
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.MessageEnvelope{Message: representation.NewMessageResponse(found)})
	}
}

// HandleListMessages answers one page of messages with its next cursor. It
// loads the target instance first (404) and authorizes (403) before listing.
//
// @Summary List messages
// @Tags messages
// @Produce json
// @Security apikey
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} core.Envelope{data=representation.MessageListResponse} "One page, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Invalid cursor"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages [get]
func HandleListMessages(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		items, next, err := messages.List(r.Context(), id,
			ParseMessagesLimit(r.URL.Query().Get("limit")), r.URL.Query().Get("cursor"))
		if err != nil {
			WriteMessageError(w, r, err)
			return
		}

		response := representation.MessageListResponse{Messages: make([]representation.MessageResponse, 0, len(items)), NextCursor: next}
		for i := range items {
			response.Messages = append(response.Messages, representation.NewMessageResponse(&items[i]))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// ParseMessagesLimit reads the limit query parameter with the message defaults.
func ParseMessagesLimit(raw string) int {
	return core.ParseLimit(raw, core.DefaultMessagesLimit, core.MaxMessagesLimit)
}

// NewMessageAcceptedResponse maps an accepted send to its 202 body: the
// queue id plus the queued status under data.message. mediaID is the stored
// media of a media upload and nil for every other send.
func NewMessageAcceptedResponse(id, instanceID uuid.UUID, mediaID *uuid.UUID) representation.MessageAcceptedResponse {
	return representation.MessageAcceptedResponse{Message: representation.AcceptedMessageResponse{
		ID:         id.String(),
		InstanceID: instanceID.String(),
		SendStatus: message.StatusQueued,
		MediaID:    mediaID,
	}}
}

// WriteMessageError maps a message service error to its HTTP status and error
// envelope. Scope denials answer 403; unknown failures answer 500 without
// leaking their cause.
func WriteMessageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		core.Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, message.ErrInstanceNotFound):
		core.Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	case errors.Is(err, message.ErrMessageNotFound):
		core.Error(w, r, http.StatusNotFound, "not_found", "message not found")
	case errors.Is(err, message.ErrInstanceNotConnected):
		core.Error(w, r, http.StatusConflict, "conflict", "instance not connected")
	case errors.Is(err, message.ErrNumberNotFound):
		core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "recipient number is not on WhatsApp")
	case errors.Is(err, message.ErrInvalidInput):
		core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid message content")
	case errors.Is(err, message.ErrResolverUnavailable):
		core.Error(w, r, http.StatusServiceUnavailable, "unavailable", "number resolution unavailable")
	case errors.Is(err, message.ErrInvalidCursor):
		core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid cursor")
	default:
		core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type MediaStore interface {
	Save(context.Context, uuid.UUID, string, string, string, string, []byte) (*model.Media, error)
}
