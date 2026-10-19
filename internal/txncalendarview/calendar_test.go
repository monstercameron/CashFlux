// SPDX-License-Identifier: MIT

package txncalendarview

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnscope"
)

func labels() Labels {
	bundle := i18n.DefaultBundle()
	return Labels{T: func(key string, args ...any) string { return bundle.T(i18n.English, key, args...) }, Ledger: func(m money.Money, _ bool) (string, string) {
		text := money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
		if m.Amount < 0 {
			text = "−" + strings.Trim(text, "()")
		} else if m.Amount > 0 {
			text = "+" + text
		}
		return text, NetTone(m.Amount)
	}, Prefs: prefs.Prefs{DateStyle: prefs.DateISO}, ScopePrefs: prefs.Prefs{DateStyle: prefs.DateISO}}
}

func day(n int) time.Time { return time.Date(2026, 10, n, 0, 0, 0, 0, time.UTC) }

func TestAmountsAndBalances(t *testing.T) {
	l := labels()
	for _, tc := range []struct {
		amount             int64
		net, tone, balance string
	}{
		{0, "$0.00", "", "$0.00"}, {123456, "+$1,234.56", "text-up", "$1,234.56"},
		{-675, "−$6.75", "text-down", "($6.75)"},
		{math.MinInt64, "−$92,233,720,368,547,758.08", "text-down", "($92,233,720,368,547,758.08)"},
		{math.MaxInt64, "+$92,233,720,368,547,758.07", "text-up", "$92,233,720,368,547,758.07"},
	} {
		t.Run(fmt.Sprint(tc.amount), func(t *testing.T) {
			if got := l.Net(txncalendar.DayStat{Net: tc.amount, Count: 1}, "USD"); got != tc.net {
				t.Fatalf("Net=%q want %q", got, tc.net)
			}
			if got := NetTone(tc.amount); got != tc.tone {
				t.Errorf("tone=%q", got)
			}
			text, tone := Balance(map[string]money.Money{"row": money.New(tc.amount, "USD")}, "row")
			wantTone := ""
			if tc.amount < 0 {
				wantTone = "text-down"
			}
			if text != tc.balance || tone != wantTone {
				t.Errorf("balance=(%q,%q)", text, tone)
			}
		})
	}
	for _, balances := range []map[string]money.Money{nil, {}, {"other": money.New(12, "USD")}} {
		if text, tone := Balance(balances, "row"); text != "" || tone != "" {
			t.Fatalf("absent balance=(%q,%q)", text, tone)
		}
	}
	if got := l.Net(txncalendar.DayStat{Net: 12}, "USD"); got != "" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		cash txncalendar.Cashflow
		want string
	}{
		{txncalendar.Cashflow{}, ""}, {txncalendar.Cashflow{Moved: 100, Count: 1}, ""},
		{txncalendar.Cashflow{Spent: 675, Net: -675, Count: 1}, "−$6.75"},
		{txncalendar.Cashflow{Income: 100, Spent: 100, Count: 2}, "$0.00"},
	} {
		if got := l.DayNet(tc.cash, "USD"); got != tc.want {
			t.Errorf("DayNet=%q want %q", got, tc.want)
		}
	}
	for _, amount := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		got := l.Moved(amount, "USD")
		if amount <= 0 && got != "" {
			t.Errorf("Moved(%d)=%q", amount, got)
		}
		if amount > 0 && (!strings.Contains(got, "Transferred $") || strings.Contains(got, "+")) {
			t.Errorf("Moved=%q", got)
		}
	}
	for _, tc := range []struct {
		cash txncalendar.Cashflow
		want string
	}{
		{txncalendar.Cashflow{}, "Day 22: no transactions."},
		{txncalendar.Cashflow{Spent: 675, Count: 1}, "1 transaction"},
		{txncalendar.Cashflow{Income: 2500, Moved: 10, Count: 2}, "2 transactions"},
	} {
		if got := l.DayAria(22, tc.cash, "USD"); !strings.Contains(got, tc.want) {
			t.Errorf("DayAria=%q want %q", got, tc.want)
		}
	}
}

