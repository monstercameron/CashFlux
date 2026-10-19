// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBR3RowCopyFormatsPlainSentenceCaseMessages(t *testing.T) {
	bundle := DefaultBundle()
	tests := []struct {
		key  string
		args []any
		want string
	}{
		{"br3row.endedOver", []any{"$244.80"}, "Ended $244.80 over"},
		{"br3row.committedDominates", []any{"$1,100.00"}, "On track — $1,100.00 of this is bills"},
		{"br3row.unbudgetedOthers", []any{"Shopping"}, "Shopping (others)"},
		{"br3row.effectiveBaseCarry", []any{"$205.20", "−", "$244.80"}, "Effective this month $205.20 (base − $244.80 carried)"},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if got := bundle.T(English, tc.key, tc.args...); got != tc.want {
				t.Fatalf("copy = %q, want %q", got, tc.want)
			}
		})
	}
}
