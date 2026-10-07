package channels

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	FollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error
	UnfollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error
	GetNewsletter(ctx context.Context, id uuid.UUID, channelJID string) (instance.Newsletter, error)
	ListNewsletters(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error)

	GetNewsletterMessages(ctx context.Context, id uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error)
	GetNewsletterUpdates(ctx context.Context, id uuid.UUID, channel string) ([]session.NewsletterMessage, error)

	CreateNewsletter(ctx context.Context, id uuid.UUID, title, description string) (instance.Newsletter, error)
	MuteNewsletter(ctx context.Context, id uuid.UUID, channel string, muted bool) error
	MarkNewsletterViewed(ctx context.Context, id uuid.UUID, channel string, serverIDs []string) error
	ReactNewsletter(ctx context.Context, id uuid.UUID, channel, serverID, reaction string) error
}
