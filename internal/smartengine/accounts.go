// SPDX-License-Identifier: MIT

package smartengine

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func init() {
	register("SMART-A1", a1BalanceAnomaly)
	register("SMART-A2", a2DormantAccount)
	register("SMART-A4", a4CashPositioning)
	register("SMART-A7", a7RecurringCharges)
	register("SMART-A8", a8OverdraftForecast)
}

// Tunables for the account engines. Kept as named constants so the thresholds
// are visible and a test can reason about them. Money figures are base-currency
// minor units.
const (
	dormantMonths      = 6        // no activity for this long → dormant
	dormantMinBalance  = 50_00    // ignore near-empty accounts ($50)
	idleBenchmarkAPR   = 4.0      // a typical high-yield savings rate, for idle-cost math
	idleLowAPR         = 1.0      // below this, cash is "idle" (earning ~nothing)
	cashPositionMinGap = 1.0      // min APR gap (pp) to suggest moving cash
	cashPositionMinBal = 1_000_00 // only nudge on a meaningful balance ($1,000)
	cashPositionMinYr  = 10_00    // only nudge if the yearly gain clears $10
	anomalyFactor      = 3        // current month this many× the trailing mean → anomaly
	anomalyMinMean     = 100_00   // ignore tiny baselines ($100/mo)
	anomalyLookback    = 3        // trailing months compared against the current month
	overdraftHorizon   = 60       // days to project an account forward
	recurringMinCount  = 3        // occurrences before a charge counts as recurring
	recurringMinItems  = 2        // only summarize accounts with at least this many
)

// liquidTypes are the asset account types treated as readily spendable cash for
// the cash-positioning engine.
var liquidTypes = map[domain.AccountType]bool{
	domain.TypeChecking: true,
	domain.TypeSavings:  true,
	domain.TypeCash:     true,
	domain.TypeDebit:    true,
}

// SMART-A2 — Dormant account nudge. Flags non-archived asset accounts with no
// activity for dormantMonths and a non-trivial balance, estimating the yearly
// cost of leaving low-yield cash parked.
func a2DormantAccount(in Input) []smart.Insight {
	var out []smart.Insight
	cutoff := dateutil.AddMonths(in.Now, -dormantMonths)
	for _, a := range activeAssetAccounts(in.Accounts) {
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil || !bal.IsPositive() {
			continue
		}
		baseBal := in.toBaseMinor(bal.Amount, a.Currency)
		if baseBal < dormantMinBalance {
			continue
		}
		last, found := lastActivity(in.Transactions, a.ID)
		// An account whose last activity is recent (or whose opening date is
		// recent, when it has no transactions) is not dormant.
		ref := a.BalanceAsOf
		if found {
			ref = last
		}
		if ref.IsZero() || ref.After(cutoff) {
			continue
		}
		months := monthsBetween(ref, in.Now)
		ins := smart.Insight{
			Feature:  "SMART-A2",
			Page:     smart.PageAccounts,
			Key:      "SMART-A2:" + a.ID,
			Title:    a.Name + " has been quiet for " + plural(months, "month"),
			Severity: smart.SeverityNudge,
		}.WithAmount(bal)
		if a.ExpectedReturnAPR < idleLowAPR {
			idle := pctOf(baseBal, idleBenchmarkAPR-a.ExpectedReturnAPR)
			ins.Detail = "No activity in " + plural(months, "month") + ". At a typical " +
				fmtPct(idleBenchmarkAPR) + " savings rate this balance could earn about " +
				in.baseMoney(idle).Format(2) + "/yr — consider moving or consolidating it."
		} else {
			ins.Detail = "No activity in " + plural(months, "month") + " — review whether it's still needed."
		}
		ins = ins.WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review account",
			Route: "/accounts", RelatedType: "account", RelatedID: a.ID})
		out = append(out, ins)
	}
	return out
}

