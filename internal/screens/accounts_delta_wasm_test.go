// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestNetWorthDeltaLineAvailabilityAndMagnitude(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		have     bool
		wantText string
		wantID   string
	}{
		{name: "unknown", amount: 25, have: false, wantText: "Net-worth change is unavailable", wantID: "acct-networth-delta-unavailable"},
		{name: "known zero", amount: 0, have: true, wantText: "No change this month", wantID: "acct-networth-delta-zero"},
		{name: "negative amount", amount: -100, have: true, wantText: "−$1.00 this month"},
		{name: "minimum amount cannot be made positive in int64", amount: math.MinInt64, have: true, wantText: "Net-worth change is unavailable", wantID: "acct-networth-delta-unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			defer func() {
				f.Render(ui.Fragment())
				f.Stabilize()
				f.Cleanup()
			}()
			f.Render(netWorthDeltaLine(money.New(tt.amount, "USD"), tt.have))
			f.Stabilize()
			if text := f.Text(); !strings.Contains(text, tt.wantText) {
				t.Fatalf("presenter text = %q, want it to contain %q", text, tt.wantText)
			}
			if tt.wantID != "" && byTestID(f, "span", tt.wantID) == nil {
				t.Fatalf("presenter did not expose state marker %q: %s", tt.wantID, f.Text())
			}
		})
	}
}
