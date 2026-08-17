// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/cadencefit"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-B16", b16CadenceMismatch)
}

// cadenceMonths is how far back the rhythm is read.
//
// Twelve, which is also cadencefit's minimum: an annual bill cannot be
// distinguished from a one-off in anything less, and reading further back would
// judge a budget on spending from before it existed.
const cadenceMonths = 12

// b16CadenceMismatch — EC-10. Finds a monthly budget whose category is actually
// spent on quarterly or annually, and points at the sinking fund.
//
// It complements SMART-BL9 rather than repeating it: BL9 acts on recurring items
// somebody DECLARED, where the cadence is already known. This one discovers the
// rhythm from what actually posted, for the spending nobody wrote down — which
// is most of it.
func b16CadenceMismatch(in Input) []smart.Insight {
	// One bucket per month INCLUDING the empty ones: the empty months are the
	// entire signal, and dropping them would make every category look annual.
	monthStart := dateutil.MonthStart(in.Now)
	idx := map[string]int{}
	bounds := make([]string, 0, cadenceMonths)
	for i := cadenceMonths; i > 0; i-- {
		key := dateutil.AddMonths(monthStart, -i).Format("2006-01")
		idx[key] = len(bounds)
		bounds = append(bounds, key)
	}

	byCat := map[string][]int64{}
	for _, t := range in.Transactions {
		if !t.IsExpense() || !t.CountsInReports() || t.CategoryID == "" {
			continue
		}
		slot, ok := idx[t.Date.Format("2006-01")]
		if !ok {
			continue
		}
		series, seen := byCat[t.CategoryID]
		if !seen {
			series = make([]int64, cadenceMonths)
			byCat[t.CategoryID] = series
		}
		series[slot] += abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency))
	}

	catName := map[string]string{}
	for _, c := range in.Categories {
		catName[c.ID] = c.Name
	}

	var out []smart.Insight
	for _, b := range in.Budgets {
		// Only a monthly budget can be mismatched this way. A quarterly or annual
		// budget already matches a lumpy rhythm, which is the remedy being
		// suggested — flagging it would be advising somebody to do what they did.
		if b.Period != domain.PeriodMonthly || b.CategoryID == "" {
			continue
		}
		fit := cadencefit.Assess(byCat[b.CategoryID])
		if !fit.Mismatched() {
			continue
		}
		name := catName[b.CategoryID]
		if name == "" {
			name = b.Name
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-B16",
			Page:     smart.PageBudgets,
			Key:      "SMART-B16:" + b.ID,
			Severity: smart.SeverityNudge,
		}.
			WithTitle("smart.b16.title", "%s is budgeted monthly but doesn't spend monthly", name).
			WithDetail(cadenceDetailKey(fit.Shape), cadenceDetailFormat(fit.Shape),
				name, plural(int64(fit.ActivePeriods), "month"), plural(int64(fit.Periods), "month"),
				in.hmoney(fit.TotalMinor), in.hmoney(fit.SuggestedMonthlyMinor)).
			// The set-aside is what acting on this is worth each month (WF-SM3) —
			// not a saving, but a real monthly figure the ranking can weigh.
			WithMonthlyAmount(in.baseMoney(fit.SuggestedMonthlyMinor)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/budgets",
				RelatedType: "budget", RelatedID: b.ID}.
				WithLabel("smart.b16.action", "Open budgets")))
	}
	return out
}

// cadenceDetailKey and cadenceDetailFormat pick the sentence for a rhythm.
//
// One key per shape rather than a translated word dropped into a shared
// sentence: the detectors are pure and carry copy as a key plus its arguments,
// so a word translated here would arrive at the surface already in English and
// stay that way in every other language.
func cadenceDetailKey(s cadencefit.Shape) string {
	switch s {
	case cadencefit.ShapeQuarterly:
		return "smart.b16.detailQuarterly"
	case cadencefit.ShapeAnnual:
		return "smart.b16.detailAnnual"
	}
	return "smart.b16.detailIrregular"
}

func cadenceDetailFormat(s cadencefit.Shape) string {
	const tail = " A monthly budget reads as blown once and unused the rest of the time. Setting aside %s a month covers it instead."
	switch s {
	case cadencefit.ShapeQuarterly:
		return "%s had spending in %s of the last %s, totalling %s — that is quarterly, not monthly." + tail
	case cadencefit.ShapeAnnual:
		return "%s had spending in %s of the last %s, totalling %s — that is once or twice a year, not monthly." + tail
	}
	return "%s had spending in %s of the last %s, totalling %s — that is occasional, not monthly." + tail
}
