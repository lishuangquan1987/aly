package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestToastShowsForTheMessage checks that a queued notification reaches the
// overlay with its text while it is fresh.
func TestToastShowsForTheMessage(t *testing.T) {
	a := testApp()
	a.notify(toastWarning, "Server ignore rules could not be updated")
	tt := ui.NewTester(a.view, 1320, 840)
	if !tt.HasText("Server ignore rules could not be updated") {
		t.Fatalf("the toast text is missing; texts=%q", tt.Texts())
	}
	if len(a.toasts) != 1 {
		t.Fatalf("toasts = %d, want the queued one", len(a.toasts))
	}
}

// TestToastExpiresAfterItsDuration checks the overlay's own timeout: an entry
// past toastDuration is pruned on the next frame instead of lingering.
func TestToastExpiresAfterItsDuration(t *testing.T) {
	a := testApp()
	a.toasts = []toast{{
		text: "Stale message",
		kind: toastInfo,
		at:   time.Now().Add(-toastDuration - time.Second),
	}}
	tt := ui.NewTester(a.view, 1320, 840)
	if len(a.toasts) != 0 {
		t.Fatalf("toasts = %d after expiry, want 0", len(a.toasts))
	}
	if tt.HasText("Stale message") {
		t.Fatalf("an expired toast is still drawn")
	}
}
