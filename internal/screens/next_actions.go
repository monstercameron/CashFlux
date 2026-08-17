// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/actionrank"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// nextActionRow is one ranked action plus the sentence explaining its place.
type nextActionRow struct {
	Title  string
	Impact string
	// Why is why this row sits above the one below it. Empty on the last row, and
	// on any pair the ranking could not honestly separate.
	Why string
}

// rankNextActions turns findings into an ordered list of things to do, with a
// reason for each position (WF-SM3).
//
// ONLY findings that declared what their amount is worth are ranked. An insight
// that carries a figure for context — a subscription's cost shown while asking
// who it belongs to — is not an action worth this much, and ranking it as one
// would put "assign this to someone" above genuinely saving money. Those are
// left out rather than guessed at, and the count of them is reported so the
// omission is visible rather than silent.
func rankNextActions(ins []smart.Insight) (rows []nextActionRow, skipped int) {
	// WF-SM4: a household that said "never suggest drawing from the Roth" should
	// not be shown "move idle cash out of the Roth" as the top thing to do. The
	// instruction constrains the SUGGESTION only — nothing stops them moving that
	// money themselves, and nothing here reports it if they do.
	book := uistate.LoadStanding()

	var actions []actionrank.Action
	byID := map[string]smart.Insight{}
	for _, i := range ins {
		if !i.HasAmount || i.Action == nil {
			continue
		}
		if i.Action.RelatedType == "account" && !book.MayProposeDrawingFrom(i.Action.RelatedID) {
			continue
		}
		a := actionrank.Action{
			ID:   i.Key,
			Name: i.Title,
			// Everything the detectors can currently justify is reversible and
			// low-effort — cancel a subscription, move cash, pay more than the
			// minimum. Claiming otherwise would be inventing an effort rating no
			// detector supplies.
			Reversible:  true,
			Effort:      actionrank.EffortLow,
			UrgencyDays: -1,
			Confidence:  1,
		}
		switch i.AmountCadence {
		case smart.AmountMonthly:
			a.MonthlyImpactMinor = absMinor(i.Amount.Amount)
		case smart.AmountAnnual:
			// Divided here rather than at the detector: a yearly fee is not twelve
			// monthly ones, and the conversion belongs where the comparison happens.
			a.MonthlyImpactMinor = absMinor(i.Amount.Amount) / 12
		case smart.AmountOneTime:
			a.OneTimeImpactMinor = absMinor(i.Amount.Amount)
		default:
			skipped++
			continue // the detector did not say what its figure is worth
		}
		actions = append(actions, a)
		byID[a.ID] = i
	}
	ranked := actionrank.Rank(actions)
	for idx, r := range ranked {
		i := byID[r.Action.ID]
		row := nextActionRow{Title: i.Title, Impact: nextActionImpact(i)}
		if idx+1 < len(ranked) {
			if reason := actionrank.Why(r, ranked[idx+1]); !reason.TooClose {
				row.Why = nextActionWhy(reason.Criterion)
			} else {
				// Saying "these two are much of a muchness" is more useful than
				// silence, and far more useful than a manufactured reason.
				row.Why = uistate.T("nextact.whyClose")
			}
		}
		rows = append(rows, row)
	}
	return rows, skipped
}

// nextActionImpact states the figure in the terms the detector declared, so a
// yearly saving is never shown as though it arrived every month.
func nextActionImpact(i smart.Insight) string {
	amt := fmtMoney(money.New(absMinor(i.Amount.Amount), i.Amount.Currency))
	switch i.AmountCadence {
	case smart.AmountMonthly:
		return uistate.T("nextact.perMonth", amt)
	case smart.AmountAnnual:
		return uistate.T("nextact.perYear", amt)
	case smart.AmountOneTime:
		return uistate.T("nextact.once", amt)
	}
	return ""
}

// nextActionWhy is the plain-English name of the criterion that decided a pair.
func nextActionWhy(c actionrank.Criterion) string {
	switch c {
	case actionrank.CritMonthly:
		return uistate.T("nextact.whyMonthly")
	case actionrank.CritOneTime:
		return uistate.T("nextact.whyOneTime")
	case actionrank.CritUrgency:
		return uistate.T("nextact.whyUrgency")
	case actionrank.CritEffort:
		return uistate.T("nextact.whyEffort")
	case actionrank.CritReversible:
		return uistate.T("nextact.whyReversible")
	case actionrank.CritConfidence:
		return uistate.T("nextact.whyConfidence")
	}
	return ""
}
