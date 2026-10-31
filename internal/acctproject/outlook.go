// SPDX-License-Identifier: MIT

package acctproject

import "time"

// OutlookKind is the one-line reading of a projection against today's balance.
type OutlookKind int

const (
	// OutlookRising means the balance never dips below where it starts.
	OutlookRising OutlookKind = iota
	// OutlookLow means the balance dips below today's figure but stays at or above zero.
	OutlookLow
	// OutlookNegative means the balance closes below zero on some day of the horizon.
	OutlookNegative
)

// Outlook is the reading of a projection: its kind, and for OutlookNegative the
// first day the balance closes below zero.
type Outlook struct {
	Kind       OutlookKind
	NegativeOn time.Time
}

// Outlook reads the projection against its own starting balance, so it always
// agrees with HasLowDip. Running out outranks a dip, and a dip outranks a steady
// or rising line. asOf is the day the projection starts from. The balance
// argument is kept for existing callers and is not consulted: the projection was
// built from that balance, and comparing against a different figure would make
// Outlook and HasLowDip disagree.
func (p Projection) Outlook(_ int64, asOf time.Time) (Outlook, error) {
	negativeOn, goesNegative, err := p.FirstNegative(asOf)
	if err != nil {
		return Outlook{}, err
	}
	switch {
	case goesNegative:
		return Outlook{Kind: OutlookNegative, NegativeOn: negativeOn}, nil
	case p.HasLowDip():
		return Outlook{Kind: OutlookLow}, nil
	default:
		return Outlook{Kind: OutlookRising}, nil
	}
}
