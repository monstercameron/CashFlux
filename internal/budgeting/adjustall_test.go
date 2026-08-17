// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func adjBudget(id string, limit int64, cur string) domain.Budget {
	return domain.Budget{ID: id, Name: id, Period: domain.PeriodMonthly, Limit: money.New(limit, cur)}
}

// zeroOverlays is the overlay map for budgets carrying no rollover and no boost.
// AdjustAllPreviewFor requires an entry per budget under both scopes,
// deliberately — a missing one means nobody has looked at that budget's period
// state, which is precisely when the inversion guard matters.
func zeroOverlays(budgets []domain.Budget) map[string]int64 {
	out := make(map[string]int64, len(budgets))
	for _, b := range budgets {
		out[b.ID] = 0
	}
	return out
}

func TestAdjustAllPreviewRaise(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("b", 5000, "USD")}

	p := AdjustAllPreviewFor(budgets, 10, AdjustEveryPeriod, zeroOverlays(budgets))
	if p.Count() != 2 {
		t.Fatalf("Count = %d, want 2", p.Count())
	}
	if p.TotalBefore != 25000 || p.TotalAfter != 27500 {
		t.Errorf("totals = %d → %d, want 25000 → 27500", p.TotalBefore, p.TotalAfter)
	}
	if p.TotalDelta() != 2500 {
		t.Errorf("TotalDelta = %d, want 2500", p.TotalDelta())
	}
	if p.Lines[0].After != 22000 || p.Lines[1].After != 5500 {
		t.Errorf("lines = %d, %d, want 22000, 5500", p.Lines[0].After, p.Lines[1].After)
	}
	if p.Currency != "USD" || p.MixedCurrency {
		t.Errorf("currency = %q mixed=%v, want USD and not mixed", p.Currency, p.MixedCurrency)
	}
}

// The preview must be exactly what the write does — same rounding, same floor.
func TestAdjustedLimitRoundsHalfAwayAndFloorsAtOne(t *testing.T) {
	cases := []struct {
		limit int64
		pct   float64
		want  int64
	}{
		{100, 5, 105},
		{101, 5, 106},  // 5.05 rounds to 5
		{110, 5, 116},  // 5.5 rounds away from zero to 6
		{100, -50, 50}, //
		{100, -90, 10},
		{1, -90, 1}, // a lower may shrink a budget, never delete it
		{3, -90, 1},
	}
	for _, c := range cases {
		if got := AdjustedLimit(c.limit, c.pct); got != c.want {
			t.Errorf("AdjustedLimit(%d, %v) = %d, want %d", c.limit, c.pct, got, c.want)
		}
	}
}

// A budget with nothing to scale is not "affected", so it must not appear in the
// count the confirmation quotes.
func TestAdjustAllPreviewSkipsNonPositiveLimits(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("zero", 0, "USD")}
	p := AdjustAllPreviewFor(budgets, 10, AdjustEveryPeriod, zeroOverlays(budgets))
	if p.Count() != 1 || p.Lines[0].Budget.ID != "a" {
		t.Errorf("preview = %+v, want only budget a", p.Lines)
	}
}

// Budgets in different currencies cannot be totalled, and the form must know not
// to print a total that would be adding dollars to euros.
func TestAdjustAllPreviewFlagsMixedCurrency(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("b", 5000, "EUR")}
	p := AdjustAllPreviewFor(budgets, 10, AdjustEveryPeriod, zeroOverlays(budgets))
	if !p.MixedCurrency {
		t.Error("MixedCurrency = false for USD + EUR budgets")
	}
	if p.Currency != "" {
		t.Errorf("Currency = %q, want empty when the budgets disagree", p.Currency)
	}
	if p.Count() != 2 {
		t.Errorf("Count = %d, want 2 — the per-budget lines are still valid", p.Count())
	}
}

func TestValidAdjustPct(t *testing.T) {
	valid := []float64{5, -10, 0.5, AdjustMinPct, AdjustMaxPct}
	for _, v := range valid {
		if !ValidAdjustPct(v) {
			t.Errorf("ValidAdjustPct(%v) = false, want true", v)
		}
	}
	invalid := []float64{0, AdjustMinPct - 0.1, AdjustMaxPct + 0.1, -1000}
	for _, v := range invalid {
		if ValidAdjustPct(v) {
			t.Errorf("ValidAdjustPct(%v) = true, want false", v)
		}
	}
}

