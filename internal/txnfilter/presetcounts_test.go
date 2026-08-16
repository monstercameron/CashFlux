// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// presetFixture is a small ledger with a deliberate split between what a search
// matches and what it does not, so a global count and a scoped count differ.
//
//	id  desc             date     amount    category  tags
//	a   CF26-PIPE one    Aug 3    -25.00    —         —
//	b   CF26-PIPE two    Aug 4   -500.00    —         —          (large)
//	c   CF26-PIPE three  Aug 5    -10.00    cat1      needs-review
//	d   Groceries        Aug 6    -90.00    —         —
//	e   Rent             Aug 7  -2000.00    cat2      —          (large)
//	f   Old charge       Jul 10   -30.00    —         needs-review
func presetFixture() []domain.Transaction {
	aug := func(day int) time.Time { return time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC) }
	july := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	return []domain.Transaction{
		{ID: "a", Desc: "CF26-PIPE one", Date: aug(3), Amount: usd(-2500)},
		{ID: "b", Desc: "CF26-PIPE two", Date: aug(4), Amount: usd(-50000)},
		{ID: "c", Desc: "CF26-PIPE three", Date: aug(5), Amount: usd(-1000), CategoryID: "cat1", Tags: []string{"needs-review"}},
		{ID: "d", Desc: "Groceries", Date: aug(6), Amount: usd(-9000)},
		{ID: "e", Desc: "Rent", Date: aug(7), Amount: usd(-200000), CategoryID: "cat2"},
		{ID: "f", Desc: "Old charge", Date: july, Amount: usd(-3000), Tags: []string{"needs-review"}},
	}
}

