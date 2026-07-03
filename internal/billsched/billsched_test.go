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

func TestOptimizeAlreadySmoothIsANoOp(t *testing.T) {
	from := date(2026, 7, 1)
	paydays := []time.Time{date(2026, 7, 1), date(2026, 7, 15)}
	// One small bill right after a payday: nothing to improve.
	items := []Item{{ID: "b", Name: "Bill", Amount: 5000, Due: date(2026, 7, 2), Movable: true}}
	res := Optimize(500000, items, paydays, 100000, from, 30, 0)
	if len(res.Moves) != 0 || res.EvenGainMinor != 0 {
		t.Fatalf("smooth schedule should be a no-op, got moves=%v gain=%d", res.Moves, res.EvenGainMinor)
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
