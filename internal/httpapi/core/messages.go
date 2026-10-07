package core

import (
	"errors"
	"io"
	"net/http"

	"wzap/internal/media"
)

const (
	// defaultMessagesLimit is the page size used when the request omits limit.
	DefaultMessagesLimit = 50
	// maxMessagesLimit caps the page size a client can request.
	MaxMessagesLimit = 100
	// mediaDirectionOutbound labels media uploaded for a send.
	MediaDirectionOutbound = "outbound"
	// mediaFormMemory is how much of an upload ParseMultipartForm keeps in
	// memory; larger file parts spill to temporary files.
	MediaFormMemory = 1 << 20
	// mediaFormOverhead is the slack above the configured media limit that
	// covers the multipart boundaries, part headers and text fields.
	MediaFormOverhead = 1 << 20
)

// ReadMediaUpload reads the file part, refusing content above the configured
// limit before it is buffered. A non-positive limit defers to the store.
func ReadMediaUpload(file io.Reader, maxBytes int64) ([]byte, error) {
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

// WriteMediaUploadError maps a media store failure during an upload: empty and
// oversized files are unprocessable and an unknown instance surfaces as
// ErrNotFound from the media foreign key.
func WriteMediaUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, media.ErrEmpty), errors.Is(err, media.ErrTooLarge):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid media file")
	case errors.Is(err, media.ErrNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	default:
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
