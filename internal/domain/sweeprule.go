// SPDX-License-Identifier: MIT

package domain

// SweepRule is a per-account surplus-sweep rule (AC7): "keep checking at $3,000;
// move the excess to savings monthly." On its cadence the app PROPOSES — never
// auto-executes — a transfer of the amount above the keep floor from a source
// account to a destination account. The user previews and approves each proposal
// (the same proposal-card surface as the payday waterfall, GL1).
//
// The rule carries its OWN keep amount (KeepMinor) — the standalone per-account
// floor field was intentionally dropped, so the sweep is self-contained. All
// money is int64 minor units in the SOURCE account's currency; cross-currency
// sweeps are out of scope (a proposal is only built when source and destination
// share a currency).
type SweepRule struct {
	// ID is the stable identity of this rule.
	ID string `json:"id"`
	// SourceAccountID is the account swept FROM (e.g. checking).
	SourceAccountID string `json:"sourceAccountId"`
	// DestAccountID is the account swept TO (e.g. savings).
	DestAccountID string `json:"destAccountId"`
	// KeepMinor is the floor to leave in the source account, in its currency minor
	// units. Anything above this is the sweepable excess.
	KeepMinor int64 `json:"keepMinor"`
	// Cadence is how often a proposal is generated. One of the SweepCadence values.
	Cadence SweepCadence `json:"cadence"`
	// LastProposed is the anchor for the cadence check — the day the last proposal
	// was generated (RFC3339 date). Empty means "never proposed"; a proposal is due
	// immediately for an enabled rule with a sweepable excess.
	LastProposed string `json:"lastProposed,omitempty"`
	// Enabled gates the rule without deleting it. A disabled rule proposes nothing.
	Enabled bool `json:"enabled,omitempty"`
	// MinSweepMinor suppresses trivial proposals: no proposal is generated unless the
	// excess is at least this much. Zero means "propose any positive excess".
	MinSweepMinor int64 `json:"minSweepMinor,omitempty"`
}

// SweepCadence is how often a sweep rule generates a proposal.
type SweepCadence string

const (
	// SweepWeekly proposes every 7 days.
	SweepWeekly SweepCadence = "weekly"
	// SweepBiweekly proposes every 14 days.
	SweepBiweekly SweepCadence = "biweekly"
	// SweepMonthly proposes every 30 days (the default cadence).
	SweepMonthly SweepCadence = "monthly"
	// SweepQuarterly proposes every 90 days.
	SweepQuarterly SweepCadence = "quarterly"
)

// CadenceDays returns the whole-day interval for a sweep cadence. An unknown or
// empty cadence falls back to monthly.
func (c SweepCadence) CadenceDays() int {
	switch c {
	case SweepWeekly:
		return 7
	case SweepBiweekly:
		return 14
	case SweepQuarterly:
		return 90
	default: // SweepMonthly and unknown
		return 30
	}
}

// Valid reports whether the cadence is one of the known values.
func (c SweepCadence) Valid() bool {
	switch c {
	case SweepWeekly, SweepBiweekly, SweepMonthly, SweepQuarterly:
		return true
	default:
		return false
	}
}
