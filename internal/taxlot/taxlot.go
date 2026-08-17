// SPDX-License-Identifier: MIT

// Package taxlot relieves cost basis when a position is sold, and reports what
// was actually realized (FP-T1d).
//
// Before this, selling meant deleting the holding. The unrealized gain vanished
// with it and nothing recorded what the sale earned — so the one number a
// household needs in April, and the only one the app was ever in a position to
// compute, was the one it threw away.
//
// The whole package rests on a single fact: shares bought on different days at
// different prices are NOT interchangeable. Two hundred shares sold from a
// position of five hundred have a specific basis and a specific holding period,
// and which two hundred were sold changes both the tax owed and whether it is
// taxed at the long-term rate. Averaging is not a simplification here; it is a
// different, wrong answer.
package taxlot

import (
	"math"
	"sort"
	"time"
)

// Method is how basis is chosen when a sale is covered by several lots.
//
// The choice is the household's, not the app's, and it is worth real money: on
// an appreciated position, FIFO usually relieves the OLDEST and cheapest shares
// (a bigger taxable gain, more often long-term), while highest-cost relieves the
// most expensive (a smaller gain, more often short-term). An app that silently
// picked one would be making a tax decision on the user's behalf and not saying
// so.
type Method string

const (
	// FIFO relieves the oldest lots first. The default, and what the tax
	// authorities assume when no other identification was made at the time of
	// sale — which is the state a household reconstructing a sale afterwards is
	// almost always in.
	FIFO Method = "fifo"
	// LIFO relieves the newest lots first.
	LIFO Method = "lifo"
	// HighestCost relieves the most expensive shares first, minimizing the
	// reported gain.
	HighestCost Method = "hifo"
)

// Normalized returns the method to actually use, defaulting an empty or unknown
// value to FIFO rather than refusing.
//
// Defaulting is right here where refusing is right elsewhere in this package: an
// unrecognized method is a stored preference the app can safely fall back on,
// whereas missing lots are missing FACTS, and inventing those changes the answer.
func (m Method) Normalized() Method {
	switch m {
	case LIFO, HighestCost:
		return m
	default:
		return FIFO
	}
}

// Lot is a dated acquisition of shares at a known cost.
type Lot struct {
	ID string
	// Date is when the shares were acquired. It sets the holding period, so a
	// zero date is treated as unknown and forces the short-term answer — the
	// conservative direction, since claiming long-term treatment the household
	// cannot support is the error that costs them.
	Date           time.Time
	Shares         float64
	CostBasisMinor int64
}

// Piece is the part of one lot that a sale relieved.
type Piece struct {
	LotID         string
	Acquired      time.Time
	Shares        float64
	BasisMinor    int64
	ProceedsMinor int64
	GainMinor     int64
	// LongTerm is whether this piece was held for MORE than a year, which is the
	// distinction that changes the rate. It is per-piece, never per-sale: one
	// disposal routinely spans both, and a single flag on the sale would be a
	// coin-flip presented as a fact.
	LongTerm bool
}

// Sale is everything one disposal realized.
type Sale struct {
	Pieces        []Piece
	SharesSold    float64
	ProceedsMinor int64
	BasisMinor    int64
	GainMinor     int64
	// ShortTermGainMinor and LongTermGainMinor split the gain by holding period.
	// They sum to GainMinor by construction.
	ShortTermGainMinor int64
	LongTermGainMinor  int64
}

// shareEpsilon is the tolerance for comparing fractional share counts.
//
// Fractional shares are real (dividend reinvestment produces them constantly),
// and float arithmetic on them accumulates error, so "did this lot cover the
// sale" cannot be an exact comparison. A millionth of a share is far below any
// quantity anyone trades and far above the error a few dozen additions produce.
const shareEpsilon = 1e-6

// IsLongTerm reports whether shares acquired on one date and sold on another
// were held long enough for long-term treatment.
//
// MORE than one year, not one year or more: a sale on the anniversary is
// short-term. The off-by-one is the whole rule, and getting it backwards would
// report the lower rate on the one day it does not apply.
//
// An unknown (zero) acquisition date reports false. The app cannot support a
// long-term claim it has no date for, and the conservative direction is the one
// that does not overstate what the household can defend.
func IsLongTerm(acquired, sold time.Time) bool {
	if acquired.IsZero() || sold.IsZero() {
		return false
	}
	return sold.After(acquired.AddDate(1, 0, 0))
}

