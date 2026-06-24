// SPDX-License-Identifier: MIT

package smartengine

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func init() {
	register("SMART-BL2", bl2CanCover)
	register("SMART-BL3", bl3MissedBill)
	register("SMART-BL6", bl6LateFeeRisk)
	register("SMART-BL4", bl4Autopay)
	register("SMART-BL7", bl7BillIncrease)
	register("SMART-BL9", bl9SinkingFund)
	register("SMART-BL13", bl13StatementClarity)
}

// SMART-BL4 — Autopay reconciliation. When a liability's most recent due date has
// a matching payment near it, notes that it looks auto-paid (so the user isn't
// nagged to pay something already handled).
func bl4Autopay(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		prevDue := dateutil.AddMonths(bills.NextDue(a.DueDayOfMonth, in.Now), -1)
		days := dateutil.DaysBetween(prevDue, in.Now)
		if days < 0 || days > missedWindowDays {
			continue
		}
		// A payment within a few days either side of the due date looks like autopay.
		from := prevDue.AddDate(0, 0, -3)
		to := prevDue.AddDate(0, 0, 5)
		if !paymentInWindow(in.Transactions, a.ID, from, to) {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL4",
			Page:    smart.PageBills,
			Key:     "SMART-BL4:" + a.ID + ":" + prevDue.Format("2006-01"),
			Title:   a.Name + " looks auto-paid",
			Detail: "A payment posted around the " + prevDue.Format("Jan 2") + " due date, so " + a.Name +
				" appears to be on autopay — no action needed.",
			Severity: smart.SeverityInfo,
		}.WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills",
			Route: "/bills", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// SMART-BL13 — Statement-vs-minimum clarity. For a revolving liability, shows the
// full balance, the minimum payment, and the monthly interest cost of paying only
// the minimum, so the cheaper choice is obvious.
func bl13StatementClarity(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.InterestRateAPR <= 0 {
			continue
		}
		minPay := abs64(in.toBaseMinor(a.MinPayment.Amount, a.Currency))
		if minPay == 0 {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		owed := abs64(in.toBaseMinor(bal.Amount, a.Currency))
		if owed <= minPay { // not revolving — the minimum clears it
			continue
		}
		monthlyInterest := pctOf(owed, a.InterestRateAPR) / 12
		out = append(out, smart.Insight{
			Feature: "SMART-BL13",
			Page:    smart.PageBills,
			Key:     "SMART-BL13:" + a.ID + ":" + in.Now.Format("2006-01"),
			Title:   a.Name + ": paying only the minimum costs you",
			Detail: a.Name + " owes " + in.baseMoney(owed).Format(2) + " at " + fmtPct(a.InterestRateAPR) +
				" APR. Paying just the " + in.baseMoney(minPay).Format(2) + " minimum leaves roughly " +
				in.baseMoney(monthlyInterest).Format(2) + "/mo in interest — paying more saves it.",
			Severity: smart.SeverityNudge,
		}.WithAmount(in.baseMoney(monthlyInterest)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills",
				Route: "/bills", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

const (
	lateFeeWindowDays = 5    // warn when a liability bill is due within this many days
	lateFeeMinAPR     = 1.0  // skip interest-cost math below this APR
	lateFeeMinCost    = 1_00 // only surface when a week's delay costs at least $1
)

// SMART-BL6 — Late-fee / interest risk. For a liability bill due very soon,
// estimates the interest cost of paying a week late so the trade-off is visible.
func bl6LateFeeRisk(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.DueDayOfMonth <= 0 {
			continue
		}
		if a.InterestRateAPR < lateFeeMinAPR {
			continue
		}
		due := bills.NextDue(a.DueDayOfMonth, in.Now)
		days := dateutil.DaysBetween(in.Now, due)
		if days < 0 || days > lateFeeWindowDays {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		owed := abs64(in.toBaseMinor(bal.Amount, a.Currency))
		// A week of interest at the card's APR ≈ balance × APR/52.
		weekInterest := pctOf(owed, a.InterestRateAPR) / 52
		if weekInterest < lateFeeMinCost {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL6",
			Page:    smart.PageBills,
			Key:     "SMART-BL6:" + a.ID + ":" + due.Format("2006-01"),
			Title:   a.Name + " is due " + due.Format("Jan 2") + " — paying late adds up",
			Detail: "At " + fmtPct(a.InterestRateAPR) + " APR, slipping a week past the " +
				due.Format("Jan 2") + " due date costs roughly " + in.baseMoney(weekInterest).Format(2) +
				" in interest (plus any late fee).",
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(weekInterest)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills",
				Route: "/bills", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

const (
	billsHorizon      = 45     // days to look ahead for bill coverage
	missedGraceDays   = 3      // days past due before calling a bill possibly missed
	missedWindowDays  = 45     // ignore due dates older than this (stale)
	priceMinCount     = 3      // occurrences before a price change is trusted
	priceMinIncrease  = 5      // ignore < 5% wobble as noise
	sinkingMinAnnual  = 200_00 // only nudge sinking funds for bills ≥ $200/yr
	priceJumpMinDelta = 1_00   // ignore < $1 absolute change
)

// SMART-BL2 — Can-you-cover-it check. Projects total liquid cash over the
// household's recurring flows and warns when it would dip below zero before the
// next inflow, naming the soonest upcoming bill at risk.
func bl2CanCover(in Input) []smart.Insight {
	liquid := totalLiquidBase(in)
	proj, err := runway.Project(liquid, in.Recurring, in.Now, billsHorizon, 0, in.Rates)
	if err != nil || !proj.WillBreach() {
		return nil
	}
	when := in.Now.AddDate(0, 0, proj.BreachDay)
	detail := "Projecting your recurring bills against liquid cash, the balance dips to about " +
		in.baseMoney(proj.MinBalance).Format(2) + " around " + when.Format("Jan 2") + "."
	if up := bills.UpcomingAll(in.Accounts, in.Recurring, in.Now); len(up) > 0 {
		soon := up[0]
		detail += " " + soon.Name + " (" + soon.Amount.Format(2) + ") is due " + soon.DueDate.Format("Jan 2") + "."
	}
	ins := smart.Insight{
		Feature:  "SMART-BL2",
		Page:     smart.PageBills,
		Key:      "SMART-BL2:" + when.Format("2006-01-02"),
		Title:    "Upcoming bills may not be covered",
		Detail:   detail,
		Severity: smart.SeverityAlert,
	}.WithAmount(in.baseMoney(proj.BreachShortfall)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

// SMART-BL3 — Missed / overdue bill detection. For each liability with a
// statement due-day, checks whether the most recent due date passed with no
// payment recorded on that account.
func bl3MissedBill(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		if a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		prevDue := dateutil.AddMonths(bills.NextDue(a.DueDayOfMonth, in.Now), -1)
		days := dateutil.DaysBetween(prevDue, in.Now)
		if days < missedGraceDays || days > missedWindowDays {
			continue // not yet overdue, or too old to be actionable
		}
		if paymentInWindow(in.Transactions, a.ID, prevDue, in.Now) {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL3",
			Page:    smart.PageBills,
			Key:     "SMART-BL3:" + a.ID + ":" + prevDue.Format("2006-01"),
			Title:   a.Name + " may have been missed",
			Detail: "The " + a.Name + " payment was due " + prevDue.Format("Jan 2") +
				" but no payment is recorded since. Check whether it was paid.",
			Severity: smart.SeverityAlert,
		}.WithAmount(a.MinPayment.Abs()).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills",
				Route: "/bills", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// SMART-BL7 — Bill increase detection. Reuses the recurring price-change detector
// to flag bills whose amount has gone up.
func bl7BillIncrease(in Input) []smart.Insight {
	changes, err := subscriptions.DetectPriceChanges(in.Transactions, in.Rates, priceMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, c := range changes {
		if !c.Increased() || c.PercentChange < priceMinIncrease || c.Delta < priceJumpMinDelta {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL7",
			Page:    smart.PageBills,
			Key:     "SMART-BL7:" + c.Name + ":" + c.ChangedAt.Format("2006-01"),
			Title:   c.Name + " went up " + itoa64(int64(c.PercentChange)) + "%",
			Detail: c.Name + " rose from " + in.baseMoney(c.OldAmount).Format(2) + " to " +
				in.baseMoney(c.NewAmount).Format(2) + " as of " + c.ChangedAt.Format("Jan 2") + ".",
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(c.Delta)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-BL9 — Annual-bill sinking-fund nudge. For large irregular recurring bills
// (yearly/quarterly), suggests a monthly set-aside ahead of the due date.
func bl9SinkingFund(in Input) []smart.Insight {
	var out []smart.Insight
	for _, r := range in.Recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		if r.Cadence != domain.CadenceYearly && r.Cadence != domain.CadenceQuarterly {
			continue
		}
		annual := abs64(in.toBaseMinor(annualMinor(r), r.Amount.Currency))
		if annual < sinkingMinAnnual {
			continue
		}
		monthly := abs64(in.toBaseMinor(r.MonthlyEquivalent(), r.Amount.Currency))
		due := r.NextDue
		if due.Before(in.Now) {
			due = r.Cadence.Next(due)
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL9",
			Page:    smart.PageBills,
			Key:     "SMART-BL9:" + r.ID,
			Title:   "Set aside for " + r.Label,
			Detail: r.Label + " is about " + in.baseMoney(abs64(annual)).Format(2) + "/yr, due " +
				due.Format("Jan 2") + ". Putting aside ~" + in.baseMoney(monthly).Format(2) +
				"/mo now avoids the lump-sum shock.",
			Severity: smart.SeverityNudge,
		}.WithAmount(in.baseMoney(monthly)).
			WithAction(smart.Action{Kind: smart.ActionCreateTask, Label: "Add a to-do",
				TaskTitle: "Set aside " + in.baseMoney(monthly).Format(2) + "/mo for " + r.Label,
				TaskNotes: "Sinking fund for " + r.Label + " (~" + in.baseMoney(abs64(annual)).Format(2) + "/yr, due " + due.Format("Jan 2") + ")."}))
	}
	return out
}

// --- bills-engine helpers -------------------------------------------------

// totalLiquidBase sums the current balance of liquid asset accounts, in base
// minor units.
func totalLiquidBase(in Input) int64 {
	var total int64
	for _, a := range activeAssetAccounts(in.Accounts) {
		if !liquidTypes[a.Type] {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		total += in.toBaseMinor(bal.Amount, a.Currency)
	}
	return total
}

// paymentInWindow reports whether any transaction touches the account (directly
// or as a transfer counterpart) within [start, end].
func paymentInWindow(txns []domain.Transaction, accountID string, start, end time.Time) bool {
	for _, t := range txns {
		if t.AccountID != accountID && t.TransferAccountID != accountID {
			continue
		}
		if t.Date.Before(start) || t.Date.After(end) {
			continue
		}
		return true
	}
	return false
}

// annualMinor returns the cadence-annualized magnitude of a recurring amount, in
// its own currency minor units (sign preserved as the raw amount's sign).
func annualMinor(r domain.Recurring) int64 {
	a := r.Amount.Amount
	switch r.Cadence {
	case domain.CadenceWeekly:
		return a * 52
	case domain.CadenceQuarterly:
		return a * 4
	case domain.CadenceYearly:
		return a
	default: // monthly
		return a * 12
	}
}
