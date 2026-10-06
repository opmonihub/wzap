package whatsmeow

import (
	"context"
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// GetProfile returns the own profile of the instance: the device push name
// (best-effort — the pinned library exposes no profile fetch), the recado
// from the user info and the photo URL (empty when the instance carries no
// photo or the upstream reports none).
func (s *instanceSession) GetProfile(ctx context.Context) (session.Profile, error) {
	if !s.IsConnected() {
		return session.Profile{}, fmt.Errorf("%w: get profile", session.ErrNotConnected)
	}
	profile := session.Profile{Name: s.client.Store.PushName}

	own, err := types.ParseJID(s.JID())
	if err != nil || own.IsEmpty() {
		return profile, nil
	}
	user := own.ToNonAD()

	getUserInfo := s.client.GetUserInfo
	if s.getUserInfoFn != nil {
		getUserInfo = s.getUserInfoFn
	}
	info, err := getUserInfo(ctx, []types.JID{user})
	if err != nil {
		if mapped := classifySessionError(err); errors.Is(mapped, session.ErrNotConnected) {
			return session.Profile{}, mapped
		}
		s.log.Warn().Str("instance_id", s.instanceID.String()).Str("op", "get-profile-user-info").Err(err).Msg("get profile user info failed")
	} else if userInfo, ok := info[user]; ok {
		profile.StatusText = userInfo.Status
	}

	getPicture := s.client.GetProfilePictureInfo
	if s.getProfilePictureInfoFn != nil {
		getPicture = s.getProfilePictureInfoFn
	}
	picture, err := getPicture(ctx, user, nil)
	if err != nil {
		if mapped := classifySessionError(err); errors.Is(mapped, session.ErrNotConnected) {
			return session.Profile{}, mapped
		}
		s.log.Warn().Str("instance_id", s.instanceID.String()).Str("op", "get-profile-picture").Err(err).Msg("get profile picture failed")
	} else if picture != nil {
		profile.PhotoURL = picture.URL
	}
	return profile, nil
}

// SetProfileName is not supported by the pinned library, which exposes no
// profile name setter: it always answers ErrUnsupported for the documented
// 501.
func (s *instanceSession) SetProfileName(_ context.Context, _ string) error {
	return fmt.Errorf("set profile name: %w", session.ErrUnsupported)
}

// SetProfileStatusText replaces the own recado (status text). An empty text
// clears it.
func (s *instanceSession) SetProfileStatusText(ctx context.Context, text string) error {
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set profile status", session.ErrNotConnected)
	}
	if err := s.client.SetStatusMessage(ctx, types.SetStatusInput{Text: &text}); err != nil {
		return classifySessionError(err)
	}
	return nil
}

// SetProfilePhoto is not supported by the pinned library, which exposes no
// profile photo setter: it always answers ErrUnsupported for the documented
// 501.
func (s *instanceSession) SetProfilePhoto(_ context.Context, _ []byte) error {
	return fmt.Errorf("set profile photo: %w", session.ErrUnsupported)
}

// GetPrivacy returns the own privacy settings of the instance, refreshed
// from the upstream.
func (s *instanceSession) GetPrivacy(ctx context.Context) (session.Privacy, error) {
	if !s.client.IsConnected() {
		return session.Privacy{}, fmt.Errorf("%w: get privacy", session.ErrNotConnected)
	}
	settings, err := s.client.TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return session.Privacy{}, classifySessionError(err)
	}
	return mapPrivacy(*settings), nil
}

// SetPrivacy applies the non-empty fields of input to the upstream privacy
// settings and returns the resulting settings, refreshed from the upstream.
// It is not atomic: fields apply one by one, so a failure can leave earlier
// fields already applied.
func (s *instanceSession) SetPrivacy(ctx context.Context, input session.Privacy) (session.Privacy, error) {
	if !s.client.IsConnected() {
		return session.Privacy{}, fmt.Errorf("%w: set privacy", session.ErrNotConnected)
	}
	for _, field := range []struct {
		name  types.PrivacySettingType
		value string
	}{
		{types.PrivacySettingTypeLastSeen, input.LastSeen},
		{types.PrivacySettingTypeProfile, input.ProfilePhoto},
		{types.PrivacySettingTypeStatus, input.Status},
		{types.PrivacySettingTypeReadReceipts, input.ReadReceipts},
		{types.PrivacySettingTypeGroupAdd, input.GroupsAdd},
	} {
		if field.value == "" {
			continue
		}
		if _, err := s.client.SetPrivacySetting(ctx, field.name, types.PrivacySetting(field.value)); err != nil {
			return session.Privacy{}, classifySessionError(err)
		}
	}
	settings, err := s.client.TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return session.Privacy{}, classifySessionError(err)
	}
	return mapPrivacy(*settings), nil
}

// mapPrivacy translates the library privacy settings into the session shape.
func mapPrivacy(settings types.PrivacySettings) session.Privacy {
	return session.Privacy{
		LastSeen:     string(settings.LastSeen),
		ProfilePhoto: string(settings.Profile),
		Status:       string(settings.Status),
		ReadReceipts: string(settings.ReadReceipts),
		GroupsAdd:    string(settings.GroupAdd),
	}
}
