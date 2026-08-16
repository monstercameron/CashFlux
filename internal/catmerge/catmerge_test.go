// SPDX-License-Identifier: MIT

package catmerge

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// fixture builds a household where EVERY kind of reference points at "old", so
// a sweep that misses one collection cannot pass.
func fixture() Refs {
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	return Refs{
		Transactions: []domain.Transaction{
			{ID: "t1", Desc: "plain", CategoryID: "old", Amount: usd(-1000)},
			{ID: "t2", Desc: "elsewhere", CategoryID: "keep", Amount: usd(-2000)},
			{ID: "t3", Desc: "split", CategoryID: "keep", Amount: usd(-3000), Splits: []domain.CategorySplit{
				{CategoryID: "old", Amount: usd(-1000)},
				{CategoryID: "other", Amount: usd(-2000)},
			}},
			{ID: "t4", Desc: "two old lines", CategoryID: "old", Amount: usd(-4000), Splits: []domain.CategorySplit{
				{CategoryID: "old", Amount: usd(-1000), MemberID: "m1"},
				{CategoryID: "old", Amount: usd(-3000), MemberID: "m2"},
			}},
		},
		Budgets: []domain.Budget{
			{ID: "b1", Name: "single", CategoryID: "old"},
			{ID: "b2", Name: "multi", CategoryID: "keep", CategoryIDs: []string{"keep", "old", "other"}},
			{ID: "b3", Name: "already both", CategoryID: "new", CategoryIDs: []string{"new", "old"}},
			{ID: "b4", Name: "unrelated", CategoryID: "other"},
		},
		Goals: []domain.Goal{
			{ID: "g1", Name: "fund", CategoryID: "old", IsSinkingFund: true},
			{ID: "g2", Name: "other", CategoryID: "other"},
		},
		Rules: []rules.Rule{
			{ID: "r1", Match: "shop", SetCategoryID: "old"},
			{ID: "r2", Match: "gas", SetCategoryID: "other"},
		},
		Recurring: []domain.Recurring{
			{ID: "rc1", CategoryID: "old"},
			{ID: "rc2", CategoryID: "other"},
		},
		Categories: []domain.Category{
			{ID: "old", Name: "Old", Kind: domain.KindExpense},
			{ID: "new", Name: "New", Kind: domain.KindExpense},
			{ID: "kid", Name: "Kid", Kind: domain.KindExpense, ParentID: "old"},
			{ID: "other", Name: "Other", Kind: domain.KindExpense},
		},
	}
}

func TestPlanCountsEveryKindOfReference(t *testing.T) {
	got := Plan(fixture(), "old", "new")
	want := Counts{Transactions: 2, Splits: 3, Budgets: 3, Goals: 1, Rules: 1, Recurring: 1, Children: 1}
	if got != want {
		t.Fatalf("Plan = %+v, want %+v", got, want)
	}
	if got.Total() != 12 {
		t.Errorf("Total = %d, want 12", got.Total())
	}
}

func TestPlanIsANoOpForSelfOrEmpty(t *testing.T) {
	if c := Plan(fixture(), "old", "old"); !c.Empty() {
		t.Errorf("merging a category into itself must move nothing, got %+v", c)
	}
	if c := Plan(fixture(), "", "new"); !c.Empty() {
		t.Errorf("an empty source must move nothing, got %+v", c)
	}
}

// TestMergeLeavesNoResidual is the guarantee the whole package exists for: after
// a merge, nothing anywhere still points at the retired category.
func TestMergeLeavesNoResidual(t *testing.T) {
	out, c := Merge(fixture(), "old", "new")
	if c.Total() != 12 {
		t.Fatalf("counts = %+v", c)
	}
	if res := Residual(out, "old"); !res.Empty() {
		t.Fatalf("references to the retired category survived: %+v", res)
	}
}

func TestMergeMovesEachCollection(t *testing.T) {
	out, _ := Merge(fixture(), "old", "new")

	byID := map[string]domain.Transaction{}
	for _, tx := range out.Transactions {
		byID[tx.ID] = tx
	}
	if byID["t1"].CategoryID != "new" {
		t.Error("a plain transaction must move")
	}
	if byID["t2"].CategoryID != "keep" {
		t.Error("an unrelated transaction must not move")
	}
	if byID["t3"].Splits[0].CategoryID != "new" || byID["t3"].Splits[1].CategoryID != "other" {
		t.Errorf("split lines must move individually: %+v", byID["t3"].Splits)
	}
	// Two lines that both named the retired category stay TWO lines: each can
	// carry its own member attribution, and collapsing them would silently
	// reassign whose spending it was.
	if len(byID["t4"].Splits) != 2 {
		t.Errorf("same-category split lines must not be collapsed: %+v", byID["t4"].Splits)
	}
	if byID["t4"].Splits[0].MemberID != "m1" || byID["t4"].Splits[1].MemberID != "m2" {
		t.Error("member attribution must survive the merge")
	}

	bud := map[string]domain.Budget{}
	for _, b := range out.Budgets {
		bud[b.ID] = b
	}
	if bud["b1"].CategoryID != "new" {
		t.Error("a single-category budget must move")
	}
	if got := bud["b2"].CategoryIDs; len(got) != 3 || got[1] != "new" {
		t.Errorf("a multi-category budget's list must move in place: %v", got)
	}
	// A budget that already tracked the destination must not end up tracking it
	// twice — that would double-count the category's spend against the cap.
	if got := bud["b3"].CategoryIDs; len(got) != 1 || got[0] != "new" {
		t.Errorf("the duplicate must be dropped: %v", got)
	}

	if out.Goals[0].CategoryID != "new" {
		t.Error("a sinking-fund goal must move")
	}
	if out.Rules[0].SetCategoryID != "new" {
		t.Error("a rule that files into the category must move")
	}
	if out.Recurring[0].CategoryID != "new" {
		t.Error("a recurring template must move")
	}
}

