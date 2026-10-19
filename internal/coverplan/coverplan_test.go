// SPDX-License-Identifier: MIT

package coverplan

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func over(id string, amount int64) Over {
	return Over{ID: id, Name: id, Currency: "USD", Shortfall: money.New(amount, "USD")}
}

func source(id string, amount int64) Source {
	return Source{ID: id, Name: id, Currency: "USD", Avail: money.New(amount, "USD")}
}

func translate(key string, args ...any) string { return key + fmt.Sprint(args...) }
func format(m money.Money) string              { return fmt.Sprintf("%s:%d", m.Currency, m.Amount) }

func TestMonthlyAmounts(t *testing.T) {
	for _, tc := range []struct {
		name          string
		amount        int64
		period        domain.Period
		monthly, room int64
		err           bool
	}{
		{"weekly", 2500, domain.PeriodWeekly, 10833, 2500, false},
		{"yearly", 330000, domain.PeriodYearly, 27500, 27500, false},
		{"zero", 0, domain.PeriodMonthly, 0, 0, false},
		{"negative", -1200, domain.PeriodYearly, -100, -1200, false},
		{"round negative", -1, domain.PeriodWeekly, -4, -4, false},
		{"overflow", math.MaxInt64, domain.PeriodWeekly, 0, 0, true},
		{"underflow", math.MinInt64, domain.PeriodWeekly, 0, 0, true},
		{"unknown cadence", 100, domain.Period("bad"), 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monthly, err := MonthlyEquivalent(money.New(tc.amount, "JPY"), tc.period)
			if (err != nil) != tc.err || monthly.Amount != tc.monthly {
				t.Fatalf("monthly=%v, %v", monthly, err)
			}
			room, err := MonthlyRoom(money.New(tc.amount, "JPY"), tc.period)
			if (err != nil) != tc.err || room.Amount != tc.room {
				t.Fatalf("room=%v, %v", room, err)
			}
			if !tc.err && (monthly.Currency != "JPY" || room.Currency != "JPY") {
				t.Fatal("currency changed")
			}
		})
	}
	for _, tc := range []struct {
		code string
		want int64
	}{{"USD", 100}, {"JPY", 1}, {"???", 100}} {
		if got := monthlyDonorMinimum(tc.code); got != tc.want {
			t.Fatalf("minimum(%s)=%d", tc.code, got)
		}
	}
}

func TestPrepare(t *testing.T) {
	for _, available := range []bool{false, true} {
		for _, saving := range []bool{false, true} {
			for _, mixed := range []bool{false, true} {
				if got := (Model{CurrencyError: mixed}).CanStart(available, saving); got != (available && !saving && !mixed) {
					t.Fatalf("CanStart(%v, %v), mixed=%v: %v", available, saving, mixed, got)
				}
			}
		}
	}
	rows := []Over{over("a", 5), over("b", 7)}
	foreign := source("eur", 10)
	foreign.Currency = "EUR"
	for _, tc := range []struct {
		name              string
		rows              []Over
		sources           []Source
		target, base, sig string
		count, decimals   int
		mixed             bool
	}{
		{"empty", nil, nil, "", "JPY", "", 0, 0, false},
		{"all", rows, nil, "", "USD", "a;b;", 2, 2, false},
		{"target", rows, nil, "b", "USD", "b;", 1, 2, false},
		{"missing", rows, nil, "missing", "JPY", "", 0, 0, false},
		{"foreign donor", rows, []Source{foreign}, "a", "JPY", "a;", 1, 0, true},
		{"only donor", nil, []Source{foreign}, "", "EUR", "", 0, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Prepare(tc.rows, tc.sources, "JPY", tc.target)
			if got.Base != tc.base || got.Signature != tc.sig || len(got.Overs) != tc.count || got.Empty != (tc.count == 0) || got.CurrencyError != tc.mixed || got.Decimals != tc.decimals {
				t.Fatalf("model=%+v", got)
			}
		})
	}
	if rows[0].ID != "a" || rows[1].ID != "b" {
		t.Fatal("target filtering mutated caller")
	}
	foreignOver := over("eur", 3)
	foreignOver.Currency = "EUR"
	if !Prepare(append(rows, foreignOver), nil, "USD", "").CurrencyError {
		t.Fatal("mixed overages accepted")
	}
}

