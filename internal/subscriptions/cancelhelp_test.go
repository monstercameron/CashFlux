// SPDX-License-Identifier: MIT

package subscriptions

import (
	"strings"
	"testing"
)

func TestCancelChecklist(t *testing.T) {
	steps := CancelChecklist("Netflix")
	if len(steps) < 3 {
		t.Fatalf("want several steps, got %d", len(steps))
	}
	if !strings.Contains(steps[0], "Netflix") {
		t.Errorf("first step should name the subscription: %q", steps[0])
	}
	joined := strings.ToLower(strings.Join(steps, " "))
	if !strings.Contains(joined, "renewal") || !strings.Contains(joined, "dispute") {
		t.Errorf("checklist should cover cancelling before renewal and disputing a re-charge: %q", joined)
	}
	// Blank name degrades gracefully.
	if got := CancelChecklist("")[0]; !strings.Contains(got, "this subscription") {
		t.Errorf("blank name should fall back: %q", got)
	}
}

func TestNegotiationTips(t *testing.T) {
	tips := NegotiationTips("Comcast")
	if len(tips) < 3 {
		t.Fatalf("want several tips, got %d", len(tips))
	}
	joined := strings.ToLower(strings.Join(tips, " "))
	if !strings.Contains(joined, "competitor") || !strings.Contains(joined, "retention") {
		t.Errorf("tips should mention a competitor rate and retention offers: %q", joined)
	}
}

func TestChecklistNotes(t *testing.T) {
	steps := []string{"Do A", "Do B"}
	notes := ChecklistNotes("Save $120/year.", steps)
	if !strings.HasPrefix(notes, "Save $120/year.") {
		t.Errorf("notes should lead with the savings line: %q", notes)
	}
	if !strings.Contains(notes, "1. Do A") || !strings.Contains(notes, "2. Do B") {
		t.Errorf("notes should number the steps: %q", notes)
	}
	// No savings line → starts at step 1, no leading blank.
	plain := ChecklistNotes("", steps)
	if !strings.HasPrefix(plain, "1. Do A") {
		t.Errorf("without a savings line, notes should start at step 1: %q", plain)
	}
}
