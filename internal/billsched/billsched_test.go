// SPDX-License-Identifier: MIT

package billsched

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestPaydaysBiweeklyStepsAnchorOntoWindow(t *testing.T) {
	// Anchor far in the past still projects onto the window.
	anchor := date(2026, 1, 2)
	got := Paydays(anchor, "biweekly", date(2026, 7, 1), 30)
	if len(got) == 0 {
		t.Fatal("expected biweekly paydays in the window")
	}
	for i, p := range got {
		if p.Before(date(2026, 7, 1)) || p.After(date(2026, 7, 31)) {
			t.Errorf("payday %d (%s) outside window", i, p)
		}
		if i > 0 {
			if days := int(p.Sub(got[i-1]).Hours() / 24); days != 14 {
				t.Errorf("gap %d days, want 14", days)
			}
		}
	}
}

func TestPaydaysSemimonthlyTwoPerMonth(t *testing.T) {
	got := Paydays(date(2026, 6, 1), "semimonthly", date(2026, 7, 1), 30)
	if len(got) < 2 {
		t.Fatalf("expected ≥2 semimonthly paydays in a month, got %d (%v)", len(got), got)
	}
}

func TestPaydaysMonthlyClampsShortMonths(t *testing.T) {
	got := Paydays(date(2026, 1, 31), "monthly", date(2026, 2, 1), 27)
	if len(got) != 1 || got[0].Day() != 28 {
		t.Fatalf("Feb payday should clamp to the 28th, got %v", got)
	}
}

func TestPaydaysZeroAnchor(t *testing.T) {
	if got := Paydays(time.Time{}, "biweekly", date(2026, 7, 1), 30); got != nil {
		t.Fatalf("zero anchor should yield no paydays, got %v", got)
	}
}

// The classic crunch: bills due right BEFORE a payday. Paying ahead can't fix
// that (the cash isn't there yet) — but a biller-side shift to just after the
// payday lifts the low point, and Suggest finds it.
func TestSuggestLiftsTheLowPoint(t *testing.T) {
	from := date(2026, 7, 1)
	paydays := []time.Time{date(2026, 7, 1), date(2026, 7, 15), date(2026, 7, 29)}
	items := []Item{
		{ID: "rent", Name: "Rent", Amount: 120000, Due: date(2026, 7, 14), Movable: true},
		{ID: "car", Name: "Car", Amount: 40000, Due: date(2026, 7, 14), Movable: false},
	}
	// $100 start, $1,000 per payday; both bills due the day BEFORE the mid-month
	// payday → raw curve bottoms out at 100+1000−1600 = −500.
	res := Optimize(10000, items, paydays, 100000, from, 30, 0)

	if res.Raw.Low >= 0 {
		t.Fatalf("raw schedule should dip negative, got %d", res.Raw.Low)
	}
	// Pay-ahead must not pretend to fix a crunch it can't: the low never worsens.
	if res.Smart.Low < res.Raw.Low {
		t.Fatalf("smart low (%d) must never be below raw low (%d)", res.Smart.Low, res.Raw.Low)
	}
	if len(res.Suggestions) == 0 {
		t.Fatal("expected biller-side suggestions for bills due just before a payday")
	}
	for _, sg := range res.Suggestions {
		if sg.LowGainMinor <= 0 {
			t.Errorf("suggestion for %s has no gain", sg.Item.Name)
		}
		if !sg.NewDue.After(sg.Item.Due) {
			t.Errorf("suggested due for %s should be later than the raw due", sg.Item.Name)
		}
	}
	// The autopay bill benefits from suggestions too (its only lever).
	found := false
	for _, sg := range res.Suggestions {
		if sg.Item.ID == "car" {
			found = true
		}
	}
	if !found {
		t.Error("autopay bill should appear in the biller-side suggestions")
	}
}

