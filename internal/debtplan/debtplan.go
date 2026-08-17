// SPDX-License-Identifier: MIT

// Package debtplan models two ways of paying debt off faster that the app could
// describe but never compute: paying every two weeks, and consolidating several
// debts into one (FP-T3c).
//
// Both are advice people are given constantly and almost never given the
// arithmetic for, which is how "biweekly payments pay your mortgage off years
// early" survives as a slogan. It is true, and the reason is duller than it
// sounds: twenty-six half-payments is thirteen monthly payments, not twelve. The
// gain comes from paying MORE, not from paying more often — and a household that
// believes otherwise will happily switch to biweekly on a card that charges a fee
// for it and wonder why nothing changed.
package debtplan

import (
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/payoff"
)

// PeriodsPerYear is how many payments a biweekly schedule makes.
//
// Twenty-six, because a year holds 26 fortnights — which is 13 monthly payments'
// worth, not 12. That single extra payment is the entire effect.
const PeriodsPerYear = 26

// Biweekly is what switching to fortnightly payments does.
type Biweekly struct {
	// MonthlyMonths and BiweeklyMonths are how long each schedule takes, in
	// months.
	MonthlyMonths, BiweeklyMonths int
	// MonthsSaved is the difference, never negative.
	MonthsSaved int
	// MonthlyInterestMinor and BiweeklyInterestMinor are the total interest each
	// schedule pays.
	MonthlyInterestMinor, BiweeklyInterestMinor int64
	// InterestSavedMinor is what the switch saves.
	InterestSavedMinor int64
	// ExtraPerYearMinor is the extra money the switch actually costs each year —
	// one whole monthly payment.
	//
	// Reported prominently because it is the mechanism, and hiding it makes the
	// saving look like a free lunch. Somebody who cannot afford a thirteenth
	// payment cannot afford this plan, and should find that out here rather than
	// three months in.
	ExtraPerYearMinor int64
	// HalfPaymentMinor is what each fortnightly payment would be.
	HalfPaymentMinor int64
}

// SimulateBiweekly compares a monthly schedule against a fortnightly one.
//
// Reports ok=false rather than a zero result when the loan cannot be modelled —
// no balance, no term, or a payment too small to cover the interest. That last
// case is the important one: a loan whose payment never touches the principal
// does not "take a long time", it never ends, and reporting a large number of
// months would dress an impossibility as a plan.
func SimulateBiweekly(balanceMinor int64, aprPct float64, termMonths int, start time.Time) (Biweekly, bool) {
	if balanceMinor <= 0 || termMonths <= 0 || aprPct < 0 {
		return Biweekly{}, false
	}
	base := payoff.AmortizeFixed(balanceMinor, aprPct, termMonths, start)
	if len(base) == 0 {
		return Biweekly{}, false
	}
	payment := base[0].PaymentMinor
	monthlyInterest, _, _ := payoff.AmortSummary(base)

	// The fortnightly schedule is modelled as the monthly one plus one extra
	// payment a year, spread evenly. Simulating 26 actual fortnights would be
	// more literal and LESS accurate about the thing being asked: lenders credit
	// biweekly payments in different ways, and the honest common denominator is
	// "you pay a thirteenth payment each year".
	extraPerMonth := payment / 12
	if extraPerMonth <= 0 {
		return Biweekly{}, false
	}
	fast := payoff.AmortizeWithExtra(balanceMinor, aprPct, termMonths, extraPerMonth, start)
	if len(fast) == 0 {
		return Biweekly{}, false
	}
	fastInterest, _, _ := payoff.AmortSummary(fast)

	b := Biweekly{
		MonthlyMonths:         len(base),
		BiweeklyMonths:        len(fast),
		MonthlyInterestMinor:  monthlyInterest,
		BiweeklyInterestMinor: fastInterest,
		ExtraPerYearMinor:     payment,
		HalfPaymentMinor:      (payment + 1) / 2,
	}
	if b.MonthsSaved = b.MonthlyMonths - b.BiweeklyMonths; b.MonthsSaved < 0 {
		b.MonthsSaved = 0
	}
	if b.InterestSavedMinor = monthlyInterest - fastInterest; b.InterestSavedMinor < 0 {
		b.InterestSavedMinor = 0
	}
	return b, true
}

// Debt is one balance being considered for consolidation.
type Debt struct {
	ID           string
	Name         string
	BalanceMinor int64
	APRPct       float64
	// MinPaymentMinor is what is currently being paid each month. Zero means
	// unknown, and a debt with no payment cannot be modelled on its own — see
	// Consolidate.
	MinPaymentMinor int64
}

// Consolidation compares keeping several debts separate against combining them.
type Consolidation struct {
	// KeepMonths and KeepInterestMinor describe paying each debt off separately
	// at its current payment.
	KeepMonths        int
	KeepInterestMinor int64
	KeepTotalMinor    int64
	// NewMonths, NewInterestMinor and NewPaymentMinor describe the combined loan.
	NewMonths        int
	NewInterestMinor int64
	NewTotalMinor    int64
	NewPaymentMinor  int64
	// InterestDeltaMinor is the new plan's interest minus the current one:
	// NEGATIVE means consolidating saves money.
	//
	// The sign is this way round deliberately. The question is "what does
	// switching do to me", and a saving is a reduction — presenting it as a
	// positive "benefit" invites the reader to add it to something.
	InterestDeltaMinor int64
	// PaymentDeltaMinor is the change in what leaves the account each month.
	// Positive means the new plan costs more per month.
	PaymentDeltaMinor int64
	// FeeMinor is the origination fee folded into the new loan.
	FeeMinor int64
	// Unmodelled names debts that had to be left out because they carry no
	// payment, so a reader can see the comparison is incomplete rather than
	// trusting a total that quietly excluded something.
	Unmodelled []string
}

