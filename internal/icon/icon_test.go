package icon

import (
	"strings"
	"testing"
)

// curated is every exported constant; keep in sync so the tests fail loudly if a
// constant is added without data (or vice versa).
var curated = []Name{
	Dashboard, Accounts, Transactions, Budgets, Goals, Todo, Settings, Page,
	Plus, Menu, Tag, Users, Planning, Allocate, Insights, Customize,
}

func TestEveryConstantResolves(t *testing.T) {
	for _, n := range curated {
		if !n.Valid() {
			t.Errorf("%q is not Valid", n)
		}
		body := n.Inner()
		if strings.TrimSpace(body) == "" {
			t.Errorf("%q has empty Inner markup", n)
		}
		// Sanity: inner markup should be SVG-ish (an opening tag, no <svg> wrapper).
		if !strings.Contains(body, "<") {
			t.Errorf("%q Inner doesn't look like markup: %q", n, body)
		}
		if strings.Contains(body, "<svg") {
			t.Errorf("%q Inner should be inner shapes only, not a full <svg>", n)
		}
	}
}

func TestValidAndInnerForUnknown(t *testing.T) {
	var u Name = "definitely-not-an-icon"
	if u.Valid() {
		t.Error("unknown name reported Valid")
	}
	if u.Inner() != "" {
		t.Errorf("unknown name Inner = %q, want empty", u.Inner())
	}
}

func TestAllMatchesCuratedSet(t *testing.T) {
	all := All()
	if len(all) != len(curated) {
		t.Fatalf("All() has %d names, curated has %d", len(all), len(curated))
	}
	// All() must be sorted and contain exactly the curated set.
	seen := map[Name]bool{}
	for i, n := range all {
		if i > 0 && all[i-1] > n {
			t.Errorf("All() not sorted at %d: %q before %q", i, all[i-1], n)
		}
		if !n.Valid() {
			t.Errorf("All() returned invalid name %q", n)
		}
		seen[n] = true
	}
	for _, n := range curated {
		if !seen[n] {
			t.Errorf("All() missing curated name %q", n)
		}
	}
}
