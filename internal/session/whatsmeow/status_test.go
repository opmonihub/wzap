package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"

	"wzap/internal/session"
)

// TestBuildStatusText pins the text status construction: a conversation
// message to the status broadcast with the text kind.
func TestBuildStatusText(t *testing.T) {
	sess := &instanceSession{}

	kind, built, err := sess.buildStatusMessage(context.Background(), session.StatusInput{Text: "bom dia"})
	if err != nil {
		t.Fatalf("buildStatusMessage text: %v", err)
	}
	if kind != session.StatusKindText {
		t.Errorf("kind = %q, want text", kind)
	}
	if built.GetConversation() != "bom dia" {
		t.Errorf("conversation = %q, want the text", built.GetConversation())
	}
}

// TestBuildStatusRejectsEmpty pins that a status without text or media is
// rejected before any upload.
func TestBuildStatusRejectsEmpty(t *testing.T) {
	sess := &instanceSession{}

	if _, _, err := sess.buildStatusMessage(context.Background(), session.StatusInput{}); err == nil {
		t.Error("buildStatusMessage empty: got nil error, want rejection")
	}
}

// TestBuildStatusMedia pins the image/video construction: the bytes upload
// under their namespace and travel with the caption and mime.
func TestBuildStatusMedia(t *testing.T) {
	upload := whatsmeow.UploadResponse{
		URL:           "https://example.com/media",
		DirectPath:    "/media",
		MediaKey:      []byte("key"),
		FileSHA256:    []byte("sha"),
		FileEncSHA256: []byte("encsha"),
		FileLength:    7,
	}
	sess := &instanceSession{
		statusUploadFn: func(_ context.Context, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
			if string(data) != "bytes" {
				t.Errorf("upload data = %q, want the input bytes", data)
			}
			if mediaType != whatsmeow.MediaImage {
				t.Errorf("upload media type = %v, want image", mediaType)
			}
			return upload, nil
		},
	}

	kind, built, err := sess.buildStatusMessage(context.Background(), session.StatusInput{
		MediaMime: "image/jpeg",
		MediaData: []byte("bytes"),
		Caption:   "olha",
	})
	if err != nil {
		t.Fatalf("buildStatusMessage image: %v", err)
	}
	if kind != session.StatusKindImage {
		t.Errorf("kind = %q, want image", kind)
	}
	image := built.GetImageMessage()
	if image == nil {
		t.Fatal("built message has no image, want an image message")
	}
	if image.GetCaption() != "olha" || image.GetMimetype() != "image/jpeg" {
		t.Errorf("image = caption %q mime %q, want olha and image/jpeg", image.GetCaption(), image.GetMimetype())
	}
}

// TestDropExpiredStatuses pins the 24h registry cutoff: entries older than
// 24h are dropped on list while entries at and after the cutoff are kept.
func TestDropExpiredStatuses(t *testing.T) {
	now := time.Now().UTC()
	statuses := []session.StatusInfo{
		{ID: "old", CreatedAt: now.Add(-25 * time.Hour)},
		{ID: "cutoff", CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "fresh", CreatedAt: now},
	}

	kept := dropExpiredStatuses(statuses, now)

	if len(kept) != 2 || kept[0].ID != "cutoff" || kept[1].ID != "fresh" {
		t.Errorf("dropExpiredStatuses kept = %v, want [cutoff fresh]", kept)
	}
}

// TestDeleteStatusEmptyID pins that an empty status id maps to
// ErrStatusNotFound (404), never to a generic error.
func TestDeleteStatusEmptyID(t *testing.T) {
	sess := &instanceSession{}

	if err := sess.DeleteStatus(context.Background(), ""); !errors.Is(err, session.ErrStatusNotFound) {
		t.Errorf("DeleteStatus empty: err = %v, want ErrStatusNotFound", err)
	}
}

// TestBuildStatusRejectsUnsupportedMime pins that non image/video media never
// reaches the upload.
func TestBuildStatusRejectsUnsupportedMime(t *testing.T) {
	sess := &instanceSession{
		statusUploadFn: func(context.Context, []byte, whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
			t.Error("upload called for an unsupported mime, want rejection first")
			return whatsmeow.UploadResponse{}, nil
		},
	}

	if _, _, err := sess.buildStatusMessage(context.Background(), session.StatusInput{
		MediaMime: "application/pdf",
		MediaData: []byte("bytes"),
	}); err == nil {
		t.Error("buildStatusMessage pdf: got nil error, want rejection")
	}
}