func presetScope() PresetScope {
	return PresetScope{
		ReviewTag:  "needs-review",
		LargeFor:   func(domain.Transaction) int64 { return 10000 }, // $100.00
		MonthStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		MonthEnd:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestCountPresetsHonoursTheActiveScope(t *testing.T) {
	txns := presetFixture()
	tests := []struct {
		name string
		crit Criteria
		want PresetCounts
	}{
		{
			name: "no filters counts the whole ledger",
			// a, b, d, f carry no category; b and e clear $100; a–e are in August.
			crit: Criteria{},
			want: PresetCounts{Uncategorized: 4, NeedsReview: 2, Large: 2, ThisMonth: 5},
		},
		{
			// The C568 case. Globally this reads 4 / 2 / 2 / 5; inside the search it is
			// 2 / 1 / 1 / 3, and those are the rows a click would actually leave.
			name: "a text search narrows every count",
			crit: Criteria{Text: "CF26-PIPE"},
			want: PresetCounts{Uncategorized: 2, NeedsReview: 1, Large: 1, ThisMonth: 3},
		},
		{
			// Each count answers "what would I have if I turned THIS on, on top of
			// everything already narrowed". With Uncategorized engaged, the only
			// needs-review row in the search (c) has a category, so adding that filter
			// really does leave nothing — and the chip says so instead of promising 1.
			name: "an engaged preset narrows its neighbours, not itself",
			crit: Criteria{Text: "CF26-PIPE", Uncategorized: true},
			want: PresetCounts{Uncategorized: 2, NeedsReview: 0, Large: 1, ThisMonth: 2},
		},
		{
			// Same rule from the other side: Large engaged keeps reporting its own
			// population (1) while the others report what remains within it.
			name: "large engaged reports its own population",
			crit: Criteria{Text: "CF26-PIPE", AmountMin: "100"},
			want: PresetCounts{Uncategorized: 1, NeedsReview: 0, Large: 1, ThisMonth: 1},
		},
		{
			// A range holding nothing empties the neighbours; This month still reports
			// its own population, because the date bounds are the dimension it writes.
			name: "an empty date range empties the other counts",
			crit: Criteria{From: "2026-01-01", To: "2026-01-31"},
			want: PresetCounts{Uncategorized: 0, NeedsReview: 0, Large: 0, ThisMonth: 5},
		},
		{
			name: "a July range leaves only the July charge for the neighbours",
			crit: Criteria{From: "2026-07-01", To: "2026-07-31"},
			want: PresetCounts{Uncategorized: 1, NeedsReview: 1, Large: 0, ThisMonth: 5},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CountPresets(txns, tc.crit.Normalize(), presetScope())
			if got != tc.want {
				t.Errorf("CountPresets() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The ticket names search, member and period as the scopes that must be honoured.
// Search is covered above; these are the other two, plus the multi-value
// dimensions the filter panel writes as pills.
func TestCountPresetsHonoursMemberAndAccountScopes(t *testing.T) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	aug := func(day int) time.Time { return time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC) }
	txns := []domain.Transaction{
		{ID: "p1", Desc: "Priya coffee", Date: aug(2), Amount: usd(-500), MemberID: "m-priya", AccountID: "acc-1"},
		{ID: "p2", Desc: "Priya laptop", Date: aug(3), Amount: usd(-150000), MemberID: "m-priya", AccountID: "acc-1"},
		{ID: "m1", Desc: "Marcus lunch", Date: aug(4), Amount: usd(-1800), MemberID: "m-marcus", AccountID: "acc-2"},
		{ID: "m2", Desc: "Marcus tyres", Date: aug(5), Amount: usd(-60000), MemberID: "m-marcus", AccountID: "acc-2"},
	}
	scope := PresetScope{
		ReviewTag:  "needs-review",
		LargeFor:   func(domain.Transaction) int64 { return 10000 },
		MonthStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		MonthEnd:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}

	all := CountPresets(txns, Criteria{}.Normalize(), scope)
	if all.Uncategorized != 4 || all.Large != 2 || all.ThisMonth != 4 {
		t.Fatalf("unscoped = %+v, want 4 uncategorized / 2 large / 4 this month", all)
	}

	// A member scope — the top bar's "viewing as" lens writes this field.
	priya := CountPresets(txns, Criteria{Member: "m-priya"}.Normalize(), scope)
	if priya.Uncategorized != 2 || priya.Large != 1 || priya.ThisMonth != 2 {
		t.Errorf("member-scoped = %+v, want 2 / 1 / 2 — the counts must describe Priya's rows only", priya)
	}

	// A single-account scope.
	acct := CountPresets(txns, Criteria{Account: "acc-2"}.Normalize(), scope)
	if acct.Uncategorized != 2 || acct.Large != 1 {
		t.Errorf("account-scoped = %+v, want 2 uncategorized / 1 large", acct)
	}

	// A multi-value dimension (the filter panel's toggle pills) still scopes the
	// counts: two accounts selected is the whole ledger here.
	both := CountPresets(txns, Criteria{Accounts: "acc-1,acc-2"}.Normalize(), scope)
	if both.Uncategorized != 4 || both.Large != 2 {
		t.Errorf("two-account scope = %+v, want the same as unscoped for this fixture", both)
	}
}

// A count is a promise about what pressing the chip yields, so it must equal the
// size of the set the preset's own filter actually produces. This checks the
// promise against Apply rather than against a hand-counted constant.
func TestPresetCountsMatchWhatApplyingThePresetProduces(t *testing.T) {
	txns := presetFixture()
	scope := presetScope()
	base := Criteria{Text: "CF26-PIPE"}.Normalize()
	got := CountPresets(txns, base, scope)

	uncat := base
	uncat.Uncategorized = true
	if n := len(Apply(txns, uncat.Normalize())); n != got.Uncategorized {
		t.Errorf("Uncategorized chip promised %d, applying it yields %d", got.Uncategorized, n)
	}

	review := base
	review.Tag = scope.ReviewTag
	if n := len(Apply(txns, review.Normalize())); n != got.NeedsReview {
		t.Errorf("Needs-review chip promised %d, applying it yields %d", got.NeedsReview, n)
	}

	large := base
	large.AmountMin = "100"
	if n := len(Apply(txns, large.Normalize())); n != got.Large {
		t.Errorf("Large chip promised %d, applying it yields %d", got.Large, n)
	}

	month := base
	month.From, month.To = "2026-08-01", "2026-08-31"
	if n := len(Apply(txns, month.Normalize())); n != got.ThisMonth {
		t.Errorf("This-month chip promised %d, applying it yields %d", got.ThisMonth, n)
	}
}

// The Amount column shows a signed figure, so it must ORDER by that figure. The
// old magnitude comparison made a refund and the purchase it reverses compare
// EQUAL, and interleaved income through a list someone sorted to find their
// biggest expenses.
func TestAmountSortIsSignedNotMagnitude(t *testing.T) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	day := func(d int) time.Time { return time.Date(2026, 8, d, 12, 0, 0, 0, time.UTC) }
	txns := []domain.Transaction{
		{ID: "spend-big", Desc: "Mortgage", Date: day(1), Amount: usd(-470000)},
		{ID: "earn-big", Desc: "Paycheck", Date: day(2), Amount: usd(470000)},
		{ID: "spend-small", Desc: "Coffee", Date: day(3), Amount: usd(-450)},
		{ID: "earn-small", Desc: "Refund", Date: day(4), Amount: usd(450)},
	}
	order := func(c Criteria) []string {
		out := []string{}
		for _, t := range Apply(txns, c.Normalize()) {
			out = append(out, t.ID)
		}
		return out
	}

	// Ascending reads down the column exactly as it is displayed: the biggest
	// expense first, the biggest income last.
	wantAsc := []string{"spend-big", "spend-small", "earn-small", "earn-big"}
	if got := order(Criteria{Sort: "amount", Dir: Asc}); !equalIDs(got, wantAsc) {
		t.Errorf("ascending = %v, want %v", got, wantAsc)
	}
	wantDesc := []string{"earn-big", "earn-small", "spend-small", "spend-big"}
	if got := order(Criteria{Sort: "amount", Dir: Desc}); !equalIDs(got, wantDesc) {
		t.Errorf("descending = %v, want %v", got, wantDesc)
	}

	// The precise old defect: ±the same figure compared equal and fell through to
	// the id tiebreak, so "earn-big" preceded "spend-big" whatever the direction.
	asc := order(Criteria{Sort: "amount", Dir: Asc})
	iSpend, iEarn := indexOf(asc, "spend-big"), indexOf(asc, "earn-big")
	if iSpend > iEarn {
		t.Errorf("a -$4,700 expense sorted after a +$4,700 income ascending: %v", asc)
	}
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

// A table ordered by a column nobody can see states nothing about its own order:
// no header carries a sort indicator, yet the rows are not in the default order.
func TestEffectiveSortFallsBackWhenItsColumnIsHidden(t *testing.T) {
	tests := []struct {
		name    string
		crit    Criteria
		hidden  map[string]bool
		wantKey string
		wantDir string
		wantWhy string
	}{
		{
			name: "a visible column keeps its sort",
			crit: Criteria{Sort: "amount", Dir: Asc}, hidden: map[string]bool{"category": true},
			wantKey: "amount", wantDir: Asc,
			wantWhy: "Amount is on screen, so it still orders the table",
		},
		{
			name: "hiding the sorted column falls back to newest first",
			crit: Criteria{Sort: "amount", Dir: Asc}, hidden: map[string]bool{"amount": true},
			wantKey: "date", wantDir: Desc,
			wantWhy: "no header could show the amount sort, so the order must not be by amount",
		},
		{
			name: "each hideable column behaves the same",
			crit: Criteria{Sort: "category", Dir: Asc}, hidden: map[string]bool{"category": true},
			wantKey: "date", wantDir: Desc,
			wantWhy: "Category is hideable too",
		},
		{
			// Date and Description cannot be hidden, so they are never in the map.
			name: "an always-visible column is unaffected",
			crit: Criteria{Sort: "payee", Dir: Asc}, hidden: map[string]bool{"amount": true, "account": true},
			wantKey: "payee", wantDir: Asc,
			wantWhy: "Description has no visibility toggle",
		},
		{
			name: "a nil map means everything is visible",
			crit: Criteria{Sort: "source", Dir: Desc}, hidden: nil,
			wantKey: "source", wantDir: Desc,
			wantWhy: "callers without column state must not lose their sort",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, dir := EffectiveSort(tc.crit, tc.hidden)
			if key != tc.wantKey || dir != tc.wantDir {
				t.Errorf("EffectiveSort() = %q/%q, want %q/%q — %s", key, dir, tc.wantKey, tc.wantDir, tc.wantWhy)
			}
		})
	}
}

// The fallback is a DISPLAY decision: the stored preference survives, so putting
// the column back restores the sort rather than making the user re-pick it.
func TestEffectiveSortDoesNotMutateTheStoredCriteria(t *testing.T) {
	c := Criteria{Sort: "amount", Dir: Asc}
	if _, _ = EffectiveSort(c, map[string]bool{"amount": true}); c.Sort != "amount" || c.Dir != Asc {
		t.Fatalf("stored criteria changed to %q/%q", c.Sort, c.Dir)
	}
	// Re-showing the column brings the sort straight back.
	key, dir := EffectiveSort(c, nil)
	if key != "amount" || dir != Asc {
		t.Errorf("restored sort = %q/%q, want amount/asc", key, dir)
	}
}

func TestCountPresetsToleratesAnUnconfiguredScope(t *testing.T) {
	got := CountPresets(presetFixture(), Criteria{}, PresetScope{})
	if got.NeedsReview != 0 || got.Large != 0 || got.ThisMonth != 0 {
		t.Errorf("an empty scope should count no tagged/large/dated presets, got %+v", got)
	}
	if got.Uncategorized != 4 {
		t.Errorf("Uncategorized = %d, want 4 — it needs no scope configuration", got.Uncategorized)
	}
}

func TestCountPresetsIgnoresTransfersWhenCountingUncategorized(t *testing.T) {
	txns := append(presetFixture(), domain.Transaction{
		ID: "t", Desc: "Move to savings", Date: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
		Amount: money.New(-40000, "USD"), TransferAccountID: "acc2",
	})
	got := CountPresets(txns, Criteria{}, presetScope())
	if got.Uncategorized != 4 {
		t.Errorf("Uncategorized = %d, want 4 — a transfer leg has no category to be missing", got.Uncategorized)
	}
}
