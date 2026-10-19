// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestBCS2RekeyMidpointsAndReplayMarkers(t *testing.T) {
	for _, tc := range []struct {
		name      string
		period    domain.Period
		old, next prefs.Prefs
		key, want string
	}{
		{"Sunday to Monday", domain.PeriodWeekly, prefs.Prefs{WeekStart: prefs.WeekSunday}, prefs.Prefs{WeekStart: prefs.WeekMonday}, "2026-10-11", "2026-10-12"},
		{"Monday to Sunday", domain.PeriodWeekly, prefs.Prefs{WeekStart: prefs.WeekMonday}, prefs.Prefs{WeekStart: prefs.WeekSunday}, "2026-10-12", "2026-10-11"},
		{"anchor plus one", domain.PeriodBiweekly, prefs.Prefs{PayCycleAnchor: "2026-10-09"}, prefs.Prefs{PayCycleAnchor: "2026-10-10"}, "2026-10-09", "2026-10-10"},
		{"anchor minus one", domain.PeriodBiweekly, prefs.Prefs{PayCycleAnchor: "2026-10-09"}, prefs.Prefs{PayCycleAnchor: "2026-10-08"}, "2026-10-09", "2026-10-08"},
		{"anchor plus seven", domain.PeriodBiweekly, prefs.Prefs{PayCycleAnchor: "2026-10-09"}, prefs.Prefs{PayCycleAnchor: "2026-10-16"}, "2026-10-09", "2026-10-16"},
		{"anchor minus seven", domain.PeriodBiweekly, prefs.Prefs{PayCycleAnchor: "2026-10-09"}, prefs.Prefs{PayCycleAnchor: "2026-10-02"}, "2026-10-09", "2026-10-16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := sharedBudget("b", "Food", "food", 30000)
			b.Period = tc.period
			b.PeriodBoosts = map[string]int64{tc.key: 5000}
			b.PeriodTopups = map[string]int64{tc.key: 700}
			b.LimitHistory = map[string]int64{tc.key: 30000}
			b.PeriodNotes = map[string]string{tc.key: "Guests"}
			b.RolloverStart = moneyDate(tc.key)
			b.RecurringCover = &domain.RecurringCover{LastAppliedPeriod: tc.key, AmountMinor: 4300}
			before, _ := json.Marshal(b)
			got, err := RekeyBudgetPeriodData([]domain.Budget{b}, tc.old, tc.next)
			if err != nil {
				t.Fatal(err)
			}
			next := got[0]
			if next.PeriodBoosts[tc.want] != 5000 || next.PeriodTopups[tc.want] != 700 || next.LimitHistory[tc.want] != 30000 || next.PeriodNotes[tc.want] != "Guests" || next.RolloverStart.Format(time.DateOnly) != tc.want || next.RecurringCover.LastAppliedPeriod != tc.want {
				t.Fatalf("migration=%+v", next)
			}
			after, _ := json.Marshal(b)
			if string(before) != string(after) {
				t.Fatal("input mutated")
			}
			again, err := RekeyBudgetPeriodData(got, tc.next, tc.next)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatalf("migration is not idempotent: %v", err)
			}
		})
	}
}

