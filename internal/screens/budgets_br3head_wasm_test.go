// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBR3HeadHeadlineShapeForPastCurrentAndFuturePeriods(t *testing.T) {
	window := period.NewWindow(period.Month, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	cases := []struct {
		name   string
		future bool
		hist   bool
		want   string
	}{
		{name: "current", want: "spent of $3,313.53 monthly plan"},
		{name: "past", hist: true, want: "spent of $3,313.53 monthly plan during Oct 2026"},
		{name: "future", future: true, want: "Planned $3,313.53 · expected income $5,900.00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(budgetSummaryStatus(budgetSummaryStatusProps{
				normalizedBudgetTotal: 331353,
				vw:                    window,
				v:                     budgetHeadlineFixture(tc.future),
				futurePeriod:          tc.future,
				hist:                  tc.hist,
				normalizationOK:       true,
				filterAttn:            ui.Handler{},
				attnCount:             2,
				funding:               budgeting.FundingRead{},
			}))
			figure := byTestID(f, "span", "budgets-status-figure")
			secondary := byTestID(f, "span", "budgets-status-sentence")
			if figure == nil || secondary == nil {
				t.Fatal("headline amount or secondary line did not render")
			}
			if got := secondary.Text(); got != tc.want {
				t.Fatalf("headline secondary = %q, want %q", got, tc.want)
			}
			wantFigure := "$1,567.50"
			if tc.future {
				wantFigure = "$0.00"
			}
			if figure.Text() != wantFigure {
				t.Errorf("spent figure = %q", figure.Text())
			}
			attn := byTestID(f, "button", "budgets-hero-attn")
			if !tc.hist && (attn == nil || attn.Text() != "2 budgets need attention") {
				t.Errorf("attention link = %#v", attn)
			}
			if meter := byTestID(f, "div", "budgets-status-meter"); meter == nil {
				t.Error("headline meter did not render")
			}
			if strings.Contains(f.Text(), "BUDGETED AGAINST INCOME") || strings.Contains(f.Text(), "→") {
				t.Errorf("headline contains template copy: %q", f.Text())
			}
		})
	}
}

func budgetHeadlineFixture(future bool) budgetView {
	spent := int64(156750)
	if future {
		spent = 0
	}
	return budgetView{Base: "USD", TotalSpent: spent, BannerIncome: 590000}
}

func TestBR3HeadMeterClampsAndUsesElapsedPeriod(t *testing.T) {
	for _, tc := range []struct {
		spent, plan int64
		want        int
	}{{0, 100, 0}, {50, 100, 50}, {150, 100, 100}, {50, 0, 0}} {
		if got := budgetHeadlinePercent(tc.spent, tc.plan); got != tc.want {
			t.Errorf("budgetHeadlinePercent(%d, %d) = %d, want %d", tc.spent, tc.plan, got, tc.want)
		}
	}
	window := period.NewWindow(period.Month, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	if got := budgetPeriodElapsedPct(window, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)); got < 48 || got > 52 {
		t.Fatalf("mid-month pace marker = %d%%, want about 50%%", got)
	}
}