// Two bills stacked on one paycheck with cash to spare: pay-ahead splits them
// across checks so the heaviest paycheck lightens.
func TestOptimizeEvensTheHeaviestPaycheck(t *testing.T) {
	from := date(2026, 7, 1)
	paydays := []time.Time{date(2026, 7, 1), date(2026, 7, 15)}
	items := []Item{
		{ID: "a", Name: "A", Amount: 50000, Due: date(2026, 7, 28), Movable: true},
		{ID: "b", Name: "B", Amount: 50000, Due: date(2026, 7, 28), Movable: true},
	}
	res := Optimize(1000000, items, paydays, 100000, from, 30, 0)
	if len(res.Moves) == 0 || res.EvenGainMinor <= 0 {
		t.Fatalf("expected an evenness gain, got moves=%d gain=%d", len(res.Moves), res.EvenGainMinor)
	}
	if got, want := maxLoad(res.Smart.Loads), maxLoad(res.Raw.Loads); got >= want {
		t.Errorf("heaviest paycheck should lighten: %d vs raw %d", got, want)
	}
	for _, mv := range res.Moves {
		if mv.PayOn.After(mv.Item.Due) || mv.PayOn.Before(from) {
			t.Errorf("%s scheduled outside its valid range: %s", mv.Item.Name, mv.PayOn)
		}
	}
}

func TestOptimizeNeverMovesAutopay(t *testing.T) {
	from := date(2026, 7, 1)
	paydays := []time.Time{date(2026, 7, 1), date(2026, 7, 15)}
	items := []Item{
		{ID: "auto", Name: "Autopay card", Amount: 90000, Due: date(2026, 7, 14), Movable: false},
		{ID: "manual", Name: "Manual bill", Amount: 90000, Due: date(2026, 7, 14), Movable: true},
	}
	res := Optimize(0, items, paydays, 100000, from, 30, 0)
	for _, mv := range res.Moves {
		if mv.Item.ID == "auto" {
			t.Fatal("autopay item must never be moved")
		}
	}
	if !res.PayOnByID["auto"].Equal(date(2026, 7, 14)) {
		t.Errorf("autopay pay-on should stay at its due date, got %s", res.PayOnByID["auto"])
	}
}

func TestOptimizeConsolidatesOntoPaydays(t *testing.T) {
	// The plan's whole point: scattered due dates become per-paycheck buckets.
	// A bill due the 2nd pays on the 1st's payday; one due the 20th pays on the
	// 15th's; one already ON a payday stays (no move reported for it).
	from := date(2026, 7, 1)
	paydays := []time.Time{date(2026, 7, 1), date(2026, 7, 15)}
	items := []Item{
		{ID: "a", Name: "A", Amount: 5000, Due: date(2026, 7, 2), Movable: true},
		{ID: "b", Name: "B", Amount: 5000, Due: date(2026, 7, 20), Movable: true},
		{ID: "c", Name: "C", Amount: 5000, Due: date(2026, 7, 15), Movable: true},
	}
	res := Optimize(500000, items, paydays, 100000, from, 30, 0)
	if got := res.PayOnByID["a"]; !got.Equal(date(2026, 7, 1)) {
		t.Errorf("a should consolidate onto the Jul 1 payday, got %s", got)
	}
	if got := res.PayOnByID["b"]; !got.Equal(date(2026, 7, 15)) {
		t.Errorf("b should consolidate onto the Jul 15 payday, got %s", got)
	}
	if got := res.PayOnByID["c"]; !got.Equal(date(2026, 7, 15)) {
		t.Errorf("c already sits on a payday and must stay, got %s", got)
	}
	if len(res.Moves) != 2 {
		t.Fatalf("want 2 moves (a and b), got %d: %v", len(res.Moves), res.Moves)
	}
	// Neither move jumps a pay cycle: each pays from the paycheck its due date
	// already belongs to — consolidation, not fronted money.
	for _, mv := range res.Moves {
		if mv.CycleAhead {
			t.Errorf("%s consolidates within its own pay period — must not be CycleAhead", mv.Item.ID)
		}
	}
}

