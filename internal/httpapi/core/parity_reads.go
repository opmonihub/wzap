package core

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	// defaultParityLimit is the page size the collection reads use when the
	// request omits limit, matching the other collection listings.
	DefaultParityLimit = 50
	// maxParityLimit caps the page size a client can request on the collection
	// reads, matching the other collection listings.
	MaxParityLimit = 100
	// maxCheckContactsBatch caps the directory lookup batch: larger batches
	// answer 422 before the session is touched.
	MaxCheckContactsBatch = 50
)

// ParityTarget loads the instance (404) and authorizes (403) for the collection
// parity handlers, returning the instance id.
func ParityTarget(w http.ResponseWriter, r *http.Request, instances InstanceReader, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := InstanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if DenyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("parity request")

	stored, err := LoadInstance(r, instances, id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("parity request failed")
		WriteInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := AuthorizeInstance(r, stored); err != nil {
		WriteForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}

// ParityContactJID reads the {jid} path value. A blank value answers 422: it
// is a malformed address, while an unknown contact surfaces as 404 via
// ErrContactNotFound from the service.
func ParityContactJID(w http.ResponseWriter, r *http.Request) (string, bool) {
	jid := strings.TrimSpace(PathParam(r, "jid"))
	if jid == "" {
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "jid is required")
		return "", false
	}
	return jid, true
}

// ParityChannel reads the {channel} path value. A blank value answers 422
// like the service ErrInvalidInput for a blank channel; an unknown channel
// surfaces as 404 via ErrNewsletterNotFound.
func ParityChannel(w http.ResponseWriter, r *http.Request) (string, bool) {
	channel := strings.TrimSpace(PathParam(r, "channel"))
	if channel == "" {
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "channel is required")
		return "", false
	}
	return channel, true
}
