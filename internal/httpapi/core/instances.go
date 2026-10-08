package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/instance"
	"wzap/internal/session"
)

const (
	// maxJSONBodyBytes caps the JSON request bodies every handler decodes.
	MaxJSONBodyBytes = 1 << 20
)

// InstanceID parses the canonical UUID path value supplied by the resolver.
// A malformed value answers 404 for handlers used outside the registered mux.
func InstanceID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if target, ok := TargetInstance(r); ok {
		return target.ID, true
	}
	id, err := uuid.Parse(PathParam(r, "id"))
	if err != nil {
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
		return uuid.Nil, false
	}
	return id, true
}

// ParseLimit reads a limit query parameter, falling back to fallback when it is
// missing or malformed and capping the page size at maxLimit.
func ParseLimit(raw string, fallback, maxLimit int) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	return min(limit, maxLimit)
}

// DecodeJSONBody decodes the request body into target, refusing bodies above
// maxJSONBodyBytes before they are buffered.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("request body contains more than one JSON value")
	}
	return nil
}

// WriteJSONBodyError maps a body decoding failure to its HTTP status: an
// oversized body answers 413 and anything else a malformed 400.
func WriteJSONBodyError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large",
			"request body exceeds the 1 MiB limit")
		return
	}
	Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
}

// WriteInstanceError maps a service error to its HTTP status and error
// envelope. Scope denials answer 403, unknown owner overrides and invalid
// webhook configurations answer 422, a missing oldest admin answers 500 with
// a clear server-state message; unknown failures answer 500 without leaking
// their cause.
func WriteInstanceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, instance.ErrNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	case errors.Is(err, instance.ErrInvalidInstanceName):
		Error(w, r, http.StatusUnprocessableEntity, "invalid_instance_name", "invalid instance name")
	case errors.Is(err, instance.ErrInstanceNameTaken):
		Error(w, r, http.StatusConflict, "instance_name_taken", "instance name already taken")
	case errors.Is(err, instance.ErrExternalRefTaken):
		Error(w, r, http.StatusConflict, "conflict", "external ref already taken")
	case errors.Is(err, instance.ErrInvalidCursor):
		Error(w, r, http.StatusBadRequest, "invalid_request", "invalid cursor")
	case errors.Is(err, instance.ErrAlreadyConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance already connected")
	case errors.Is(err, instance.ErrNotConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance not connected")

	case errors.Is(err, session.ErrNotConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance not connected")
	case errors.Is(err, instance.ErrNoPairingChannel):
		Error(w, r, http.StatusConflict, "conflict", "no open pairing channel")
	case errors.Is(err, instance.ErrInvalidInput):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid operation input")
	case errors.Is(err, instance.ErrGroupNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "group not found")

	case errors.Is(err, instance.ErrContactNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "contact not found")
	case errors.Is(err, instance.ErrNewsletterNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "newsletter not found")
	case errors.Is(err, instance.ErrStatusNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "status not found")
	case errors.Is(err, instance.ErrUnsupported):
		Error(w, r, http.StatusNotImplemented, "not_supported", "operation not supported by the upstream")
	case errors.Is(err, instance.ErrForbidden):
		Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, instance.ErrOwnerNotFound):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "unknown owner")
	case errors.Is(err, instance.ErrInvalidWebhook):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid webhook config")
	case errors.Is(err, instance.ErrNoAdmin):
		Error(w, r, http.StatusInternalServerError, "internal_error", "no admin user exists")
	default:
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
