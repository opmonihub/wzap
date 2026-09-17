package webhook

import (
	"slices"
	"testing"
)

// TestCallEventsAreCanonicalOptIn pins that the unified call event type is
// accepted as an explicit webhook subscription without entering the default
// set: existing webhooks keep receiving exactly the four original types
// unless they opt in.
func TestCallEventsAreCanonicalOptIn(t *testing.T) {
	got, err := ValidateEvents([]string{"call.offer"})
	if err != nil {
		t.Fatalf("ValidateEvents([call.offer]): %v, want accepted as opt-in", err)
	}
	if !slices.Equal(got, []string{"call.offer"}) {
		t.Errorf("ValidateEvents([call.offer]) = %v, want the single type back", got)
	}
	if Subscribed(DefaultEvents(), "call.offer") {
		t.Error("default events contain call.offer, want opt-in only")
	}

	def := DefaultEvents()
	want := []string{"message", "receipt", "connection", "message.status"}
	if !slices.Equal(def, want) {
		t.Errorf("DefaultEvents() = %v, want the pinned %v", def, want)
	}
}
