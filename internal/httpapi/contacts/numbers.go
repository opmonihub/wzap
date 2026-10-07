package contacts

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"wzap/internal/httpapi/core"
	"wzap/internal/message"
)

// NumberResolver is the number resolution contract consumed by the handlers.
type NumberResolver interface {
	Resolve(ctx context.Context, instanceID uuid.UUID, phone string) (jid string, err error)
}

// The resolver satisfies the handler contract; the assertion catches signature
// drift at build time.
var _ NumberResolver = (*message.JIDResolver)(nil)

// NumberCheckRequest is the POST /instances/{id}/numbers/check payload.
type NumberCheckRequest struct {
	Phone string `json:"phone"`
}

// NumberCheckResponse is the JSON result of a number check. A phone that
// already contains '@' counts as resolved because the JID is returned as-is.
type NumberCheckResponse struct {
	Exists     bool   `json:"exists" binding:"required"`
	JID        string `json:"jid" binding:"required"`
	Normalized string `json:"normalized" binding:"required"`
}

// HandleCheckNumber answers 200 with the resolution of a phone number: exists
// is false and jid empty when the number is malformed or absent from WhatsApp.
// An instance without a usable session answers 503 so a caller can retry
// instead of taking a negative answer that was never verified.
//
// @Summary Check a phone number
// @Tags numbers
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body NumberCheckRequest true "Phone payload"
// @Success 200 {object} core.Envelope{data=NumberCheckResponse} "Resolution, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body or missing phone"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 503 {object} core.ErrorEnvelope "Number resolution unavailable"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/numbers/check [post]
func HandleCheckNumber(instances InstanceService, numbers NumberResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		var request NumberCheckRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		phone := strings.TrimSpace(request.Phone)
		if phone == "" {
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "phone is required")
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

		normalized := message.NormalizePhone(phone)
		jid, err := numbers.Resolve(r.Context(), id, phone)
		switch {
		case err == nil:
			core.JSON(w, r, http.StatusOK, NumberCheckResponse{Exists: true, JID: jid, Normalized: normalized})
		case errors.Is(err, message.ErrNumberNotFound):
			core.JSON(w, r, http.StatusOK, NumberCheckResponse{Exists: false, Normalized: normalized})
		case errors.Is(err, message.ErrResolverUnavailable):
			core.Error(w, r, http.StatusServiceUnavailable, "unavailable", "number resolution unavailable")
		default:
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		}
	}
}