// Every reduction is asked about, because it takes money out of every plan at
// once; so is any large raise, which is far likelier to be a typo than a plan.
func TestIsLargeAdjust(t *testing.T) {
	for _, v := range []float64{-1, -50, 26, 500} {
		if !IsLargeAdjust(v) {
			t.Errorf("IsLargeAdjust(%v) = false, want true", v)
		}
	}
	for _, v := range []float64{1, 5, 25} {
		if IsLargeAdjust(v) {
			t.Errorf("IsLargeAdjust(%v) = true, want false", v)
		}
	}
}

// ─── C671: scope ─────────────────────────────────────────────────────────────

func TestAdjustScopeValidity(t *testing.T) {
	if !AdjustThisPeriod.Valid() || !AdjustEveryPeriod.Valid() {
		t.Error("both real scopes must validate")
	}
	if AdjustScope("").Valid() || AdjustScope("forever").Valid() {
		t.Error("an unknown scope must not validate")
	}
	if AdjustThisPeriod.IsPermanent() {
		t.Error("a this-period adjustment does not outlive the period")
	}
	if !AdjustEveryPeriod.IsPermanent() {
		t.Error("an every-period adjustment is permanent")
	}
}

// C671: reach, not just magnitude, is what makes a bulk adjustment worth asking
// about. A 1% cut that never ends still rewrites the plan.
func TestAdjustNeedsAck(t *testing.T) {
	cases := []struct {
		name  string
		pct   float64
		scope AdjustScope
		want  bool
	}{
		{"tiny permanent raise still asks", 1, AdjustEveryPeriod, true},
		{"tiny permanent cut still asks", -1, AdjustEveryPeriod, true},
		{"large permanent cut asks", -40.7, AdjustEveryPeriod, true},
		{"tiny this-period raise does not ask", 1, AdjustThisPeriod, false},
		{"large this-period raise asks", 40, AdjustThisPeriod, true},
		{"any this-period cut asks", -1, AdjustThisPeriod, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AdjustNeedsAck(c.pct, c.scope); got != c.want {
				t.Errorf("AdjustNeedsAck(%v, %q) = %v, want %v", c.pct, c.scope, got, c.want)
			}
		})
	}
}

