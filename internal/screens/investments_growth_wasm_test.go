// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type growthTestID struct{ tag, id string }

func TestInvestGrowthHeadlineRendersPreparedFactsAndAvailability(t *testing.T) {
	tests := []struct {
		name      string
		props     investGrowthHeadlineProps
		want      []string
		wantNodes []growthTestID
	}{
		{
			name: "normal series preserves signed delta and percent",
			props: investGrowthHeadlineProps{Change: portfolio.GrowthChange{
				Start: money.New(10000, "USD"), End: money.New(12500, "USD"),
				Delta: money.New(2500, "USD"), Percent: 25, CurrentKnown: true,
				DeltaKnown: true, PercentKnown: true,
			}, Symbol: "$", Decimals: 2},
			want:      []string{"$125.00", "▲ $25.00 (+25.0%)"},
			wantNodes: []growthTestID{{"span", "invest-growth-now"}, {"span", "invest-growth-delta"}},
		},
		{
			name: "zero baseline states percent unavailable",
			props: investGrowthHeadlineProps{Change: portfolio.GrowthChange{
				End: money.New(100, "USD"), Delta: money.New(100, "USD"),
				CurrentKnown: true, DeltaKnown: true,
			}, Symbol: "$", Decimals: 2},
			want:      []string{"$1.00", "percent unavailable from a zero starting value"},
			wantNodes: []growthTestID{{"span", "invest-growth-now"}, {"span", "invest-growth-delta"}},
		},
		{
			name: "negative delta preserves signed-money presentation",
			props: investGrowthHeadlineProps{Change: portfolio.GrowthChange{
				End: money.New(7500, "USD"), Delta: money.New(-2500, "USD"), Percent: -25,
				CurrentKnown: true, DeltaKnown: true, PercentKnown: true,
			}, Symbol: "$", Decimals: 2},
			want:      []string{"$75.00", "▼ −$25.00 (-25.0%)"},
			wantNodes: []growthTestID{{"span", "invest-growth-now"}, {"span", "invest-growth-delta"}},
		},
		{
			name:      "failed series does not render a zero figure",
			props:     investGrowthHeadlineProps{SeriesErr: true, Symbol: "$", Decimals: 2},
			want:      []string{"Growth history unavailable", "Change unavailable"},
			wantNodes: []growthTestID{{"p", "invest-growth-current-unavailable"}, {"p", "invest-growth-change-unavailable"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(investGrowthHeadline, tt.props))
			f.Stabilize()
			for _, text := range tt.want {
				if !strings.Contains(f.Text(), text) {
					t.Errorf("render is missing %q: %s", text, f.Text())
				}
			}
			for _, want := range tt.wantNodes {
				if node := byTestID(f, want.tag, want.id); node == nil {
					t.Errorf("missing %s data-testid %q", want.tag, want.id)
				}
			}
		})
	}
}
