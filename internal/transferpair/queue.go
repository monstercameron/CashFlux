// SPDX-License-Identifier: MIT

package transferpair

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Item is one row that needs a person, and what the matcher can offer them.
type Item struct {
	Match
	// Kind says what is wrong, so the queue can group rather than present a flat
	// list of equally-urgent-looking rows.
	Kind ItemKind
}

// ItemKind is why a row is in the queue.
type ItemKind int

const (
	// KindOrphan is a row already flagged as a transfer whose far side is nowhere
	// in the ledger. Until it is resolved, one account is wrong by the moved
	// amount — this is the most serious thing the queue holds.
	KindOrphan ItemKind = iota
	// KindAmbiguous is a flagged transfer with more than one possible counterpart.
	// Nothing is wrong yet; the app simply must not choose.
	KindAmbiguous
	// KindUnflagged is a row that is NOT marked as a transfer but has a
	// counterpart that looks like one. Left alone it inflates income and spending
	// on both sides.
	KindUnflagged
)

// String renders the kind for logs and test failures.
func (k ItemKind) String() string {
	switch k {
	case KindOrphan:
		return "orphan"
	case KindAmbiguous:
		return "ambiguous"
	default:
		return "unflagged"
	}
}

// Report is the queue: every row a person needs to look at, and the verified
// pairs that needed nobody.
type Report struct {
	// Items are the rows needing attention, most serious first.
	Items []Item
	// VerifiedUnflagged are rows the matcher is confident about that carry no
	// transfer flag yet. A caller may act on these without asking; that is what
	// Verified means.
	VerifiedUnflagged []Match
}

// Build walks the ledger once and sorts every transfer-shaped row into the
// queue.
//
// A row is transfer-SHAPED if it is already flagged, or if it has an exactly
// opposite counterpart on another account within the window. That second half is
// what catches an import: both legs arrive as ordinary income and spending, and
// nothing in either row says they are one movement.
//
// Pairs are reported once, from the leg the money LEFT, so a matched pair does
// not appear twice saying the same thing from both ends.
func Build(txns []domain.Transaction, accounts []domain.Account) Report {
	var r Report
	claimed := map[string]bool{}

	// Deterministic order: the ledger arrives in whatever order the store returns,
	// and a queue that reshuffles between renders is one nobody can work through.
	ordered := append([]domain.Transaction(nil), txns...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].Date.Equal(ordered[j].Date) {
			return ordered[i].Date.Before(ordered[j].Date)
		}
		return ordered[i].ID < ordered[j].ID
	})

	for _, t := range ordered {
		if claimed[t.ID] {
			continue
		}
		m := For(t, ordered, accounts)

		if t.IsTransfer() {
			switch m.Confidence {
			case Unmatched:
				r.Items = append(r.Items, Item{Match: m, Kind: KindOrphan})
			case Possible:
				if m.Has(ReasonSeveralCandidates) || m.Has(ReasonDescriptorDisagrees) {
					r.Items = append(r.Items, Item{Match: m, Kind: KindAmbiguous})
				}
				// A single candidate with no descriptor evidence is the ordinary
				// case for a hand-entered transfer. It is not a problem to report.
				claimed[t.ID], claimed[m.Partner.ID] = true, true
			case Verified:
				claimed[t.ID], claimed[m.Partner.ID] = true, true
			}
			continue
		}

		// Not flagged. Only worth raising when something is actually on the other
		// side; an ordinary purchase has no counterpart and belongs nowhere near
		// this queue.
		if m.Confidence == Unmatched {
			continue
		}
		// Report from the outgoing leg so one movement produces one entry.
		if !t.Amount.IsNegative() {
			continue
		}
		if m.Confidence == Verified {
			r.VerifiedUnflagged = append(r.VerifiedUnflagged, m)
		} else {
			r.Items = append(r.Items, Item{Match: m, Kind: KindUnflagged})
		}
		claimed[t.ID], claimed[m.Partner.ID] = true, true
	}

	sort.SliceStable(r.Items, func(i, j int) bool {
		if r.Items[i].Kind != r.Items[j].Kind {
			return r.Items[i].Kind < r.Items[j].Kind
		}
		return r.Items[i].Leg.Date.Before(r.Items[j].Leg.Date)
	})
	return r
}

// Orphans returns just the rows whose far side is missing — the figure a health
// check reports, and the only kind in the queue that means an account is
// currently wrong.
func (r Report) Orphans() []Item {
	var out []Item
	for _, it := range r.Items {
		if it.Kind == KindOrphan {
			out = append(out, it)
		}
	}
	return out
}
