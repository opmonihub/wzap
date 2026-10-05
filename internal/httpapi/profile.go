package httpapi

import (
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/session"
)

const (
	// maxProfileNameLen caps the display name in characters (Ruling D3:
	// 1..100). Longer names are rejected with 422 before the session.
	maxProfileNameLen = 100
	// maxProfileStatusLen caps the recado (status text) in characters
	// (Ruling D3: 0..500, empty clears it).
	maxProfileStatusLen = 500
	// defaultProfilePhotoBytes caps the profile photo upload when no media
	// cap is configured.
	defaultProfilePhotoBytes = 5 << 20
)

// profileResponse is the JSON representation of the own profile: the display
// name, the recado and the photo URL (empty when the instance carries no
// photo).
type profileResponse struct {
	Name       string `json:"name"`
	StatusText string `json:"status_text"`
	PhotoURL   string `json:"photo_url"`
}

// updateProfileRequest is the PATCH /instances/{id}/profile payload: nil
// fields keep their upstream value, an explicit empty recado clears it while
// an empty name is rejected (names carry 1..100 characters).
type updateProfileRequest struct {
	Name       *string `json:"name"`
	StatusText *string `json:"status_text"`
}

// profilePhotoResponse is the answer to a replaced profile photo.
type profilePhotoResponse struct {
	Updated bool `json:"updated"`
}

// privacyResponse is the JSON representation of the own privacy settings, one
// value per field. Last seen, profile photo, status and groups add take all,
// contacts, contact_blacklist or none; read receipts take all or none (the
// upstream models receipts as a two-value switch, so the API exposes the
// allowlist instead of a boolean — Ruling D3).
type privacyResponse struct {
	LastSeen     string `json:"last_seen"`
	ProfilePhoto string `json:"profile_photo"`
	Status       string `json:"status"`
	ReadReceipts string `json:"read_receipts"`
	GroupsAdd    string `json:"groups_add"`
}

// updatePrivacyRequest is the PUT /instances/{id}/privacy payload: nil
// fields keep their upstream value; at least one field must be present.
type updatePrivacyRequest struct {
	LastSeen     *string `json:"last_seen"`
	ProfilePhoto *string `json:"profile_photo"`
	Status       *string `json:"status"`
	ReadReceipts *string `json:"read_receipts"`
	GroupsAdd    *string `json:"groups_add"`
}

// newProfileResponse maps a stored profile to its JSON representation.
func newProfileResponse(profile session.Profile) profileResponse {
	return profileResponse{
		Name:       profile.Name,
		StatusText: profile.StatusText,
		PhotoURL:   profile.PhotoURL,
	}
}

// newPrivacyResponse maps stored privacy settings to their JSON
// representation.
func newPrivacyResponse(privacy session.Privacy) privacyResponse {
	return privacyResponse{
		LastSeen:     privacy.LastSeen,
		ProfilePhoto: privacy.ProfilePhoto,
		Status:       privacy.Status,
		ReadReceipts: privacy.ReadReceipts,
		GroupsAdd:    privacy.GroupsAdd,
	}
}

// validProfileName reports whether name fits the display name limit of 1..100
// characters.
func validProfileName(name string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	return n >= 1 && n <= maxProfileNameLen
}

// validProfileStatus reports whether text fits the recado limit of 0..500
// characters (empty clears it).
func validProfileStatus(text string) bool {
	return utf8.RuneCountInString(text) <= maxProfileStatusLen
}

// validPrivacyValue reports whether value is one of the upstream literals for
// the four-valued privacy fields: last seen, profile photo, status and who
// may add the instance to groups.
func validPrivacyValue(value string) bool {
	switch value {
	case "all", "contacts", "contact_blacklist", "none":
		return true
	}
	return false
}

// validReadReceiptsValue reports whether value is one of the upstream
// literals for read receipts: all or none.
func validReadReceiptsValue(value string) bool {
	return value == "all" || value == "none"
}

// profileTarget loads the instance (404) and authorizes (403) for the profile
// and privacy handlers, returning the instance id.
func profileTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := instanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if denyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("profile request")

	stored, err := instances.Get(r.Context(), id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("profile request failed")
		writeInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := authorizeInstance(r, stored); err != nil {
		writeForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}

// handleGetProfile answers the own profile of the instance. An offline
// session answers 409.
//
// @Summary Get the own profile
// @Tags profile
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=profileResponse} "Own profile, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile [get]
func handleGetProfile(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := profileTarget(w, r, instances, log, "profile-get")
		if !ok {
			return
		}

		profile, err := instances.GetProfile(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-get").Err(err).Msg("get profile failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newProfileResponse(profile))
	}
}

