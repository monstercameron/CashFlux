// SPDX-License-Identifier: MIT

package methodology

import (
	"strings"
	"testing"
)

// The whole point of the package is that a benchmark arrives with its source. A
// note that judges a figure against an unattributed number is the exact defect
// this exists to fix, so it fails here rather than shipping.
func TestEveryBenchmarkNamesItsSource(t *testing.T) {
	for _, n := range All() {
		for _, b := range n.Benchmarks {
			if strings.TrimSpace(b.SourceKey) == "" {
				t.Errorf("%s: benchmark %q has no source", n.SectionID, b.LabelKey)
			}
			if strings.TrimSpace(b.LabelKey) == "" || strings.TrimSpace(b.ValueKey) == "" {
				t.Errorf("%s: benchmark is missing a label or value: %+v", n.SectionID, b)
			}
		}
	}
}

// Every money-flow section must declare the transfer exclusion. Transfers are
// the single most common reason a total does not match what someone believes
// they spent, and a section that stays quiet about them is the one that gets
// disbelieved.
func TestMoneyFlowSectionsDeclareTheTransferExclusion(t *testing.T) {
	for _, id := range []string{"rpta-01", "rpta-02", "rpta-03", "rpta-04", "rpta-05", "rpta-07"} {
		n, ok := For(id)
		if !ok {
			t.Fatalf("%s has no methodology note", id)
		}
		found := false
		for _, e := range n.ExclusionKeys {
			if e == transfersExcluded {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not declare that transfers are excluded", id)
		}
	}
}

func TestEveryNoteHasContentAndAUniqueSection(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range All() {
		if n.SectionID == "" || seen[n.SectionID] {
			t.Errorf("section id %q is empty or duplicated", n.SectionID)
		}
		seen[n.SectionID] = true
		if !n.HasContent() {
			t.Errorf("%s: an empty note would open a drawer onto nothing", n.SectionID)
		}
		if len(n.FormulaKeys) == 0 {
			t.Errorf("%s: no formula — a drawer that only lists exclusions does not explain the figure", n.SectionID)
		}
	}
}

func TestFor(t *testing.T) {
	if _, ok := For("rpta-08"); !ok {
		t.Error("a known section did not resolve")
	}
	// The appendix has no computed figures of its own, so it has no note — and
	// asking for one must report absence rather than an empty drawer.
	if n, ok := For("rpta-11"); ok {
		t.Errorf("the appendix resolved to %+v", n)
	}
	if _, ok := For("nope"); ok {
		t.Error("an unknown section resolved")
	}
}

func TestZeroNoteHasNoContent(t *testing.T) {
	if (Note{}).HasContent() {
		t.Error("the zero Note claimed to have content")
	}
}