func TestBCS2PreferenceBoundaryRekeysAndPersistsOnce(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-14") }
	from, to := sharedBudget("from", "Source", "source", 30000), sharedBudget("to", "Destination", "dest", 10000)
	to.Period = domain.PeriodWeekly
	to.RecurringCover = &domain.RecurringCover{AmountMinor: 5000, Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}
	for _, b := range []domain.Budget{from, to} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	shares := map[string]money.Money{"from": money.New(5000, "USD")}
	if err := a.ApplyRecurringBudgetCoverAt("to", shares, a.clock()); err != nil {
		t.Fatal(err)
	}
	cursor, _ := a.store.ChangeCursor()
	next := prefs.Default()
	next.WeekStart = prefs.WeekMonday
	if err := a.SaveBudgetPeriodPreferences(next); err != nil {
		t.Fatal(err)
	}
	got := bc4Budget(t, a, "to")
	if got.PeriodBoosts["2026-10-12"] != 5000 || got.RecurringCover.LastAppliedPeriod != "2026-10-12" {
		t.Fatalf("got=%+v", got)
	}
	p, _, err := a.budgetPreferences()
	if err != nil || p.WeekStart != prefs.WeekMonday {
		t.Fatalf("preferences=%+v err=%v", p, err)
	}
	end, _ := a.store.ChangeCursor()
	page, err := a.store.CaptureChanges(cursor, end, 100)
	if err != nil {
		t.Fatal(err)
	}
	hasBudget, hasPrefs := false, false
	for _, change := range page.Changes {
		hasBudget = hasBudget || change.Table == "budgets"
		hasPrefs = hasPrefs || change.Table == "settingskv"
	}
	if !hasBudget || !hasPrefs {
		t.Fatal("migration omitted journal records")
	}
	before, _ := json.Marshal(a.Budgets())
	if err := a.ApplyRecurringBudgetCoverAt("to", shares, a.clock()); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(a.Budgets())
	if string(before) != string(after) {
		t.Fatal("cover replayed after migration")
	}
	if err := a.SaveBudgetPeriodPreferences(next); err != nil {
		t.Fatal(err)
	}
	after, _ = json.Marshal(a.Budgets())
	if string(before) != string(after) {
		t.Fatal("repeated preference save changed money")
	}
}

func TestBCS2FailedPreferenceMigrationPreservesAllState(t *testing.T) {
	a := newApp(t, false)
	good, bad := sharedBudget("a", "Good", "good", 30000), sharedBudget("z", "Bad", "bad", 30000)
	good.Period = domain.PeriodWeekly
	bad.Period = domain.PeriodWeekly
	good.PeriodBoosts = map[string]int64{"2026-10-11": 5000}
	bad.PeriodNotes = map[string]string{"broken-date": "Do not silently drop me"}
	for _, b := range []domain.Budget{good, bad} {
		if err := a.store.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := json.Marshal(a.Budgets())
	next := prefs.Default()
	next.WeekStart = prefs.WeekMonday
	if err := a.SaveBudgetPeriodPreferences(next); err == nil {
		t.Fatal("malformed source key accepted")
	}
	after, _ := json.Marshal(a.Budgets())
	if string(before) != string(after) {
		t.Fatal("partial migration")
	}
	p, _, _ := a.budgetPreferences()
	if p.WeekStart != prefs.WeekSunday {
		t.Fatal("preferences advanced after migration failure")
	}
	good.PeriodNotes = nil
	if err := a.store.PutBudget(good); err != nil {
		t.Fatal(err)
	}
	writes := []domain.Budget{good}
	writes[0].Limit.Amount++
	if err := a.MigrateBudgetPeriodsAtomic(writes, time.Monday, time.Time{}); err == nil {
		t.Fatal("migration bypassed standing plan policy")
	}
}

func TestBCS2CadenceChangePreservesCoverAndReplay(t *testing.T) {
	for _, changeSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "destination", true: "source"}[changeSource], func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-10-19") }
			from, to := sharedBudget("from", "Source", "source", 30000), sharedBudget("to", "Destination", "dest", 10000)
			to.RecurringCover = &domain.RecurringCover{AmountMinor: 5000, Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}
			for _, b := range []domain.Budget{from, to} {
				if err := a.PutBudget(b); err != nil {
					t.Fatal(err)
				}
			}
			shares := map[string]money.Money{"from": money.New(5000, "USD")}
			if err := a.ApplyRecurringBudgetCoverAt("to", shares, a.clock()); err != nil {
				t.Fatal(err)
			}
			id := "to"
			if changeSource {
				id = "from"
			}
			b := bc4Budget(t, a, id)
			b.Period = domain.PeriodWeekly
			if err := a.PutBudgetAt(b, a.clock()); err != nil {
				t.Fatal(err)
			}
			if err := a.ApplyRecurringBudgetCoverAt("to", shares, a.clock()); err != nil {
				t.Fatal(err)
			}
			var total int64
			for _, b := range a.Budgets() {
				for _, boost := range b.PeriodBoosts {
					total += boost
				}
			}
			if total != 0 {
				t.Fatalf("cadence change lost money: %+v", a.Budgets())
			}
			for _, b := range a.Budgets() {
				start, _ := budgeting.PeriodRangeAnchored(b.Period, a.clock(), time.Sunday, time.Time{})
				want := int64(5000)
				if b.ID == "from" {
					want = -5000
				}
				if b.PeriodBoost(start) != want {
					t.Fatalf("%s leg=%v want=%d", b.ID, b.PeriodBoosts, want)
				}
			}
		})
	}
}

