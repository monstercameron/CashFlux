// SPDX-License-Identifier: MIT

package batchresolve

import (
	"strconv"
	"testing"
)

// rows builds n rows for one payee/category pair, with unique txn ids.
func rows(payee, cat string, n int, prefix string) []Row {
	out := make([]Row, 0, n)
	for i := range n {
		out = append(out, Row{TxnID: prefix + strconv.Itoa(i), Payee: payee, CategoryID: cat})
	}
	return out
}

// The headline case: a big backlog is a handful of decisions repeated.
func TestBuildCompressesABacklogIntoAFewRules(t *testing.T) {
	var in []Row
	in = append(in, rows("Kroger", "c-groceries", 28, "k")...)
	in = append(in, rows("Shell", "c-fuel", 14, "s")...)
	in = append(in, rows("Netflix", "c-subs", 6, "n")...)

	p := Build(in)
	if len(p.Proposals) != 3 {
		t.Fatalf("got %d proposals, want 3: %+v", len(p.Proposals), p.Proposals)
	}
	// Biggest win first — that is the one worth reading first.
	if p.Proposals[0].Match != "Kroger" || p.Proposals[0].Count() != 28 {
		t.Errorf("first proposal = %+v", p.Proposals[0])
	}
	if p.TotalRows != 48 || p.ResolvedRows() != 48 || p.Unresolved != 0 {
		t.Errorf("plan = total %d resolved %d unresolved %d", p.TotalRows, p.ResolvedRows(), p.Unresolved)
	}
	if p.ReductionPct() != 100 {
		t.Errorf("ReductionPct = %d", p.ReductionPct())
	}
	for _, pr := range p.Proposals {
		if pr.Tier != TierConfident {
			t.Errorf("%q is %v, want confident (its rows all agree)", pr.Match, pr.Tier)
		}
		// The rows ARE the evidence; a proposal that cannot show them is asking to
		// be trusted.
		if len(pr.TxnIDs) != pr.Count() || pr.Count() == 0 {
			t.Errorf("%q carries no evidence rows", pr.Match)
		}
	}
}

// A merchant filed three different ways is not a rule; it is a question. The
// proposal still appears — the user should see it — but in the needs-you tier.
func TestDisagreeingRowsLandInNeedsYou(t *testing.T) {
	var in []Row
	in = append(in, rows("Target", "c-groceries", 6, "a")...)
	in = append(in, rows("Target", "c-home", 4, "b")...)

	p := Build(in)
	if len(p.Proposals) != 1 {
		t.Fatalf("got %d proposals", len(p.Proposals))
	}
	pr := p.Proposals[0]
	if pr.Tier != TierNeedsYou {
		t.Errorf("tier = %v, want needs-you at %.2f agreement", pr.Tier, pr.Agreement)
	}
	if pr.Disagreed != 4 {
		t.Errorf("Disagreed = %d, want 4 — the rows a user should look at", pr.Disagreed)
	}
	// The four dissenting rows are NOT resolved by this proposal, and saying so
	// is what stops the reduction figure from being a lie.
	if p.Unresolved != 4 {
		t.Errorf("Unresolved = %d, want 4", p.Unresolved)
	}
	if p.ResolvedRows() != 6 {
		t.Errorf("ResolvedRows = %d, want 6", p.ResolvedRows())
	}
	if p.ReductionPct() != 60 {
		t.Errorf("ReductionPct = %d, want 60", p.ReductionPct())
	}
}

// One outlier in twenty should not push the most useful proposal into the
// needs-you pile.
func TestOneOutlierStaysConfident(t *testing.T) {
	var in []Row
	in = append(in, rows("Kroger", "c-groceries", 19, "a")...)
	in = append(in, rows("Kroger", "c-gifts", 1, "b")...)

	pr := Build(in).Proposals[0]
	if pr.Tier != TierConfident {
		t.Errorf("tier = %v at %.3f agreement, want confident", pr.Tier, pr.Agreement)
	}
	if pr.CategoryID != "c-groceries" {
		t.Errorf("CategoryID = %q, want the majority", pr.CategoryID)
	}
}

// Two rows agreeing is a coincidence often enough that a rule built on it is one
// the user corrects later — and a wrong rule mis-files every future match.
func TestTooFewRowsProposesNothing(t *testing.T) {
	p := Build(rows("Rare Shop", "c-misc", MinRows-1, "r"))
	if len(p.Proposals) != 0 {
		t.Errorf("a %d-row merchant produced %+v", MinRows-1, p.Proposals)
	}
	if p.Unresolved != MinRows-1 {
		t.Errorf("Unresolved = %d, want %d", p.Unresolved, MinRows-1)
	}
	// Exactly MinRows is enough.
	if got := Build(rows("Rare Shop", "c-misc", MinRows, "r")); len(got.Proposals) != 1 {
		t.Errorf("a %d-row merchant produced %d proposals", MinRows, len(got.Proposals))
	}
}

