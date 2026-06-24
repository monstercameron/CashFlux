// SPDX-License-Identifier: MIT

package smartai

import (
	"strings"
	"testing"
)

func TestImplemented(t *testing.T) {
	for _, c := range []string{"SMART-A5", "SMART-P3"} {
		if !Implemented(c) {
			t.Errorf("%s should be implemented", c)
		}
	}
	if Implemented("SMART-NOPE") {
		t.Errorf("unknown code should not be implemented")
	}
	if len(ImplementedCodes()) < 2 {
		t.Errorf("expected at least two implemented AI features, got %d", len(ImplementedCodes()))
	}
}

func TestOutlook(t *testing.T) {
	r := Outlook("Net worth: $42,000\nThis month: +$800")
	if r.System != OutlookSystem {
		t.Errorf("system prompt mismatch")
	}
	if !strings.Contains(r.User, "Net worth: $42,000") {
		t.Errorf("context not embedded: %q", r.User)
	}
}

func TestAcceptable(t *testing.T) {
	good := []string{"You have $4,200 in checking.", "Your savings grew the most, up $1,000."}
	for _, g := range good {
		if !Acceptable(g) {
			t.Errorf("expected acceptable: %q", g)
		}
	}
	bad := []string{"", " ", "I can't help with that.", "I don't know.", "As an AI, I cannot..."}
	for _, b := range bad {
		if Acceptable(b) {
			t.Errorf("expected NOT acceptable: %q", b)
		}
	}
}

func TestAccountQA(t *testing.T) {
	r := AccountQA("  Which account has the most?  ", "Checking: $1,000\nSavings: $5,000")
	if r.System != AccountQASystem {
		t.Errorf("system prompt mismatch")
	}
	if !strings.Contains(r.User, "Which account has the most?") {
		t.Errorf("question not embedded: %q", r.User)
	}
	if !strings.Contains(r.User, "Savings: $5,000") {
		t.Errorf("context not embedded: %q", r.User)
	}
	// The question should be trimmed.
	if strings.Contains(r.User, "  Which") {
		t.Errorf("question not trimmed: %q", r.User)
	}
}
