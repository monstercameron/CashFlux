// SPDX-License-Identifier: MIT

package smartengine

import (
	"sort"
	"strings"
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
	register("SMART-BL1", bl1PredictVariable)
	register("SMART-BL4", bl4Autopay)
	register("SMART-BL5", bl5OptimalPayDate)
	register("SMART-BL8", bl8PaycheckGrouping)
	register("SMART-BL15", bl15GracePeriod)
	register("SMART-BL7", bl7BillIncrease)
	register("SMART-BL9", bl9SinkingFund)
	register("SMART-BL13", bl13StatementClarity)
}

const (
	bl1MinCharges = 3     // need this many charges to predict
	bl1MinAmount  = 20_00 // ignore trivial billers
)

// SMART-BL1 — Predicted amount for variable bills. For a biller whose charges
// vary month to month (utilities, statements), predicts the next amount from the
// recent average so the upcoming list shows a real estimate, not the minimum.
func bl1PredictVariable(in Input) []smart.Insight {
	type charge struct {
		date   time.Time
		amount int64 // base minor, magnitude
	}
	byMerchant := map[string][]charge{}
	labels := map[string]string{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		label := txnLabel(t)
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" {
			continue
		}
		byMerchant[key] = append(byMerchant[key], charge{date: t.Date, amount: abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency))})
		labels[key] = label
	}
	var out []smart.Insight
	for key, charges := range byMerchant {
		if len(charges) < bl1MinCharges {
			continue
		}
		sort.Slice(charges, func(i, j int) bool { return charges[i].date.Before(charges[j].date) })
		// Variable means the amounts aren't all identical.
		min, mx := charges[0].amount, charges[0].amount
		for _, c := range charges {
			if c.amount < min {
				min = c.amount
			}
			if c.amount > mx {
				mx = c.amount
			}
		}
		if min == mx {
			continue // fixed bill — the minimum/recurring amount already covers it
		}
		// Predict from the last 3 charges.
		last3 := charges
		if len(last3) > 3 {
			last3 = last3[len(last3)-3:]
		}
		var sum int64
		for _, c := range last3 {
			sum += c.amount
		}
		pred := sum / int64(len(last3))
		if pred < bl1MinAmount {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-BL1",
			Page:    smart.PageBills,
			Key:     "SMART-BL1:" + key,
			Title:   labels[key] + ": about " + in.baseMoney(pred).Format(2) + " expected",
			Detail: labels[key] + " varies; its last " + plural(int64(len(last3)), "charge") + " averaged " +
				in.baseMoney(pred).Format(2) + " (range " + in.baseMoney(min).Format(2) + "–" + in.baseMoney(mx).Format(2) + ").",
			Severity: smart.SeverityInfo,
		}.WithAmount(in.baseMoney(pred)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills", Route: "/bills"}))
	}
	return out
}

// SMART-BL5 — Optimal pay-date suggestion. When bills cluster before the next
// paycheck, suggests timing flexible payments to just after payday to smooth the
// month's cash flow.
func bl5OptimalPayDate(in Input) []smart.Insight {
	payday, ok := recentPayday(in)
	if !ok {
		return nil
	}
	nextPay := dateutil.NextMonthlyDue(in.Now, payday)
	var beforeN int
	var beforeTotal int64
	for _, b := range bills.UpcomingAll(in.Accounts, in.Recurring, in.Now) {
		if b.DueDate.Before(in.Now) || !b.DueDate.Before(nextPay) {
			continue
		}
		beforeN++
		beforeTotal += in.toBaseMinor(b.Amount.Amount, b.Amount.Currency)
	}
	if beforeN < 2 { // only worth smoothing when several cluster pre-payday
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-BL5",
		Page:    smart.PageBills,
		Key:     "SMART-BL5:" + nextPay.Format("2006-01-02"),
		Title:   "Time flexible payments to just after payday",
		Detail: plural(int64(beforeN), "bill") + " (about " + in.baseMoney(beforeTotal).Format(2) +
			") land before your next paycheck around " + nextPay.Format("Jan 2") +
			". Where a biller allows it, shifting payment to just after payday smooths the month.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(beforeTotal)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills", Route: "/bills"})
	return []smart.Insight{ins}
}