// TestMergeRehomesChildrenOntoTheSurvivor: a merge says "these are the same
// thing", so the children belong to the thing that survives — not to the retired
// category's parent, which is where a plain delete would put them.
func TestMergeRehomesChildren(t *testing.T) {
	out, _ := Merge(fixture(), "old", "new")
	for _, c := range out.Categories {
		if c.ID == "kid" {
			if c.ParentID != "new" {
				t.Fatalf("child re-homed onto %q, want new", c.ParentID)
			}
			return
		}
	}
	t.Fatal("the child category disappeared")
}

func TestMergeRemovesTheSourceCategory(t *testing.T) {
	out, _ := Merge(fixture(), "old", "new")
	for _, c := range out.Categories {
		if c.ID == "old" {
			t.Fatal("the merged category must stop existing")
		}
	}
	if len(out.Categories) != 3 {
		t.Errorf("got %d categories, want 3", len(out.Categories))
	}
}

// TestMergeDoesNotMutateInput guards the pure-function contract: a preview must
// be safe to compute without changing anything.
func TestMergeDoesNotMutateInput(t *testing.T) {
	in := fixture()
	_, _ = Merge(in, "old", "new")
	if in.Transactions[0].CategoryID != "old" {
		t.Error("Merge mutated the caller's transactions")
	}
	if in.Transactions[2].Splits[0].CategoryID != "old" {
		t.Error("Merge mutated the caller's split lines")
	}
	if in.Budgets[1].CategoryIDs[1] != "old" {
		t.Error("Merge mutated the caller's budget list")
	}
	if len(in.Categories) != 4 {
		t.Error("Merge mutated the caller's categories")
	}
}

func TestMergeIntoItselfChangesNothing(t *testing.T) {
	in := fixture()
	out, c := Merge(in, "old", "old")
	if !c.Empty() {
		t.Errorf("counts = %+v, want empty", c)
	}
	if len(out.Categories) != len(in.Categories) {
		t.Error("merging into itself must not delete the category")
	}
}

// TestMergeOfAnUnreferencedCategoryStillRetiresIt: a category nothing points at
// is the easiest case to get wrong, because the counts are all zero and an
// early return would skip the deletion.
func TestMergeOfAnUnreferencedCategoryStillRetiresIt(t *testing.T) {
	r := Refs{Categories: []domain.Category{
		{ID: "lonely", Name: "Lonely", Kind: domain.KindExpense},
		{ID: "new", Name: "New", Kind: domain.KindExpense},
	}}
	out, c := Merge(r, "lonely", "new")
	if !c.Empty() {
		t.Errorf("counts = %+v, want empty", c)
	}
	if len(out.Categories) != 1 || out.Categories[0].ID != "new" {
		t.Fatalf("the unreferenced category must still be retired: %+v", out.Categories)
	}
}

func TestMergeHandlesEmptyRefs(t *testing.T) {
	out, c := Merge(Refs{}, "old", "new")
	if !c.Empty() {
		t.Errorf("counts = %+v", c)
	}
	if len(out.Categories) != 0 {
		t.Error("nothing in, nothing out")
	}
}

