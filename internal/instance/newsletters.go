package instance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session"
)

// Newsletter is the metadata of a channel: the live upstream view. UpdatedAt
// is the last metadata refresh (storage write-through); it is zero when no
// metadata store is wired.
type Newsletter struct {
	ChannelJID    string
	Title         string
	Description   string
	FollowerCount int
	UpdatedAt     time.Time
}

// FollowNewsletter subscribes the instance to channelJID through the session.
// An unknown channel is ErrNewsletterNotFound and a disconnected session is
// ErrNotConnected; anything else is returned unchanged for a 500.
func (s *Service) FollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("follow newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("follow newsletter: create session: %w", err)
	}
	if err := sess.FollowNewsletter(ctx, channelJID); err != nil {
		return mapNewsletterError("follow newsletter", err)
	}
	// The follow itself carries no metadata: refresh the cache from the live
	// view so a later consult finds it. A refresh failure never fails the
	// follow the upstream already accepted.
	if info, err := sess.GetNewsletter(ctx, channelJID); err == nil {
		s.refreshNewsletterCache(ctx, id, newsletterFromSession(info))
	} else {
		s.log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-cache").Err(err).Msg("refresh newsletter metadata failed")
	}
	return nil
}

// UnfollowNewsletter ends the subscription of the instance to channelJID.
// Error mapping follows FollowNewsletter.
func (s *Service) UnfollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("unfollow newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("unfollow newsletter: create session: %w", err)
	}
	if err := sess.UnfollowNewsletter(ctx, channelJID); err != nil {
		return mapNewsletterError("unfollow newsletter", err)
	}
	return nil
}

// GetNewsletter returns the live metadata of channelJID through the session:
// the stored metadata is a cache refreshed on demand, never the source of
// truth. Error mapping follows FollowNewsletter.
func (s *Service) GetNewsletter(ctx context.Context, id uuid.UUID, channelJID string) (Newsletter, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Newsletter{}, mapError("get newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Newsletter{}, fmt.Errorf("get newsletter: create session: %w", err)
	}
	info, err := sess.GetNewsletter(ctx, channelJID)
	if err != nil {
		return Newsletter{}, mapNewsletterError("get newsletter", err)
	}
	return s.refreshNewsletterCache(ctx, id, newsletterFromSession(info)), nil
}

// ListNewsletters returns one page of the live subscribed channels ordered by
// channel JID with the cursor of the next page, empty on the last page. The
// cursor is the channel JID of the last item of the previous page; paging is
// lexicographic, so an unknown cursor positions without failing. Error
// mapping follows FollowNewsletter.
func (s *Service) ListNewsletters(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]Newsletter, string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, "", mapError("list newsletters", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, "", fmt.Errorf("list newsletters: create session: %w", err)
	}
	infos, err := sess.ListNewsletters(ctx)
	if err != nil {
		return nil, "", mapNewsletterError("list newsletters", err)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ChannelJID < infos[j].ChannelJID })

	items := make([]Newsletter, 0, len(infos))
	for _, info := range infos {
		if cursor != "" && info.ChannelJID <= cursor {
			continue
		}
		items = append(items, s.refreshNewsletterCache(ctx, id, newsletterFromSession(info)))
	}
	next := ""
	if limit > 0 && len(items) > limit {
		items = items[:limit]
		next = items[limit-1].ChannelJID
	}
	return items, next, nil
}

// newsletterFromSession translates the session channel metadata into the
// service shape.
func newsletterFromSession(info session.NewsletterInfo) Newsletter {
	return Newsletter{
		ChannelJID:    info.ChannelJID,
		Title:         info.Title,
		Description:   info.Description,
		FollowerCount: info.FollowerCount,
	}
}

// mapNewsletterError translates a newsletter session call failure into the
// service sentinel the HTTP layer maps to a status code, preserving the
// operation context for the logs. Unknown failures pass through unchanged for
// a 500 without leaking their cause (the handler never echoes them).
func mapNewsletterError(op string, err error) error {
	switch {
	case errors.Is(err, session.ErrNotFound):
		return fmt.Errorf("%s: %w", op, ErrNewsletterNotFound)
	case errors.Is(err, session.ErrForbidden):
		return fmt.Errorf("%s: %w", op, ErrForbidden)
	default:
		return mapSessionError(op, err)
	}
}

// refreshNewsletterCache writes the live channel view through to the metadata
// cache and stamps the returned channel with the stored updated_at. Without
// a wired store the live view passes through untouched; a cache failure is
// logged and the live view is returned, so a storage hiccup never fails a
// read the upstream already answered.
func (s *Service) refreshNewsletterCache(ctx context.Context, id uuid.UUID, newsletter Newsletter) Newsletter {
	if s.newsletters == nil {
		return newsletter
	}
	stored, err := s.newsletters.Upsert(ctx, model.NewsletterMetadata{
		InstanceID:    id,
		ChannelJID:    newsletter.ChannelJID,
		Title:         newsletter.Title,
		Description:   newsletter.Description,
		FollowerCount: newsletter.FollowerCount,
	})
	if err != nil {
		s.log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-cache").Err(err).Msg("refresh newsletter metadata failed")
		return newsletter
	}
	newsletter.UpdatedAt = stored.UpdatedAt
	return newsletter
}
