// SPDX-License-Identifier: MIT

// Package credithealth computes a deterministic, local "credit health" proxy
// from a household's own transaction and account data. It is explicitly NOT a
// FICO score or credit bureau report — see Disclaimer.
//
// The proxy is built from three signals:
//
//  1. Credit utilization — per-card and aggregate revolving balance vs. limit.
//  2. On-time payment proxy — did a payment arrive near each card's due date in
//     the past three calendar months?
//  3. Account-age proxy — months from BalanceAsOf to Now for each credit card.
//
// Weights are re-normalized over only the factors that are computable from the
// available data (a card with no DueDayOfMonth contributes zero on-time data;
// a card with no CreditLimit contributes zero to utilization).
//
// Pure Go, no syscall/js, no I/O. Unit-testable on native Go.
package credithealth

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// Disclaimer must be surfaced in every user-facing credit-health presentation.
const Disclaimer = "This is a local estimate based on your data — not a FICO score or credit bureau report."

// UtilBand is the qualitative tier for a credit-utilization percentage.
//
// Thresholds (inlined from the R22 design):
//
//	≤10 % → BandUtilBest
//	≤30 % → BandUtilGood
//	≤50 % → BandUtilFair
//	≤80 % → BandUtilPoor
//	>80 % → BandUtilWorst
//
// TODO: dedupe with healthscore when a shared thresholds package is added.
type UtilBand string

const (
	BandUtilBest   UtilBand = "Best"   // 0–10 %
	BandUtilGood   UtilBand = "Good"   // 11–30 %
	BandUtilFair   UtilBand = "Fair"   // 31–50 %
	BandUtilPoor   UtilBand = "Poor"   // 51–80 %
	BandUtilWorst  UtilBand = "Worst"  // >80 %
	BandUtilNoData UtilBand = "No data" // no credit limit set
)

// utilBandFor maps a utilization percentage to its UtilBand.
// pct == -1 (no limit) → BandUtilNoData.
func utilBandFor(pct int) UtilBand {
	switch {
	case pct < 0:
		return BandUtilNoData
	case pct <= 10:
		return BandUtilBest
	case pct <= 30:
		return BandUtilGood
	case pct <= 50:
		return BandUtilFair
	case pct <= 80:
		return BandUtilPoor
	default:
		return BandUtilWorst
	}
}

// utilScore converts a utilization percentage (0–100) into a 0–100 factor score.
// pct == -1 means no limit → 0 (excluded from weighting via separate logic).
//
// Scoring curve (matches R22 / healthscore utilizationScore logic):
//
//	≤10 % → 100
//	10–30 % → linear 100→70
//	30–80 % → linear 70→0
//	>80 % → 0
//
// TODO: dedupe with healthscore when a shared thresholds package is added.
func utilScore(pct int) int {
	if pct < 0 {
		return 0
	}
	p := pct
	switch {
	case p <= 10:
		return 100
	case p < 30:
		// 100 at 10, 70 at 30: slope = -1.5/pct
		return clamp(100 - (p-10)*3/2)
	case p < 80:
		// 70 at 30, 0 at 80: slope ≈ -1.4/pct (integer approx)
		return clamp(70 - (p-30)*70/50)
	default:
		return 0
	}
}

// Band is the qualitative tier for the overall proxy score.
type Band string

const (
	BandExcellent Band = "Excellent" // proxy ≥ 75
	BandGood      Band = "Good"      // proxy ≥ 55
	BandFair      Band = "Fair"      // proxy ≥ 35
	BandPoor      Band = "Poor"      // proxy < 35
)

// bandFor maps a 0–100 proxy score to its Band.
func bandFor(score int) Band {
	switch {
	case score >= 75:
		return BandExcellent
	case score >= 55:
		return BandGood
	case score >= 35:
		return BandFair
	default:
		return BandPoor
	}
}

// CardUtil holds per-card utilization details.
type CardUtil struct {
	// AccountID and Name identify the card.
	AccountID string
	Name      string

	// BalanceMinor is the card's current balance in minor units (e.g. cents).
	// For a credit card this is typically negative (money owed); the raw value
	// from Inputs.Balances is stored unchanged.
	BalanceMinor int64

	// LimitMinor is CreditLimit.Amount from the Account; 0 if unset.
	LimitMinor int64

	// UtilPct is the utilization percentage (0–100+), or -1 if LimitMinor == 0.
	UtilPct int

	// Target30Minor is the amount to pay to bring utilization to ≤30 %:
	//   max(0, |balance| − limit*30/100)
	// Zero when already at or under 30 %, or when no limit is set.
	Target30Minor int64

	// Target10Minor is the amount to pay to bring utilization to ≤10 %:
	//   max(0, |balance| − limit*10/100)
	// Zero when already at or under 10 %, or when no limit is set.
	Target10Minor int64

	// Band is the utilization tier for this card.
	Band UtilBand
}

