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
	register("SMART-BL7", bl7BillIncrease)
	register("SMART-BL9", bl9SinkingFund)
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
