// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Cadence is discovery's rich internal repeat rhythm — a superset of
// domain.RecurringCadence that additionally distinguishes every-4-weeks from
// monthly and carries a semiannual rhythm, both of which the deterministic
// rhythm stage must tell apart to be honest. Map a detected cadence to a
// persistable domain cadence with DomainCadence when a candidate is confirmed;
// the true rhythm always stays available on Evidence.Cadence.
type Cadence int8

const (
	// CadenceUnknown means the rhythm could not be established (too few or too
	// irregular occurrences).
	CadenceUnknown Cadence = iota
	CadenceWeekly
	CadenceBiweekly    // every ~14 days, constant gap
	CadenceSemimonthly // twice a month on two anchor days (~1st & 15th)
	CadenceMonthly     // once a month, day-of-month anchored
	CadenceEvery4Weeks // every 28 days — 13 a year, day-of-month drifts earlier
	CadenceQuarterly   // every ~3 months
	CadenceSemiannual  // every ~6 months
	CadenceAnnual      // every ~12 months
)

// String renders a plain lowercase cadence name for evidence and logs.
func (c Cadence) String() string {
	switch c {
	case CadenceWeekly:
		return "weekly"
	case CadenceBiweekly:
		return "biweekly"
	case CadenceSemimonthly:
		return "semi-monthly"
	case CadenceMonthly:
		return "monthly"
	case CadenceEvery4Weeks:
		return "every 4 weeks"
	case CadenceQuarterly:
		return "quarterly"
	case CadenceSemiannual:
		return "semiannual"
	case CadenceAnnual:
		return "annual"
	default:
		return "unknown"
	}
}

// nominalGap is the cadence's expected inter-arrival gap in days, used for gap
// fitting, liveness, and death stepping.
func (c Cadence) nominalGap() float64 {
	switch c {
	case CadenceWeekly:
		return 7
	case CadenceBiweekly:
		return 14
	case CadenceSemimonthly:
		return 365.25 / 24 // 24 charges a year ≈ 15.22 days
	case CadenceMonthly:
		return 365.25 / 12 // ≈ 30.44
	case CadenceEvery4Weeks:
		return 28
	case CadenceQuarterly:
		return 365.25 / 4 // ≈ 91.31
	case CadenceSemiannual:
		return 365.25 / 2 // ≈ 182.63
	case CadenceAnnual:
		return 365.25
	default:
		return 0
	}
}

// evidenceFloor is the minimum occurrence count for this cadence to be proposed:
// 3 for monthly-or-faster, 2 for the long semiannual/annual cadences (accepted at
// reduced confidence because a year of history yields few points).
func (c Cadence) evidenceFloor() int {
	switch c {
	case CadenceSemiannual, CadenceAnnual:
		return 2
	default:
		return 3
	}
}

// DomainCadence maps a detected cadence to the nearest persistable
// domain.RecurringCadence for confirming a commitment. The domain enum has no
// exact every-4-weeks or semiannual value, so those map to the cadence with the
// closest inter-arrival gap — every-4-weeks → monthly, semiannual → quarterly —
// and the confirm flow should surface the true rhythm from Evidence.Cadence
// (e.g. "detected every 4 weeks; scheduled monthly"). CadenceUnknown maps to
// monthly as a neutral default.
func DomainCadence(c Cadence) domain.RecurringCadence {
	switch c {
	case CadenceWeekly:
		return domain.CadenceWeekly
	case CadenceBiweekly:
		return domain.CadenceBiweekly
	case CadenceSemimonthly:
		return domain.CadenceSemimonthly
	case CadenceMonthly, CadenceEvery4Weeks:
		return domain.CadenceMonthly
	case CadenceQuarterly, CadenceSemiannual:
		return domain.CadenceQuarterly
	case CadenceAnnual:
		return domain.CadenceYearly
	default:
		return domain.CadenceMonthly
	}
}

// AmountKind classifies a candidate's cost behaviour.
type AmountKind int8

const (
	// AmountFixed is the same amount every time.
	AmountFixed AmountKind = iota
	// AmountBanded is variable within a median ± tolerance band.
	AmountBanded
	// AmountStepped is a single durable price change on the same commitment (a
	// creep signal), never a reason to split.
	AmountStepped
)

// String renders the amount kind for evidence and logs.
func (k AmountKind) String() string {
	switch k {
	case AmountFixed:
		return "fixed"
	case AmountBanded:
		return "banded"
	case AmountStepped:
		return "stepped"
	default:
		return "unknown"
	}
}

