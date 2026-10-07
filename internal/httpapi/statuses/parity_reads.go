package statuses

import (
	"net/http"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

// HandleGetStatusPrivacy answers the own status audience. A disconnected
// session answers 409.
//
// @Summary Get the status privacy
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=representation.StatusPrivacyResponse} "Audience, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/privacy [get]
func HandleGetStatusPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "status-privacy-get")
		if !ok {
			return
		}

		privacy, err := instances.GetStatusPrivacy(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-privacy-get").Err(err).Msg("get status privacy failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.NewStatusPrivacyResponse(privacy))
	}
}