func TestOptimizeFlagsCycleAheadMoves(t *testing.T) {
	// A bill due Aug 5 belongs to the Jul 31 paycheck (latest payday <= due).
	// If balancing pays it from Jul 17's check instead, that's fronted money —
	// flagged CycleAhead; paying it ON Jul 31 would not be.
	from := date(2026, 7, 3)
	paydays := []time.Time{date(2026, 7, 3), date(2026, 7, 17), date(2026, 7, 31), date(2026, 8, 14), date(2026, 8, 28)}
	items := []Item{
		// Heavy immovable bill already in the Jul 31 period forces the Aug 5
		// bill off that paycheck.
		{ID: "rent", Name: "Rent", Amount: 90000, Due: date(2026, 8, 1), Movable: false},
		{ID: "hoa", Name: "HOA", Amount: 40000, Due: date(2026, 8, 5), Movable: true},
	}
	res := Optimize(1000000, items, paydays, 100000, from, 60, 0)
	p, ok := res.PayOnByID["hoa"]
	if !ok || p.After(date(2026, 8, 5)) {
		t.Fatalf("hoa must pay on or before its due date, got %s", p)
	}
	if p.Before(date(2026, 7, 31)) != res.AheadByID["hoa"] {
		t.Errorf("AheadByID must mark exactly the cross-period placement: payOn=%s ahead=%v", p, res.AheadByID["hoa"])
	}
	if !p.Before(date(2026, 7, 31)) {
		t.Fatalf("balancing should push hoa off the rent-heavy Jul 31 check, got %s", p)
	}
	for _, mv := range res.Moves {
		if mv.Item.ID == "hoa" && !mv.CycleAhead {
			t.Error("hoa jumps into an earlier paycheck — must be CycleAhead")
		}
	}
}

func TestOptimizeNoPaydaysReturnsRaw(t *testing.T) {
	from := date(2026, 7, 1)
	items := []Item{{ID: "b", Name: "Bill", Amount: 5000, Due: date(2026, 7, 20), Movable: true}}
	res := Optimize(10000, items, nil, 0, from, 30, 0)
	if len(res.Moves) != 0 || res.Smart.Low != res.Raw.Low {
		t.Fatal("no paydays → raw schedule unchanged")
	}
	if !res.PayOnByID["b"].Equal(date(2026, 7, 20)) {
		t.Errorf("pay-on should be the due date, got %s", res.PayOnByID["b"])
	}
}

func TestOptimizeOverdueBillPayableToday(t *testing.T) {
	from := date(2026, 7, 10)
	paydays := []time.Time{date(2026, 7, 15)}
	items := []Item{{ID: "late", Name: "Overdue", Amount: 5000, Due: date(2026, 7, 3), Movable: true}}
	res := Optimize(100000, items, paydays, 50000, from, 30, 0)
	if got := res.PayOnByID["late"]; !got.Equal(from) {
		t.Errorf("overdue bill should be scheduled today (%s), got %s", from, got)
	}
}

func TestOptimizeEvensBothMonthsOverA60DayHorizon(t *testing.T) {
	// Regression: two bills stacked on one paycheck in July AND two more in
	// August. No single move improves the GLOBAL max load, so a max-only
	// objective deadlocks and reports "already even" — the sorted-load-vector
	// objective must split both months.
	from := date(2026, 7, 1)
	paydays := []time.Time{
		date(2026, 7, 1), date(2026, 7, 15), date(2026, 7, 29),
		date(2026, 8, 12), date(2026, 8, 26),
	}
	items := []Item{
		{ID: "jul-a", Name: "Rent", Amount: 50000, Due: date(2026, 7, 28), Movable: true},
		{ID: "jul-b", Name: "Power", Amount: 50000, Due: date(2026, 7, 28), Movable: true},
		{ID: "aug-a", Name: "Rent", Amount: 50000, Due: date(2026, 8, 28), Movable: true},
		{ID: "aug-b", Name: "Power", Amount: 50000, Due: date(2026, 8, 28), Movable: true},
	}
	res := Optimize(1000000, items, paydays, 100000, from, 60, 0)
	if got := maxLoad(res.Smart.Loads); got != 50000 {
		t.Errorf("heaviest paycheck under the plan = %d, want 50000 (both months split)", got)
	}
	if res.EvenGainMinor != 50000 {
		t.Errorf("EvenGainMinor = %d, want 50000", res.EvenGainMinor)
	}
	if len(res.Moves) < 2 {
		t.Fatalf("want pay-ahead moves in both months, got %d: %v", len(res.Moves), res.Moves)
	}
	// At least one move must pull an AUGUST occurrence onto an earlier payday —
	// the cross-month pay-ahead the feature exists for.
	crossMonth := false
	for _, mv := range res.Moves {
		if mv.Item.Due.Month() == time.August && mv.PayOn.Before(mv.Item.Due) {
			crossMonth = true
		}
	}
	if !crossMonth {
		t.Errorf("no August occurrence was paid ahead: %v", res.Moves)
	}
}

