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

func TestNewBuilders(t *testing.T) {
	cases := []struct {
		name string
		r    Request
		want string // a substring that must appear in User
	}{
		{"goal", GoalDraft("save for a $6k Japan trip", "surplus $500/mo"), "Japan trip"},
		{"health", AccountHealth("Checking: $1,000"), "Checking: $1,000"},
		{"overlap", OverlapDetect("Spotify $10\nApple Music $11"), "Apple Music"},
		{"alloc", AllocationIntent("pay the card, keep $1k liquid", "Visa $5,000"), "keep $1k liquid"},
		{"scenario", ScenarioDraft("what if I get a $500 raise", "net $42k"), "$500 raise"},
		{"todo", TodoParse("move $200 to savings next Friday"), "move $200 to savings"},
		{"import", ImportMapping("Date,Description,Amount"), "Date,Description,Amount"},
	}
	for _, c := range cases {
		if c.r.System == "" {
			t.Errorf("%s: empty system prompt", c.name)
		}
		if !strings.Contains(c.r.User, c.want) {
			t.Errorf("%s: User %q missing %q", c.name, c.r.User, c.want)
		}
	}
}

func TestAllImplementedHaveNonEmptyCodes(t *testing.T) {
	for _, c := range ImplementedCodes() {
		if c == "" {
			t.Errorf("empty implemented code")
		}
	}
	if len(ImplementedCodes()) < 8 {
		t.Errorf("expected >= 8 implemented AI features, got %d", len(ImplementedCodes()))
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

func TestParseRuleSuggestions(t *testing.T) {
	cats := map[string]string{"Dining": "cat-dining", "Groceries": "cat-groc"}
	answer := "" +
		"uber eats => Dining\n" +
		"- \"Greenfield Market\" => groceries\n" + // bullet + quotes + case-insensitive category
		"xy => Dining\n" + // phrase too short
		"casino => Gambling\n" + // invented category → dropped
		"UBER EATS => Dining\n" + // duplicate phrase → dropped
		"not a rule line\n"
	got := ParseRuleSuggestions(answer, cats)
	if len(got) != 2 {
		t.Fatalf("parsed %d suggestions, want 2: %+v", len(got), got)
	}
	if got[0].Match != "uber eats" || got[0].CategoryID != "cat-dining" {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Match != "Greenfield Market" || got[1].CategoryID != "cat-groc" || got[1].CategoryName != "Groceries" {
		t.Fatalf("second = %+v", got[1])
	}
}

func TestParseRuleSuggestionsCap(t *testing.T) {
	cats := map[string]string{"Dining": "d"}
	var b string
	for i := 0; i < 10; i++ {
		b += string(rune('a'+i)) + "aaa => Dining\n"
	}
	if got := ParseRuleSuggestions(b, cats); len(got) != 6 {
		t.Fatalf("cap = %d, want 6", len(got))
	}
}
