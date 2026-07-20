// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

// TestRecurringPausedActive verifies the additive Paused flag and the Active
// predicate: default recurrings are active, pausing flips it, and the flag
// round-trips through JSON with omitempty (absent when false).
func TestRecurringPausedActive(t *testing.T) {
	tests := []struct {
		name       string
		paused     bool
		wantActive bool
		wantJSON   bool // whether "paused" should appear in the JSON
	}{
		{name: "default is active", paused: false, wantActive: true, wantJSON: false},
		{name: "paused is inactive", paused: true, wantActive: false, wantJSON: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Recurring{
				ID:      "r1",
				Label:   "Gym",
				Amount:  money.New(-5000, "USD"),
				Cadence: CadenceMonthly,
				Paused:  tt.paused,
			}
			if got := r.Active(); got != tt.wantActive {
				t.Errorf("Active() = %v, want %v", got, tt.wantActive)
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(b, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, present := raw["paused"]; present != tt.wantJSON {
				t.Errorf("paused key present = %v, want %v (json: %s)", present, tt.wantJSON, b)
			}

			var back Recurring
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("round-trip unmarshal: %v", err)
			}
			if back.Paused != tt.paused {
				t.Errorf("round-trip Paused = %v, want %v", back.Paused, tt.paused)
			}
		})
	}
}