func TestLabelsAndPickNotice(t *testing.T) {
	l := labels()
	for _, tc := range []struct{ from, to, want string }{
		{"", "", "All dates"}, {"2026-10-01", "2026-10-31", "October 2026"},
		{"2026-10-22", "2026-10-22", "2026-10-22"}, {"2026-10-02", "2026-10-08", "2026-10-02"},
		{"", "2026-10-08", "…"}, {"2026-10-02", "", "…"},
	} {
		got := l.Caption(txnscope.Of(tc.from, tc.to, day(18)))
		if !strings.Contains(got, tc.want) {
			t.Errorf("caption=%q want %q", got, tc.want)
		}
		if !strings.Contains(got, "October 2026") {
			t.Errorf("grid month absent: %q", got)
		}
	}
	for month := time.January; month <= time.December; month++ {
		got := l.MonthYear(time.Date(2024, month, 29, 23, 0, 0, 0, time.FixedZone("west", -7*3600)))
		want := month.String() + " 2024"
		if got != want {
			t.Errorf("MonthYear=%q want %q", got, want)
		}
	}
	for start := time.Sunday; start <= time.Saturday; start++ {
		headers := l.Headers(start)
		if len(headers) != 7 {
			t.Fatal(len(headers))
		}
		for i, h := range headers {
			wd := time.Weekday((int(start) + i) % 7)
			if h.Aria != wd.String() || h.Text != wd.String()[:3] {
				t.Errorf("header=%+v day=%s", h, wd)
			}
		}
	}
	for _, tc := range []struct{ key, from, to, phrase string }{
		{"2026-10-22", "", "", "All dates"}, {"2026-10-22", "2026-10-01", "", "2026-10-22"},
		{"invalid", "", "2026-10-31", "invalid"},
	} {
		if got := l.PickNotice(tc.key, tc.from, tc.to); !strings.Contains(got, tc.phrase) {
			t.Errorf("PickNotice=%q want %q", got, tc.phrase)
		}
	}
}

func TestWeekSummaries(t *testing.T) {
	l := labels()
	for _, tc := range []struct {
		name    string
		in      WeekInput
		last    string
		classes []string
	}{
		{"past empty", WeekInput{}, "\u2003", []string{"txn-cal-week-count text-dim"}},
		{"future", WeekInput{Future: true}, l.T("transactions.tq3.weekEmpty"), []string{"sr-only txn-cal-empty-week text-dim", "txn-cal-week-count text-dim"}},
		{"scheduled", WeekInput{Scheduled: 2}, l.T("transactions.tq3.weekScheduled", 2), []string{"sr-only txn-cal-empty-week text-dim", "txn-cal-week-count"}},
		{"one", WeekInput{Cashflow: txncalendar.Cashflow{Spent: 675, Count: 1}}, l.T("transactions.tq3.weekOne"), []string{"txn-cal-week-spent text-down", "txn-cal-week-count"}},
		{"many", WeekInput{Cashflow: txncalendar.Cashflow{Spent: 100, Income: 200, Moved: 300, Count: 3}}, l.T("transactions.tq3.weekCount", 3), []string{"txn-cal-week-spent text-down", "txn-cal-week-income text-up", "txn-cal-week-moved", "txn-cal-week-count"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.Base = "USD"
			tc.in.WeekOf = "2026-10-04"
			if got := l.WeekLastLine(tc.in); got != tc.last {
				t.Errorf("last=%q want %q", got, tc.last)
			}
			got := l.WeekSummary(tc.in)
			var classes []string
			for _, line := range got.Lines {
				classes = append(classes, line.Class)
			}
			if !reflect.DeepEqual(classes, tc.classes) {
				t.Errorf("classes=%v want %v", classes, tc.classes)
			}
			if got.Lines[len(got.Lines)-1].Text != tc.last || !strings.Contains(got.Aria, tc.in.WeekOf) || strings.Contains(got.Aria, "+") {
				t.Errorf("summary=%+v", got)
			}
		})
	}
}