func TestSelectionHelpers(t *testing.T) {
	for _, tc := range []struct {
		signature, previous string
		reset               bool
	}{
		{"", "", false}, {"a;", "", true}, {"a;", "a;", false}, {"b;", "a;", true}, {"", "b;", true},
	} {
		choices, reset := SeedChoices(Model{Signature: tc.signature}, tc.previous)
		if reset != tc.reset || (choices != nil) != tc.reset || len(choices) != 0 {
			t.Fatalf("seed=%v reset=%v for %+v", choices, reset, tc)
		}
	}
	for _, tc := range []struct {
		name        string
		chosen      map[string]string
		donor, next bool
	}{
		{"nil", nil, false, false}, {"skip", map[string]string{"a": ""}, false, false},
		{"donor", map[string]string{"a": "s"}, true, false}, {"next", map[string]string{"a": NextSource}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if HasDonor(tc.chosen) != tc.donor || HasNextMonthSelection(tc.chosen) != tc.next {
				t.Fatal("selection flags")
			}
			next := Choose(tc.chosen, "a", "new")
			if next["a"] != "new" || tc.chosen["a"] == "new" {
				t.Fatal("choice did not clone map")
			}
		})
	}
	sources := []Source{source("usd", 2), source("other", 3), source("usd2", 4)}
	sources[1].Currency = "EUR"
	for _, tc := range []struct {
		code, first string
		count       int
	}{{"USD", "usd", 2}, {"EUR", "other", 1}, {"JPY", "", 0}} {
		o := Over{Currency: tc.code}
		if DefaultSource(o, sources) != tc.first || len(SourcesFor(o, sources)) != tc.count {
			t.Fatalf("donors for %s", tc.code)
		}
	}
	if DefaultSource(Over{}, nil) != "" || len(SourcesFor(Over{}, nil)) != 0 {
		t.Fatal("nil sources")
	}
	for _, tc := range []struct {
		a, b Source
		less bool
	}{
		{Source{Name: "Z", FreeRoom: money.New(10, "USD")}, Source{Name: "A", FreeRoom: money.New(5, "USD")}, true},
		{Source{Name: "A"}, Source{Name: "B"}, true}, {Source{Name: "B"}, Source{Name: "A"}, false}, {Source{}, Source{}, false},
	} {
		if SourceLess(tc.a, tc.b) != tc.less {
			t.Fatal("rank")
		}
	}
	date := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	if !CoveredAt(Over{ThisStart: date}).Equal(date) || !CoveredAt(Over{}).IsZero() {
		t.Fatal("covered period")
	}
}

func TestSelectedTotals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rows   []Over
		chosen map[string]string
		amount int64
		count  int
		err    bool
	}{
		{"nil", nil, nil, 0, 0, false},
		{"selected", []Over{over("a", 10), over("b", 20)}, map[string]string{"a": "s", "stale": "s"}, 10, 1, false},
		{"signed", []Over{over("a", -10), over("b", 0)}, map[string]string{"a": "s", "b": NextSource}, -10, 2, false},
		{"overflow", []Over{over("a", math.MaxInt64), over("b", 1)}, map[string]string{"a": "s", "b": "s"}, 0, 0, true},
		{"underflow", []Over{over("a", math.MinInt64), over("b", -1)}, map[string]string{"a": "s", "b": "s"}, 0, 0, true},
		{"unavailable", []Over{{ID: "a", Error: money.ErrOverflow}}, map[string]string{"a": "s"}, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, c, err := SelectedTotalChecked(tc.rows, tc.chosen)
			if a != tc.amount || c != tc.count || (err != nil) != tc.err {
				t.Fatalf("got %d/%d/%v", a, c, err)
			}
			if tc.err && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("error cause=%v", err)
			}
			if a2, c2 := SelectedTotal(tc.rows, tc.chosen); a2 != a || c2 != c {
				t.Fatal("unchecked helper differs")
			}
		})
	}
}

