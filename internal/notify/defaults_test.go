// SPDX-License-Identifier: MIT

package notify

import "testing"

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	if len(rules) != 9 {
		t.Fatalf("got %d default rules, want 9", len(rules))
	}

	seenID := map[string]bool{}
	seenEvent := map[Event]bool{}
	for _, r := range rules {
		if r.ID == "" || seenID[r.ID] {
			t.Errorf("rule id %q is empty or duplicated", r.ID)
		}
		seenID[r.ID] = true
		seenEvent[r.Event] = true

		if !r.Enabled {
			t.Errorf("default rule %q should be enabled", r.ID)
		}
		if !r.HasChannel(ChannelInApp) {
			t.Errorf("default rule %q should deliver in-app", r.ID)
		}
		// Defaults don't impose quiet hours.
		if r.QuietStartMin != r.QuietEndMin {
			t.Errorf("default rule %q has quiet hours set", r.ID)
		}
		// Every default rule is eligible to fire at any time.
		if !r.CanFireAt(at(12, 0)) {
			t.Errorf("default rule %q can't fire", r.ID)
		}
	}

	// One rule per recommended event, and bill-due carries its lead-time threshold.
	for _, e := range []Event{EventBillDue, EventBudgetThreshold, EventStaleBalance, EventDigest, EventBackupDue, EventLargeTransaction, EventLowBalance, EventPaycheckLanded, EventUnusualCharge} {
		if !seenEvent[e] {
			t.Errorf("missing default rule for event %q", e)
		}
	}
	for _, r := range rules {
		if r.Event == EventBillDue && r.Threshold != defaultBillLeadDays {
			t.Errorf("bill-due threshold = %d, want %d", r.Threshold, defaultBillLeadDays)
		}
		if r.Event == EventLargeTransaction && r.Threshold != defaultLargeTxnMinor {
			t.Errorf("large-transaction threshold = %d, want %d", r.Threshold, defaultLargeTxnMinor)
		}
		if r.Event == EventLowBalance && r.Threshold != defaultLowBalanceMinor {
			t.Errorf("low-balance threshold = %d, want %d", r.Threshold, defaultLowBalanceMinor)
		}
		if r.Event == EventPaycheckLanded && r.Threshold != defaultPaycheckMinor {
			t.Errorf("paycheck-landed threshold = %d, want %d", r.Threshold, defaultPaycheckMinor)
		}
	}
}
