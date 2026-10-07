package representation

import (
	"wzap/internal/session"
)

// StatusPrivacyResponse answers the own status audience.
type StatusPrivacyResponse struct {
	Mode string   `json:"mode" binding:"required"`
	JIDs []string `json:"jids" binding:"required"`
}

// NewStatusPrivacyResponse maps the own status audience to its JSON
// representation, normalizing a nil JIDs slice to an empty one.
func NewStatusPrivacyResponse(privacy session.StatusPrivacy) StatusPrivacyResponse {
	jids := privacy.JIDs
	if jids == nil {
		jids = []string{}
	}
	return StatusPrivacyResponse{Mode: privacy.Mode, JIDs: jids}
}
