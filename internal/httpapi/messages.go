package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/media"
	"wzap/internal/message"
	"wzap/internal/model"
)

const (
	// defaultMessagesLimit is the page size used when the request omits limit.
	defaultMessagesLimit = 50
	// maxMessagesLimit caps the page size a client can request.
	maxMessagesLimit = 100
	// mediaDirectionOutbound labels media uploaded for a send.
	mediaDirectionOutbound = "outbound"
	// mediaFormMemory is how much of an upload ParseMultipartForm keeps in
	// memory; larger file parts spill to temporary files.
	mediaFormMemory = 1 << 20
	// mediaFormOverhead is the slack above the configured media limit that
	// covers the multipart boundaries, part headers and text fields.
	mediaFormOverhead = 1 << 20
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

// messageAcceptedResponse, messageResponse and messageListResponse live in
// dto.go with the other public shapes.

// sendTextRequest is the POST /instances/{id}/messages/text payload.
type sendTextRequest struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// sendLocationRequest is the POST /instances/{id}/messages/location payload.
// The coordinates are pointers so a missing field is distinct from zero.
type sendLocationRequest struct {
	To        string   `json:"to"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// sendContactRequest is the POST /instances/{id}/messages/contact payload.
type sendContactRequest struct {
	To          string `json:"to"`
	DisplayName string `json:"display_name"`
	VCard       string `json:"vcard"`
}

// sendListSection is one section of a generic list message request.
type sendListSection struct {
	Title string        `json:"title"`
	Rows  []sendListRow `json:"rows"`
}

// sendListRow is one selectable row of a generic list section request.
type sendListRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// sendButton is one quick-reply button of a generic buttons message request.
// A PIX key travels as pass-through content (for example in the title): there
// is no distinct PIX button type.
type sendButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// sendMessageRequest is the POST /instances/{id}/messages payload. It carries
// the rich types (poll, reaction, list, buttons); the historical text,
// location and contact types keep their dedicated routes, and stickers ride
// /messages/media with type=sticker. Only the fields of the chosen type are
// used. SelectableCount defaults to 1 when omitted; an empty reaction emoji
// removes the reaction on the same target.
type sendMessageRequest struct {
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
	Sections        []sendListSection `json:"sections,omitempty"`
	Footer          string            `json:"footer,omitempty"`
	Text            string            `json:"text,omitempty"`
	Buttons         []sendButton      `json:"buttons,omitempty"`
}

// handleSendMessage accepts a rich message (poll, reaction, list or buttons)
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
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body sendMessageRequest true "Rich payload: type poll|reaction|list|buttons plus its fields"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=messageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid content, unknown number, unsupported type, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 503 {object} errorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages [post]
func handleSendMessage(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request sendMessageRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		input, ok := richEnqueueInput(request)
		if !ok {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				`type must be poll, reaction, list or buttons (text, location and contact keep their routes, sticker rides /messages/media)`)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, input)
		if err != nil {
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusAccepted, newMessageAcceptedResponse(messageID, id, nil))
	}
}

// richEnqueueInput maps a generic send request onto the service input. It
// reports false for types the generic endpoint does not carry.
func richEnqueueInput(request sendMessageRequest) (message.EnqueueInput, bool) {
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

// handleSendText accepts a text message and answers 202 with its id. It loads
// the target instance first (404) and authorizes (403) before reading the
// body or enqueueing anything.
//
// @Summary Send a text message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body sendTextRequest true "Text payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=messageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 503 {object} errorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/text [post]
func handleSendText(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request sendTextRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type: message.TypeText,
			To:   request.To,
			Text: request.Text,
		})
		if err != nil {
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusAccepted, newMessageAcceptedResponse(messageID, id, nil))
	}
}

// handleSendLocation accepts a location message and answers 202 with its id.
// It loads the target instance first (404) and authorizes (403) before
// reading the body or enqueueing anything.
//
// @Summary Send a location message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body sendLocationRequest true "Location payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=messageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 503 {object} errorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/location [post]
func handleSendLocation(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request sendLocationRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.Latitude == nil || request.Longitude == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "latitude and longitude are required")
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type:      message.TypeLocation,
			To:        request.To,
			Latitude:  *request.Latitude,
			Longitude: *request.Longitude,
		})
		if err != nil {
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusAccepted, newMessageAcceptedResponse(messageID, id, nil))
	}
}

// handleSendContact accepts a contact message and answers 202 with its id. It
// loads the target instance first (404) and authorizes (403) before reading
// the body or enqueueing anything.
//
// @Summary Send a contact message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body sendContactRequest true "Contact payload"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=messageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid content, unknown number, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 503 {object} errorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/contact [post]
func handleSendContact(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request sendContactRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		messageID, err := messages.Enqueue(r.Context(), id, message.EnqueueInput{
			Type:        message.TypeContact,
			To:          request.To,
			DisplayName: request.DisplayName,
			VCard:       request.VCard,
		})
		if err != nil {
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusAccepted, newMessageAcceptedResponse(messageID, id, nil))
	}
}

// handleSendMedia accepts a multipart media upload, stores the file and
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
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param to formData string true "Recipient phone"
// @Param type formData string true "Media kind: image, video, audio, document or sticker (webp only, stored apart from media)"
// @Param caption formData string false "Caption"
// @Param filename formData string false "Override filename"
// @Param ptt formData string false "Push-to-talk flag for audio"
// @Param file formData file true "Media file"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=messageAcceptedResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Invalid multipart body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 422 {object} errorEnvelope "Invalid file, mismatched type, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 503 {object} errorEnvelope "Number resolution unavailable"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/media [post]
func handleSendMedia(instances InstanceService, messages MessageService, mediaStore MediaStore, maxBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		target, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, target); err != nil {
			writeForbidden(w, r)
			return
		}

		if maxBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes+mediaFormOverhead)
		}
		if err := r.ParseMultipartForm(mediaFormMemory); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file exceeds the size limit")
				return
			}
			Error(w, r, http.StatusBadRequest, "invalid_request", "invalid multipart body")
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		to := strings.TrimSpace(r.FormValue("to"))
		kind := strings.TrimSpace(r.FormValue("type"))
		caption := r.FormValue("caption")
		filename := strings.TrimSpace(r.FormValue("filename"))
		if to == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "to is required")
			return
		}
		if !validMediaKind(kind) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type must be image, video, audio, document or sticker")
			return
		}
		ptt, err := parseFormBool(r.FormValue("ptt"))
		if err != nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "ptt must be a boolean")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file is required")
			return
		}
		defer func() { _ = file.Close() }()

		mimetype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
		if err != nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file type is not supported")
			return
		}
		if kind == message.TypeSticker {
			if !message.ValidStickerMime(mimetype) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "sticker must be webp")
				return
			}
		} else {
			fileKind, ok := media.Kind(mimetype)
			if !ok {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file type is not supported")
				return
			}
			if fileKind != kind {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"type does not match the file content type")
				return
			}
		}
		if filename == "" {
			filename = header.Filename
		}

		data, err := readMediaUpload(file, maxBytes)
		if err != nil {
			if errors.Is(err, media.ErrTooLarge) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file exceeds the size limit")
				return
			}
			Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if len(data) == 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "file is empty")
			return
		}

		stored, err := mediaStore.Save(r.Context(), id, mediaDirectionOutbound, "", mimetype, filename, data)
		if err != nil {
			writeMediaUploadError(w, r, err)
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
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusAccepted, newMessageAcceptedResponse(messageID, id, &stored.ID))
	}
}

// validMediaKind reports whether kind is one of the outbound media kinds,
// including sticker for webp uploads stored apart from media.
func validMediaKind(kind string) bool {
	switch kind {
	case media.KindImage, media.KindVideo, media.KindAudio, media.KindDocument, message.TypeSticker:
		return true
	}
	return false
}

// parseFormBool reads an optional form boolean: "true"/"1" are true, an empty
// value, "false" and "0" are false and anything else is an error.
func parseFormBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	}
	return false, errors.New("invalid boolean")
}

// readMediaUpload reads the file part, refusing content above the configured
// limit before it is buffered. A non-positive limit defers to the store.
func readMediaUpload(file io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(file)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, media.ErrTooLarge
	}
	return data, nil
}

// writeMediaUploadError maps a media store failure during an upload: empty and
// oversized files are unprocessable and an unknown instance surfaces as
// ErrNotFound from the media foreign key.
func writeMediaUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, media.ErrEmpty), errors.Is(err, media.ErrTooLarge):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid media file")
	case errors.Is(err, media.ErrNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	default:
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// handleGetMessage answers 200 with one message of the instance. It loads the
// target instance first (404) and authorizes (403) before looking the message
// up.
//
// @Summary Get a message
// @Tags messages
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param message_id path string true "Message ID (UUID)"
// @Success 200 {object} envelope{data=messageEnvelope} "Message, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or message not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/{message_id} [get]
func handleGetMessage(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		messageID, err := uuid.Parse(r.PathValue("message_id"))
		if err != nil {
			Error(w, r, http.StatusNotFound, "not_found", "message not found")
			return
		}

		found, err := messages.Get(r.Context(), id, messageID)
		if err != nil {
			writeMessageError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, messageEnvelope{Message: newMessageResponse(found)})
	}
}

// handleListMessages answers one page of messages with its next cursor. It
// loads the target instance first (404) and authorizes (403) before listing.
//
// @Summary List messages
// @Tags messages
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} envelope{data=messageListResponse} "One page, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Invalid cursor"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages [get]
func handleListMessages(instances InstanceService, messages MessageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		items, next, err := messages.List(r.Context(), id,
			parseMessagesLimit(r.URL.Query().Get("limit")), r.URL.Query().Get("cursor"))
		if err != nil {
			writeMessageError(w, r, err)
			return
		}

		response := messageListResponse{Items: make([]messageEnvelope, 0, len(items)), NextCursor: next}
		for i := range items {
			response.Items = append(response.Items, messageEnvelope{Message: newMessageResponse(&items[i])})
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// parseMessagesLimit reads the limit query parameter with the message defaults.
func parseMessagesLimit(raw string) int {
	return parseLimit(raw, defaultMessagesLimit, maxMessagesLimit)
}

// newMessageAcceptedResponse maps an accepted send to its 202 body: the
// queue id plus the queued status under data.message. mediaID is the stored
// media of a media upload and nil for every other send.
func newMessageAcceptedResponse(id, instanceID uuid.UUID, mediaID *uuid.UUID) messageAcceptedResponse {
	return messageAcceptedResponse{Message: acceptedMessageResponse{
		ID:         id.String(),
		InstanceID: instanceID.String(),
		SendStatus: message.StatusQueued,
		MediaID:    mediaID,
	}}
}

// writeMessageError maps a message service error to its HTTP status and error
// envelope. Scope denials answer 403; unknown failures answer 500 without
// leaking their cause.
func writeMessageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, message.ErrInstanceNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	case errors.Is(err, message.ErrMessageNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "message not found")
	case errors.Is(err, message.ErrInstanceNotConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance not connected")
	case errors.Is(err, message.ErrNumberNotFound):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "recipient number is not on WhatsApp")
	case errors.Is(err, message.ErrInvalidInput):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid message content")
	case errors.Is(err, message.ErrResolverUnavailable):
		Error(w, r, http.StatusServiceUnavailable, "unavailable", "number resolution unavailable")
	case errors.Is(err, message.ErrInvalidCursor):
		Error(w, r, http.StatusBadRequest, "invalid_request", "invalid cursor")
	default:
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
