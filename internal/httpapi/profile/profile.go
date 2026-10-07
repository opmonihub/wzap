package profile

import (
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/session"
)

const (
	// maxProfileNameLen caps the display name in characters (Ruling D3:
	// 1..100). Longer names are rejected with 422 before the session.
	MaxProfileNameLen = 100
	// maxProfileStatusLen caps the recado (status text) in characters
	// (Ruling D3: 0..500, empty clears it).
	MaxProfileStatusLen = 500
	// defaultProfilePhotoBytes caps the profile photo upload when no media
	// cap is configured.
	DefaultProfilePhotoBytes = 5 << 20
)

// UpdateProfileRequest is the PATCH /instances/{id}/profile payload: nil
// fields keep their upstream value, an explicit empty recado clears it while
// an empty name is rejected (names carry 1..100 characters).
type UpdateProfileRequest struct {
	Name       *string `json:"name"`
	StatusText *string `json:"status_text"`
}

// ProfilePhotoResponse is the answer to a replaced profile photo.
type ProfilePhotoResponse struct {
	Updated bool `json:"updated" binding:"required"`
}

// UpdatePrivacyRequest is the PUT /instances/{id}/privacy payload: nil
// fields keep their upstream value; at least one field must be present.
type UpdatePrivacyRequest struct {
	LastSeen     *string `json:"last_seen"`
	ProfilePhoto *string `json:"profile_photo"`
	Status       *string `json:"status"`
	ReadReceipts *string `json:"read_receipts"`
	GroupsAdd    *string `json:"groups_add"`
}

// ValidProfileName reports whether name fits the display name limit of 1..100
// characters.
func ValidProfileName(name string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	return n >= 1 && n <= MaxProfileNameLen
}

// ValidProfileStatus reports whether text fits the recado limit of 0..500
// characters (empty clears it).
func ValidProfileStatus(text string) bool {
	return utf8.RuneCountInString(text) <= MaxProfileStatusLen
}

// ValidPrivacyValue reports whether value is one of the upstream literals for
// the four-valued privacy fields: last seen, profile photo, status and who
// may add the instance to groups.
func ValidPrivacyValue(value string) bool {
	switch value {
	case "all", "contacts", "contact_blacklist", "none":
		return true
	}
	return false
}

// ValidReadReceiptsValue reports whether value is one of the upstream
// literals for read receipts: all or none.
func ValidReadReceiptsValue(value string) bool {
	return value == "all" || value == "none"
}

// HandleGetProfile answers the own profile of the instance. An offline
// session answers 409.
//
// @Summary Get the own profile
// @Tags profile
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=representation.ProfileResponse} "Own profile, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile [get]
func HandleGetProfile(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ProfileTarget(w, r, instances, log, "profile-get")
		if !ok {
			return
		}

		profile, err := instances.GetProfile(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-get").Err(err).Msg("get profile failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.NewProfileResponse(profile))
	}
}

// HandleUpdateProfile applies the name/recado patch and answers the refreshed
// profile. At least one field must be present; values outside the allowlists
// answer 422 before the session. A name change the upstream cannot apply
// answers 501 with the stable not_supported code (the pinned companion
// library exposes no profile name setter).
//
// @Summary Update the own profile
// @Tags profile
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body UpdateProfileRequest true "Profile patch"
// @Success 200 {object} core.Envelope{data=representation.ProfileResponse} "Refreshed profile, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Empty patch or values outside the allowlists"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 501 {object} core.ErrorEnvelope "Upstream cannot apply the name change; when name is present nothing is applied, including status_text"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile [patch]
func HandleUpdateProfile(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ProfileTarget(w, r, instances, log, "profile-update")
		if !ok {
			return
		}

		var request UpdateProfileRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if request.Name == nil && request.StatusText == nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name or status_text is required")
			return
		}
		var name, statusText *string
		if request.Name != nil {
			trimmed := strings.TrimSpace(*request.Name)
			if !ValidProfileName(trimmed) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name must be 1..100 characters")
				return
			}
			name = &trimmed
		}
		if request.StatusText != nil {
			if !ValidProfileStatus(*request.StatusText) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "status_text must be max 500 characters")
				return
			}
			statusText = request.StatusText
		}

		if name != nil {
			if err := instances.SetProfileName(r.Context(), id, *name); err != nil {
				log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
				core.WriteInstanceError(w, r, err)
				return
			}
		}
		if statusText != nil {
			if err := instances.SetProfileStatusText(r.Context(), id, *statusText); err != nil {
				log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
				core.WriteInstanceError(w, r, err)
				return
			}
		}

		profile, err := instances.GetProfile(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.NewProfileResponse(profile))
	}
}