// C671: the two scopes are two different writes. A permanent one moves the base
// limit; a this-period one records a boost of the same size and leaves the plan
// where it was, so next period starts from the original number.
func TestApplyAdjustWritesPerScope(t *testing.T) {
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	budgets := []domain.Budget{
		{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")},
		{ID: "b2", CategoryID: "c2", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")},
	}
	preview := AdjustAllPreviewFor(budgets, -40, AdjustEveryPeriod, zeroOverlays(budgets))
	if preview.Count() != 2 {
		t.Fatalf("preview count = %d, want 2", preview.Count())
	}
	starts := func(domain.Budget) time.Time { return periodStart }

	perm := ApplyAdjust(preview, AdjustEveryPeriod, starts)
	if len(perm) != 2 {
		t.Fatalf("permanent write returned %d budgets, want 2", len(perm))
	}
	if perm[0].Limit.Amount != 60000 || perm[1].Limit.Amount != 30000 {
		t.Errorf("permanent limits = %d/%d, want 60000/30000", perm[0].Limit.Amount, perm[1].Limit.Amount)
	}
	if len(perm[0].PeriodBoosts) != 0 {
		t.Errorf("a permanent adjustment must not leave a period boost: %v", perm[0].PeriodBoosts)
	}

	this := ApplyAdjust(preview, AdjustThisPeriod, starts)
	if len(this) != 2 {
		t.Fatalf("this-period write returned %d budgets, want 2", len(this))
	}
	for i, want := range []int64{100000, 50000} {
		if this[i].Limit.Amount != want {
			t.Errorf("this-period write moved the base limit to %d, want it left at %d", this[i].Limit.Amount, want)
		}
	}
	// The boost carries the same delta the preview promised, so the effective cap
	// this period matches the previewed "after".
	for i, line := range preview.Lines {
		if got := this[i].PeriodBoost(periodStart); got != line.Delta() {
			t.Errorf("budget %s boost = %d, want the previewed delta %d", line.Budget.ID, got, line.Delta())
		}
		if eff := this[i].Limit.Amount + this[i].PeriodBoost(periodStart); eff != line.After {
			t.Errorf("budget %s effective cap = %d, want the previewed after %d", line.Budget.ID, eff, line.After)
		}
	}
	// And it lapses: a later period sees the untouched plan.
	next := periodStart.AddDate(0, 1, 0)
	if got := this[0].PeriodBoost(next); got != 0 {
		t.Errorf("the boost leaked into the next period (%d) — a this-period change must revert", got)
	}
}

// A this-period adjustment with no way to resolve the period must write nothing
// rather than fall back to rewriting base limits — the one outcome the scope
// exists to prevent.
func TestApplyAdjustThisPeriodWithoutAPeriodWritesNothing(t *testing.T) {
	budgets := []domain.Budget{{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}}
	preview := AdjustAllPreviewFor(budgets, -40, AdjustEveryPeriod, zeroOverlays(budgets))
	if got := ApplyAdjust(preview, AdjustThisPeriod, nil); len(got) != 0 {
		t.Errorf("wrote %d budgets with no period resolver, want 0", len(got))
	}
}

// C671 (review round 1): a this-period adjustment lands on the EFFECTIVE cap —
// base plus rollover carry-in plus any boost already recorded — so the preview
// has to measure against that, not the base. Measuring against the base promised
// a figure the write could not deliver on any budget carrying anything.
func TestAdjustPreviewThisPeriodScalesTheEffectiveCap(t *testing.T) {
	// Base $400, $50 already carried in this period: the card shows a $450 cap.
	b := domain.Budget{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}
	overlays := map[string]int64{"b1": 5000} // $50 carried in on a $400 base

	p := AdjustAllPreviewFor([]domain.Budget{b}, -20, AdjustThisPeriod, overlays)
	if p.Count() != 1 {
		t.Fatalf("preview count = %d, want 1", p.Count())
	}
	if p.Lines[0].Before != 45000 {
		t.Errorf("before = %d, want the effective cap 45000 — the base is not what a boost moves", p.Lines[0].Before)
	}
	if p.Lines[0].After != 36000 {
		t.Errorf("after = %d, want 36000 (20%% off the cap)", p.Lines[0].After)
	}

	// And the write must land the cap exactly where the preview said.
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	out := ApplyAdjust(p, AdjustThisPeriod, func(domain.Budget) time.Time { return start })
	if len(out) != 1 {
		t.Fatalf("write returned %d budgets, want 1", len(out))
	}
	// Effective cap = base + carry-in (unchanged at +5000) + the recorded boost.
	if eff := out[0].Limit.Amount + 5000 + out[0].PeriodBoost(start); eff != p.Lines[0].After {
		t.Errorf("effective cap after the write = %d, want the previewed %d", eff, p.Lines[0].After)
	}
	if out[0].Limit.Amount != 40000 {
		t.Errorf("the base limit moved to %d — a this-period change must leave it alone", out[0].Limit.Amount)
	}

	// The permanent path is unaffected: it still scales the stored limit.
	perm := AdjustAllPreviewFor([]domain.Budget{b}, -20, AdjustEveryPeriod, overlays)
	if perm.Lines[0].Before != 40000 || perm.Lines[0].After != 32000 {
		t.Errorf("permanent preview = %d → %d, want 40000 → 32000", perm.Lines[0].Before, perm.Lines[0].After)
	}
}

// C671 (review round 1): applying twice in one period must not silently
// accumulate. The base never moves under this-period scope, so a preview built
// from the base showed the same figures on the second pass while the boost
// compounded — enough repeats drove the effective cap negative.
func TestAdjustThisPeriodRepeatIsVisibleAndFloored(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	starts := func(domain.Budget) time.Time { return start }
	b := domain.Budget{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}

	capOf := func(x domain.Budget) int64 { return x.Limit.Amount + x.PeriodBoost(start) }
	for i := range 6 {
		overlays := map[string]int64{"b1": capOf(b) - b.Limit.Amount}
		p := AdjustAllPreviewFor([]domain.Budget{b}, -90, AdjustThisPeriod, overlays)
		if p.Count() == 0 {
			break // nothing left to scale, which is itself an honest stop
		}
		if p.Lines[0].Before != capOf(b) {
			t.Fatalf("pass %d previewed %d, want the current cap %d", i, p.Lines[0].Before, capOf(b))
		}
		out := ApplyAdjust(p, AdjustThisPeriod, starts)
		b = out[0]
		if got := capOf(b); got != p.Lines[0].After {
			t.Fatalf("pass %d landed the cap on %d, want the previewed %d", i, got, p.Lines[0].After)
		}
		if got := capOf(b); got < 1 {
			t.Fatalf("pass %d drove the effective cap to %d — a bulk lower may shrink a budget, never invert it", i, got)
		}
	}
}

// A this-period preview cannot be built for a budget whose cap is unknown:
// falling back to the base is exactly the mismatch this fix removes.
func TestAdjustPreviewThisPeriodSkipsBudgetsWithoutACap(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")},
		{ID: "b2", CategoryID: "c2", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")},
	}
	p := AdjustAllPreviewFor(budgets, -10, AdjustThisPeriod, map[string]int64{"b2": 10000})
	if p.Count() != 1 || p.Lines[0].Budget.ID != "b2" {
		t.Fatalf("preview covered %d budgets (%+v), want only the one with a known cap", p.Count(), p.Lines)
	}
	// An exhausted cap has nothing left to scale.
	if got := AdjustAllPreviewFor(budgets, -10, AdjustThisPeriod, map[string]int64{"b1": -100000, "b2": -50500}); got.Count() != 0 {
		t.Errorf("previewed %d budgets with non-positive caps, want 0", got.Count())
	}
}

// C671 (review round 2): the cross-scope path. A this-period reduction leaves a
// boost on the period; a later PERMANENT cut only ever looked at the base, so the
// boost rode along on top of the new limit and could push the effective cap below
// zero — previewed as a healthy figure, written as a negative plan.
func TestAdjustPermanentRefusesToInvertACapCarryingABoost(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	starts := func(domain.Budget) time.Time { return start }
	b := domain.Budget{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}

	// Step 1: reconcile this period, -90%. Cap 10000 -> 1000, base untouched.
	first := AdjustAllPreviewFor([]domain.Budget{b}, -90, AdjustThisPeriod, map[string]int64{"b1": 0})
	b = ApplyAdjust(first, AdjustThisPeriod, starts)[0]
	cap := b.Limit.Amount + b.PeriodBoost(start)
	if b.Limit.Amount != 10000 || cap != 1000 {
		t.Fatalf("after the this-period pass: base %d cap %d, want 10000 / 1000", b.Limit.Amount, cap)
	}

	// Step 2: a permanent -20% on the same budget, in the same period. Scaling the
	// base gives 8000, and the -9000 boost still sits on top: 8000-9000 = -1000.
	second := AdjustAllPreviewFor([]domain.Budget{b}, -20, AdjustEveryPeriod, map[string]int64{"b1": cap - b.Limit.Amount})
	if second.Count() != 0 {
		t.Errorf("previewed a permanent change that would invert the cap: %+v", second.Lines)
	}
	skipped := second.SkippedFor(SkipWouldInvert)
	if len(skipped) != 1 || skipped[0].Budget.ID != "b1" {
		t.Fatalf("the budget was not reported as skipped-for-inversion: %+v", second.Skipped)
	}
	// And nothing is written for it.
	if got := ApplyAdjust(second, AdjustEveryPeriod, starts); len(got) != 0 {
		t.Errorf("wrote %d budgets that the preview excluded", len(got))
	}

	// A permanent cut the cap CAN absorb still goes through untouched.
	ok := AdjustAllPreviewFor([]domain.Budget{b}, 50, AdjustEveryPeriod, map[string]int64{"b1": cap - b.Limit.Amount})
	if ok.Count() != 1 {
		t.Fatalf("a raise the cap can absorb was refused: %+v", ok.Skipped)
	}
	out := ApplyAdjust(ok, AdjustEveryPeriod, starts)
	if newCap := out[0].Limit.Amount + out[0].PeriodBoost(start); newCap < 1 {
		t.Errorf("the surviving write still inverted the cap: %d", newCap)
	}
}

// A skipped budget is reported with its reason, so the form can say why the row
// count is short instead of leaving the omission to be discovered.
func TestAdjustPreviewReportsWhyBudgetsAreSkipped(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "keep", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "empty", CategoryID: "c2", Period: domain.PeriodMonthly, Limit: money.New(0, "USD")},
	}
	perm := AdjustAllPreviewFor(budgets, -10, AdjustEveryPeriod, zeroOverlays(budgets))
	if perm.Count() != 1 {
		t.Fatalf("preview covered %d budgets, want 1", perm.Count())
	}
	if got := perm.SkippedFor(SkipNothingToScale); len(got) != 1 || got[0].Budget.ID != "empty" {
		t.Errorf("nothing-to-scale skip not reported: %+v", perm.Skipped)
	}

	// Under this-period scope, a budget with no known cap is reported too.
	this := AdjustAllPreviewFor(budgets, -10, AdjustThisPeriod, map[string]int64{"keep": 12000})
	if got := this.SkippedFor(SkipUnknownOverlay); len(got) != 1 || got[0].Budget.ID != "empty" {
		t.Errorf("unknown-cap skip not reported: %+v", this.Skipped)
	}
}

