package statuses

import (
	"errors"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/media"
	"wzap/internal/session"
)

const (
	// maxStatusTextLen caps a text status and a media caption in characters:
	// longer content is rejected with 422 before the session is touched.
	MaxStatusTextLen = 700
	// defaultStatusMediaBytes caps a status image/video upload when no media
	// cap is configured.
	DefaultStatusMediaBytes = 5 << 20
)

// PublishStatusRequest is the POST /instances/{id}/status/updates payload: text
// statuses only. Image and video statuses ride POST
// /instances/{id}/status/updates/media with the file bytes.
type PublishStatusRequest struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// StatusPublishResponse is the 202 answer to a published status: the upstream
// id with the same idempotency semantics as the message sends (a repeated
// Idempotency-Key replays it with X-Idempotent-Replay).
type StatusPublishResponse struct {
	MessageID string `json:"message_id" binding:"required"`
	Status    string `json:"status" binding:"required"`
}

// OwnStatusResponse is one own status of the GET /instances/{id}/status/updates listing.
type OwnStatusResponse struct {
	ID        string `json:"id" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Text      string `json:"text,omitempty"`
	Caption   string `json:"caption,omitempty"`
	CreatedAt string `json:"created_at" binding:"required"`
}

// StatusListResponse is the 200 answer to the own status listing. Items is
// never nil so the field keeps its array shape on an empty listing.
type StatusListResponse struct {
	Statuses []OwnStatusResponse `json:"statuses" binding:"required"`
}

// StatusDeleteResponse is the 200 answer to a removed own status.
type StatusDeleteResponse struct {
	Deleted bool `json:"deleted" binding:"required"`
}

// ValidStatusKind reports whether kind is one of the status media kinds the
// multipart publish accepts.
func ValidStatusKind(kind string) bool {
	return kind == session.StatusKindImage || kind == session.StatusKindVideo
}

// StatusTextValid reports whether text fits a text status or a media caption:
// 1..maxStatusTextLen characters.
func StatusTextValid(text string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	return n >= 1 && n <= MaxStatusTextLen
}

// HandlePublishStatus publishes a text status and answers 202 with its
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
// @Param request body PublishStatusRequest true "Status payload: type text plus its text"
// @Header 202 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 202 {object} core.Envelope{data=StatusPublishResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid type or text, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates [post]
func HandlePublishStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish").Msg("publish status request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish").Err(err).Msg("publish status failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request PublishStatusRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if strings.TrimSpace(request.Type) != session.StatusKindText {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type must be text (image and video ride /status/updates/media)")
			return
		}
		text := strings.TrimSpace(request.Text)
		if !StatusTextValid(text) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "text is required, max 700 characters")
			return
		}

		statusID, err := instances.PublishStatus(r.Context(), id, session.StatusInput{Text: text})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish").Err(err).Msg("publish status failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish").Str("status_id", statusID).Msg("publish status result")
		core.JSON(w, r, http.StatusAccepted, StatusPublishResponse{MessageID: statusID, Status: "published"})
	}
}

// HandlePublishStatusMedia publishes an image or video status from a
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
// @Success 202 {object} core.Envelope{data=StatusPublishResponse} "Accepted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Invalid multipart body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid file, mismatched type, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates/media [post]
func HandlePublishStatusMedia(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultStatusMediaBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish-media").Msg("publish status media request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish-media").Err(err).Msg("publish status media failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
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

		kind := strings.TrimSpace(r.PostFormValue("type"))
		if !ValidStatusKind(kind) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "type must be image or video")
			return
		}
		caption := strings.TrimSpace(r.PostFormValue("caption"))
		if caption != "" && !StatusTextValid(caption) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "caption must be max 700 characters")
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
		fileKind, ok := media.Kind(mimetype)
		if !ok || fileKind != kind {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
				"type does not match the file content type")
			return
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

		statusID, err := instances.PublishStatus(r.Context(), id, session.StatusInput{
			MediaMime: mimetype,
			MediaData: data,
			Caption:   caption,
		})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-publish-media").Err(err).Msg("publish status media failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-publish-media").Str("status_id", statusID).Msg("publish status media result")
		core.JSON(w, r, http.StatusAccepted, StatusPublishResponse{MessageID: statusID, Status: "published"})
	}
}

// HandleListStatuses answers the own statuses published through the instance
// session. Entries older than 24h are dropped by the session registry. It
// loads the target first (404) and authorizes (403) before
// touching the session; an offline session answers 409.
//
// @Summary List own statuses
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=StatusListResponse} "Own statuses published since boot, entries older than 24h are dropped, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates [get]
func HandleListStatuses(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-list").Msg("list statuses request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-list").Err(err).Msg("list statuses failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		statuses, err := instances.ListStatuses(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-list").Err(err).Msg("list statuses failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := StatusListResponse{Statuses: make([]OwnStatusResponse, 0, len(statuses))}
		for _, info := range statuses {
			response.Statuses = append(response.Statuses, NewStatusResponse(info))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// HandleDeleteStatus removes the own status and answers 200. It loads the
// target first (404) and authorizes (403) before touching the session; an
// unknown status answers 404 and an offline session 409.
//
// @Summary Delete an own status
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param status_id path string true "Status ID"
// @Success 200 {object} core.Envelope{data=StatusDeleteResponse} "Deleted, only statuses published since boot are tracked and the ~24h protocol expiry applies, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or status not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/updates/{status_id} [delete]
func HandleDeleteStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-delete").Msg("delete status request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-delete").Err(err).Msg("delete status failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		statusID := strings.TrimSpace(core.PathParam(r, "status_id"))
		if statusID == "" {
			core.Error(w, r, http.StatusNotFound, "not_found", "status not found")
			return
		}

		if err := instances.DeleteStatus(r.Context(), id, statusID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-delete").Err(err).Msg("delete status failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status-delete").Bool("deleted", true).Msg("delete status result")
		core.JSON(w, r, http.StatusOK, StatusDeleteResponse{Deleted: true})
	}
}

// NewStatusResponse maps a published status to its JSON representation.
func NewStatusResponse(info session.StatusInfo) OwnStatusResponse {
	return OwnStatusResponse{
		ID:        info.ID,
		Type:      info.Kind,
		Text:      info.Text,
		Caption:   info.Caption,
		CreatedAt: info.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
