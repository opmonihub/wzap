package whatsmeow

import (
	"context"
	"testing"
)

func TestNewsletterMarkViewedRejectsEmptyServerIDs(t *testing.T) {
	s := newParityTestSession(t)
	if err := s.NewsletterMarkViewed(context.Background(), "12036300000001@newsletter", nil); err == nil {
		t.Fatalf("NewsletterMarkViewed empty serverIDs err = nil, want error")
	}
}

func TestCreateNewsletterRejectsEmptyTitle(t *testing.T) {
	s := newParityTestSession(t)
	if _, err := s.CreateNewsletter(context.Background(), "", "description"); err == nil {
		t.Fatalf("CreateNewsletter empty title err = nil, want error")
	}
}