// HandleSetProfilePhoto replaces the own photo with the raw image bytes of
// the request body (jpeg, png or webp). The pinned companion library exposes
// no photo setter, so an upstream that cannot apply it answers 501 with the
// stable not_supported code.
//
// @Summary Update the own profile photo
// @Tags profile
// @Accept image/*
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param image body string true "Raw image bytes with an image/* Content-Type (for example JPEG, PNG or WebP)"
// @Success 200 {object} core.Envelope{data=ProfilePhotoResponse} "Updated, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Image exceeds the cap"
// @Failure 422 {object} core.ErrorEnvelope "Missing or non image body"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 501 {object} core.ErrorEnvelope "Upstream cannot apply the photo"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile/photo [put]
func HandleSetProfilePhoto(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultProfilePhotoBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ProfileTarget(w, r, instances, log, "profile-photo")
		if !ok {
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "content type must be an image")
			return
		}
		image, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		if len(image) == 0 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "image body is required")
			return
		}
		if int64(len(image)) > maxBytes {
			core.Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "image exceeds the size cap")
			return
		}

		if err := instances.SetProfilePhoto(r.Context(), id, image); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-photo").Err(err).Msg("set profile photo failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, ProfilePhotoResponse{Updated: true})
	}
}

// HandleGetPrivacy answers the own privacy settings, one value per field. An
// offline session answers 409.
//
// @Summary Get the own privacy settings
// @Tags privacy
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=representation.PrivacyResponse} "Own privacy, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/privacy [get]
func HandleGetPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ProfileTarget(w, r, instances, log, "privacy-get")
		if !ok {
			return
		}

		privacy, err := instances.GetPrivacy(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "privacy-get").Err(err).Msg("get privacy failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.NewPrivacyResponse(privacy))
	}
}

// HandleSetPrivacy applies the privacy patch and answers the resulting
// settings. At least one field must be present; every value is validated
// against its allowlist before the session (last seen, profile photo, status
// and groups add take all, contacts, contact_blacklist or none; read receipts
// take all or none).
//
// @Summary Update the own privacy settings
// @Description All privacy operations are supported upstream (no 501). Fields apply sequentially, so a later failure can leave earlier fields applied (non-atomic).
// @Tags privacy
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body UpdatePrivacyRequest true "Privacy patch"
// @Success 200 {object} core.Envelope{data=representation.PrivacyResponse} "Applied settings, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Empty patch or values outside the allowlists"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/privacy [put]
func HandleSetPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ProfileTarget(w, r, instances, log, "privacy-set")
		if !ok {
			return
		}

		var request UpdatePrivacyRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if request.LastSeen == nil && request.ProfilePhoto == nil && request.Status == nil &&
			request.ReadReceipts == nil && request.GroupsAdd == nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "at least one privacy field is required")
			return
		}
		input := session.Privacy{}
		if request.LastSeen != nil {
			value := strings.TrimSpace(*request.LastSeen)
			if !ValidPrivacyValue(value) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"last_seen must be all, contacts, contact_blacklist or none")
				return
			}
			input.LastSeen = value
		}
		if request.ProfilePhoto != nil {
			value := strings.TrimSpace(*request.ProfilePhoto)
			if !ValidPrivacyValue(value) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"profile_photo must be all, contacts, contact_blacklist or none")
				return
			}
			input.ProfilePhoto = value
		}
		if request.Status != nil {
			value := strings.TrimSpace(*request.Status)
			if !ValidPrivacyValue(value) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"status must be all, contacts, contact_blacklist or none")
				return
			}
			input.Status = value
		}
		if request.ReadReceipts != nil {
			value := strings.TrimSpace(*request.ReadReceipts)
			if !ValidReadReceiptsValue(value) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"read_receipts must be all or none")
				return
			}
			input.ReadReceipts = value
		}
		if request.GroupsAdd != nil {
			value := strings.TrimSpace(*request.GroupsAdd)
			if !ValidPrivacyValue(value) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"groups_add must be all, contacts, contact_blacklist or none")
				return
			}
			input.GroupsAdd = value
		}

		privacy, err := instances.SetPrivacy(r.Context(), id, input)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "privacy-set").Err(err).Msg("set privacy failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.NewPrivacyResponse(privacy))
	}
}
