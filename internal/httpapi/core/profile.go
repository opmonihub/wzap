package core

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ProfileTarget loads the instance (404) and authorizes (403) for the profile
// and privacy handlers, returning the instance id.
func ProfileTarget(w http.ResponseWriter, r *http.Request, instances InstanceReader, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := InstanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if DenyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("profile request")

	stored, err := LoadInstance(r, instances, id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("profile request failed")
		WriteInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := AuthorizeInstance(r, stored); err != nil {
		WriteForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}
