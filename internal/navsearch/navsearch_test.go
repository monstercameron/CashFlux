// SPDX-License-Identifier: MIT

package navsearch

import (
	"strings"
	"testing"
)

// rail mirrors the real sidebar closely enough that the ordering assertions below
// mean something: two destinations sharing a word, a section whose name collides
// with a destination's, and user-named pages that the household could call
// anything.
func rail() []Item {
	return []Item{
		{Label: "Dashboard", Path: "/", Section: "Primary"},
		{Label: "Transactions", Path: "/transactions", Section: "Primary"},
		{Label: "Accounts", Path: "/accounts", Section: "Primary"},
		{Label: "Budgets", Path: "/budgets", Section: "Primary"},
		{Label: "Goals", Path: "/goals", Section: "Primary"},
		{Label: "Reports", Path: "/reports", Section: "Understand"},
		{Label: "Net worth", Path: "/networth", Section: "Understand"},
		{Label: "Planning", Path: "/planning", Section: "Plan & forecast"},
		{Label: "Annual budget grid", Path: "/annual", Section: "Plan & forecast"},
		{Label: "Settings", Path: "/settings", Section: "System"},
		{Label: "Side hustle", Path: "/p/side-hustle", Section: "My pages"},
	}
}

func labels(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func TestEmptyQueryReturnsNothing(t *testing.T) {
	// Not "everything": the caller reads nil as "not searching" and renders the
	// normal rail with its sections and drag handles. Returning the full list would
	// make an untouched box look like a filter that had matched all of it.
	for _, q := range []string{"", "   ", "\t"} {
		if got := Filter(rail(), q); got != nil {
			t.Errorf("Filter(%q) = %v, want nil", q, labels(got))
		}
	}
}

func TestPrefixOutranksSubstring(t *testing.T) {
	// The reason the package exists. "bud" in menu order puts "Annual budget grid"
	// above "Budgets", so the reader has to scan past the thing they asked for.
	got := labels(Filter(rail(), "bud"))
	if len(got) < 2 {
		t.Fatalf("expected both budget destinations, got %v", got)
	}
	if got[0] != "Budgets" {
		t.Errorf("ranked %q first for \"bud\"; want Budgets (prefix beats substring). Full: %v", got[0], got)
	}
}

func TestWordStartBeatsMidWord(t *testing.T) {
	// "worth" starts the second word of "Net worth" — deliberate. Nothing else in
	// the fixture contains it mid-word, so this pins the tier rather than the order.
	got := labels(Filter(rail(), "worth"))
	if len(got) != 1 || got[0] != "Net worth" {
		t.Errorf("Filter(\"worth\") = %v, want [Net worth]", got)
	}
}

func TestExactLabelWinsOutright(t *testing.T) {
	got := labels(Filter(rail(), "goals"))
	if len(got) == 0 || got[0] != "Goals" {
		t.Errorf("Filter(\"goals\") = %v, want Goals first", got)
	}
}

func TestSectionMatchesGatherThatSection(t *testing.T) {
	// Typing a section name is a legitimate way to ask "what is in there?", so its
	// contents come back — but ranked below any destination matching by name.
	got := labels(Filter(rail(), "understand"))
	if len(got) != 2 {
		t.Fatalf("Filter(\"understand\") = %v, want both Understand destinations", got)
	}
	for _, want := range []string{"Reports", "Net worth"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q missing from %v", want, got)
		}
	}
}

func TestLabelMatchOutranksSectionMatch(t *testing.T) {
	// "Planning" is a destination AND "Plan & forecast" is a section. The
	// destination has to come first, or typing its exact name buries it under its
	// own neighbours.
	got := labels(Filter(rail(), "plan"))
	if len(got) == 0 {
		t.Fatal("no matches for \"plan\"")
	}
	if got[0] != "Planning" {
		t.Errorf("ranked %q first for \"plan\"; want Planning (label beats section). Full: %v", got[0], got)
	}
}

func TestTermsAreAndedNotOred(t *testing.T) {
	// An OR here returns most of the rail for a two-word query, which is the
	// opposite of narrowing.
	got := labels(Filter(rail(), "net worth"))
	if len(got) != 1 || got[0] != "Net worth" {
		t.Errorf("Filter(\"net worth\") = %v, want exactly [Net worth]", got)
	}
	// And order within the query must not matter.
	if rev := labels(Filter(rail(), "worth net")); len(rev) != 1 || rev[0] != "Net worth" {
		t.Errorf("Filter(\"worth net\") = %v, want exactly [Net worth]", rev)
	}
}

func TestCaseAndSurroundingSpaceIgnored(t *testing.T) {
	for _, q := range []string{"BUDGETS", "  Budgets  ", "bUdGeTs"} {
		got := labels(Filter(rail(), q))
		if len(got) == 0 || got[0] != "Budgets" {
			t.Errorf("Filter(%q) = %v, want Budgets first", q, got)
		}
	}
}

func TestNoMatchReturnsEmpty(t *testing.T) {
	if got := Filter(rail(), "zzzz"); len(got) != 0 {
		t.Errorf("Filter(\"zzzz\") = %v, want no matches", labels(got))
	}
}

func TestPathIsNeverMatched(t *testing.T) {
	// Matching the route would make "/p/" pull in every custom page, and "p" pull
	// in a route substring nobody typed on purpose.
	if got := Filter(rail(), "/p/"); len(got) != 0 {
		t.Errorf("Filter(\"/p/\") matched paths: %v", labels(got))
	}
	if got := Filter(rail(), "networth"); len(got) != 0 {
		t.Errorf("Filter(\"networth\") matched a path: %v", labels(got))
	}
}

func TestUserNamedPagesAreSearchable(t *testing.T) {
	// Custom pages are the destinations most likely to be searched, because their
	// names are the household's own words and they sit in a collapsed section.
	got := labels(Filter(rail(), "hustle"))
	if len(got) != 1 || got[0] != "Side hustle" {
		t.Errorf("Filter(\"hustle\") = %v, want [Side hustle]", got)
	}
}

func TestEqualScoresKeepMenuOrder(t *testing.T) {
	// Ties must not reshuffle: a list that reorders for reasons the reader cannot
	// see is worse than one that never reorders at all.
	items := []Item{
		{Label: "Alpha report", Section: "S"},
		{Label: "Beta report", Section: "S"},
		{Label: "Gamma report", Section: "S"},
	}
	got := labels(Filter(items, "report"))
	want := "Alpha report Beta report Gamma report"
	if strings.Join(got, " ") != want {
		t.Errorf("tie order = %v, want menu order", got)
	}
}