func TestSubmission(t *testing.T) {
	a, b := over("a", 60), over("b", 50)
	roll := a
	roll.Rollover = true
	foreign := source("s", 100)
	foreign.Currency = "EUR"
	for _, tc := range []struct {
		name     string
		rows     []Over
		sources  []Source
		chosen   map[string]string
		count    int
		key      string
		overflow bool
	}{
		{"empty", nil, nil, nil, 0, "", false},
		{"skip", []Over{a}, nil, nil, 0, "", false},
		{"donor", []Over{a}, []Source{source("s", 60)}, map[string]string{"a": "s"}, 1, "", false},
		{"next", []Over{a}, nil, map[string]string{"a": NextSource}, 1, "", false},
		{"rollover", []Over{roll}, nil, map[string]string{"a": NextSource}, 0, "coverAll.rolloverBorrowBlocked", false},
		{"missing", []Over{a}, nil, map[string]string{"a": "s"}, 0, "coverAll.currencyError", false},
		{"foreign", []Over{a}, []Source{foreign}, map[string]string{"a": "s"}, 0, "coverAll.currencyError", false},
		{"aggregate shortage", []Over{a, b}, []Source{source("s", 100)}, map[string]string{"a": "s", "b": "s"}, 0, "coverAll.sourceShort", false},
		{"zero", []Over{over("a", 0)}, nil, map[string]string{"a": NextSource}, 0, "", false},
		{"negative", []Over{over("a", -1)}, nil, map[string]string{"a": NextSource}, 0, "", false},
		{"total overflow", []Over{over("a", math.MaxInt64), over("b", 1)}, nil, map[string]string{"a": NextSource, "b": NextSource}, 0, "", true},
		{"draw overflow", []Over{over("a", math.MaxInt64), over("b", -math.MaxInt64), over("c", 1)}, []Source{source("s", math.MaxInt64)}, map[string]string{"a": "s", "b": NextSource, "c": "s"}, 0, "", true},
		{"shortage overflow", []Over{a}, []Source{source("s", math.MinInt64)}, map[string]string{"a": "s"}, 0, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, problem := Submission(Prepare(tc.rows, tc.sources, "USD", ""), tc.chosen)
			if tc.key != "" || tc.overflow {
				if problem == nil || problem.Key != tc.key || errors.Is(problem.Err, money.ErrOverflow) != tc.overflow {
					t.Fatalf("problem=%+v", problem)
				}
				if tc.key == "coverAll.sourceShort" && (problem.Name != "s" || problem.Amount.Amount != 10) {
					t.Fatalf("shortage=%+v", problem)
				}
				return
			}
			if problem != nil || len(got) != tc.count {
				t.Fatalf("requests=%+v problem=%+v", got, problem)
			}
			if len(got) == 1 {
				if got[0].DestinationID != "a" {
					t.Fatal("destination")
				}
				if tc.chosen["a"] == NextSource {
					if got[0].BorrowNext != a.Shortfall || len(got[0].Shares) != 0 {
						t.Fatal("borrow request")
					}
				} else if got[0].Shares["s"] != a.Shortfall || got[0].BorrowNext.Amount != 0 {
					t.Fatal("donor request")
				}
			}
		})
	}
}

func TestPresentation(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		rows                 []Over
		sources              []Source
		chosen               map[string]string
		saving               bool
		intro, label, impact string
		disabled, hint       bool
	}{
		{"empty", nil, nil, nil, false, "coverAll.intro0USD:0", "bv7g.cover.applyEmpty", "budgets.impactReachThisPeriod", true, false},
		{"one", []Over{over("a", 10)}, nil, nil, false, "coverAll.introOneUSD:10", "bv7g.cover.applyEmpty", "budgets.impactReachThisPeriod", true, false},
		{"donor", []Over{over("a", 10)}, nil, map[string]string{"a": "s"}, false, "coverAll.introOneUSD:10", "bv7g.cover.applyUSD:10", "budgets.impactReachThisPeriodFunded", false, false},
		{"next", []Over{over("a", 10)}, nil, map[string]string{"a": NextSource}, false, "coverAll.introOneUSD:10", "bv7g.cover.applyUSD:10", "budgets.impactReachThisAndNext", false, true},
		{"saving", []Over{over("a", 10)}, nil, map[string]string{"a": "s"}, true, "coverAll.introOneUSD:10", "bv7g.cover.applyUSD:10", "budgets.impactReachThisPeriodFunded", true, false},
		{"both", []Over{over("a", 10), over("b", 20)}, nil, map[string]string{"a": "s", "b": NextSource}, false, "coverAll.intro2USD:30", "bv7g.cover.applyUSD:30", "budgets.impactReachThisAndNextFunded", false, true},
		{"total overflow", []Over{over("a", math.MaxInt64), over("b", 1)}, nil, map[string]string{"b": NextSource}, false, "budgets.incomplete", "bv7g.cover.applyUSD:1", "budgets.impactReachThisAndNext", true, true},
		{"selected overflow", []Over{over("a", math.MaxInt64), over("b", -1), over("c", 1)}, nil, map[string]string{"a": "s", "c": "s"}, false, "budgets.incomplete", "bv7g.cover.applyEmpty", "budgets.impactReachThisPeriodFunded", true, false},
		{"unavailable", []Over{{ID: "a", Error: money.ErrOverflow}}, nil, nil, false, "budgets.incomplete", "bv7g.cover.applyEmpty", "budgets.impactReachThisPeriod", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Present(Prepare(tc.rows, tc.sources, "USD", ""), tc.chosen, tc.saving, translate, format)
			want := Display{Intro: tc.intro, ApplyLabel: tc.label, ImpactKey: tc.impact, Disabled: tc.disabled, NextMonthHint: tc.hint}
			if got != want {
				t.Fatalf("got %+v; want %+v", got, want)
			}
		})
	}
	foreign := source("s", 100)
	foreign.Currency = "EUR"
	m := Prepare([]Over{over("a", 1)}, []Source{foreign}, "USD", "")
	if got := Present(m, map[string]string{"a": "s"}, false, translate, format); got.Intro != "coverAll.currencyError" || !got.Disabled {
		t.Fatalf("mixed=%+v", got)
	}
	m.Overs = []Over{over("a", math.MaxInt64), over("b", 1)}
	if got := Present(m, nil, false, translate, format); got.Intro != "budgets.incomplete" {
		t.Fatal("overflow must take precedence")
	}
}

