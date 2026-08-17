// SPDX-License-Identifier: MIT

package portfolio

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/taxlot"
)

// ─── FP-T1d: the seam between a stored holding and the tax-lot engine ────────
//
// `taxlot` knows nothing about domain types on purpose — it is arithmetic about
// shares and dates, and giving it a Holding would tie a tax calculation to a
// storage shape. This file is the one place the two meet, so a caller cannot
// convert them slightly differently somewhere else.

// TaxLots is a holding's purchase history in the form the engine takes.
func TaxLots(h domain.Holding) []taxlot.Lot {
	out := make([]taxlot.Lot, 0, len(h.Lots))
	for _, l := range h.Lots {
		out = append(out, taxlot.Lot{
			ID: l.ID, Date: l.Date, Shares: l.Shares, CostBasisMinor: l.CostBasisMinor,
		})
	}
	return out
}

// ApplyLots writes relieved lots back onto a holding, and re-derives the
// position's share count and cost basis FROM those lots.
//
// Re-deriving rather than adjusting separately is the point. Shares, cost basis
// and lots are three statements about the same thing, and letting a sale update
// them independently is how a position ends up holding 40 shares whose lots say
// 60 — a disagreement nothing downstream can resolve, because both numbers look
// equally authoritative.
//
// Notes on each lot are carried through: they are the household's own record of
// what a purchase was, and a partial sale is no reason to lose it.
func ApplyLots(h domain.Holding, lots []taxlot.Lot) domain.Holding {
	kept := make([]domain.Lot, 0, len(lots))
	notes := make(map[string]string, len(h.Lots))
	for _, l := range h.Lots {
		notes[l.ID] = l.Note
	}
	var shares float64
	var basis int64
	for _, l := range lots {
		if l.Shares <= 0 {
			continue
		}
		kept = append(kept, domain.Lot{
			ID: l.ID, Date: l.Date, Shares: l.Shares,
			CostBasisMinor: l.CostBasisMinor, Note: notes[l.ID],
		})
		shares += l.Shares
		basis += l.CostBasisMinor
	}
	h.Lots = kept
	h.Shares = shares
	h.CostBasisMinor = basis
	return h
}
