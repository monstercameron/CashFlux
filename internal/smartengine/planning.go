// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-P10", p10BillShock)
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