// SMART-A4 — Cash-positioning suggestions. Compares liquid asset accounts'
// stored APRs and suggests moving idle cash from a low-yield account to the
// best-yield one, quantifying the yearly gain.
func a4CashPositioning(in Input) []smart.Insight {
	// Find the best-yield liquid account.
	var best domain.Account
	bestAPR := -1.0
	for _, a := range activeAssetAccounts(in.Accounts) {
		if !liquidTypes[a.Type] {
			continue
		}
		if a.ExpectedReturnAPR > bestAPR {
			best, bestAPR = a, a.ExpectedReturnAPR
		}
	}
	if bestAPR <= 0 {
		return nil // no account carries a yield; nothing to optimize toward
	}
	var out []smart.Insight
	for _, a := range activeAssetAccounts(in.Accounts) {
		if a.ID == best.ID || !liquidTypes[a.Type] {
			continue
		}
		if bestAPR-a.ExpectedReturnAPR < cashPositionMinGap {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil || !bal.IsPositive() {
			continue
		}
		baseBal := in.toBaseMinor(bal.Amount, a.Currency)
		if baseBal < cashPositionMinBal {
			continue
		}
		gain := pctOf(baseBal, bestAPR-a.ExpectedReturnAPR)
		if gain < cashPositionMinYr {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-A4",
			Page:    smart.PageAccounts,
			Key:     "SMART-A4:" + a.ID,
			Title:   "Move idle cash from " + a.Name + " to earn more",
			Detail: bal.Format(2) + " sits at " + fmtPct(a.ExpectedReturnAPR) + " in " + a.Name +
				". Moving it to " + best.Name + " (" + fmtPct(bestAPR) + ") would earn about " +
				in.baseMoney(gain).Format(2) + "/yr.",
			Severity: smart.SeverityNudge,
		}.WithAmount(in.baseMoney(gain)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open accounts",
				Route: "/accounts", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// SMART-A1 — Balance anomaly watch. Flags an account whose current-month
// spending is unusually large versus its own trailing-months baseline.
func a1BalanceAnomaly(in Input) []smart.Insight {
	curStart := dateutil.MonthStart(in.Now)
	var out []smart.Insight
	for _, a := range activeAssetAccounts(in.Accounts) {
		txns := txnsForAccount(in.Transactions, a.ID)
		if len(txns) == 0 {
			continue
		}
		cur := in.toBaseMinor(monthExpenseMag(txns, curStart, in.Now.AddDate(0, 0, 1)), a.Currency)
		// Trailing baseline: mean of prior whole months' expense magnitude.
		var sum, n int64
		for k := 1; k <= anomalyLookback; k++ {
			s := dateutil.AddMonths(curStart, -k)
			e := dateutil.AddMonths(curStart, -k+1)
			mag := in.toBaseMinor(monthExpenseMag(txns, s, e), a.Currency)
			if mag > 0 {
				sum += mag
				n++
			}
		}
		if n < 2 {
			continue // not enough history to call something anomalous
		}
		mean := sum / n
		if mean < anomalyMinMean || cur < mean*anomalyFactor {
			continue
		}
		ratio := cur / mean
		out = append(out, smart.Insight{
			Feature: "SMART-A1",
			Page:    smart.PageAccounts,
			Key:     "SMART-A1:" + a.ID + ":" + curStart.Format("2006-01"),
			Title:   a.Name + " is spending faster than usual",
			Detail: "This month's spending of " + in.baseMoney(cur).Format(2) + " is about " +
				itoa64(ratio) + "× the recent monthly average (" + in.baseMoney(mean).Format(2) + ").",
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(cur)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "View transactions",
				Route: "/transactions", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// SMART-A7 — Recurring-charge detection per account. Summarizes the recurring
// debits hitting each account and their monthly burden.
func a7RecurringCharges(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		txns := txnsForAccount(in.Transactions, a.ID)
		if len(txns) == 0 {
			continue
		}
		subs, err := subscriptions.Detect(txns, in.Rates, recurringMinCount)
		if err != nil || len(subs) < recurringMinItems {
			continue
		}
		monthly := subscriptions.MonthlyTotal(subs)
		out = append(out, smart.Insight{
			Feature: "SMART-A7",
			Page:    smart.PageAccounts,
			Key:     "SMART-A7:" + a.ID,
			Title:   plural(int64(len(subs)), "recurring charge") + " on " + a.Name,
			Detail: "About " + in.baseMoney(monthly).Format(2) + "/mo in recurring debits run through " +
				a.Name + ". Open Subscriptions to review them.",
			Severity: smart.SeverityInfo,
		}.WithAmount(in.baseMoney(monthly)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions",
				Route: "/subscriptions", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// SMART-A8 — Low-balance / overdraft forecast. Projects each asset account
// forward over its recurring flows and warns before it dips below zero.
func a8OverdraftForecast(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range activeAssetAccounts(in.Accounts) {
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		start := in.toBaseMinor(bal.Amount, a.Currency)
		recs := recurringForAccount(in.Recurring, a.ID)
		if len(recs) == 0 {
			continue
		}
		proj, err := runway.Project(start, recs, in.Now, overdraftHorizon, 0, in.Rates)
		if err != nil || !proj.WillBreach() {
			continue
		}
		when := in.Now.AddDate(0, 0, proj.BreachDay)
		out = append(out, smart.Insight{
			Feature: "SMART-A8",
			Page:    smart.PageAccounts,
			Key:     "SMART-A8:" + a.ID,
			Title:   a.Name + " may dip below zero around " + when.Format("Jan 2"),
			Detail: "Projecting known recurring flows, " + a.Name + " reaches about " +
				in.baseMoney(proj.MinBalance).Format(2) + " on " + when.Format("Jan 2") +
				" — short by " + in.baseMoney(proj.BreachShortfall).Format(2) + ".",
			Severity: smart.SeverityAlert,
		}.WithAmount(in.baseMoney(proj.BreachShortfall)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning",
				Route: "/planning", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// --- account-engine helpers ----------------------------------------------

// recurringForAccount returns the recurring flows tied to an account.
func recurringForAccount(recs []domain.Recurring, accountID string) []domain.Recurring {
	var out []domain.Recurring
	for _, r := range recs {
		if r.AccountID == accountID {
			out = append(out, r)
		}
	}
	return out
}

// monthExpenseMag sums the magnitude of non-transfer expense amounts (money out)
// for transactions in [start, end), in their native currency. Callers convert.
func monthExpenseMag(txns []domain.Transaction, start, end time.Time) int64 {
	var mag int64
	for _, t := range txns {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		mag += -t.Amount.Amount
	}
	return mag
}

// monthsBetween returns whole months between a and b (b after a), floored at 0.
func monthsBetween(a, b time.Time) int64 {
	if b.Before(a) {
		return 0
	}
	m := int64(b.Year()-a.Year())*12 + int64(b.Month()-a.Month())
	if b.Day() < a.Day() {
		m--
	}
	if m < 0 {
		m = 0
	}
	return m
}

// pctOf returns minor × pct%, rounded to nearest minor unit (pct is a percentage
// like 4.5 meaning 4.5%).
func pctOf(minor int64, pct float64) int64 {
	return int64(float64(minor)*pct/100.0 + sign(float64(minor))*0.5)
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