func TestGhostAndCellSummaries(t *testing.T) {
	l := labels()
	for _, tc := range []struct {
		name           string
		ghost          txncalendar.Ghost
		rates          currency.Rates
		amount, phrase string
	}{
		{"negative", txncalendar.Ghost{Amount: -22000, Currency: "USD"}, currency.Rates{Base: "USD"}, "$220.00", "is due"},
		{"zero", txncalendar.Ghost{Currency: "USD"}, currency.Rates{Base: "USD"}, "$0.00", "is due"},
		{"missed", txncalendar.Ghost{Amount: 10, Currency: "USD", NotPosted: true}, currency.Rates{Base: "USD"}, "$0.10", "not posted"},
		{"liability", txncalendar.Ghost{Amount: -100, Currency: "USD", Liability: true}, currency.Rates{Base: "USD"}, "$1.00", "is due on this day"},
		{"min overflow", txncalendar.Ghost{Amount: math.MinInt64, Currency: "USD"}, currency.Rates{Base: "USD"}, l.T("transactions.calScheduledAmountUnavailable"), "is due"},
		{"missing rate", txncalendar.Ghost{Amount: -100, Currency: "EUR"}, currency.Rates{Base: "USD"}, l.T("transactions.calScheduledAmountUnavailable"), "is due"},
		{"converted", txncalendar.Ghost{Amount: -100, Currency: "EUR"}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, "$2.00", "is due"},
		{"fx overflow", txncalendar.Ghost{Amount: math.MaxInt64, Currency: "EUR"}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, l.T("transactions.calScheduledAmountUnavailable"), "is due"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.ghost.Label = "Electric bill"
			if tc.name == "fx overflow" {
				// Characterize the pre-existing FX dependency rather than change
				// its out-of-range conversion policy during component extraction.
				converted, err := tc.rates.Convert(money.New(tc.ghost.Amount, tc.ghost.Currency), tc.rates.Base)
				if err != nil {
					t.Fatal(err)
				}
				tc.amount = l.unsigned(converted.Amount, converted.Currency)
			}
			got := l.GhostSummary(tc.ghost, tc.rates)
			if !strings.Contains(got.Status, tc.amount) || !strings.Contains(got.Aria, tc.amount) || !strings.Contains(got.Aria, tc.phrase) || got.Label != tc.ghost.Label {
				t.Errorf("ghost=%+v", got)
			}
			if strings.Contains(got.Class, "missed") != tc.ghost.NotPosted {
				t.Error(got.Class)
			}
			if got.Aria != l.GhostAria(tc.ghost, tc.amount) {
				t.Error(got.Aria)
			}
		})
	}
	for _, tc := range []struct {
		name          string
		in            Day
		label         string
		pick, current bool
		class         string
	}{
		{"default", Day{Day: 22, InMonth: true, NetStr: "$1.00"}, "22", true, false, "cal-cell txn-cal-cell"},
		{"today", Day{Day: 22, InMonth: true, Today: true, AriaLabel: "explicit", MovedStr: "Transferred $1.00"}, "explicit", true, true, "cal-cell txn-cal-cell today"},
		{"padding", Day{Day: 30, MonthName: "September 2026", Today: true}, "September 2026", false, false, "cal-cell txn-cal-cell out today"},
		{"ghosts", Day{Day: 22, InMonth: true, Rates: currency.Rates{Base: "USD"}, Ghosts: []txncalendar.Ghost{{Label: "One", Currency: "USD"}, {Label: "Two", Currency: "USD"}}}, "; ", true, false, "cal-cell txn-cal-cell"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := l.CellSummary(tc.in)
			if !strings.Contains(got.Aria, tc.label) || got.Class != tc.class || got.Pickable != tc.pick || got.Current != tc.current || got.DayText != fmt.Sprint(tc.in.Day) || got.ShowNet != (tc.in.NetStr != "") || got.ShowMoved != (tc.in.MovedStr != "") || len(got.Ghosts) != len(tc.in.Ghosts) {
				t.Errorf("cell=%+v", got)
			}
		})
	}
	for _, tc := range []struct{ count, want int }{{-1, 0}, {0, 0}, {1, 1}, {5, 1}, {6, 2}, {25, 5}, {26, 5}, {math.MaxInt, 5}} {
		if got := DotCount(tc.count); got != tc.want {
			t.Errorf("DotCount(%d)=%d", tc.count, got)
		}
	}
	ghosts := []txncalendar.Ghost{{Label: "same"}}
	if ScheduledGhosts(ghosts, false) != nil || len(ScheduledGhosts(nil, true)) != 0 || &ScheduledGhosts(ghosts, true)[0] != &ghosts[0] {
		t.Fatal("scheduled visibility changed slice")
	}
}

