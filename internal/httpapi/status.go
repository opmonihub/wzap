package httpapi

import (
	"errors"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"wzap/internal/media"
	"wzap/internal/session"
)

const (
	// maxStatusTextLen caps a text status and a media caption in characters:
	// longer content is rejected with 422 before the session is touched.
	maxStatusTextLen = 700
	// defaultStatusMediaBytes caps a status image/video upload when no media
	// cap is configured.
	defaultStatusMediaBytes = 5 << 20
)

// publishStatusRequest is the POST /instances/{id}/status/updates payload: text
// statuses only. Image and video statuses ride POST
// /instances/{id}/status/updates/media with the file bytes.
type publishStatusRequest struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// statusPublishResponse is the 202 answer to a published status: the upstream
// id with the same idempotency semantics as the message sends (a repeated
// Idempotency-Key replays it with X-Idempotent-Replay).
type statusPublishResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// ownStatusResponse is one own status of the GET /instances/{id}/status/updates listing.
type ownStatusResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Caption   string `json:"caption,omitempty"`
	CreatedAt string `json:"created_at"`
}

// statusListResponse is the 200 answer to the own status listing. Items is
// never nil so the field keeps its array shape on an empty listing.
type statusListResponse struct {
	Items []ownStatusResponse `json:"items"`
}

// statusDeleteResponse is the 200 answer to a removed own status.
type statusDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

// validStatusKind reports whether kind is one of the status media kinds the
// multipart publish accepts.
func validStatusKind(kind string) bool {
	return kind == session.StatusKindImage || kind == session.StatusKindVideo
}

// statusTextValid reports whether text fits a text status or a media caption:
// 1..maxStatusTextLen characters.
func statusTextValid(text string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	return n >= 1 && n <= maxStatusTextLen
}

// handlePublishStatus publishes a text status and answers 202 with its
// upstream id. It loads the target first (404) and authorizes (403) before
// reading the body or touching the session; an offline session answers 409
// and an invalid payload 422. Image and video statuses ride
// POST /instances/{id}/status/updates/media with the file bytes.
//
// @Summary Publish a text status
// @Description Status publish is a synchronous fire-and-forget broadcast (no outbox retry); 202 carries message_id with the same idempotency replay as message sends.
// @Tags status
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body publishStatusRequest true "Status payload: type text plus its text"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=statusPublishResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid type or text, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates [post]
func handlePublishStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish").Msg("publish status request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish").Err(err).Msg("publish status failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request publishStatusRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if strings.TrimSpace(request.Type) != session.StatusKindText {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type must be text (image and video ride /status/updates/media)")
			return
		}
		text := strings.TrimSpace(request.Text)
		if !statusTextValid(text) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "text is required, max 700 characters")
			return
		}

		statusID, err := instances.PublishStatus(r.Context(), id, session.StatusInput{Text: text})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish").Err(err).Msg("publish status failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish").Str("status_id", statusID).Msg("publish status result")
		JSON(w, r, http.StatusAccepted, statusPublishResponse{MessageID: statusID, Status: "published"})
	}
}

// handlePublishStatusMedia publishes an image or video status from a
// multipart upload and answers 202 with its upstream id. The declared type
// must match the uploaded content type and the file must fit the configured
// media cap; anything else answers 422 before the session is touched. It
// loads the target first (404) and authorizes (403) before reading the
// upload; an offline session answers 409.
//
// @Summary Publish an image or video status
// @Description Status publish is a synchronous fire-and-forget broadcast (no outbox retry); 202 carries message_id with the same idempotency replay as message sends.
// @Tags status
// @Accept multipart/form-data
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param type formData string true "Media kind: image or video"
// @Param caption formData string false "Caption, max 700 characters"
// @Param file formData file true "Media file"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} envelope{data=statusPublishResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Invalid multipart body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 422 {object} errorEnvelope "Invalid file, mismatched type, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates/media [post]
func handlePublishStatusMedia(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = defaultStatusMediaBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish-media").Msg("publish status media request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish-media").Err(err).Msg("publish status media failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
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

		kind := strings.TrimSpace(r.FormValue("type"))
		if !validStatusKind(kind) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "type must be image or video")
			return
		}
		caption := strings.TrimSpace(r.FormValue("caption"))
		if caption != "" && !statusTextValid(caption) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "caption must be max 700 characters")
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
		fileKind, ok := media.Kind(mimetype)
		if !ok || fileKind != kind {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type does not match the file content type")
			return
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

		statusID, err := instances.PublishStatus(r.Context(), id, session.StatusInput{
			MediaMime: mimetype,
			MediaData: data,
			Caption:   caption,
		})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish-media").Err(err).Msg("publish status media failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish-media").Str("status_id", statusID).Msg("publish status media result")
		JSON(w, r, http.StatusAccepted, statusPublishResponse{MessageID: statusID, Status: "published"})
	}
}

// handleListStatuses answers the own statuses published through the instance
// session. Entries older than 24h are dropped by the session registry. It
// loads the target first (404) and authorizes (403) before
// touching the session; an offline session answers 409.
//
// @Summary List own statuses
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=statusListResponse} "Own statuses published since boot, entries older than 24h are dropped, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates [get]
func handleListStatuses(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-list").Msg("list statuses request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-list").Err(err).Msg("list statuses failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		statuses, err := instances.ListStatuses(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-list").Err(err).Msg("list statuses failed")
			writeInstanceError(w, r, err)
			return
		}
		response := statusListResponse{Items: make([]ownStatusResponse, 0, len(statuses))}
		for _, info := range statuses {
			response.Items = append(response.Items, newStatusResponse(info))
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// handleDeleteStatus removes the own status and answers 200. It loads the
// target first (404) and authorizes (403) before touching the session; an
// unknown status answers 404 and an offline session 409.
//
// @Summary Delete an own status
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param status_id path string true "Status ID"
// @Success 200 {object} envelope{data=statusDeleteResponse} "Deleted, only statuses published since boot are tracked and the ~24h protocol expiry applies, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or status not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates/{status_id} [delete]
func handleDeleteStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-delete").Msg("delete status request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-delete").Err(err).Msg("delete status failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		statusID := strings.TrimSpace(r.PathValue("status_id"))
		if statusID == "" {
			Error(w, r, http.StatusNotFound, "not_found", "status not found")
			return
		}

		if err := instances.DeleteStatus(r.Context(), id, statusID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-delete").Err(err).Msg("delete status failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-delete").Bool("deleted", true).Msg("delete status result")
		JSON(w, r, http.StatusOK, statusDeleteResponse{Deleted: true})
	}
}

// newStatusResponse maps a published status to its JSON representation.
func newStatusResponse(info session.StatusInfo) ownStatusResponse {
	return ownStatusResponse{
		ID:        info.ID,
		Type:      info.Kind,
		Text:      info.Text,
		Caption:   info.Caption,
		CreatedAt: info.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
