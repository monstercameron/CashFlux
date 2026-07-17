// SPDX-License-Identifier: MIT

package notify

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		id   string
		want Target
	}{
		{"default-unusual@unusual:tx-coffee-anomaly-2026-06", Target{TargetTxn, "tx-coffee-anomaly-2026-06"}},
		{"default-large@txn:tx-123", Target{TargetTxn, "tx-123"}},
		{"default-paycheck@paycheck:tx-pay-1", Target{TargetTxn, "tx-pay-1"}},
		{"default-stale@acct-hysa@2026-W27", Target{TargetAccount, "acct-hysa"}},
		{"default-low-balance@lowbal:acct-cash@2026-W27", Target{TargetAccount, "acct-cash"}},
		{"default-bill-due@acct-card@2026-07-21", Target{TargetAccount, "acct-card"}},
		{"default-budget@budget-groceries:over@2026-07", Target{TargetBudget, "budget-groceries"}},
		{"default-digest@digest@2026-W27", Target{}},      // no specific entity
		{"malformed-no-at-sign", Target{}},                // not a dedupe key
	}
	for _, c := range cases {
		if got := ParseTarget(c.id); got != c.want {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", c.id, got, c.want)
		}
	}
}