func TestBuildScopesAndSettlements(t *testing.T) {
	l := labels()
	recurring := []domain.Recurring{{ID: "bill", Label: "Electric", AccountID: "checking", CategoryID: "utilities", Amount: money.New(-2000, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(5)}}
	accounts := []domain.Account{
		{ID: "checking", OwnerID: "m1", Scope: domain.ScopeIndividual},
		{ID: "card", Name: "Card", OwnerID: "m1", Scope: domain.ScopeIndividual, Class: domain.ClassLiability, DueDayOfMonth: 22, MinPayment: money.New(1000, "USD")},
		{ID: "foreign", Name: "Foreign", OwnerID: "m2", Scope: domain.ScopeIndividual, Class: domain.ClassLiability, DueDayOfMonth: 22, MinPayment: money.New(4000, "USD")},
	}
	for _, tc := range []struct {
		name    string
		filter  txnfilter.Criteria
		settled map[string]bool
		paid    bool
		want    int
	}{
		{name: "nil map", want: 2}, {name: "search", filter: txnfilter.Criteria{Text: "electric"}, want: 0},
		{name: "whitespace search", filter: txnfilter.Criteria{Text: "  "}, want: 2},
		{name: "category", filter: txnfilter.Criteria{Category: "utilities"}, want: 1},
		{name: "multi category", filter: txnfilter.Criteria{Categories: "utilities"}, want: 1},
		{name: "settled recurring", settled: map[string]bool{domain.OccurrenceKey("bill", day(5)): true}, want: 1},
		{name: "settled card", paid: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := make(map[string]bool)
			for k, v := range tc.settled {
				before[k] = v
			}
			in := Input{Now: day(18), WeekStart: time.Monday, Filter: tc.filter, MemberLens: []string{"m1"}, Recurring: recurring, Accounts: accounts, Settled: tc.settled, Rates: currency.Rates{Base: "USD"}}
			if tc.paid {
				in.Transactions = []domain.Transaction{{ID: "payment", BillAccountID: "card", Date: day(15), Amount: money.New(-1000, "USD")}}
			}
			got := l.Build(in)
			if got.Err != nil || len(got.Weeks) != 5 {
				t.Fatalf("month=%+v", got)
			}
			count := 0
			for _, week := range got.Weeks {
				if len(week.Days) != 7 || week.Total.SummaryID != "txn-cal-week-"+week.Days[0].DateKey {
					t.Errorf("week=%+v", week)
				}
				scheduled := 0
				for _, cell := range week.Days {
					scheduled += len(cell.Ghosts)
					if cell.DateKey == "2026-10-18" && !cell.Today {
						t.Error("today missing")
					}
					for _, g := range cell.Ghosts {
						if g.Label == "Foreign" {
							t.Error("foreign scope leaked")
						}
						if g.RecurringID == "bill" && !g.NotPosted {
							t.Error("past due not marked")
						}
					}
				}
				if week.Total.Scheduled != scheduled {
					t.Errorf("scheduled=%d want %d", week.Total.Scheduled, scheduled)
				}
				count += scheduled
			}
			if count != tc.want {
				t.Errorf("ghosts=%d want %d", count, tc.want)
			}
			if len(tc.settled) != len(before) {
				t.Error("input map mutated")
			}
			for k, v := range before {
				if tc.settled[k] != v {
					t.Error("settlement flag mutated")
				}
			}
		})
	}
}

