package events

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestGroupSubjects pins the verbatim subjects of the group inbound events:
// membership moves and metadata changes travel under group.*, not message.*
// (Ruling C1).
func TestGroupSubjects(t *testing.T) {
	id := uuid.New()
	prefix := "wzap.instances." + id.String() + "."

	if got := Subjects.GroupParticipants(id); got != prefix+"group.participants" {
		t.Errorf("GroupParticipants subject = %q, want %q", got, prefix+"group.participants")
	}
	if got := Subjects.GroupInfo(id); got != prefix+"group.info" {
		t.Errorf("GroupInfo subject = %q, want %q", got, prefix+"group.info")
	}
	for _, subject := range []string{
		Subjects.GroupParticipants(id), Subjects.GroupInfo(id),
	} {
		if !strings.HasPrefix(subject, "wzap.") {
			t.Errorf("subject = %q, want the wzap namespace", subject)
		}
	}
}
