package instance

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// GetProfile returns the own profile of the instance through its session. It
// is synchronous and direct: no outbox, no retry. A disconnected session is
// ErrNotConnected; anything else is returned unchanged for a 500.
func (s *Service) GetProfile(ctx context.Context, id uuid.UUID) (session.Profile, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.Profile{}, mapError("get profile", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.Profile{}, fmt.Errorf("get profile: create session: %w", err)
	}
	profile, err := sess.GetProfile(ctx)
	if err != nil {
		return session.Profile{}, mapSessionError("get profile", err)
	}
	return profile, nil
}

// SetProfileName replaces the own display name through the instance session.
// The name is validated by the handler before the session is touched. When
// the upstream exposes no setter the session reports ErrUnsupported for the
// documented 501. Other error mapping follows GetProfile.
func (s *Service) SetProfileName(ctx context.Context, id uuid.UUID, name string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set profile name", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set profile name: create session: %w", err)
	}
	if err := sess.SetProfileName(ctx, name); err != nil {
		return mapSessionError("set profile name", err)
	}
	return nil
}

// SetProfileStatusText replaces the own recado (status text) through the
// instance session. An empty text clears it. Error mapping follows
// SetProfileName.
func (s *Service) SetProfileStatusText(ctx context.Context, id uuid.UUID, text string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set profile status", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set profile status: create session: %w", err)
	}
	if err := sess.SetProfileStatusText(ctx, text); err != nil {
		return mapSessionError("set profile status", err)
	}
	return nil
}

// SetProfilePhoto replaces the own photo with the image bytes through the
// instance session. When the upstream exposes no setter the session reports
// ErrUnsupported for the documented 501. Other error mapping follows
// GetProfile.
func (s *Service) SetProfilePhoto(ctx context.Context, id uuid.UUID, image []byte) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set profile photo", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set profile photo: create session: %w", err)
	}
	if err := sess.SetProfilePhoto(ctx, image); err != nil {
		return mapSessionError("set profile photo", err)
	}
	return nil
}

// GetPrivacy returns the own privacy settings of the instance through its
// session. Error mapping follows GetProfile.
func (s *Service) GetPrivacy(ctx context.Context, id uuid.UUID) (session.Privacy, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.Privacy{}, mapError("get privacy", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.Privacy{}, fmt.Errorf("get privacy: create session: %w", err)
	}
	privacy, err := sess.GetPrivacy(ctx)
	if err != nil {
		return session.Privacy{}, mapSessionError("get privacy", err)
	}
	return privacy, nil
}

// SetPrivacy applies the non-empty fields of input to the upstream privacy
// settings through the instance session and returns the resulting settings.
// Every value is validated by the handler before the session is touched.
// Error mapping follows GetProfile.
func (s *Service) SetPrivacy(ctx context.Context, id uuid.UUID, input session.Privacy) (session.Privacy, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.Privacy{}, mapError("set privacy", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.Privacy{}, fmt.Errorf("set privacy: create session: %w", err)
	}
	privacy, err := sess.SetPrivacy(ctx, input)
	if err != nil {
		return session.Privacy{}, mapSessionError("set privacy", err)
	}
	return privacy, nil
}
