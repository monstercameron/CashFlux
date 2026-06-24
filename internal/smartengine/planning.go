// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func init() {
	register("SMART-P1", p1DiscoverRecurring)
	register("SMART-P4", p4Affordability)
	register("SMART-P6", p6ConfidenceBand)
	register("SMART-P8", p8ExtraDebt)
	register("SMART-P9", p9BreakEven)
	register("SMART-P10", p10BillShock)
}

const (
	confidenceMonths   = 6     // months of net history for the variance band
	confidenceMinSwing = 10_00 // ignore a trivially flat band
)

// SMART-P6 — Forecast confidence band. Measures how much monthly net swings
// (good months vs lean) so the user plans with a margin rather than a single line.
func p6ConfidenceBand(in Input) []smart.Insight {
	nets := monthlyNets(in, confidenceMonths)
	if len(nets) < 3 {
		return nil
	}
	lo, hi := nets[0], nets[0]
	for _, v := range nets {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	swing := (hi - lo) / 2
	if swing < confidenceMinSwing {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-P6",
		Page:    smart.PagePlanning,
		Key:     "SMART-P6:" + in.Now.Format("2006-01"),
		Title:   "Your monthly net swings about ±" + in.baseMoney(swing).Format(2),
		Detail: "Over the last " + plural(int64(len(nets)), "month") + " your monthly net ranged from " +
			in.baseMoney(lo).Format(2) + " to " + in.baseMoney(hi).Format(2) + ". Plan with that margin, not a single line.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(swing)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

// SMART-P9 — Break-even finder. Surfaces the spending threshold that keeps cash
// flow positive, given typical income.
func p9BreakEven(in Input) []smart.Insight {
	income, expense := in.trailingMonthly()
	if income <= 0 {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-P9",
		Page:    smart.PagePlanning,
		Key:     "SMART-P9:" + in.Now.Format("2006-01"),
		Title:   "Break-even spending: " + in.baseMoney(income).Format(2) + "/mo",
		Detail: "You stay cash-positive as long as monthly spending stays under about " +
			in.baseMoney(income).Format(2) + " (your typical income). You're running near " +
			in.baseMoney(expense).Format(2) + "/mo.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(income)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

// monthlyNets returns the net (income − expense, base minor) for each of the
// prior `months` whole months that had any activity, most-recent first.
func monthlyNets(in Input, months int) []int64 {
	curStart := dateutil.MonthStart(in.Now)
	var out []int64
	for k := 1; k <= months; k++ {
		s := dateutil.AddMonths(curStart, -k)
		e := dateutil.AddMonths(curStart, -k+1)
		var inc, exp int64
		any := false
		for _, t := range in.Transactions {
			if t.IsTransfer() || t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			any = true
			base := in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)
			if t.Amount.IsPositive() {
				inc += base
			} else {
				exp += -base
			}
		}
		if any {
			out = append(out, inc-exp)
		}
	}
	return out
}

const p4MinEssentials = 50_00 // need this much essential spend to suggest a buffer

// SMART-P4 — Suggested affordability & runway inputs. Derives a sensible cash
// buffer from real essential monthly spend, so the runway floor and the "Can I
// afford it?" reserve aren't guesses.
func p4Affordability(in Input) []smart.Insight {
	essentials := in.avgMonthlyExpenseBase()
	if essentials < p4MinEssentials {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-P4",
		Page:    smart.PagePlanning,
		Key:     "SMART-P4:" + in.Now.Format("2006-01"),
		Title:   "Suggested cash buffer: " + in.baseMoney(essentials).Format(2),
		Detail: "Your essentials run about " + in.baseMoney(essentials).Format(2) +
			"/mo. Using that as the runway floor and the affordability reserve keeps the projections grounded in real spending rather than a guess.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(essentials)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

// SMART-P1 — Auto-discovered recurring cash flows. Scans the transaction history
// and reports recurring charges that aren't yet in the Planning recurring set, so
// the user can add them for a sharper forecast.
func p1DiscoverRecurring(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil || len(subs) == 0 {
		return nil
	}
	existing := map[string]bool{}
	for _, r := range in.Recurring {
		existing[strings.ToLower(strings.TrimSpace(r.Label))] = true
	}
	var newCount int
	var monthly int64
	for _, s := range subs {
		if existing[strings.ToLower(strings.TrimSpace(s.Name))] {
			continue
		}
		newCount++
		monthly += s.MonthlyAmount()
	}
	if newCount == 0 {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-P1",
		Page:    smart.PagePlanning,
		Key:     "SMART-P1:" + in.Now.Format("2006-01"),
		Title:   plural(int64(newCount), "recurring charge") + " not in your plan yet",
		Detail: "Your history shows about " + in.baseMoney(monthly).Format(2) + "/mo of recurring charges that aren't " +
			"in Planning. Adding them sharpens the forecast and runway.",
		Severity: smart.SeverityNudge,
	}.WithAmount(in.baseMoney(monthly)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

const p8MinExtra = 25_00 // only suggest an extra payment worth at least $25/mo

// SMART-P8 — Auto-suggested extra debt payment. When there's debt and spare
// monthly surplus, recommends the largest sensible extra payment (capped by the
// surplus so it never pushes cash flow negative).
func p8ExtraDebt(in Input) []smart.Insight {
	debts := buildDebts(in)
	if len(debts) == 0 {
		return nil
	}
	surplus := in.monthlySurplusBase()
	if surplus <= 0 {
		return nil
	}
	extra := payoff.SuggestedExtra(debts)
	if extra > surplus {
		extra = surplus // never recommend more than you free up
	}
	if extra < p8MinExtra {
		return nil
	}
	// Name the highest-APR debt as the place to send it.
	target := highestAPRDebt(debts)
	ins := smart.Insight{
		Feature: "SMART-P8",
		Page:    smart.PagePlanning,
		Key:     "SMART-P8:" + in.Now.Format("2006-01"),
		Title:   "Put an extra " + in.baseMoney(extra).Format(2) + "/mo toward debt",
		Detail: "You free up about " + in.baseMoney(surplus).Format(2) + "/mo. Sending " +
			in.baseMoney(extra).Format(2) + " of it to " + target + " each month clears your debt faster and saves interest.",
		Severity: smart.SeverityNudge,
	}.WithAmount(in.baseMoney(extra)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open planning", Route: "/planning"})
	return []smart.Insight{ins}
}

// buildDebts assembles payoff.Debt records from non-archived liability accounts
// with a balance owed, in base-currency minor units.
func buildDebts(in Input) []payoff.Debt {
	var out []payoff.Debt
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		owed := abs64(in.toBaseMinor(bal.Amount, a.Currency))
		if owed <= 0 {
			continue
		}
		out = append(out, payoff.Debt{
			Name:       a.Name,
			Balance:    owed,
			AprPercent: a.InterestRateAPR,
			MinPayment: abs64(in.toBaseMinor(a.MinPayment.Amount, a.Currency)),
		})
	}
	return out
}

// highestAPRDebt returns the name of the debt with the highest APR, or "your
// highest-interest debt" when none stands out.
func highestAPRDebt(debts []payoff.Debt) string {
	name := ""
	best := -1.0
	for _, d := range debts {
		if d.AprPercent > best {
			best, name = d.AprPercent, d.Name
		}
	}
	if name == "" {
		return "your highest-interest debt"
	}
	return name
}

const (
	planningHorizonDays = 75     // look this far ahead for a large irregular charge
	billShockMinAnnual  = 300_00 // only warn for irregular charges ≥ $300/yr
)

// SMART-P10 — Bill-shock early warning. Projects large irregular recurring
// charges (yearly/quarterly) that land within the planning horizon and suggests
// setting money aside ahead of time.
func p10BillShock(in Input) []smart.Insight {
	var out []smart.Insight
	horizonEnd := in.Now.AddDate(0, 0, planningHorizonDays)
	for _, r := range in.Recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		if r.Cadence != domain.CadenceYearly && r.Cadence != domain.CadenceQuarterly {
			continue
		}
		annual := abs64(in.toBaseMinor(annualMinor(r), r.Amount.Currency))
		if annual < billShockMinAnnual {
			continue
		}
		due := r.NextDue
		for due.Before(in.Now) {
			due = r.Cadence.Next(due)
		}
		if due.After(horizonEnd) {
			continue // not imminent enough to warn about yet
		}
		charge := abs64(in.toBaseMinor(r.Amount.Amount, r.Amount.Currency))
		monthsOut := max(monthsBetween(in.Now, due), 1)
		setAside := charge / monthsOut
		out = append(out, smart.Insight{
			Feature: "SMART-P10",
			Page:    smart.PagePlanning,
			Key:     "SMART-P10:" + r.ID + ":" + due.Format("2006-01"),
			Title:   r.Label + " of " + in.baseMoney(charge).Format(2) + " lands " + due.Format("Jan 2"),
			Detail: "A large " + r.Label + " charge (" + in.baseMoney(charge).Format(2) + ") is coming " +
				due.Format("Jan 2") + ". Setting aside about " + in.baseMoney(setAside).Format(2) +
				"/mo until then softens the hit.",
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(charge)).
			WithAction(smart.Action{Kind: smart.ActionCreateTask, Label: "Add a to-do",
				TaskTitle: "Set aside " + in.baseMoney(setAside).Format(2) + "/mo for " + r.Label,
				TaskNotes: r.Label + " (" + in.baseMoney(charge).Format(2) + ") is due " + due.Format("Jan 2") + "."}))
	}
	return out
}