// handleUpdateProfile applies the name/recado patch and answers the refreshed
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
// @Param request body updateProfileRequest true "Profile patch"
// @Success 200 {object} envelope{data=profileResponse} "Refreshed profile, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Empty patch or values outside the allowlists"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 501 {object} errorEnvelope "Upstream cannot apply the name change; when name is present nothing is applied, including status_text"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile [patch]
func handleUpdateProfile(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := profileTarget(w, r, instances, log, "profile-update")
		if !ok {
			return
		}

		var request updateProfileRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.Name == nil && request.StatusText == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name or status_text is required")
			return
		}
		var name, statusText *string
		if request.Name != nil {
			trimmed := strings.TrimSpace(*request.Name)
			if !validProfileName(trimmed) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name must be 1..100 characters")
				return
			}
			name = &trimmed
		}
		if request.StatusText != nil {
			if !validProfileStatus(*request.StatusText) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "status_text must be max 500 characters")
				return
			}
			statusText = request.StatusText
		}

		if name != nil {
			if err := instances.SetProfileName(r.Context(), id, *name); err != nil {
				log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
				writeInstanceError(w, r, err)
				return
			}
		}
		if statusText != nil {
			if err := instances.SetProfileStatusText(r.Context(), id, *statusText); err != nil {
				log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
				writeInstanceError(w, r, err)
				return
			}
		}

		profile, err := instances.GetProfile(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-update").Err(err).Msg("update profile failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newProfileResponse(profile))
	}
}

// handleSetProfilePhoto replaces the own photo with the raw image bytes of
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
// @Success 200 {object} envelope{data=profilePhotoResponse} "Updated, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Image exceeds the cap"
// @Failure 422 {object} errorEnvelope "Missing or non image body"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 501 {object} errorEnvelope "Upstream cannot apply the photo"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/profile/photo [put]
func handleSetProfilePhoto(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = defaultProfilePhotoBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := profileTarget(w, r, instances, log, "profile-photo")
		if !ok {
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "content type must be an image")
			return
		}
		image, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		if len(image) == 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "image body is required")
			return
		}
		if int64(len(image)) > maxBytes {
			Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "image exceeds the size cap")
			return
		}

		if err := instances.SetProfilePhoto(r.Context(), id, image); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "profile-photo").Err(err).Msg("set profile photo failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, profilePhotoResponse{Updated: true})
	}
}

// handleGetPrivacy answers the own privacy settings, one value per field. An
// offline session answers 409.
//
// @Summary Get the own privacy settings
// @Tags privacy
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=privacyResponse} "Own privacy, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/privacy [get]
func handleGetPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := profileTarget(w, r, instances, log, "privacy-get")
		if !ok {
			return
		}

		privacy, err := instances.GetPrivacy(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "privacy-get").Err(err).Msg("get privacy failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newPrivacyResponse(privacy))
	}
}

// handleSetPrivacy applies the privacy patch and answers the resulting
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
// @Param request body updatePrivacyRequest true "Privacy patch"
// @Success 200 {object} envelope{data=privacyResponse} "Applied settings, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Empty patch or values outside the allowlists"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/privacy [put]
func handleSetPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := profileTarget(w, r, instances, log, "privacy-set")
		if !ok {
			return
		}

		var request updatePrivacyRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.LastSeen == nil && request.ProfilePhoto == nil && request.Status == nil &&
			request.ReadReceipts == nil && request.GroupsAdd == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "at least one privacy field is required")
			return
		}
		input := session.Privacy{}
		if request.LastSeen != nil {
			value := strings.TrimSpace(*request.LastSeen)
			if !validPrivacyValue(value) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"last_seen must be all, contacts, contact_blacklist or none")
				return
			}
			input.LastSeen = value
		}
		if request.ProfilePhoto != nil {
			value := strings.TrimSpace(*request.ProfilePhoto)
			if !validPrivacyValue(value) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"profile_photo must be all, contacts, contact_blacklist or none")
				return
			}
			input.ProfilePhoto = value
		}
		if request.Status != nil {
			value := strings.TrimSpace(*request.Status)
			if !validPrivacyValue(value) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"status must be all, contacts, contact_blacklist or none")
				return
			}
			input.Status = value
		}
		if request.ReadReceipts != nil {
			value := strings.TrimSpace(*request.ReadReceipts)
			if !validReadReceiptsValue(value) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"read_receipts must be all or none")
				return
			}
			input.ReadReceipts = value
		}
		if request.GroupsAdd != nil {
			value := strings.TrimSpace(*request.GroupsAdd)
			if !validPrivacyValue(value) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity",
					"groups_add must be all, contacts, contact_blacklist or none")
				return
			}
			input.GroupsAdd = value
		}

		privacy, err := instances.SetPrivacy(r.Context(), id, input)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "privacy-set").Err(err).Msg("set privacy failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newPrivacyResponse(privacy))
	}
}
