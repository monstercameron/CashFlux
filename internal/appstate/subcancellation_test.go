package appstate

import (
	"testing"
	"time"
)

func TestMarkAndUnmarkSubscriptionCancelled(t *testing.T) {
	app, err := New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	cancelDate := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	// Mark a subscription as cancelled.
	if err := app.MarkSubscriptionCancelled("Netflix", cancelDate); err != nil {
		t.Fatalf("mark: %v", err)
	}
	cs := app.Cancellations()
	if len(cs) != 1 {
		t.Fatalf("after mark: count = %d, want 1", len(cs))
	}
	if cs[0].SubName != "Netflix" {
		t.Errorf("SubName = %q, want Netflix", cs[0].SubName)
	}
	if !cs[0].CancelledOn.Equal(cancelDate) {
		t.Errorf("CancelledOn = %v, want %v", cs[0].CancelledOn, cancelDate)
	}

	// Dedupe: marking again with a different date updates, not duplicates.
	newDate := cancelDate.AddDate(0, 1, 0)
	if err := app.MarkSubscriptionCancelled("Netflix", newDate); err != nil {
		t.Fatalf("second mark: %v", err)
	}
	cs2 := app.Cancellations()
	if len(cs2) != 1 {
		t.Fatalf("after second mark: count = %d, want 1 (dedupe)", len(cs2))
	}
	if !cs2[0].CancelledOn.Equal(newDate) {
		t.Errorf("updated CancelledOn = %v, want %v", cs2[0].CancelledOn, newDate)
	}

	// Case-insensitive dedupe: "netflix" should match "Netflix".
	if err := app.MarkSubscriptionCancelled("netflix", cancelDate); err != nil {
		t.Fatalf("case-insensitive mark: %v", err)
	}
	cs3 := app.Cancellations()
	if len(cs3) != 1 {
		t.Fatalf("after case-insensitive mark: count = %d, want 1", len(cs3))
	}

	// Unmark removes the record.
	if err := app.UnmarkSubscriptionCancelled("Netflix"); err != nil {
		t.Fatalf("unmark: %v", err)
	}
	cs4 := app.Cancellations()
	if len(cs4) != 0 {
		t.Fatalf("after unmark: count = %d, want 0", len(cs4))
	}

	// Unmark of a non-existent subscription is a no-op (no error).
	if err := app.UnmarkSubscriptionCancelled("Spotify"); err != nil {
		t.Errorf("unmark non-existent: %v", err)
	}
}

func TestMarkSubscriptionCancelled_EmptyName(t *testing.T) {
	app, err := New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if err := app.MarkSubscriptionCancelled("", time.Now()); err == nil {
		t.Error("expected error for empty subName, got nil")
	}
}