func TestMessages(t *testing.T) {
	for _, tc := range []struct {
		count int
		want  Completion
	}{
		{0, Completion{Empty: true, Text: "coverAll.doneNone"}},
		{1, Completion{Text: "coverAll.doneOne"}},
		{2, Completion{Text: "coverAll.doneMany2"}},
	} {
		model := Prepare([]Over{over("a", 1)}, nil, "USD", "")
		if got := Complete(model, nil, make([]budgeting.CoverRequest, tc.count), translate, format); got != tc.want {
			t.Fatalf("completion=%+v want %+v", got, tc.want)
		}
	}
	for _, tc := range []struct {
		rows []Over
		want string
	}{
		{nil, "ckpt.beforeCoverAllbudgets.countMany0"},
		{[]Over{over("a", 1)}, "ckpt.beforeCoverAllbudgets.countOne"},
		{[]Over{over("a", 1), over("b", 2)}, "ckpt.beforeCoverAllbudgets.countMany2"},
	} {
		if got := CheckpointText(Prepare(tc.rows, nil, "USD", ""), translate); got != tc.want {
			t.Fatalf("checkpoint=%q want %q", got, tc.want)
		}
	}
	for _, tc := range []struct {
		problem Problem
		want    string
	}{
		{Problem{Err: money.ErrOverflow}, "error:money: amount overflow"},
		{Problem{Key: "coverAll.currencyError"}, "coverAll.currencyError"},
		{Problem{Key: "coverAll.rolloverBorrowBlocked", Name: "Food"}, "coverAll.rolloverBorrowBlockedFood"},
		{Problem{Key: "coverAll.sourceShort", Name: "Food", Amount: money.New(10, "USD")}, "coverAll.sourceShortFoodUSD:10"},
	} {
		if got := ProblemText(&tc.problem, translate, format, func(err error) string { return "error:" + err.Error() }); got != tc.want {
			t.Fatalf("text=%q want %q", got, tc.want)
		}
	}
	m := Prepare([]Over{over("a", 10), over("b", 20)}, []Source{source("s", 100)}, "USD", "")
	for _, tc := range []struct {
		chosen  map[string]string
		applied int
		want    string
	}{
		{nil, 2, "coverAll.doneMany2"}, {nil, 0, "coverAll.doneMany0"},
		{map[string]string{"a": NextSource}, 1, "coverAll.doneOne"},
		{map[string]string{"a": "s"}, 1, "br3dlg.coveredFromUSD:10s"},
		{map[string]string{"b": "s"}, 1, "coverAll.doneOne"},
	} {
		if got := CompletionText(m, tc.chosen, tc.applied, translate, format); got != tc.want {
			t.Fatalf("notice=%q want %q", got, tc.want)
		}
	}
	if got := CompletionText(Model{}, nil, 1, translate, format); got != "coverAll.doneMany1" {
		t.Fatal(got)
	}
}

func dataInput() budgeting.ViewInput {
	anchor := time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)
	return budgeting.ViewInput{Base: "USD", Rates: currency.Rates{Base: "USD"}, Period: budgeting.ViewPeriod{Anchor: anchor, Now: anchor, WeekStart: time.Monday}}
}

func status(id string, amount int64, state budgeting.State) budgeting.Status {
	b := domain.Budget{ID: id, Name: id, CategoryID: id, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(amount, "USD")}
	return budgeting.Status{Budget: b, State: state, Remaining: money.New(amount, "USD")}
}