func TestOptimizeKeepsMovesWhenGlobalMaxIsImmovable(t *testing.T) {
	// July's stack is all-autopay (immovable, holds the global max); August's
	// stack can move. Evening August is real progress and must be kept even
	// though the global max cannot improve — EvenGainMinor is honestly 0.
	from := date(2026, 7, 1)
	paydays := []time.Time{
		date(2026, 7, 1), date(2026, 7, 15), date(2026, 7, 29),
		date(2026, 8, 12), date(2026, 8, 26),
	}
	items := []Item{
		{ID: "jul-a", Name: "Rent", Amount: 50000, Due: date(2026, 7, 28), Movable: false},
		{ID: "jul-b", Name: "Power", Amount: 50000, Due: date(2026, 7, 28), Movable: false},
		{ID: "aug-a", Name: "Water", Amount: 40000, Due: date(2026, 8, 28), Movable: true},
		{ID: "aug-b", Name: "Trash", Amount: 40000, Due: date(2026, 8, 28), Movable: true},
	}
	res := Optimize(1000000, items, paydays, 100000, from, 60, 0)
	if len(res.Moves) < 1 {
		t.Fatalf("August's movable stack should still be evened, got no moves")
	}
	if res.EvenGainMinor != 0 {
		t.Errorf("EvenGainMinor = %d, want 0 (the global max is the immovable July stack)", res.EvenGainMinor)
	}
	for _, mv := range res.Moves {
		if !mv.Item.Movable {
			t.Errorf("moved an immovable item: %v", mv)
		}
	}
}

func TestOptimizeSameCalendarDayAcrossLocationsIsNotAMove(t *testing.T) {
	// Regression: liability due dates are built in LOCAL time while parsed
	// anchors/paydays are UTC. A bill due ON a payday (same calendar day,
	// different locations) must not be reported as a move — mixed-location
	// midnights made "Jul 17 local" read as after "Jul 17 UTC".
	loc := time.FixedZone("EDT", -4*60*60)
	from := time.Date(2026, 7, 3, 0, 0, 0, 0, loc)
	paydays := []time.Time{date(2026, 7, 3), date(2026, 7, 17)} // UTC midnights
	items := []Item{{ID: "loan", Name: "Loan", Amount: 5000, Due: time.Date(2026, 7, 17, 0, 0, 0, 0, loc), Movable: true}}
	res := Optimize(500000, items, paydays, 100000, from, 30, 0)
	if len(res.Moves) != 0 {
		t.Fatalf("a bill due ON a payday must not be a move, got %v", res.Moves)
	}
	if got := res.PayOnByID["loan"]; got.Format("2006-01-02") != "2026-07-17" {
		t.Errorf("pay-on must stay the due day, got %s", got)
	}
}

// ─── WF9: the exposed day-by-day curve ──────────────────────────────────────

func projDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestProjectWalksEveryDayOfTheHorizon(t *testing.T) {
	from := projDay(2026, time.August, 1)
	pr, ok := Project(100_000, nil, nil, nil, 0, from, 30)
	if !ok {
		t.Fatal("expected a projection")
	}
	if len(pr.Days) != 31 {
		t.Errorf("days = %d, want 31 (inclusive of both ends)", len(pr.Days))
	}
	if pr.Days[0].ClosingMinor != 100_000 || pr.Days[30].ClosingMinor != 100_000 {
		t.Error("a horizon with no movements must stay flat")
	}
}

// "Nothing projected" and "projected, and flat" are different answers.
func TestProjectRefusesANonHorizon(t *testing.T) {
	if _, ok := Project(100_000, nil, nil, nil, 0, projDay(2026, time.August, 1), 0); ok {
		t.Error("a zero horizon must refuse rather than return an empty curve")
	}
	if _, ok := Project(100_000, nil, nil, nil, 0, time.Time{}, 30); ok {
		t.Error("no start date must refuse")
	}
}