// AggUtil holds household-level (aggregate) utilization across all cards with a limit.
type AggUtil struct {
	TotalBalanceMinor int64
	TotalLimitMinor   int64

	// UtilPct is aggregate utilization, or -1 if no cards have a limit.
	UtilPct int

	// Band is the aggregate utilization tier.
	Band UtilBand

	// CardsMissingLimit is the count of credit-card accounts with no CreditLimit set.
	CardsMissingLimit int
}

// Inputs are the data required to evaluate credit health.
type Inputs struct {
	// Accounts is the full account list; only TypeCreditCard accounts are used.
	Accounts []domain.Account

	// Balances maps accountID → current balance in minor units. A negative value
	// means money is owed (typical for credit cards). If nil or a card is absent,
	// the balance is treated as zero.
	Balances map[string]int64

	// Transactions are all household transactions, used for the on-time payment proxy.
	Transactions []domain.Transaction

	// Now is the reference time for "past 3 months" and age calculations.
	Now time.Time
}

// Result is the full credit-health proxy output.
type Result struct {
	// Cards holds per-card utilization details (credit cards only).
	Cards []CardUtil

	// Agg is the aggregate utilization across all cards with a limit.
	Agg AggUtil

	// OnTimeScore is the on-time payment proxy, 0–100, or -1 when no credit card
	// has DueDayOfMonth > 0 (not enough data to compute).
	OnTimeScore int

	// AgeScore is the account-age proxy, 0–100, or -1 when no card has a
	// non-zero BalanceAsOf (not enough data to compute).
	AgeScore int

	// ProxyScore is the weighted composite, 0–100. Weights are re-normalized over
	// only the factors with available data (util, on-time, age).
	// Weights before re-normalization: util=0.55, on-time=0.30, age=0.15.
	ProxyScore int

	// Band is the qualitative tier for ProxyScore.
	Band Band

	// Disclaimer must be displayed alongside any user-facing credit-health presentation.
	Disclaimer string
}

// Evaluate runs the deterministic credit-health model and returns a Result.
// It reads only credit-card accounts (domain.TypeCreditCard).
func Evaluate(in Inputs) Result {
	// Gather credit card accounts.
	var cards []domain.Account
	for _, a := range in.Accounts {
		if a.Type == domain.TypeCreditCard && !a.Archived {
			cards = append(cards, a)
		}
	}

	// --- Utilization ---
	cardUtils, agg := computeUtilization(cards, in.Balances)

	// --- On-time payment proxy ---
	onTimeScore := computeOnTime(cards, in.Transactions, in.Now)

	// --- Account-age proxy ---
	ageScore := computeAge(cards, in.Now)

	// --- Proxy score (weighted, re-normalized) ---
	proxyScore := computeProxy(agg.UtilPct, onTimeScore, ageScore)

	return Result{
		Cards:       cardUtils,
		Agg:         agg,
		OnTimeScore: onTimeScore,
		AgeScore:    ageScore,
		ProxyScore:  proxyScore,
		Band:        bandFor(proxyScore),
		Disclaimer:  Disclaimer,
	}
}

// computeUtilization derives per-card and aggregate utilization figures.
func computeUtilization(cards []domain.Account, balances map[string]int64) ([]CardUtil, AggUtil) {
	var (
		totalBal   int64
		totalLimit int64
		missing    int
	)
	out := make([]CardUtil, 0, len(cards))

	for _, a := range cards {
		bal := int64(0)
		if balances != nil {
			bal = balances[a.ID]
		}
		limit := a.CreditLimit.Amount

		cu := CardUtil{
			AccountID:    a.ID,
			Name:         a.Name,
			BalanceMinor: bal,
			LimitMinor:   limit,
		}

		pct, ok := ledger.Utilization(bal, limit)
		if !ok {
			cu.UtilPct = -1
			cu.Band = BandUtilNoData
			missing++
		} else {
			cu.UtilPct = pct
			cu.Band = utilBandFor(pct)

			// Target amounts: how much to pay to reach 30 % / 10 %.
			owed := bal
			if owed < 0 {
				owed = -owed
			}
			if limit > 0 {
				thresh30 := limit * 30 / 100
				if owed > thresh30 {
					cu.Target30Minor = owed - thresh30
				}
				thresh10 := limit * 10 / 100
				if owed > thresh10 {
					cu.Target10Minor = owed - thresh10
				}
			}

			totalBal += owed
			totalLimit += limit
		}

		out = append(out, cu)
	}

	agg := AggUtil{
		TotalBalanceMinor: totalBal,
		TotalLimitMinor:   totalLimit,
		CardsMissingLimit: missing,
	}
	aggPct, ok := ledger.Utilization(totalBal, totalLimit)
	if !ok {
		agg.UtilPct = -1
		agg.Band = BandUtilNoData
	} else {
		agg.UtilPct = aggPct
		agg.Band = utilBandFor(aggPct)
	}

	return out, agg
}

