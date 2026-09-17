package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"wzap/internal/session"
)

// PublishStatus publishes an own status (story) and returns its upstream id.
// A text status travels as a conversation message to the status broadcast;
// an image or video status is uploaded first and sent with its caption. The
// published entry is tracked in the process-local registry ListStatuses
// reads: the supported runtime is one replica and the upstream expiry still
// applies, so an entry the protocol already dropped may be gone on read.
func (s *instanceSession) PublishStatus(ctx context.Context, input session.StatusInput) (string, error) {
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: publish status", session.ErrNotConnected)
	}
	kind, waMsg, err := s.buildStatusMessage(ctx, input)
	if err != nil {
		return "", err
	}

	send := s.statusSendFn
	if send == nil {
		send = func(ctx context.Context, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
			return s.client.SendMessage(ctx, types.StatusBroadcastJID, msg)
		}
	}
	resp, err := send(ctx, waMsg)
	if err != nil {
		return "", classifySessionError(err)
	}
	id := string(resp.ID)

	s.statusMu.Lock()
	s.statuses = append(s.statuses, session.StatusInfo{
		ID:        id,
		Kind:      kind,
		Text:      input.Text,
		Caption:   input.Caption,
		CreatedAt: time.Now().UTC(),
	})
	s.statusMu.Unlock()
	return id, nil
}

// statusTTL bounds an own status listing: entries expire upstream after
// ~24h, so the registry drops anything older on list.
const statusTTL = 24 * time.Hour

// dropExpiredStatuses returns the entries published after the 24h cutoff,
// oldest first. It backs ListStatuses so a stale registry never resurfaces
// statuses the protocol already expired.
func dropExpiredStatuses(statuses []session.StatusInfo, now time.Time) []session.StatusInfo {
	cutoff := now.UTC().Add(-statusTTL)
	kept := make([]session.StatusInfo, 0, len(statuses))
	for _, info := range statuses {
		if info.CreatedAt.UTC().After(cutoff) || info.CreatedAt.UTC().Equal(cutoff) {
			kept = append(kept, info)
		}
	}
	return kept
}

// ListStatuses returns the own statuses published through this session that
// are still tracked here, oldest first. Entries expire upstream after ~24h;
// the registry drops them on list.
func (s *instanceSession) ListStatuses(ctx context.Context) ([]session.StatusInfo, error) {
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list statuses", session.ErrNotConnected)
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.statuses = dropExpiredStatuses(s.statuses, time.Now().UTC())
	return append([]session.StatusInfo(nil), s.statuses...), nil
}

// DeleteStatus removes the own statusID published through this session: it
// sends the protocol revocation to the status broadcast and drops the tracked
// entry. An empty or unknown id is ErrStatusNotFound.
func (s *instanceSession) DeleteStatus(ctx context.Context, statusID string) error {
	if statusID == "" {
		return fmt.Errorf("%w: empty status id", session.ErrStatusNotFound)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: delete status", session.ErrNotConnected)
	}
	s.statusMu.Lock()
	found := false
	for _, info := range s.statuses {
		if info.ID == statusID {
			found = true
			break
		}
	}
	s.statusMu.Unlock()
	if !found {
		return fmt.Errorf("%w: %s", session.ErrStatusNotFound, statusID)
	}

	send := s.sendRevokeFn
	if send == nil {
		send = func(ctx context.Context, chat types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
			return s.client.SendMessage(ctx, chat, msg)
		}
	}
	if _, err := send(ctx, types.StatusBroadcastJID,
		s.client.BuildRevoke(types.StatusBroadcastJID, types.EmptyJID, types.MessageID(statusID))); err != nil {
		return classifySessionError(err)
	}

	s.statusMu.Lock()
	kept := s.statuses[:0]
	for _, info := range s.statuses {
		if info.ID != statusID {
			kept = append(kept, info)
		}
	}
	s.statuses = kept
	s.statusMu.Unlock()
	return nil
}

// buildStatusMessage translates a status input into its wire message,
// uploading image/video bytes first. The kind travels back for the registry.
func (s *instanceSession) buildStatusMessage(ctx context.Context, input session.StatusInput) (string, *waE2E.Message, error) {
	if len(input.MediaData) == 0 {
		if input.Text == "" {
			return "", nil, errors.New("publish status: text is required")
		}
		return session.StatusKindText, &waE2E.Message{Conversation: proto.String(input.Text)}, nil
	}
	mediaType, kind, err := statusMediaType(input.MediaMime)
	if err != nil {
		return "", nil, err
	}
	upload := s.statusUploadFn
	if upload == nil {
		upload = func(ctx context.Context, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
			return s.client.Upload(ctx, data, mediaType)
		}
	}
	uploaded, err := upload(ctx, input.MediaData, mediaType)
	if err != nil {
		return "", nil, classifySessionError(err)
	}
	caption := optionalString(input.Caption)
	switch kind {
	case session.StatusKindImage:
		return kind, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileSHA256:    uploaded.FileSHA256,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Mimetype:      proto.String(input.MediaMime),
			Caption:       caption,
		}}, nil
	default:
		return kind, &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileSHA256:    uploaded.FileSHA256,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Mimetype:      proto.String(input.MediaMime),
			Caption:       caption,
		}}, nil
	}
}

// statusMediaType maps a status media mime onto the upload namespace and the
// status kind. Anything outside image/* and video/* is rejected before the
// session uploads anything.
func statusMediaType(mime string) (whatsmeow.MediaType, string, error) {
	switch {
	case len(mime) > 6 && mime[:6] == "image/":
		return whatsmeow.MediaImage, session.StatusKindImage, nil
	case len(mime) > 6 && mime[:6] == "video/":
		return whatsmeow.MediaVideo, session.StatusKindVideo, nil
	default:
		return "", "", fmt.Errorf("publish status: unsupported media type %q", mime)
	}
}
