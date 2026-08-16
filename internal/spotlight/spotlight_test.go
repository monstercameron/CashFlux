// SPDX-License-Identifier: MIT

package spotlight

import (
	"strings"
	"testing"
)

func TestARealQuestionFindsItsControl(t *testing.T) {
	for _, tc := range []struct {
		ask  string
		want string
	}{
		{"how do I add a new income source?", "add-recurring"},
		{"where do I import my bank statement?", "import-csv"},
		{"I want to set up a budget", "add-budget"},
		{"how do I turn on AI?", "ai-key"},
		{"where's dark mode?", "appearance"},
		{"can I see what changed?", "activity"},
	} {
		got, ok := Find(tc.ask)
		if !ok {
			t.Errorf("Find(%q) found nothing", tc.ask)
			continue
		}
		if got.ID != tc.want {
			t.Errorf("Find(%q) = %s, want %s", tc.ask, got.ID, tc.want)
		}
	}
}

func TestTheLongestMatchWins(t *testing.T) {
	// "add a subscription" must not lose to a target that merely contains "add".
	got, ok := Find("I want to add a subscription")
	if !ok || got.ID != "add-recurring" {
		t.Fatalf("Find = %s/%v, want add-recurring", got.ID, ok)
	}
}

func TestAnUnmatchedRequestSaysSoRatherThanGuessing(t *testing.T) {
	// Pointing confidently at the wrong control is worse than admitting ignorance:
	// the person follows the wrong instruction all the way to its end.
	for _, ask := range []string{"", "   ", "how do I refinance my mortgage in Belgium"} {
		if got, ok := Find(ask); ok {
			t.Errorf("Find(%q) guessed %s", ask, got.ID)
		}
	}
}

func TestEveryTargetIsUsableAsAnInstruction(t *testing.T) {
	for _, tr := range All() {
		if strings.TrimSpace(tr.ID) == "" {
			t.Error("a target has no id")
		}
		if !strings.HasPrefix(tr.Route, "/") {
			t.Errorf("%s has route %q, which is not an app route", tr.ID, tr.Route)
		}
		if strings.TrimSpace(tr.What) == "" {
			t.Errorf("%s has no description, so the step cannot be narrated", tr.ID)
		}
	}
}

func TestIDsAreUniqueSoTheModelCannotAmbiguouslyName_One(t *testing.T) {
	seen := map[string]bool{}
	for _, tr := range All() {
		if seen[tr.ID] {
			t.Errorf("duplicate target id %q", tr.ID)
		}
		seen[tr.ID] = true
	}
}

func TestGetFindsATargetByIDCaseInsensitively(t *testing.T) {
	if _, ok := Get("ADD-ACCOUNT"); !ok {
		t.Fatal("Get is case sensitive")
	}
	if _, ok := Get("nope"); ok {
		t.Fatal("Get invented a target")
	}
}

func TestNamesListsEveryTargetSoTheModelNeedNotInventOne(t *testing.T) {
	names := Names()
	if len(names) != len(All()) {
		t.Fatalf("Names has %d entries, targets has %d", len(names), len(All()))
	}
	for _, n := range names {
		if _, ok := Get(n); !ok {
			t.Errorf("Names lists %q, which Get cannot resolve", n)
		}
	}
}