// C671 (review round 3): the inversion guard must not be conditional on having
// happened to resolve a cap. A budget whose status could not be evaluated — an FX
// gap, say — is exactly the one whose overlay nobody has looked at, so previewing
// it with the check quietly disabled is the same defect as having no check.
func TestAdjustPreviewSkipsUnknownCapsUnderBothScopes(t *testing.T) {
	// A budget already pulled to nothing for this period: base $100, cap $0.10.
	// Under permanent scope a −20% cut would leave 8000 + (10 − 10000) = −1990.
	budgets := []domain.Budget{{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}}
	for _, scope := range []AdjustScope{AdjustThisPeriod, AdjustEveryPeriod} {
		t.Run(string(scope), func(t *testing.T) {
			p := AdjustAllPreviewFor(budgets, -20, scope, nil)
			if p.Count() != 0 {
				t.Errorf("previewed %d budgets with no cap information — the guard was skipped, not satisfied", p.Count())
			}
			if got := p.SkippedFor(SkipUnknownOverlay); len(got) != 1 {
				t.Errorf("the unresolvable budget was not reported: %+v", p.Skipped)
			}
		})
	}
	// With the cap known, permanent still refuses the inverting cut.
	p := AdjustAllPreviewFor(budgets, -20, AdjustEveryPeriod, map[string]int64{"b1": -9990})
	if p.Count() != 0 || len(p.SkippedFor(SkipWouldInvert)) != 1 {
		t.Errorf("a known inverting cut was not refused: lines %+v skipped %+v", p.Lines, p.Skipped)
	}
}