// SMART-BL15 — Grace-period & due-date confidence. Learns a liability's real
// payment timing from history and shows the typical days-after-due it's actually
// paid, so the effective last-safe-pay date is clear.
func bl15GracePeriod(in Input) []smart.Insight {
	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.DueDayOfMonth <= 0 {
			continue
		}
		// Average the observed offset between each recent due date and the nearest
		// payment that follows it within the cycle.
		var sumDays, n int
		for k := 1; k <= 4; k++ {
			due := dateutil.AddMonths(bills.NextDue(a.DueDayOfMonth, in.Now), -k)
			if pd, ok := firstPaymentOnOrAfter(in.Transactions, a.ID, due, due.AddDate(0, 0, 25)); ok {
				sumDays += dateutil.DaysBetween(due, pd)
				n++
			}
		}
		if n < 2 {
			continue // not enough history to characterize the pattern
		}
		avg := sumDays / n
		ins := smart.Insight{
			Feature: "SMART-BL15",
			Page:    smart.PageBills,
			Key:     "SMART-BL15:" + a.ID,
			Title:   a.Name + " is typically paid " + plural(int64(avg), "day") + " after the due date",
			Detail: "Across recent cycles " + a.Name + " was paid about " + plural(int64(avg), "day") +
				" after the " + ordinalDay(a.DueDayOfMonth) + " — your effective last-safe-pay date, not just the nominal due date.",
			Severity: smart.SeverityInfo,
		}
		out = append(out, ins.WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills",
			Route: "/bills", RelatedType: "account", RelatedID: a.ID}))
	}
	return out
}

// firstPaymentOnOrAfter returns the date of the first transaction touching the
// account within [from, to], and whether one was found.
func firstPaymentOnOrAfter(txns []domain.Transaction, accountID string, from, to time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	for _, t := range txns {
		if t.AccountID != accountID && t.TransferAccountID != accountID {
			continue
		}
		if t.Date.Before(from) || t.Date.After(to) {
			continue
		}
		if !found || t.Date.Before(best) {
			best, found = t.Date, true
		}
	}
	return best, found
}

// ordinalDay renders a day-of-month with its ordinal suffix ("5th", "1st").
func ordinalDay(d int) string {
	suffix := "th"
	switch {
	case d%100 >= 11 && d%100 <= 13:
		suffix = "th"
	case d%10 == 1:
		suffix = "st"
	case d%10 == 2:
		suffix = "nd"
	case d%10 == 3:
		suffix = "rd"
	}
	return itoa64(int64(d)) + suffix
}

// SMART-BL8 — Paycheck-aligned grouping. Detects the user's payday from recent
// income and flags how many upcoming bills fall before the next paycheck, so a
// cash crunch between paychecks is visible.
func bl8PaycheckGrouping(in Input) []smart.Insight {
	payday, ok := recentPayday(in)
	if !ok {
		return nil
	}
	nextPay := dateutil.NextMonthlyDue(in.Now, payday)
	var n int
	var total int64
	for _, b := range bills.UpcomingAll(in.Accounts, in.Recurring, in.Now) {
		if b.DueDate.Before(in.Now) || !b.DueDate.Before(nextPay) {
			continue
		}
		n++
		total += in.toBaseMinor(b.Amount.Amount, b.Amount.Currency)
	}
	if n == 0 {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-BL8",
		Page:    smart.PageBills,
		Key:     "SMART-BL8:" + nextPay.Format("2006-01-02"),
		Title:   plural(int64(n), "bill") + " due before your next paycheck",
		Detail: plural(int64(n), "bill") + " totaling about " + in.baseMoney(total).Format(2) +
			" fall before your next paycheck around " + nextPay.Format("Jan 2") + " — make sure they're covered.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(total)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open bills", Route: "/bills"})
	return []smart.Insight{ins}
}

// recentPayday infers the user's payday (day of month) from the most recent
// income transaction in the trailing 90 days.
func recentPayday(in Input) (int, bool) {
	cut := in.Now.AddDate(0, 0, -90)
	var last time.Time
	day := 0
	for _, t := range in.Transactions {
		if !t.IsIncome() || t.Date.Before(cut) || t.Date.After(in.Now) {
			continue
		}
		if day == 0 || t.Date.After(last) {
			last, day = t.Date, t.Date.Day()
		}
	}
	return day, day > 0
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
