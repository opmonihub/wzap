package representation

import (
	"wzap/internal/session"
)

// ProfileResponse is the JSON representation of the own profile: the display
// name, the recado and the photo URL (empty when the instance carries no
// photo).
type ProfileResponse struct {
	Name       string `json:"name" binding:"required"`
	StatusText string `json:"status_text" binding:"required"`
	PhotoURL   string `json:"photo_url,omitempty"`
}

// PrivacyResponse is the JSON representation of the own privacy settings, one
// value per field. Last seen, profile photo, status and groups add take all,
// contacts, contact_blacklist or none; read receipts take all or none (the
// upstream models receipts as a two-value switch, so the API exposes the
// allowlist instead of a boolean — Ruling D3).
type PrivacyResponse struct {
	LastSeen     string `json:"last_seen" binding:"required"`
	ProfilePhoto string `json:"profile_photo" binding:"required"`
	Status       string `json:"status" binding:"required"`
	ReadReceipts string `json:"read_receipts" binding:"required"`
	GroupsAdd    string `json:"groups_add" binding:"required"`
}

// NewProfileResponse maps a stored profile to its JSON representation.
func NewProfileResponse(profile session.Profile) ProfileResponse {
	return ProfileResponse{
		Name:       profile.Name,
		StatusText: profile.StatusText,
		PhotoURL:   profile.PhotoURL,
	}
}

// NewPrivacyResponse maps stored privacy settings to their JSON
// representation.
func NewPrivacyResponse(privacy session.Privacy) PrivacyResponse {
	return PrivacyResponse{
		LastSeen:     privacy.LastSeen,
		ProfilePhoto: privacy.ProfilePhoto,
		Status:       privacy.Status,
		ReadReceipts: privacy.ReadReceipts,
		GroupsAdd:    privacy.GroupsAdd,
	}
}