// Saves reports whether consolidating costs less interest overall.
func (c Consolidation) Saves() bool { return c.InterestDeltaMinor < 0 }

// TermShortenFactor is how much shorter a new term has to be before the saving
// is mostly about the term rather than the rate.
//
// A quarter shorter. Below that the two effects are comparable and separating
// them would be false precision; above it, the term is doing most of the work
// and saying "you saved by consolidating" would credit the wrong thing.
const TermShortenFactor = 0.75

// TermDriven reports whether the saving comes mainly from paying the debt off
// FASTER rather than from a better rate.
//
// This is the trap the whole comparison would otherwise walk into. Rolling a
// thirty-year mortgage into a four-year loan "saves" enormous interest at almost
// any rate, because the term collapsed — and presenting that as the benefit of
// consolidating tells someone a 12% loan beat their 6% mortgage. The saving is
// real; the reason is not the one the reader will assume, and the higher monthly
// payment is what buys it.
func (c Consolidation) TermDriven() bool {
	if !c.Saves() || c.KeepMonths <= 0 || c.NewMonths <= 0 {
		return false
	}
	return float64(c.NewMonths) < float64(c.KeepMonths)*TermShortenFactor
}

// Consolidate compares keeping debts separate against one new loan.
//
// feePct is an origination fee as a percentage of the amount borrowed, added to
// the new balance — the fee is almost always financed rather than paid up front,
// and a comparison that ignores it flatters every consolidation offer ever made.
//
// Reports ok=false when there is nothing to compare: fewer than two debts with a
// balance, or a new term of zero.
func Consolidate(debts []Debt, newAPRPct float64, newTermMonths int, feePct float64, start time.Time) (Consolidation, bool) {
	var c Consolidation
	usable := make([]Debt, 0, len(debts))
	var total int64
	for _, d := range debts {
		if d.BalanceMinor <= 0 {
			continue
		}
		usable = append(usable, d)
		total += d.BalanceMinor
	}
	if len(usable) < 2 || newTermMonths <= 0 || newAPRPct < 0 || total <= 0 {
		return c, false
	}
	// Deterministic order so the unmodelled list and every derived figure come
	// out the same way twice.
	sort.SliceStable(usable, func(i, j int) bool { return usable[i].ID < usable[j].ID })

	for _, d := range usable {
		if d.MinPaymentMinor <= 0 {
			c.Unmodelled = append(c.Unmodelled, d.Name)
			continue
		}
		rows := payoff.AmortizeAtPayment(d.BalanceMinor, d.APRPct, d.MinPaymentMinor, start)
		if len(rows) == 0 {
			// A payment that never touches the principal. Named rather than
			// silently dropped: it is the single most important fact about that debt.
			c.Unmodelled = append(c.Unmodelled, d.Name)
			continue
		}
		interest, paid, _ := payoff.AmortSummary(rows)
		c.KeepInterestMinor += interest
		c.KeepTotalMinor += paid
		if len(rows) > c.KeepMonths {
			// The current plan is finished when the LAST debt is, not the first.
			c.KeepMonths = len(rows)
		}
	}

	fee := int64(math.Round(float64(total) * feePct / 100))
	borrowed := total + fee
	newRows := payoff.AmortizeFixed(borrowed, newAPRPct, newTermMonths, start)
	if len(newRows) == 0 {
		return c, false
	}
	newInterest, newPaid, _ := payoff.AmortSummary(newRows)
	c.FeeMinor = fee
	c.NewMonths = len(newRows)
	c.NewInterestMinor = newInterest
	c.NewTotalMinor = newPaid
	c.NewPaymentMinor = newRows[0].PaymentMinor
	c.InterestDeltaMinor = newInterest - c.KeepInterestMinor

	var currentPayment int64
	for _, d := range usable {
		currentPayment += d.MinPaymentMinor
	}
	c.PaymentDeltaMinor = c.NewPaymentMinor - currentPayment
	return c, true
}

// WeightedAPRPct is the blended rate across debts, weighted by balance.
//
// The number a consolidation offer has to beat, and the one people compare
// against the wrong thing: against their WORST rate, which almost any offer
// beats, rather than against what they are actually paying overall.
//
// Reports ok=false with no balances, because an average of nothing is not zero.
func WeightedAPRPct(debts []Debt) (float64, bool) {
	var total int64
	var weighted float64
	for _, d := range debts {
		if d.BalanceMinor <= 0 {
			continue
		}
		total += d.BalanceMinor
		weighted += float64(d.BalanceMinor) * d.APRPct
	}
	if total == 0 {
		return 0, false
	}
	return weighted / float64(total), true
}
