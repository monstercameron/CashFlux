// SPDX-License-Identifier: MIT

// Package investincome rolls up the income a portfolio PAID OUT — dividends,
// interest, distributions — as distinct from what it grew (FP-T1f).
//
// The two are different facts and the app conflated them. A dividend landing in
// a brokerage account raises the balance, so the growth chart counts it as the
// portfolio doing well; but it is cash the household actually received and could
// have spent, and it is taxed in the year it arrived whether or not it was
// reinvested. A position can be flat for a decade and still have paid for a
// holiday every year, and nothing in the app could say so.
//
// The seam is one field: a transaction tagged with the holding it came from.
// Income that is not tagged is reported as untagged rather than spread across
// positions — a yield figure assembled from guesses is worse than an absent one,
// because it looks like a measurement.
package investincome

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// HoldingIncome is what one position paid out over the window.
type HoldingIncome struct {
	HoldingID  string
	TotalMinor int64
	Count      int
	// FirstDate and LastDate bound the payments actually seen. They are what a
	// yield can honestly be annualized over: three months of dividends is not a
	// year's income, and stating the span stops a surface from pretending it is.
	FirstDate, LastDate time.Time
}

// Summary is portfolio income over a window.
type Summary struct {
	ByHolding map[string]HoldingIncome
	// TotalMinor counts only TAGGED income — the figure that can be attributed.
	TotalMinor int64
	// UntaggedMinor is income posted to investment accounts with no holding named.
	//
	// Reported rather than silently dropped or spread. A household whose dividends
	// are mostly untagged should see that the number is incomplete; the same
	// number with the untagged part quietly excluded reads as the whole truth.
	UntaggedMinor int64
	UntaggedCount int
}

// Gather rolls up investment income between from (inclusive) and to (exclusive).
//
// Only income counts: a fee or a commission posted against a holding is real and
// worth recording, but it is not a payout, and folding it in would net two facts
// the household needs separately. Transfers are excluded for the same reason they
// are excluded from a return — moving money in is not being paid.
func Gather(txns []domain.Transaction, accountIDs map[string]bool, from, to time.Time, rates currency.Rates) Summary {
	s := Summary{ByHolding: map[string]HoldingIncome{}}
	for _, t := range txns {
		if !t.IsIncome() || !t.CountsInReports() || !dateutil.InRange(t.Date, from, to) {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		if t.HoldingID == "" {
			// Untagged income only counts when it landed in an investment account.
			// A salary deposit is income, and counting it here would make a
			// portfolio look like it yielded a paycheck.
			if accountIDs != nil && accountIDs[t.AccountID] {
				s.UntaggedMinor += amt.Amount
				s.UntaggedCount++
			}
			continue
		}
		h := s.ByHolding[t.HoldingID]
		h.HoldingID = t.HoldingID
		h.TotalMinor += amt.Amount
		h.Count++
		if h.FirstDate.IsZero() || t.Date.Before(h.FirstDate) {
			h.FirstDate = t.Date
		}
		if t.Date.After(h.LastDate) {
			h.LastDate = t.Date
		}
		s.ByHolding[t.HoldingID] = h
		s.TotalMinor += amt.Amount
	}
	return s
}

// YieldOnCostPct is income as a percentage of what the shares cost.
//
// Yield on COST, not on current value, and the distinction is the point: a
// position bought at $10 that now trades at $100 and pays $1 yields 10% on what
// the household actually spent and 1% on what it is worth. The first is the
// honest answer to "was this a good buy"; the second reprices the past.
//
// Reports ok=false on a zero or negative basis rather than dividing — an
// infinite yield is not a good result, it is a missing cost.
func YieldOnCostPct(incomeMinor, costBasisMinor int64) (float64, bool) {
	if costBasisMinor <= 0 {
		return 0, false
	}
	return float64(incomeMinor) / float64(costBasisMinor) * 100, true
}

// MinIncomeDays is the shortest span a yield is annualized over.
//
// Ninety days, because dividends are lumpy: most positions pay quarterly, so a
// shorter window either catches one payment and annualizes it fourfold or
// catches none and reports zero. Both are confident answers to a question the
// data cannot support.
const MinIncomeDays = 90

// AnnualizedYieldPct scales a yield to a year from the span it was earned over.
//
// It takes the OBSERVED span (first payment to last, plus the period they
// represent), not the window the user happened to select: a filter set to two
// years over a position bought last month would otherwise halve a real yield.
func AnnualizedYieldPct(incomeMinor, costBasisMinor int64, days int) (float64, bool) {
	if days < MinIncomeDays {
		return 0, false
	}
	y, ok := YieldOnCostPct(incomeMinor, costBasisMinor)
	if !ok {
		return 0, false
	}
	return y * 365.0 / float64(days), true
}

// Span is the number of days a holding's income covers, counting the payment
// period rather than the gap between payments.
//
// One payment covers a period, not an instant. Measuring first-to-last would
// make a single dividend span zero days and a year of quarterly dividends span
// nine months, understating both. Adding the average interval closes the period
// the last payment stands for.
func Span(h HoldingIncome) int {
	if h.Count == 0 || h.FirstDate.IsZero() {
		return 0
	}
	if h.Count == 1 {
		// One payment says nothing about cadence, so there is no period to close.
		// Callers get zero and the annualizer refuses, which is the right answer.
		return 0
	}
	gap := int(h.LastDate.Sub(h.FirstDate).Hours() / 24)
	if gap <= 0 {
		return 0
	}
	interval := gap / (h.Count - 1)
	return gap + interval
}
