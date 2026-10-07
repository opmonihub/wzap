package representation

import (
	"time"

	"wzap/internal/instance"
)

// NewsletterResponse is the JSON representation of a channel. UpdatedAt is the
// last metadata refresh (zero when no metadata store is wired).
type NewsletterResponse struct {
	Channel       string     `json:"channel" binding:"required"`
	Title         string     `json:"title" binding:"required"`
	Description   string     `json:"description,omitempty"`
	FollowerCount int        `json:"follower_count" binding:"required"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// NewNewsletterResponse maps a stored channel to its JSON representation.
func NewNewsletterResponse(newsletter instance.Newsletter) NewsletterResponse {
	return NewsletterResponse{
		Channel:       newsletter.ChannelJID,
		Title:         newsletter.Title,
		Description:   newsletter.Description,
		FollowerCount: newsletter.FollowerCount,
		UpdatedAt:     OptionalTime(newsletter.UpdatedAt),
	}
}
