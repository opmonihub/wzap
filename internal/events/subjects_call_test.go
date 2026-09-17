package events

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestCallSubjects pins the verbatim subject of the unified call event: the
// offer, the accept, the reject and the end travel as call.offer with their
// state field.
func TestCallSubjects(t *testing.T) {
	id := uuid.New()
	prefix := "wzap.instances." + id.String() + "."

	if got := Subjects.CallOffer(id); got != prefix+"call.offer" {
		t.Errorf("CallOffer subject = %q, want %q", got, prefix+"call.offer")
	}
	if subject := Subjects.CallOffer(id); !strings.HasPrefix(subject, "wzap.") {
		t.Errorf("subject = %q, want the wzap namespace", subject)
	}
}
