// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v4/testkit/render"
)

// TestMeterBarComponent is the reference example for the middle layer of the test
// pyramid: a real component mounted and queried through GoWebComponents' testkit
// (mock DOM), with no browser and no Playwright. It's fast, deterministic, and
// runs under `GOOS=js GOARCH=wasm go test` (see e2e/README.md for the runner).
//
// It exercises MeterBar's contract: the role/aria wiring and the value clamping
// that a screen relies on but that a pixel test can't see.
func TestMeterBarComponent(t *testing.T) {
	// testkit fixtures are process-global on js/wasm, so use ONE fixture and
	// re-render per case (never create a second while the first is active).
	f := render.New(t)

	cases := []struct {
		name    string
		value   float64
		wantNow string // aria-valuenow after MeterBar's clamp/compute
	}{
		{"Groceries", 25, "25"},  // in range → verbatim
		{"Overspent", 150, "100"}, // above max → clamps to full
		{"Under", -10, "0"},       // below min → clamps to empty
	}
	for _, c := range cases {
		f.Render(MeterBar(MeterBarProps{Label: c.name, Value: c.value, Min: 0, Max: 100}))
		m := f.ByRole("meter", c.name)
		if !m.Exists() {
			t.Fatalf("%s: MeterBar did not render a role=meter element", c.name)
		}
		if got := m.Attr("aria-valuemax"); got != "100" {
			t.Errorf("%s: aria-valuemax = %q, want 100", c.name, got)
		}
		if got := m.Attr("aria-valuenow"); got != c.wantNow {
			t.Errorf("%s: aria-valuenow = %q, want %q", c.name, got, c.wantNow)
		}
	}
}