// ─── C548: the test the AC actually asked for ────────────────────────────────
//
// TestMergeLeavesNoResidual cannot serve as the proof. Residual walks the same
// collections Merge just rewrote, so it is circular: it can only ever report on
// what Refs already knows about, and it is blind by construction to any
// reference class the sweep forgot. That is exactly how the learned-corrections
// tally and the saved widget filters survived a merge for as long as they did.
//
// This is the honest version: serialize everything and look for the retired id
// as a STRING. It knows nothing about the shape of the data, so it catches a
// reference wherever it hides.
func TestMergeLeavesNoTraceInSerializedData(t *testing.T) {
	const from, to = "cat-old", "cat-new"

	r := Refs{
		Categories: []domain.Category{
			{ID: from, Name: "Groceries (old)", Kind: domain.KindExpense},
			{ID: to, Name: "Groceries", Kind: domain.KindExpense},
			{ID: "cat-child", Name: "Produce", ParentID: from, Kind: domain.KindExpense},
		},
		Transactions: []domain.Transaction{
			{ID: "t1", CategoryID: from},
			{ID: "t2", Splits: []domain.CategorySplit{{CategoryID: from}, {CategoryID: to}}},
		},
		Budgets: []domain.Budget{
			{ID: "b1", CategoryID: from},
			{ID: "b2", CategoryIDs: []string{from, "cat-other"}},
		},
		Goals:     []domain.Goal{{ID: "g1", CategoryID: from}},
		Rules:     []rules.Rule{{ID: "r1", SetCategoryID: from}},
		Recurring: []domain.Recurring{{ID: "rec1", CategoryID: from}},
		// The two classes C548 is about.
		Tally: learntally.Tally{
			"whole foods": {from: 11, "cat-other": 2},
		},
		Widgets: []domain.WidgetSpec{{
			ID: "w1", Kind: domain.KindChart,
			Pipeline: &domain.Pipeline{Source: domain.Source{
				Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "flow", Filter: "cat:" + from},
			}},
		}},
	}

	// Guard against a vacuous scan: the id must be in there BEFORE the merge, or
	// "not found afterwards" proves nothing.
	if before, err := json.Marshal(r); err != nil {
		t.Fatalf("marshal input: %v", err)
	} else if !bytes.Contains(before, []byte(from)) {
		t.Fatal("the fixture does not reference the retired id — the scan below would pass vacuously")
	}

	out, counts := Merge(r, from, to)
	if counts.Tally != 1 {
		t.Errorf("Counts.Tally = %d, want 1 — the confirmation must say the learned "+
			"corrections move too", counts.Tally)
	}
	if counts.Widgets != 1 {
		t.Errorf("Counts.Widgets = %d, want 1", counts.Widgets)
	}

	blob, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(blob, []byte(from)) {
		t.Errorf("the retired id %q still appears in the serialized data after a merge:\n%s",
			from, blob)
	}

	// And the merge moved the meaning, not just the pointer: eleven corrections
	// taught against the old id belong to the survivor, added to what it already
	// had — a household that corrected a payee eleven times has taught the app
	// something real, and a merge is a rename from where they sit.
	if got := out.Tally["whole foods"][to]; got != 11 {
		t.Errorf("tally for the surviving category = %d, want 11", got)
	}
	if got := out.Tally["whole foods"]["cat-other"]; got != 2 {
		t.Errorf("an unrelated tally entry changed: %d, want 2", got)
	}
	if got := out.Widgets[0].Pipeline.Source.Series.Filter; got != "cat:"+to {
		t.Errorf("widget filter = %q, want %q", got, "cat:"+to)
	}
	// The input must not have been mutated through the shared pipeline pointer.
	if got := r.Widgets[0].Pipeline.Source.Series.Filter; got != "cat:"+from {
		t.Errorf("Merge reached into the caller's spec: filter is now %q", got)
	}
}

// A tally counted against BOTH categories folds into one entry rather than
// losing either half.
func TestMergeFoldsOverlappingTallies(t *testing.T) {
	const from, to = "cat-old", "cat-new"
	r := Refs{
		Categories: []domain.Category{{ID: from}, {ID: to}},
		Tally:      learntally.Tally{"costco": {from: 4, to: 3}},
	}
	out, _ := Merge(r, from, to)
	if got := out.Tally["costco"][to]; got != 7 {
		t.Errorf("folded tally = %d, want 7 (4 + 3)", got)
	}
	if _, still := out.Tally["costco"][from]; still {
		t.Error("the retired id survives in the tally")
	}
}

// A filter that names a category by DISPLAY NAME is deliberately left alone:
// widgetsource accepts either form, the name still means what it said, and the
// surviving category may legitimately be called something else.
func TestMergeLeavesNameBasedFiltersAlone(t *testing.T) {
	const from, to = "cat-old", "cat-new"
	r := Refs{
		Categories: []domain.Category{{ID: from, Name: "Groceries (old)"}, {ID: to, Name: "Groceries"}},
		Widgets: []domain.WidgetSpec{{ID: "w1", Pipeline: &domain.Pipeline{
			Source: domain.Source{Series: domain.SeriesSpec{Metric: "flow", Filter: "cat:Groceries"}},
		}}},
	}
	out, counts := Merge(r, from, to)
	if counts.Widgets != 0 {
		t.Errorf("Counts.Widgets = %d, want 0 for a name-based filter", counts.Widgets)
	}
	if got := out.Widgets[0].Pipeline.Source.Series.Filter; got != "cat:Groceries" {
		t.Errorf("a name-based filter was rewritten to %q", got)
	}
}

// A spec with no pipeline at all (a KPI, a native tile) must not panic the sweep.
func TestMergeToleratesSpecsWithoutPipelines(t *testing.T) {
	const from, to = "cat-old", "cat-new"
	r := Refs{
		Categories: []domain.Category{{ID: from}, {ID: to}},
		Widgets: []domain.WidgetSpec{
			{ID: "kpi", Kind: domain.KindKPI, Scalar: &domain.ScalarBind{Expr: "net_worth"}},
			{ID: "native", Kind: domain.KindNative, NativeID: "bills"},
		},
	}
	out, counts := Merge(r, from, to)
	if counts.Widgets != 0 || len(out.Widgets) != 2 {
		t.Errorf("counts=%d widgets=%d, want 0 and 2", counts.Widgets, len(out.Widgets))
	}
}