func TestData(t *testing.T) {
	in := dataInput()
	if a, b := Data(nil, in, in.Period); a != nil || b != nil {
		t.Fatal("empty data")
	}
	rows := []budgeting.Status{status("z-over", -10, budgeting.StateOver), status("a-over", -20, budgeting.StateOver), status("small", 100, budgeting.StateOK), status("first", 1001, budgeting.StateOK), status("second", 1001, budgeting.StateOK), status("near", 5000, budgeting.StateNear), status("zero", 0, budgeting.StateOK), status("saving", 5000, budgeting.StateOK), status("no-room", 1, budgeting.StateOK), status("fx", 1000, budgeting.StateOK)}
	rows[7].Budget.Direction = domain.DirectionSave
	rows[8].Remaining.Amount = 10
	rows[9].Budget.Limit.Currency = "EUR"
	a, b := Data(rows, in, in.Period)
	if len(a) != 2 || a[0].ID != "a-over" || a[0].Shortfall.Amount != 20 || len(b) != 2 || b[0].ID != "first" || b[1].ID != "second" || b[0].Avail.Amount != 1000 {
		t.Fatalf("overs=%+v donors=%+v", a, b)
	}
	if dateutil.FormatDate(a[0].ThisStart) != "2024-02-01" || dateutil.FormatDate(a[0].NextStart) != "2024-03-01" || b[0].Monthly.Amount != 1000 {
		t.Fatal("periods or monthly room")
	}
	if rows[0].Budget.ID != "z-over" {
		t.Fatal("sorted caller's statuses")
	}
	// Equal room/name donors must retain their input order.
	rows = []budgeting.Status{status("second", 1001, budgeting.StateOK), status("first", 1001, budgeting.StateOK)}
	rows[0].Budget.Name = "same"
	rows[1].Budget.Name = "same"
	_, b = Data(rows, in, in.Period)
	if len(b) != 2 || b[0].ID != "second" {
		t.Fatalf("unstable donors=%+v", b)
	}
	for _, tc := range []struct {
		name   string
		amount int64
		period domain.Period
	}{
		{"invalid period", 10000, domain.Period("bad")}, {"monthly overflow", math.MaxInt64, domain.PeriodWeekly}, {"nonpositive", -5, domain.PeriodMonthly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := status("s", tc.amount, budgeting.StateOK)
			st.Budget.Period = tc.period
			st.Remaining.Amount = 1
			_, got := Data([]budgeting.Status{st}, in, in.Period)
			if len(got) != 0 {
				t.Fatalf("donors=%+v", got)
			}
		})
	}
	st := status("min", math.MinInt64, budgeting.StateOver)
	a, _ = Data([]budgeting.Status{st}, in, in.Period)
	if len(a) != 1 || !errors.Is(a[0].Error, money.ErrOverflow) {
		t.Fatalf("unrepresentable over=%+v", a)
	}
	st.Remaining = money.New(0, "USD")
	a, _ = Data([]budgeting.Status{st}, in, in.Period)
	if a[0].Shortfall.Amount != 0 {
		t.Fatal("nonnegative remaining")
	}
}

func TestDataCalendarBoundariesAndMissingFX(t *testing.T) {
	for _, tc := range []struct {
		name       string
		anchor     time.Time
		cadence    domain.Period
		start, end string
	}{
		{"leap month end", time.Date(2024, 2, 29, 23, 0, 0, 0, time.FixedZone("west", -8*3600)), domain.PeriodMonthly, "2024-02-01", "2024-03-01"},
		{"year end", time.Date(2026, 12, 31, 23, 0, 0, 0, time.FixedZone("east", 14*3600)), domain.PeriodYearly, "2026-01-01", "2027-01-01"},
		{"weekly", time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("west", -8*3600)), domain.PeriodWeekly, "2026-10-26", "2026-11-02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := dataInput()
			p := in.Period
			p.Anchor = dateutil.WallClockUTC(tc.anchor)
			st := status("over", -1, budgeting.StateOver)
			st.Budget.Period = tc.cadence
			got, _ := Data([]budgeting.Status{st}, in, p)
			if dateutil.FormatDate(got[0].ThisStart) != tc.start || dateutil.FormatDate(got[0].NextStart) != tc.end {
				t.Fatalf("bounds=%+v", got)
			}
		})
	}
	in := dataInput()
	st := status("eur", 1001, budgeting.StateOK)
	st.Budget.Limit.Currency = "EUR"
	if _, got := Data([]budgeting.Status{st}, in, in.Period); len(got) != 0 {
		t.Fatal("missing FX donor offered")
	}
	in.Rates.Rates = map[string]float64{"EUR": 1.1}
	if _, got := Data([]budgeting.Status{st}, in, in.Period); len(got) != 1 || got[0].Currency != "EUR" {
		t.Fatalf("native FX donor=%+v", got)
	}
}