func TestBCS2LegacyBoostsPreserveMoneyAndProvenanceAcrossExport(t *testing.T) {
	a := newApp(t, false)
	b := sharedBudget("b", "Food", "food", 30000)
	b.Period = domain.PeriodWeekly
	b.PeriodBoosts = map[string]int64{"2026-10-11": 5000, "2026-10-04": -3000}
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	next := prefs.Default()
	next.WeekStart = prefs.WeekMonday
	for range 2 {
		if err := a.SaveBudgetPeriodPreferences(next); err != nil {
			t.Fatal(err)
		}
	}
	got := a.Budgets()[0]
	if got.PeriodBoosts["2026-10-12"] != 5000 || got.PeriodBoosts["2026-10-05"] != -3000 || len(got.PeriodTopups) != 0 {
		t.Fatalf("legacy migration=%+v", got)
	}
	periods := func(domain.Budget) (time.Time, time.Time) { return moneyDate("2026-10-12"), moneyDate("2026-10-19") }
	if plan := monthclose.CopyBoosts([]domain.Budget{got}, periods, nil); len(plan) != 0 {
		t.Fatal("legacy boosts copied")
	}
	got, err := got.WithPeriodBoostChecked(moneyDate("2026-10-12"), 700)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.PutBudgetAt(got, moneyDate("2026-10-14")); err != nil {
		t.Fatal(err)
	}
	data, err := a.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := store.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Budgets) != 1 || !reflect.DeepEqual(ds.Budgets[0].PeriodTopups, got.PeriodTopups) {
		t.Fatal("top-up provenance lost on export")
	}
	if plan := monthclose.CopyBoosts(ds.Budgets, periods, nil); plan["b"] != 700 {
		t.Fatalf("copy=%v", plan)
	}
}

func TestBCS2AdjustPermanentIsForwardOnlyFromEditedHistory(t *testing.T) {
	a := newApp(t, false)
	b := sharedBudget("b", "Food", "food", 60000)
	b.LimitHistory = map[string]int64{"2026-01-01": 30000, "2026-10-01": 60000}
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	ref := moneyDate("2026-08-19")
	starts := func(domain.Budget) time.Time { return moneyDate("2026-08-01") }
	bases, err := budgeting.AdjustBasesAt(a.Budgets(), starts)
	if err != nil {
		t.Fatal(err)
	}
	preview := budgeting.AdjustAllPreviewFor(bases, 10, budgeting.AdjustEveryPeriod, budgeting.AdjustOverlays{"b": 0})
	writes, err := budgeting.ApplyAdjustChecked(preview, budgeting.AdjustEveryPeriod, starts)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.PutBudgetsAtomicAt(writes, ref); err != nil {
		t.Fatal(err)
	}
	for date, want := range map[string]int64{"2026-07-19": 30000, "2026-08-19": 33000, "2026-10-19": 33000, "2027-01-19": 33000} {
		bc4Cap(t, a, "b", date, want)
	}
}

func TestBCS2CadenceCollisionAddsCheckedAmounts(t *testing.T) {
	b := sharedBudget("b", "Food", "food", 30000)
	b.Period = domain.PeriodWeekly
	b.PeriodBoosts = map[string]int64{"2026-10-04": 100, "2026-10-11": 200}
	next := b
	next.Period = domain.PeriodMonthly
	got, err := prepareBudgetWrite(next, &b, moneyDate("2026-10-19"), time.Sunday, time.Time{})
	if err != nil || got.PeriodBoosts["2026-10-01"] != 300 {
		t.Fatalf("got=%v err=%v", got.PeriodBoosts, err)
	}
	b.PeriodBoosts["2026-10-04"] = math.MaxInt64
	if _, err := prepareBudgetWrite(next, &b, moneyDate("2026-10-19"), time.Sunday, time.Time{}); err == nil {
		t.Fatal("collision sum overflow accepted")
	}
}