// PriceStep is a single durable level shift within one candidate — a price
// change emitted on the SAME candidate rather than splitting it.
type PriceStep struct {
	FromMinor int64     // the amount before the change
	ToMinor   int64     // the amount after the change
	At        time.Time // date of the first occurrence at the new level
}

// AmountModel describes a candidate's cost. All values are integer minor units.
type AmountModel struct {
	Kind AmountKind
	// Typical is the representative amount: the exact amount when fixed, the
	// median when banded, the current (post-step) level when stepped.
	Typical int64
	// LowMinor / HighMinor bound the observed amounts (band edges, or the two
	// levels of a step).
	LowMinor  int64
	HighMinor int64
	// ToleranceMinor is the ± band width for AmountBanded (0 otherwise).
	ToleranceMinor int64
	// Step is set only for AmountStepped.
	Step *PriceStep
}

// Evidence is everything the UI needs to justify a candidate without recomputing:
// how many occurrences, the detected cadence, the anchor day and posting window,
// the cost model, the date span, and the contributing transaction refs. The UI
// renders it through i18n (e.g. "9 payments · monthly around the 9th, usually
// posts by the 11th · $75.00 every time · last Jul 9").
type Evidence struct {
	Count int
	// Cadence is the true detected rhythm (may be finer than the domain cadence a
	// confirm would persist).
	Cadence Cadence
	// AnchorDay is the day the charge is anchored to: day-of-month (1..31) for the
	// monthly/semi-monthly/quarterly/annual family, or ISO weekday (1=Mon..7=Sun)
	// for the weekly/biweekly/every-4-weeks family.
	AnchorDay int
	// WindowSpread is how many days later than the anchor the charge has drifted
	// (weekend-shift and processing lag), so the UI can say "usually posts by the
	// Nth".
	WindowSpread int
	// PostsBy is AnchorDay + WindowSpread — the day the charge has typically posted
	// by (a day-of-month for the monthly family, an ISO weekday for the weekly
	// family).
	PostsBy int
	Amount  AmountModel
	// FirstSeen / LastSeen bound the observed occurrences.
	FirstSeen time.Time
	LastSeen  time.Time
	// TxnIDs are the contributing transaction refs in chronological order — the
	// prior cycles a confirm back-claims.
	TxnIDs []string
	// Currency is carried through from the contributing transactions (informational).
	Currency string
}

// Tier buckets a candidate by confidence for the review trust ladder.
type Tier int8

const (
	// TierSilent is a weak signal, shown only under "weak signals" in Detection
	// preferences — including patterns that appear to have stopped.
	TierSilent Tier = iota
	// TierNeedsReview is a plausible pattern the user should confirm before it is
	// trusted.
	TierNeedsReview
	// TierLikely is a strong, one-tap candidate.
	TierLikely
)

// String renders the tier for logs.
func (t Tier) String() string {
	switch t {
	case TierLikely:
		return "likely"
	case TierNeedsReview:
		return "needs-review"
	default:
		return "silent"
	}
}

// Candidate is a proposed new recurring commitment with the evidence behind it.
type Candidate struct {
	Signature string    // canonical quarantined signature of the source cluster
	Payee     string    // representative display payee (most common among the cluster)
	Direction Direction // in (income) or out (bill/subscription)
	AccountID string
	// IsIncome flags a stable inbound flow (a paycheck) — the income candidate the
	// pinch/tideline needs to draw the pay cycle.
	IsIncome   bool
	Confidence float64 // 0..1
	Tier       Tier
	Evidence   Evidence
}

// Commitment describes an existing recurring commitment just enough for dedupe:
// discovery re-derives its signature from Payee and matches a cluster on
// (signature, account, direction). A matched cluster becomes cycle-matches for
// the commitment rather than a fresh candidate.
type Commitment struct {
	ID        string
	Payee     string
	AccountID string
	Direction Direction
}

// Options tunes a discovery run. Now anchors liveness and last-seen reasoning;
// when zero it defaults to the latest transaction date so the package stays pure
// (no wall clock). MinOccurrences, when > 0, overrides the monthly-or-faster
// evidence floor (the long-cadence floor of 2 is unaffected).
type Options struct {
	Now            time.Time
	MinOccurrences int
}

// CommitmentCycle is a cluster that matched an existing commitment: its evidence
// and the contributing transaction refs (back-claimable prior cycles) reported
// against that commitment instead of proposed anew.
type CommitmentCycle struct {
	CommitmentID string
	Evidence     Evidence
	TxnIDs       []string
}

// Result is the outcome of a discovery run: new candidates and the cluster
// matches that belong to existing commitments.
type Result struct {
	Candidates   []Candidate
	CycleMatches []CommitmentCycle
}
