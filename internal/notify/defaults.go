// SPDX-License-Identifier: MIT

package notify

// defaultBillLeadDays is how many days before a bill's due date the default
// bill-due rule starts notifying.
const defaultBillLeadDays = 7

// defaultLargeTxnMinor is the default "large transaction" threshold in base-
// currency minor units (e.g. 50000 = $500.00). Users tune it per their budget.
const defaultLargeTxnMinor = 50000

// defaultLowBalanceMinor is the default "low balance" floor in base-currency
// minor units (e.g. 10000 = $100.00). A zero Threshold disables the alert,
// so a user can silence it without deleting the rule.
const defaultLowBalanceMinor = 10000

// DefaultRules returns the recommended Phase-A notification rules — one per
// supported event — all enabled and delivered in-app, with no quiet hours and no
// frequency cap (the per-event occurrence keys already bound how often each can
// fire). They're the sensible out-of-the-box set the UI seeds for a new user and
// later lets them tweak (channels, thresholds, quiet hours) in Settings.
//
// Only the bill-due rule uses Threshold (its lead time in days); the other
// events take their cue from existing logic (freshness windows, budgeting's
// near/over classification, the digest period), so their Threshold stays 0.
func DefaultRules() []Rule {
	inApp := []Channel{ChannelInApp}
	return []Rule{
		{ID: "default-bill-due", Event: EventBillDue, Enabled: true, Channels: inApp, Threshold: defaultBillLeadDays},
		{ID: "default-budget", Event: EventBudgetThreshold, Enabled: true, Channels: inApp},
		{ID: "default-stale", Event: EventStaleBalance, Enabled: true, Channels: inApp},
		{ID: "default-digest", Event: EventDigest, Enabled: true, Channels: inApp},
		{ID: "default-backup", Event: EventBackupDue, Enabled: true, Channels: inApp},
		{ID: "default-large", Event: EventLargeTransaction, Enabled: true, Channels: inApp, Threshold: defaultLargeTxnMinor},
		{ID: "default-low-balance", Event: EventLowBalance, Enabled: true, Channels: inApp, Threshold: defaultLowBalanceMinor},
	}
}
