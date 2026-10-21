// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestReportsShowUnavailableWhenCheckedPresentationCannotBePrepared(t *testing.T) {
	now := time.Date(2026, time.October, 10, 12, 0, 0, 0, time.UTC)
	start, end := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{{Date: now, Desc: "Extreme expense", CategoryID: "medical", Amount: money.New(math.MinInt64, "USD"), Custom: map[string]any{"project": "Work"}}}
	cases := []struct {
		name string
		node func() ui.Node
	}{
		{name: "custom field", node: func() ui.Node {
			return customFieldSpendSection(reportSectionContext{}, txns,
				[]customfields.Def{{Key: "project", Label: "Project"}}, "project", nil,
				start, end, rates, "USD", func(v int64) string { return money.FormatMinor(v, 2) }, period.Window{})
		}},
		{name: "investment totals", node: func() ui.Node {
			accounts := []domain.Account{
				{ID: "inv-a", Name: "A", Currency: "USD", Type: domain.TypeInvestment, OpeningBalance: money.New(math.MaxInt64, "USD")},
				{ID: "inv-b", Name: "B", Currency: "USD", Type: domain.TypeInvestment, OpeningBalance: money.New(1, "USD")},
			}
			return investmentPerformanceSection(reportSectionContext{}, accounts, nil, rates, "USD",
				func(v int64) string { return money.FormatMinor(v, 2) }, period.Window{})
		}},
		{name: "deductible totals", node: func() ui.Node {
			return deductibleSection(reportSectionContext{}, txns,
				[]domain.Category{{ID: "medical", Name: "Medical", Deductible: true}},
				start, end, rates, "USD", func(v int64) string { return money.FormatMinor(v, 2) }, period.Window{})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			defer func() {
				f.Render(ui.Fragment())
				f.Stabilize()
				f.Cleanup()
			}()
			f.Render(tc.node())
			f.Stabilize()
			if byTestID(f, "p", "report-aggregate-unavailable") == nil {
				t.Fatalf("unrepresentable report amount was not shown as unavailable: %s", f.Text())
			}
		})
	}
}

func TestInvestmentPerformanceSectionPreservesExactTotalReturnText(t *testing.T) {
	for _, tc := range []struct {
		gain int64
		want string
	}{
		{gain: 1, want: "+6.2%"},
		{gain: 3, want: "+18.8%"},
		{gain: -1, want: "-6.2%"},
		{gain: -3, want: "-18.8%"},
	} {
		f := render.New(t)
		func() {
			defer func() { f.Render(ui.Fragment()); f.Stabilize(); f.Cleanup() }()
			f.Render(investmentPerformanceSection(reportSectionContext{}, []domain.Account{{
				ID: "investment", Name: "Investment", Currency: "USD", Type: domain.TypeInvestment,
				OpeningBalance: money.New(16, "USD"),
			}}, []domain.Transaction{{AccountID: "investment", Amount: money.New(tc.gain, "USD")}},
				currency.Rates{Base: "USD"}, "USD", func(v int64) string { return money.FormatMinor(v, 2) }, period.Window{}))
			f.Stabilize()
			total := byTestID(f, "p", "invperf-total")
			if total == nil || !strings.Contains(total.Text(), tc.want) {
				t.Fatalf("gain %d: total = %v; want %s", tc.gain, total, tc.want)
			}
		}()
	}
}