// Relieve sells `shares` out of `lots` for `proceedsMinor` total, returning what
// was realized and the lots that remain.
//
// It reports ok=false — rather than a zero basis — when the lots do not cover
// the sale. A zero basis is not "we do not know"; it is a claim that every dollar
// of the proceeds was profit, which is the largest possible tax bill and would be
// presented with the same confidence as a correct one. Missing lots are a fact
// the household has to supply, and the surface must ask rather than guess.
func Relieve(lots []Lot, shares float64, proceedsMinor int64, soldOn time.Time, m Method) (Sale, []Lot, bool) {
	if shares <= 0 || proceedsMinor < 0 || soldOn.IsZero() {
		return Sale{}, lots, false
	}
	usable := make([]Lot, 0, len(lots))
	var have float64
	for _, l := range lots {
		if l.Shares <= 0 {
			continue
		}
		usable = append(usable, l)
		have += l.Shares
	}
	if have+shareEpsilon < shares {
		return Sale{}, lots, false
	}
	sortLots(usable, m.Normalized())

	perShare := float64(proceedsMinor) / shares
	sale := Sale{SharesSold: shares, ProceedsMinor: proceedsMinor}
	remaining := make([]Lot, 0, len(usable))
	left := shares

	for i := range usable {
		l := usable[i]
		if left <= shareEpsilon {
			remaining = append(remaining, l)
			continue
		}
		take := l.Shares
		if take > left {
			take = left
		}
		// A fully relieved lot hands over its EXACT remaining basis rather than a
		// recomputed share of it. Proportional arithmetic on the last slice of a
		// lot leaves a cent or two behind that belongs to nobody, and those cents
		// accumulate into a basis that no longer matches what was paid.
		basis := l.CostBasisMinor
		if take < l.Shares-shareEpsilon {
			basis = int64(math.Round(float64(l.CostBasisMinor) * take / l.Shares))
			l.CostBasisMinor -= basis
			l.Shares -= take
			remaining = append(remaining, l)
		}
		proceeds := int64(math.Round(perShare * take))
		p := Piece{
			LotID: l.ID, Acquired: l.Date, Shares: take,
			BasisMinor: basis, ProceedsMinor: proceeds,
			LongTerm: IsLongTerm(l.Date, soldOn),
		}
		sale.Pieces = append(sale.Pieces, p)
		sale.BasisMinor += basis
		left -= take
	}

	// The pieces must sum to exactly what was received. Rounding per piece leaves
	// a cent or two unallocated, and a sale whose parts do not add up to its total
	// is the kind of discrepancy that costs an hour to chase in April.
	if n := len(sale.Pieces); n > 0 {
		var allocated int64
		for _, p := range sale.Pieces {
			allocated += p.ProceedsMinor
		}
		sale.Pieces[n-1].ProceedsMinor += proceedsMinor - allocated
	}
	for i := range sale.Pieces {
		p := &sale.Pieces[i]
		p.GainMinor = p.ProceedsMinor - p.BasisMinor
		sale.GainMinor += p.GainMinor
		if p.LongTerm {
			sale.LongTermGainMinor += p.GainMinor
		} else {
			sale.ShortTermGainMinor += p.GainMinor
		}
	}
	return sale, remaining, true
}

// sortLots orders lots into the sequence the method relieves them in. Ties break
// on date then ID so the same sale always relieves the same shares — a
// non-deterministic basis would make the same disposal report different tax in
// two sessions.
func sortLots(lots []Lot, m Method) {
	sort.SliceStable(lots, func(i, j int) bool {
		a, b := lots[i], lots[j]
		switch m {
		case LIFO:
			if !a.Date.Equal(b.Date) {
				return a.Date.After(b.Date)
			}
		case HighestCost:
			ap, bp := perShareCost(a), perShareCost(b)
			if ap != bp {
				return ap > bp
			}
			if !a.Date.Equal(b.Date) {
				return a.Date.Before(b.Date)
			}
		default:
			if !a.Date.Equal(b.Date) {
				return a.Date.Before(b.Date)
			}
		}
		return a.ID < b.ID
	})
}

// perShareCost is a lot's cost per share, zero when it holds no shares.
func perShareCost(l Lot) float64 {
	if l.Shares <= 0 {
		return 0
	}
	return float64(l.CostBasisMinor) / l.Shares
}

// TotalShares sums the shares across lots, so a caller can tell whether the lots
// describe the whole position or only part of it.
func TotalShares(lots []Lot) float64 {
	var n float64
	for _, l := range lots {
		if l.Shares > 0 {
			n += l.Shares
		}
	}
	return n
}

// TotalBasisMinor sums the cost basis across lots.
func TotalBasisMinor(lots []Lot) int64 {
	var b int64
	for _, l := range lots {
		if l.Shares > 0 {
			b += l.CostBasisMinor
		}
	}
	return b
}

// Covers reports whether lots account for a position's full share count.
//
// Separate from Relieve because the surface needs to say "these lots cover 300
// of your 500 shares" BEFORE the user tries to sell — discovering an incomplete
// history at the moment of recording a sale is discovering it too late.
func Covers(lots []Lot, positionShares float64) bool {
	return TotalShares(lots)+shareEpsilon >= positionShares
}
