package webhook

import (
	"slices"
	"testing"
)

// TestRichEventsAreCanonicalOptIn pins that the rich inbound event types are
// accepted as explicit webhook subscriptions without entering the default set:
// existing webhooks keep receiving exactly the four original types unless they
// opt in.
func TestRichEventsAreCanonicalOptIn(t *testing.T) {
	for _, eventType := range []string{"poll.vote", "message.reaction", "interactive.response"} {
		got, err := ValidateEvents([]string{eventType})
		if err != nil {
			t.Errorf("ValidateEvents([%q]): %v, want accepted as opt-in", eventType, err)
			continue
		}
		if !slices.Equal(got, []string{eventType}) {
			t.Errorf("ValidateEvents([%q]) = %v, want the single type back", eventType, got)
		}
		if Subscribed(DefaultEvents(), eventType) {
			t.Errorf("default events contain %q, want opt-in only", eventType)
		}
	}

	def := DefaultEvents()
	want := []string{"message", "receipt", "connection", "message.status"}
	if !slices.Equal(def, want) {
		t.Errorf("DefaultEvents() = %v, want the pinned %v", def, want)
	}
}