// computeOnTime returns an on-time payment proxy score 0–100, or -1.
//
// A "payment" is any negative-amount (money-out) transaction on a credit card
// account that falls within ±5 days of the due date in a given calendar month.
// For each of the past 3 complete calendar months (relative to in.Now) we check
// whether at least one qualifying payment exists. The score is (hits/3)*100.
//
// Returns -1 when no credit card has DueDayOfMonth > 0 (no due-date data).
func computeOnTime(cards []domain.Account, txns []domain.Transaction, now time.Time) int {
	// Collect card IDs and their due days.
	type cardDue struct {
		id  string
		due int
	}
	var eligible []cardDue
	for _, a := range cards {
		if a.DueDayOfMonth > 0 {
			eligible = append(eligible, cardDue{a.ID, a.DueDayOfMonth})
		}
	}
	if len(eligible) == 0 {
		return -1
	}

	// Build a set of (accountID, year, month) → true for months that had a
	// qualifying payment.
	type key struct {
		id    string
		year  int
		month time.Month
	}
	paid := map[key]bool{}

	for _, t := range txns {
		// A payment on a credit card is money going out (negative amount).
		if t.Amount.Amount >= 0 {
			continue
		}
		for _, cd := range eligible {
			if t.AccountID != cd.id {
				continue
			}
			// Is the transaction within ±5 days of the due date for its month?
			y, mo, d := t.Date.Year(), t.Date.Month(), t.Date.Day()
			due := cd.due
			diff := d - due
			if diff < 0 {
				diff = -diff
			}
			if diff <= 5 {
				paid[key{cd.id, y, mo}] = true
			}
		}
	}

	// Evaluate the past 3 complete calendar months.
	// "Complete" means months strictly before the current month of Now.
	hits := 0
	for i := 1; i <= 3; i++ {
		// Go back i months from the first day of Now's month.
		ref := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).
			AddDate(0, -i, 0)
		y, mo := ref.Year(), ref.Month()

		// The month counts as on-time if at least one eligible card had a payment.
		for _, cd := range eligible {
			if paid[key{cd.id, y, mo}] {
				hits++
				break
			}
		}
	}

	return clamp(hits * 100 / 3)
}

// computeAge returns an account-age proxy score 0–100, or -1.
//
// The age is derived from BalanceAsOf → Now (months). Score is capped at 100 at
// 84 months (7 years); linear below that. Returns -1 when no card has a non-zero
// BalanceAsOf.
func computeAge(cards []domain.Account, now time.Time) int {
	var totalMonths int
	count := 0
	for _, a := range cards {
		if a.BalanceAsOf.IsZero() {
			continue
		}
		months := monthsBetween(a.BalanceAsOf, now)
		if months < 0 {
			months = 0
		}
		totalMonths += months
		count++
	}
	if count == 0 {
		return -1
	}
	avg := totalMonths / count
	// 0 months → 0, 84 months → 100 (linear, capped).
	score := avg * 100 / 84
	return clamp(score)
}

// monthsBetween returns the whole number of months from from to to (truncated).
func monthsBetween(from, to time.Time) int {
	years := to.Year() - from.Year()
	months := int(to.Month()) - int(from.Month())
	total := years*12 + months
	// Adjust for partial month at the end.
	if to.Day() < from.Day() {
		total--
	}
	return total
}

// computeProxy computes the weighted proxy score from available signals.
//
// Base weights: utilization=0.55, on-time=0.30, age=0.15.
// A signal with score == -1 is excluded; remaining weights are re-normalized to
// sum to 1 before weighting. Returns 0 when no signal is available.
func computeProxy(utilPct, onTime, ageScore int) int {
	type factor struct {
		score  int
		weight float64
	}

	factors := []factor{
		{utilScore(utilPct), 0.55},
		{onTime, 0.30},
		{ageScore, 0.15},
	}

	var totalWeight float64
	var weighted float64
	for _, f := range factors {
		if f.score < 0 {
			continue
		}
		totalWeight += f.weight
		weighted += float64(f.score) * f.weight
	}
	if totalWeight == 0 {
		return 0
	}
	return clamp(int(weighted / totalWeight))
}

// clamp restricts v to [0, 100].
func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