func TestBuildAmountsErrorsAndDates(t *testing.T) {
	l := labels()
	for _, tc := range []struct {
		name          string
		txns          []domain.Transaction
		rates         map[string]float64
		issues        int
		overflow      bool
		spent, income int64
	}{
		{name: "empty"},
		{name: "mixed", txns: []domain.Transaction{{Date: day(1), Amount: money.New(-100, "USD")}, {Date: day(31), Amount: money.New(200, "EUR")}}, rates: map[string]float64{"EUR": 2}, spent: 100, income: 400},
		{name: "missing FX", txns: []domain.Transaction{{Date: day(1), Amount: money.New(-100, "EUR")}}, issues: 1},
		{name: "zero", txns: []domain.Transaction{{Date: day(1), Amount: money.New(0, "USD")}}},
		{name: "negative overflow", txns: []domain.Transaction{{Date: day(1), Amount: money.New(math.MinInt64, "USD")}}, overflow: true},
		{name: "sum overflow", txns: []domain.Transaction{{Date: day(1), Amount: money.New(math.MaxInt64, "USD")}, {Date: day(1), Amount: money.New(1, "USD")}}, overflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := l.Build(Input{Now: day(18), Shown: tc.txns, Rates: currency.Rates{Base: "USD", Rates: tc.rates}})
			if (got.Err != nil) != tc.overflow || got.RateIssues != tc.issues {
				t.Fatalf("err=%v issues=%d", got.Err, got.RateIssues)
			}
			spent, income, count := int64(0), int64(0), 0
			for _, week := range got.Weeks {
				spent += week.Total.Cashflow.Spent
				income += week.Total.Cashflow.Income
				count += week.Total.Cashflow.Count
			}
			if !tc.overflow && (spent != tc.spent || income != tc.income || count != len(tc.txns)) {
				t.Errorf("total=(%d,%d,%d)", spent, income, count)
			}
		})
	}
	for _, now := range []time.Time{
		time.Date(2024, 2, 29, 23, 59, 0, 0, time.FixedZone("west", -7*3600)),
		time.Date(2026, 10, 31, 23, 59, 0, 0, time.FixedZone("east", 14*3600)),
		time.Date(2026, 3, 8, 3, 0, 0, 0, time.FixedZone("DST", -4*3600)),
	} {
		got := l.Build(Input{Now: now, WeekStart: time.Saturday, Rates: currency.Rates{Base: "USD"}})
		today := 0
		days := 0
		for _, week := range got.Weeks {
			for _, cell := range week.Days {
				if cell.Today {
					today++
				}
				if cell.InMonth {
					days++
				}
			}
		}
		wantDays := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if today != 1 || days != wantDays || got.Err != nil {
			t.Errorf("zone %s today=%d days=%d want %d", now, today, days, wantDays)
		}
	}
}

func TestBuildLargeInputReconciles(t *testing.T) {
	txns := make([]domain.Transaction, 10000)
	for i := range txns {
		txns[i] = domain.Transaction{ID: fmt.Sprint(i), Date: day(1 + i%31), Amount: money.New(int64(i%200-100), "USD")}
	}
	checkReconciles(t, txns)
}

