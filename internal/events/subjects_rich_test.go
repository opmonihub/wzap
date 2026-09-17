package events

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestRichSubjects pins the verbatim subjects of the rich inbound events: the
// vote, the reaction and the unified interactive response.
func TestRichSubjects(t *testing.T) {
	id := uuid.New()
	prefix := "wzap.instances." + id.String() + "."

	if got := Subjects.PollVote(id); got != prefix+"message.poll.vote" {
		t.Errorf("PollVote subject = %q, want %q", got, prefix+"message.poll.vote")
	}
	if got := Subjects.Reaction(id); got != prefix+"message.reaction" {
		t.Errorf("Reaction subject = %q, want %q", got, prefix+"message.reaction")
	}
	if got := Subjects.InteractiveResponse(id); got != prefix+"message.interactive.response" {
		t.Errorf("InteractiveResponse subject = %q, want %q", got, prefix+"message.interactive.response")
	}
	for _, subject := range []string{
		Subjects.PollVote(id), Subjects.Reaction(id), Subjects.InteractiveResponse(id),
	} {
		if !strings.HasPrefix(subject, "wzap.") {
			t.Errorf("subject = %q, want the wzap namespace", subject)
		}
	}
}
