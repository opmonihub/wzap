package media

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/media"
	"wzap/internal/model"
)

// MediaStore is the media storage contract consumed by the handlers: Open
// serves a download and Save stores an upload. *media.Storage implements it.
type MediaStore interface {
	Open(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.Media, error)
	Save(
		ctx context.Context, instanceID uuid.UUID, direction, messageID, mimetype, filename string, data []byte,
	) (*model.Media, error)
}

// HandleGetMedia streams one media content. The success body is the raw
// content, not the JSON envelope used by the other endpoints. The media loads
// first (missing or expired → 404) and the owning instance authorizes next
// (403) under the instance-scoped rule.
//
// @Summary Download media
// @Tags media
// @Produce octet-stream,json
// @Security apikey
// @Param id path string true "Media ID (UUID)"
// @Success 200 {file} binary "Raw media bytes with the stored media Content-Type, outside the core.JSON core.Envelope"
// @Header 200 {string} Content-Type "Stored media MIME type"
// @Header 200 {integer} Content-Length "Stored media size in bytes"
// @Header 200 {string} Content-Disposition "Attachment filename"
// @Header 200 {string} X-Content-Type-Options "nosniff"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Media not found or expired"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /media/{id} [get]
func HandleGetMedia(instances InstanceService, store MediaStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(core.PathParam(r, "id"))
		if err != nil {
			core.Error(w, r, http.StatusNotFound, "not_found", "media not found")
			return
		}

		body, record, err := store.Open(r.Context(), id)
		if err != nil {
			WriteMediaError(w, r, err)
			return
		}
		defer func() { _ = body.Close() }()

		if core.DenyForeignInstanceKey(w, r, record.InstanceID) {
			return
		}
		owner, err := instances.Get(r.Context(), record.InstanceID)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, owner); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		w.Header().Set("Content-Type", record.Mimetype)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", MediaContentDisposition(record))
		w.Header().Set("Content-Length", strconv.FormatInt(record.SizeBytes, 10))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, body)
	}
}

// MediaContentDisposition builds an attachment disposition with a sanitized
// filename: the name is sender controlled, so it must never be trusted as a
// path or as a header value. An empty name falls back to the media id.
func MediaContentDisposition(record *model.Media) string {
	name := SanitizeMediaFilename(record.Filename)
	if name == "" {
		name = record.ID.String()
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if disposition == "" {
		return "attachment"
	}
	return disposition
}

// SanitizeMediaFilename reduces a sender supplied filename to its base name
// without control characters, so it cannot traverse paths or inject headers.
func SanitizeMediaFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	switch name {
	case "", ".", "..", "/":
		return ""
	}
	return name
}

// WriteMediaError maps a media storage error to its HTTP status and error
// envelope. Unknown and expired media are indistinguishable to the client.
// Scope denials answer 403.
func WriteMediaError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		core.Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, media.ErrNotFound), errors.Is(err, media.ErrExpired):
		core.Error(w, r, http.StatusNotFound, "not_found", "media not found")
	default:
		core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