func checkReconciles(t *testing.T, txns []domain.Transaction) {
	t.Helper()
	before := append([]domain.Transaction(nil), txns...)
	l := labels()
	month := l.Build(Input{Now: day(18), Shown: txns, Rates: currency.Rates{Base: "USD"}})
	if month.Err != nil {
		t.Fatal(month.Err)
	}
	byDay := make(map[string]txncalendar.Cashflow)
	wantNet := int64(0)
	// Oracle inputs are small signed amounts, so these sums cannot overflow.
	for _, txn := range txns {
		key := txncalendar.DayKey(txn.Date)
		cash := byDay[key]
		cash.Count++
		cash.Net += txn.Amount.Amount
		if txn.Amount.Amount < 0 {
			cash.Spent -= txn.Amount.Amount
		} else {
			cash.Income += txn.Amount.Amount
		}
		byDay[key] = cash
		wantNet += txn.Amount.Amount
	}
	count, net := 0, int64(0)
	for _, week := range month.Weeks {
		total := txncalendar.Cashflow{}
		for _, cell := range week.Days {
			cash := byDay[cell.DateKey]
			if !cell.InMonth {
				cash = txncalendar.Cashflow{}
			}
			if cell.Count != cash.Count {
				t.Fatalf("day %s count=%d want %d", cell.DateKey, cell.Count, cash.Count)
			}
			wantText := ""
			if cash.Spent != 0 || cash.Income != 0 {
				wantText, _ = l.Ledger(money.New(cash.Net, "USD"), false)
			}
			if cell.NetStr != wantText {
				t.Fatalf("day %s net=%q want %q", cell.DateKey, cell.NetStr, wantText)
			}
			total.Count += cash.Count
			total.Net += cash.Net
			total.Spent += cash.Spent
			total.Income += cash.Income
		}
		if total != week.Total.Cashflow {
			t.Fatalf("week total=%+v want %+v", week.Total.Cashflow, total)
		}
		count += total.Count
		net += total.Net
	}
	if count != len(txns) || net != wantNet {
		t.Fatalf("count=%d net=%d want %d/%d", count, net, len(txns), wantNet)
	}
	if !reflect.DeepEqual(before, txns) && len(txns) > 0 {
		t.Fatal("projection mutated transactions")
	}
}

func FuzzDensityAndReconciliation(f *testing.F) {
	f.Add([]byte{0, 1, 5, 6, 25, 255})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2048 {
			data = data[:2048]
		}
		txns := make([]domain.Transaction, len(data))
		for i, b := range data {
			txns[i] = domain.Transaction{ID: fmt.Sprint(i), Date: day(i%31 + 1), Amount: money.New(int64(b)-128, "USD")}
		}
		checkReconciles(t, txns)
		previous := 0
		for i := 0; i <= len(data); i++ {
			dots := DotCount(i)
			if dots < previous || dots < 0 || dots > 5 {
				t.Fatalf("invalid density %d=%d", i, dots)
			}
			previous = dots
		}
	})
}

func TestCaptionUsesPersistedDateStyleWhilePickUsesLivePreferences(t *testing.T) {
	l := labels()
	l.ScopePrefs.DateStyle = prefs.DateEU
	l.Prefs.DateStyle = prefs.DateUS
	scope := txnscope.Of("2026-10-22", "2026-10-22", day(18))
	if got := l.Caption(scope); !strings.Contains(got, "22/10/2026") {
		t.Fatalf("caption=%q", got)
	}
	if got := l.PickNotice("2026-10-22", "", ""); !strings.Contains(got, "10/22/2026") {
		t.Fatalf("pick=%q", got)
	}
}

func TestDaySelectionPreservesTheLatestUnrelatedFilters(t *testing.T) {
	l := labels()
	for _, key := range []string{"2024-02-29", "2026-10-31", "invalid", ""} {
		t.Run(key, func(t *testing.T) {
			previous := txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31"}
			selection := l.PickDay(key, previous.From, previous.To)
			if selection.DateKey != key || selection.ViewMode != "table" || selection.Notice != l.PickNotice(key, previous.From, previous.To) {
				t.Fatalf("selection=%+v", selection)
			}
			latest := txnfilter.Criteria{Text: "recent search", Account: "checking", Category: "utilities", Member: "m1", PeriodPreset: "custom", From: "2026-09-01", To: "2026-09-30"}
			want := latest
			want.From, want.To = key, key
			if got := selection.Apply(latest); !reflect.DeepEqual(got, want) {
				t.Fatalf("filter=%+v want %+v", got, want)
			}
			if latest.From != "2026-09-01" || latest.To != "2026-09-30" {
				t.Fatal("selection mutated the input")
			}
		})
	}
}
