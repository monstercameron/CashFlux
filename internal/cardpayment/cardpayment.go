// SPDX-License-Identifier: MIT

// Package cardpayment computes the honest core of credit-card payment budgeting (BG7):
// when you spend on a credit card, the money you budgeted for that spending should be set
// aside to pay the card, so "can I pay this statement in full?" is answerable from your
// budgets rather than your mood.
//
// # Scope (what this package does and does NOT do)
//
// This is the deliberately minimal core the BG7 planning gate asked for, not YNAB's full
// mechanic:
//
//   - It reads a card's purchases over a statement period and reports how much of that
//     spending was BUDGETED — i.e. fell in a category some budget tracks. That budgeted-
//     and-spent total is the amount that "should" have moved into a payment envelope, so it
//     is the card's "funded to pay" figure. The statement total is every purchase in the
//     period. Fully funded means funded ≥ statement.
//   - The statement period is a minimal notion: the account's DueDayOfMonth as a statement
//     day when set, else a plain calendar month (StatementPeriod). It deliberately does NOT
//     resurrect the dropped reconcile-session idea — no cleared-balance reconciliation, no
//     posted-vs-pending, no per-statement locking.
//
// Deferred (documented, not built): carried/pre-existing debt is NOT funded here — a
// payment envelope funds only the current period's budgeted spend, so a card that arrived
// carrying a balance shows that balance as unfunded (honest, not hidden). The cover
// machinery (coverformula) is not invoked to conjure money when a category had no budget —
// unbudgeted card spend simply reads as unfunded, which is the true state. Actual movement
// of money into a real envelope entity, and payment-transaction/transfer bookkeeping, are
// left to the wiring layer; this package is the pure read-model it computes from.
package cardpayment

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Period is a half-open statement window [Start, End).
type Period struct {
	Start time.Time
	End   time.Time
}

// CardFunding is one card's payment-envelope read for a statement period. All amounts are
// magnitudes (positive) in the base currency's minor units.
type CardFunding struct {
	AccountID   string
	AccountName string
	Period      Period
	// StatementMinor is every purchase posted to the card in the period — what you owe for
	// the period's spending.
	StatementMinor int64
	// FundedMinor is the slice of StatementMinor that fell in a budgeted category, so the
	// budgeted money is available to pay it — the payment envelope's balance.
	FundedMinor int64
	Currency    string
}

// FullyFunded reports whether the period's budgeted money covers the whole statement.
func (f CardFunding) FullyFunded() bool { return f.FundedMinor >= f.StatementMinor }

// UnfundedMinor is the shortfall you can't cover from budgeted money (never negative).
func (f CardFunding) UnfundedMinor() int64 {
	if f.FundedMinor >= f.StatementMinor {
		return 0
	}
	return f.StatementMinor - f.FundedMinor
}

// StatementPeriod returns the statement window containing `now` for the account. When the
// account records a DueDayOfMonth it is used as the statement day (the window runs from that
// day of the month up to the same day next month); otherwise the window is the calendar
// month containing `now`. This is the minimal statement notion BG7 needs — enough to answer
// "in full for this period" without a reconcile session.
func StatementPeriod(acct domain.Account, now time.Time) Period {
	loc := now.Location()
	day := acct.DueDayOfMonth
	if day <= 0 {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		return Period{Start: start, End: dateutil.AddMonths(start, 1)}
	}
	// Anchor a monthly window on `day`. The period containing `now` starts on `day` of the
	// current month if now is on/after it, else `day` of the previous month.
	anchor := clampedDay(now.Year(), now.Month(), day, loc)
	if now.Before(anchor) {
		prev := dateutil.AddMonths(anchor, -1)
		return Period{Start: prev, End: anchor}
	}
	return Period{Start: anchor, End: dateutil.AddMonths(anchor, 1)}
}

// clampedDay builds the date for `day` of the given month, clamping to the month's last day
// (so a statement day of 31 lands on Feb 28/29). AddMonths on the first-of-month keeps the
// window boundaries stable across short months.
func clampedDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	lastDay := first.AddDate(0, 1, -1).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// Compute returns the payment-envelope funding for every non-archived credit-card
// liability account, for the statement period containing `now`. It only runs when the
// methodology is envelope or flex (the two methods where money is set aside per category
// rather than merely capped); for any other method it returns nil, so the surface stays
// scoped as BG7 requires. Cards with no purchases in the period are omitted. Results are
// sorted by unfunded amount, largest first (the cards that most need attention), then by id.
func Compute(accounts []domain.Account, txns []domain.Transaction, budgets []domain.Budget, cats []domain.Category, rates currency.Rates, now time.Time, method budgeting.Methodology) ([]CardFunding, error) {
	if method != budgeting.MethodEnvelope && method != budgeting.MethodFlex {
		return nil, nil
	}
	covered := budgeting.CoveredCategories(budgets, cats)
	var out []CardFunding
	for _, a := range accounts {
		if a.Archived || !a.IsLiability() || a.Type != domain.TypeCreditCard {
			continue
		}
		f, err := fundingForCard(a, txns, covered, rates, now)
		if err != nil {
			return nil, err
		}
		if f.StatementMinor == 0 {
			continue
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UnfundedMinor() != out[j].UnfundedMinor() {
			return out[i].UnfundedMinor() > out[j].UnfundedMinor()
		}
		return out[i].AccountID < out[j].AccountID
	})
	return out, nil
}

// fundingForCard computes one card's funding over its current statement period.
func fundingForCard(acct domain.Account, txns []domain.Transaction, covered map[string]bool, rates currency.Rates, now time.Time) (CardFunding, error) {
	period := StatementPeriod(acct, now)
	f := CardFunding{
		AccountID: acct.ID, AccountName: acct.Name, Period: period, Currency: rates.Base,
	}
	add := func(categoryID string, amt money.Money) error {
		conv, err := rates.Convert(amt.Abs(), rates.Base)
		if err != nil {
			return err
		}
		f.StatementMinor += conv.Amount
		if covered[categoryID] {
			f.FundedMinor += conv.Amount
		}
		return nil
	}
	for _, t := range txns {
		if t.AccountID != acct.ID || t.IsTransfer() || !t.IsExpense() {
			continue // only purchases posted to this card; payments (transfers) excluded
		}
		if !dateutil.InRange(t.Date, period.Start, period.End) {
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if err := add(s.CategoryID, s.Amount); err != nil {
					return CardFunding{}, err
				}
			}
			continue
		}
		if err := add(t.CategoryID, t.Amount); err != nil {
			return CardFunding{}, err
		}
	}
	return f, nil
}
