// SPDX-License-Identifier: MIT

package recurring

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func testText() Text {
	t := func(key string, args ...any) string { return key + fmt.Sprint(args...) }
	return Text{T: t, TN: func(one, many string, n int, args ...any) string {
		if n == 1 {
			return t(one, args...)
		}
		return t(many, args...)
	}, Date: func(d time.Time) string { return d.Format("2006-01-02") }, FaintClass: "faint-token"}
}

func TestPresentSummary(t *testing.T) {
	for _, tc := range []struct {
		count, excluded int
		suffix          string
	}{{0, 0, "Many"}, {1, 1, "One"}, {2, 2, "Many"}} {
		m := Model{Count: tc.count, Excluded: make([]string, tc.excluded), TotalMonthly: money.New(123456, "USD"), CoveredMonthly: money.New(120000, "USD"), UnbudgetedMonthly: money.New(3456, "USD")}
		got := PresentSummary(m, testText())
		if got.Total != "budgets.recurring.totalVal$1,234.56" || got.Count != "budgets.recurring.bill"+tc.suffix || got.FoldHint != "budgets.recurring.billFold"+tc.suffix+"$1,234.56" || got.Covered != "$1,200.00" || got.Unbudgeted != "budgets.recurring.unbudgetedMonthly$34.56" || got.ExcludedCount != "budgets.recurring.excluded"+tc.suffix || got.ShowExcluded != (tc.excluded > 0) {
			t.Fatalf("summary=%+v", got)
		}
	}
}

func TestPresentDisclosure(t *testing.T) {
	for _, tc := range []struct {
		open                 bool
		class, key, expanded string
	}{{false, "budget-fold-caret", "budgets.recurring.showAria", "false"}, {true, "budget-fold-caret is-open", "budgets.recurring.hideAria", "true"}} {
		got := PresentDisclosure(tc.open)
		if got.CaretClass != tc.class || got.AriaKey != tc.key || got.Expanded != tc.expanded || got.NextOpen == tc.open {
			t.Fatalf("disclosure=%+v", got)
		}
		if PresentDisclosure(got.NextOpen).NextOpen != tc.open {
			t.Fatal("toggle does not round-trip")
		}
	}
}

func day(month time.Month, date int) time.Time {
	return time.Date(2026, month, date, 0, 0, 0, 0, time.UTC)
}

func TestDateStateKeyAndClass(t *testing.T) {
	start, end, now := day(time.October, 1), day(time.November, 1), day(time.October, 19)
	for _, tc := range []struct {
		name                  string
		next, now, start, end time.Time
		state                 budgeting.RecurDateState
		key                   string
	}{
		{"unscheduled", time.Time{}, now, start, end, budgeting.RecurUnscheduled, "dueInPeriod"},
		{"before", day(time.September, 30), now, start, end, budgeting.RecurBeforePeriod, "beforePeriod"},
		{"start boundary", start, now, start, end, budgeting.RecurOverdue, "overdue"},
		{"today", now, now, start, end, budgeting.RecurDueInPeriod, "dueInPeriod"},
		{"month end", day(time.October, 31), now, start, end, budgeting.RecurDueInPeriod, "dueInPeriod"},
		{"end boundary", end, now, start, end, budgeting.RecurAfterPeriod, "afterPeriod"},
		{"missing window overdue", start, now, time.Time{}, time.Time{}, budgeting.RecurOverdue, "overdue"},
		{"missing window future", end, now, time.Time{}, time.Time{}, budgeting.RecurDueInPeriod, "dueInPeriod"},
		{"west of UTC", day(time.October, 19), time.Date(2026, 10, 18, 23, 30, 0, 0, time.FixedZone("west", -4*3600)), start, end, budgeting.RecurDueInPeriod, "dueInPeriod"},
		{"east of UTC", day(time.October, 19), time.Date(2026, 10, 19, 0, 30, 0, 0, time.FixedZone("east", 14*3600)), start, end, budgeting.RecurOverdue, "overdue"},
		{"leap month", time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), time.Date(2028, 2, 28, 23, 59, 0, 0, time.UTC), time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC), budgeting.RecurDueInPeriod, "dueInPeriod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := DateState(tc.next, tc.now, tc.start, tc.end)
			if state != tc.state || DateKey(state) != "budgets.recurring."+tc.key {
				t.Fatalf("state=%q key=%q", state, DateKey(state))
			}
			want := "brc-date faint-token"
			if state == budgeting.RecurOverdue {
				want = "brc-date is-overdue"
			}
			if got := DateClass(state, "faint-token"); got != want {
				t.Fatalf("class=%q want %q", got, want)
			}
		})
	}
	if DateKey("unknown") != "budgets.recurring.dueInPeriod" || DateClass("unknown", "") != "brc-date " {
		t.Fatal("unknown state fallback")
	}
}

func TestPresentRow(t *testing.T) {
	for _, tc := range []struct {
		cadence       domain.RecurringCadence
		amount        int64
		show          bool
		cadenceSuffix string
	}{
		{domain.CadenceMonthly, -1200, false, "Monthly"}, {domain.CadenceWeekly, -1200, true, "Weekly"}, {domain.CadenceBiweekly, -1200, true, "Biweekly"}, {domain.CadenceSemimonthly, -1200, true, "Semimonthly"}, {domain.CadenceQuarterly, -1200, true, "Quarterly"}, {domain.CadenceYearly, -1200, true, "Yearly"}, {"unknown", 0, true, "Monthly"}, {domain.CadenceWeekly, math.MinInt64, false, "Weekly"},
	} {
		t.Run(string(tc.cadence), func(t *testing.T) {
			r := bill("Untrimmed label ", tc.amount, "USD", tc.cadence)
			r.CategoryID = "home"
			r.NextDue = day(time.October, 10)
			r.Autopay = true
			got := PresentRow(r, map[string]string{"home": "Housing"}, day(time.October, 19), day(time.October, 1), day(time.November, 1), testText())
			if got.Label != r.Label || got.Category != "Housing" || !got.Autopay || got.Cadence != "recurring.cadence"+tc.cadenceSuffix || got.ShowPerMonth != tc.show || !got.ShowDate || got.Date != ", budgets.recurring.overdue2026-10-10" || got.DateClass != "brc-date is-overdue" || got.DateTestID != "brc-date-overdue" {
				t.Fatalf("row=%+v", got)
			}
			if tc.amount < 0 && !strings.HasPrefix(got.Amount, "($") {
				t.Fatalf("signed charge=%q", got.Amount)
			}
			if tc.show && !strings.HasPrefix(got.PerMonth, "budgets.recurring.perMonth") {
				t.Fatalf("monthly=%q", got.PerMonth)
			}
		})
	}
	for _, categories := range []map[string]string{nil, {}, {"home": ""}} {
		got := PresentRow(domain.Recurring{}, categories, time.Time{}, time.Time{}, time.Time{}, testText())
		if got.Category != "budgets.recurring.uncategorized" || got.ShowDate || got.Autopay || got.Date != "" || got.DateClass != "" || got.DateTestID != "" {
			t.Fatalf("fallback=%+v", got)
		}
	}
}