// Chaining scopes on one budget: this-period, then permanent, then this-period
// again. Each pass must preview against the live cap and land exactly there, and
// no pass may invert the plan.
func TestAdjustChainedScopesStayHonest(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	starts := func(domain.Budget) time.Time { return start }
	b := domain.Budget{ID: "b1", CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	capOf := func(x domain.Budget) int64 { return x.Limit.Amount + x.PeriodBoost(start) }

	for _, step := range []struct {
		scope AdjustScope
		pct   float64
	}{{AdjustThisPeriod, -90}, {AdjustEveryPeriod, -5}, {AdjustThisPeriod, -50}} {
		p := AdjustAllPreviewFor([]domain.Budget{b}, step.pct, step.scope, map[string]int64{"b1": capOf(b) - b.Limit.Amount})
		if p.Count() != 1 {
			t.Fatalf("%s %v%%: refused (%+v) — each of these is absorbable", step.scope, step.pct, p.Skipped)
		}
		overlay := capOf(b) - b.Limit.Amount //nolint:staticcheck // reused below for the permanent case
		out := ApplyAdjust(p, step.scope, starts)
		b = out[0]
		want := p.Lines[0].After
		if step.scope == AdjustEveryPeriod {
			want += overlay // a permanent write moves the base; the overlay rides along
		}
		if got := capOf(b); got != want {
			t.Fatalf("%s %v%%: cap landed on %d, want %d", step.scope, step.pct, got, want)
		}
		if capOf(b) < 1 {
			t.Fatalf("%s %v%%: cap inverted to %d", step.scope, step.pct, capOf(b))
		}
	}
}