func TestLargeSelectionsAndPlannerReconciliation(t *testing.T) {
	const n = 10000
	rows := make([]Over, n)
	chosen := make(map[string]string, n)
	for i := range rows {
		rows[i] = over(fmt.Sprint(i), 1)
		chosen[rows[i].ID] = "s"
	}
	m := Prepare(rows, []Source{source("s", n)}, "USD", "")
	requests, problem := Submission(m, chosen)
	if problem != nil || len(requests) != n {
		t.Fatalf("large selection=%d %+v", len(requests), problem)
	}
	if a, c, err := SelectedTotalChecked(rows, chosen); a != n || c != n || err != nil {
		t.Fatal("large total")
	}
	in := dataInput()
	dest := status("a", 1000, budgeting.StateOK).Budget
	donor := status("s", 2000, budgeting.StateOK).Budget
	in.Transactions = []domain.Transaction{{ID: "expense", CategoryID: "a", Date: in.Period.Anchor, Amount: money.New(-1100, "USD")}}
	m = Prepare([]Over{over("a", 100)}, []Source{source("s", 1999)}, "USD", "")
	r, problem := Submission(m, map[string]string{"a": "s"})
	if problem != nil {
		t.Fatal(problem)
	}
	writes, err := budgeting.PlanCover([]domain.Budget{dest, donor}, r, in)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, b := range writes {
		for _, amount := range b.PeriodBoosts {
			sum += amount
		}
	}
	if len(writes) != 2 || sum != 0 {
		t.Fatalf("unbalanced cover=%+v sum=%d", writes, sum)
	}
}

func FuzzSelectionReconciliation(f *testing.F) {
	f.Add([]byte{1, 2, 0, 255}, true)
	f.Add([]byte{}, false)
	f.Fuzz(func(t *testing.T, amounts []byte, next bool) {
		if len(amounts) > 10000 {
			amounts = amounts[:10000]
		}
		rows := make([]Over, len(amounts))
		chosen := map[string]string{}
		var want int64
		count := 0
		for i, a := range amounts {
			id := fmt.Sprint(i)
			rows[i] = over(id, int64(a))
			if i%2 == 0 {
				chosen[id] = "s"
				if next {
					chosen[id] = NextSource
				}
				want += int64(a)
				count++
			}
		}
		a, c, err := SelectedTotalChecked(rows, chosen)
		if err != nil || a != want || c != count {
			t.Fatal("selected total invariant")
		}
		before := slices.Clone(rows)
		old := Choose(chosen, "extra", "")
		r, p := Submission(Prepare(rows, []Source{source("s", want)}, "USD", ""), chosen)
		if p != nil {
			t.Fatal(p)
		}
		var total int64
		for _, request := range r {
			total += request.BorrowNext.Amount
			for _, share := range request.Shares {
				total += share.Amount
			}
		}
		if total != want || !reflect.DeepEqual(rows, before) || !reflect.DeepEqual(Choose(chosen, "extra", ""), old) {
			t.Fatal("request or immutability invariant")
		}
	})
}

func FuzzStableDonorRanking(f *testing.F) {
	f.Add([]byte{1, 2, 1, 0, 255, 2})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, amounts []byte) {
		if len(amounts) > 2048 {
			amounts = amounts[:2048]
		}
		sources := make([]Source, len(amounts))
		positions := map[string]int{}
		for i, amount := range amounts {
			sources[i] = Source{ID: fmt.Sprint(i), Name: fmt.Sprint(amount % 3), FreeRoom: money.New(int64(int8(amount)), "USD")}
			positions[sources[i].ID] = i
		}
		sort.SliceStable(sources, func(i, j int) bool { return SourceLess(sources[i], sources[j]) })
		for i := 1; i < len(sources); i++ {
			a, b := sources[i-1], sources[i]
			if SourceLess(b, a) {
				t.Fatal("donors not ranked")
			}
			if !SourceLess(a, b) && positions[a.ID] > positions[b.ID] {
				t.Fatal("equal donors changed relative order")
			}
		}
	})
}