func TestIncomeAndDebitsLandOnTheirOwnDays(t *testing.T) {
	from := projDay(2026, time.August, 1)
	items := []Item{{ID: "rent", Amount: 120_000, Due: projDay(2026, time.August, 5)}}
	assign := map[string]time.Time{"rent": projDay(2026, time.August, 5)}
	paydays := []time.Time{projDay(2026, time.August, 10)}
	pr, ok := Project(100_000, items, assign, paydays, 200_000, from, 20)
	if !ok {
		t.Fatal("expected a projection")
	}
	byDate := map[string]Day{}
	for _, d := range pr.Days {
		byDate[d.Date.Format("2006-01-02")] = d
	}
	if got := byDate["2026-08-05"]; got.OutMinor != 120_000 {
		t.Errorf("the 5th shows %d out, want 120000", got.OutMinor)
	}
	if got := byDate["2026-08-10"]; got.InMinor != 200_000 {
		t.Errorf("the 10th shows %d in, want 200000", got.InMinor)
	}
}

// "When does this break" is actionable in a way "how bad does it get" is not,
// and the deepest point is often weeks after the first trouble.
func TestFirstShortfallIsSeparateFromTheLowPoint(t *testing.T) {
	from := projDay(2026, time.August, 1)
	items := []Item{
		{ID: "a", Amount: 150_000, Due: projDay(2026, time.August, 3)},
		{ID: "b", Amount: 100_000, Due: projDay(2026, time.August, 20)},
	}
	assign := map[string]time.Time{
		"a": projDay(2026, time.August, 3),
		"b": projDay(2026, time.August, 20),
	}
	pr, ok := Project(100_000, items, assign, nil, 0, from, 30)
	if !ok {
		t.Fatal("expected a projection")
	}
	if !pr.AnyShortfall() {
		t.Fatal("this horizon goes negative and must say so")
	}
	if !pr.FirstShortDate.Equal(projDay(2026, time.August, 3)) {
		t.Errorf("first shortfall = %v, want the 3rd", pr.FirstShortDate)
	}
	if !pr.LowDate.Equal(projDay(2026, time.August, 20)) {
		t.Errorf("low point = %v, want the 20th — later than the first trouble", pr.LowDate)
	}
	if pr.LowMinor != -150_000 {
		t.Errorf("low = %d, want -150000", pr.LowMinor)
	}
}

func TestDaysShortCountsEveryNegativeDay(t *testing.T) {
	from := projDay(2026, time.August, 1)
	items := []Item{{ID: "a", Amount: 150_000, Due: projDay(2026, time.August, 2)}}
	assign := map[string]time.Time{"a": projDay(2026, time.August, 2)}
	paydays := []time.Time{projDay(2026, time.August, 6)}
	pr, _ := Project(100_000, items, assign, paydays, 200_000, from, 10)
	// Negative from the 2nd through the 5th: four days.
	if pr.DaysShort != 4 {
		t.Errorf("days short = %d, want 4", pr.DaysShort)
	}
}

func TestAHealthyHorizonReportsNoShortfall(t *testing.T) {
	pr, _ := Project(500_000, nil, nil, nil, 0, projDay(2026, time.August, 1), 30)
	if pr.AnyShortfall() || pr.DaysShort != 0 {
		t.Errorf("a flat positive horizon reported a shortfall: %+v", pr)
	}
	if !pr.FirstShortDate.IsZero() {
		t.Error("no shortfall means no first-shortfall date, not a zero-time one to render")
	}
}

// The curve and the low-point metric must agree — two simulators that can
// disagree about the same month is exactly what exposing this avoided.
func TestTheCurveAgreesWithTheMetricsLowPoint(t *testing.T) {
	from := projDay(2026, time.August, 1)
	items := []Item{
		{ID: "a", Amount: 90_000, Due: projDay(2026, time.August, 4)},
		{ID: "b", Amount: 60_000, Due: projDay(2026, time.August, 18)},
	}
	assign := map[string]time.Time{
		"a": projDay(2026, time.August, 4),
		"b": projDay(2026, time.August, 18),
	}
	paydays := []time.Time{projDay(2026, time.August, 15)}
	m := simulate(200_000, items, assign, paydays, 180_000, from, 30)
	pr, _ := Project(200_000, items, assign, paydays, 180_000, from, 30)
	if m.Low != pr.LowMinor || !m.LowDate.Equal(pr.LowDate) {
		t.Errorf("metrics say low %d on %v; curve says %d on %v",
			m.Low, m.LowDate, pr.LowMinor, pr.LowDate)
	}
}