// A plan that quietly ignores rows it cannot handle reports a reduction it did
// not achieve.
func TestRowsWithNoSuggestionCountAsUnresolved(t *testing.T) {
	in := append(rows("Kroger", "c-groceries", 4, "k"),
		Row{TxnID: "x1", Payee: "Mystery"},
		Row{TxnID: "x2", CategoryID: "c-something"},
	)
	p := Build(in)
	if p.TotalRows != 6 {
		t.Fatalf("TotalRows = %d", p.TotalRows)
	}
	if p.Unresolved != 2 {
		t.Errorf("Unresolved = %d, want 2", p.Unresolved)
	}
	if p.ResolvedRows() != 4 {
		t.Errorf("ResolvedRows = %d, want 4", p.ResolvedRows())
	}
}

// Store numbers are noise: "KROGER #418" and "Kroger #522" are one merchant.
func TestStoreNumbersAreFoldedIntoOneMerchant(t *testing.T) {
	in := []Row{
		{TxnID: "a", Payee: "KROGER #418", CategoryID: "c-groceries"},
		{TxnID: "b", Payee: "Kroger #522", CategoryID: "c-groceries"},
		{TxnID: "c", Payee: "kroger 1201", CategoryID: "c-groceries"},
	}
	p := Build(in)
	if len(p.Proposals) != 1 {
		t.Fatalf("got %d proposals, want 1: %+v", len(p.Proposals), p.Proposals)
	}
	if p.Proposals[0].Count() != 3 {
		t.Errorf("count = %d, want 3", p.Proposals[0].Count())
	}
}

// Merging two real merchants is worse than failing to merge two spellings of
// one: the first mis-files forever, the second leaves a row in the queue.
func TestALeadingNumberIsNotAStoreNumber(t *testing.T) {
	in := []Row{
		{TxnID: "a", Payee: "76 Gas", CategoryID: "c-fuel"},
		{TxnID: "b", Payee: "76 Gas", CategoryID: "c-fuel"},
		{TxnID: "c", Payee: "76 Gas", CategoryID: "c-fuel"},
		{TxnID: "d", Payee: "Gas", CategoryID: "c-fuel"},
		{TxnID: "e", Payee: "Gas", CategoryID: "c-fuel"},
		{TxnID: "f", Payee: "Gas", CategoryID: "c-fuel"},
	}
	p := Build(in)
	if len(p.Proposals) != 2 {
		t.Fatalf("got %d proposals, want 2 — two merchants were merged: %+v",
			len(p.Proposals), p.Proposals)
	}
}

func TestTierSplitAndCounts(t *testing.T) {
	var in []Row
	in = append(in, rows("Clear", "c-a", 10, "c")...)
	in = append(in, rows("Muddy", "c-a", 5, "m")...)
	in = append(in, rows("Muddy", "c-b", 4, "n")...)

	p := Build(in)
	if len(p.Confident()) != 1 || p.Confident()[0].Match != "Clear" {
		t.Errorf("Confident = %+v", p.Confident())
	}
	if len(p.NeedsYou()) != 1 || p.NeedsYou()[0].Match != "Muddy" {
		t.Errorf("NeedsYou = %+v", p.NeedsYou())
	}
	if p.ConfidentRows() != 10 {
		t.Errorf("ConfidentRows = %d, want 10", p.ConfidentRows())
	}
}

func TestOrderIsStable(t *testing.T) {
	var in []Row
	in = append(in, rows("Bravo", "c-a", 5, "b")...)
	in = append(in, rows("Alpha", "c-a", 5, "a")...)

	first, second := Build(in), Build(in)
	for i := range first.Proposals {
		if first.Proposals[i].Match != second.Proposals[i].Match {
			t.Fatalf("order changed between identical runs at %d", i)
		}
	}
	// Equal counts break on the match string, so the list does not reshuffle.
	if first.Proposals[0].Match != "Alpha" {
		t.Errorf("tie broken as %q, want Alpha", first.Proposals[0].Match)
	}
}

func TestEmptyBacklog(t *testing.T) {
	p := Build(nil)
	if len(p.Proposals) != 0 || p.TotalRows != 0 {
		t.Errorf("Build(nil) = %+v", p)
	}
	// Never divide by an empty backlog.
	if p.ReductionPct() != 0 {
		t.Errorf("ReductionPct on an empty backlog = %d", p.ReductionPct())
	}
}

func TestProposalRendersARule(t *testing.T) {
	pr := Build(rows("Kroger", "c-groceries", 4, "k")).Proposals[0]
	r := pr.Rule("rule-1")
	if r.ID != "rule-1" || r.Match != "Kroger" || r.SetCategoryID != "c-groceries" {
		t.Errorf("Rule = %+v", r)
	}
}
