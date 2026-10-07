package representation

import (
	"time"

	"wzap/internal/instance"
)

// GroupParticipantResponse is one member of a group in the REST contract.
type GroupParticipantResponse struct {
	JID          string `json:"jid" binding:"required"`
	IsAdmin      bool   `json:"is_admin" binding:"required"`
	IsSuperAdmin bool   `json:"is_super_admin" binding:"required"`
}

// GroupResponse is the JSON representation of a group. InviteCode travels
// only on creation; UpdatedAt is the last metadata refresh (zero when no
// metadata store is wired).
type GroupResponse struct {
	JID              string                     `json:"jid" binding:"required"`
	Name             string                     `json:"name" binding:"required"`
	Description      string                     `json:"description,omitempty"`
	Participants     []GroupParticipantResponse `json:"participants" binding:"required"`
	ParticipantCount int                        `json:"participant_count" binding:"required"`
	InviteCode       string                     `json:"invite_code,omitempty"`
	UpdatedAt        *time.Time                 `json:"updated_at,omitempty"`
}

// NewGroupResponse maps a stored group to its JSON representation. A nil
// participant list is emitted as an empty array so the field keeps its array
// shape on every read.
func NewGroupResponse(group instance.Group) GroupResponse {
	participants := make([]GroupParticipantResponse, 0, len(group.Participants))
	for _, p := range group.Participants {
		participants = append(participants, GroupParticipantResponse{
			JID:          p.JID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	return GroupResponse{
		JID:              group.JID,
		Name:             group.Name,
		Description:      group.Description,
		Participants:     participants,
		ParticipantCount: group.ParticipantCount,
		UpdatedAt:        OptionalTime(group.UpdatedAt),
	}
}
