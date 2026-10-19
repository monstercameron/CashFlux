// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestV5BulkReviewInboxUsesButtonTitle(t *testing.T) {
	if got := reviewInboxTitle(); got != "Review" {
		t.Fatalf("review dialog title = %q, want the button title %q", got, "Review")
	}
}
