// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func init() {
	register("SMART-SU1", su1CancelCandidates)
	register("SMART-SU4", su4AnnualSavings)
	register("SMART-SU14", su14CancellationTally)
}

const (
	cancelHighSharePct = 20    // a sub at this share of the recurring total is "big"
	annualPlanDiscount = 16    // typical % saving for paying a sub annually
	su4MinAnnual       = 60_00 // only suggest annual switch above $60/yr
)

// SMART-SU1 — Cancel-candidate recommendations. Combines staleness, recent price
// rises, and a high share of recurring spend into a ranked "consider cutting"
// shortlist with the yearly saving.
func su1CancelCandidates(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil || len(subs) == 0 {
		return nil
	}
	total := subscriptions.MonthlyTotal(subs)
	hiked := increasedNames(in)
	var out []smart.Insight
	for _, s := range subs {
		var reasons []string
		if subscriptions.NeedsReview(s, in.Now) {
			reasons = append(reasons, "no charge in a while")
		}
		if hiked[strings.ToLower(strings.TrimSpace(s.Name))] {
			reasons = append(reasons, "the price went up recently")
		}
		if total > 0 && s.MonthlyAmount()*100 >= int64(cancelHighSharePct)*total {
			reasons = append(reasons, "it's a big share of your subscriptions")
		}
		if len(reasons) == 0 {
			continue
		}
		annual := s.AnnualAmount()
		out = append(out, smart.Insight{
			Feature:  "SMART-SU1",
			Page:     smart.PageSubscriptions,
			Key:      "SMART-SU1:" + strings.ToLower(s.Name),
			Title:    "Consider cutting " + s.Name + " — save " + mny(annual, s.Currency).Format(2) + "/yr",
			Detail:   s.Name + " stands out because " + joinReasons(reasons) + ".",
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(annual, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-SU4 — Annual-vs-monthly savings finder. For monthly subscriptions, flags
// the typical saving of switching to an annual plan.
func su4AnnualSavings(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		if s.Cadence != subscriptions.CadenceMonthly {
			continue
		}
		annual := s.AnnualAmount()
		if annual < su4MinAnnual {
			continue
		}
		saving := annual * annualPlanDiscount / 100
		out = append(out, smart.Insight{
			Feature:  "SMART-SU4",
			Page:     smart.PageSubscriptions,
			Key:      "SMART-SU4:" + strings.ToLower(s.Name),
			Title:    "Pay " + s.Name + " annually to save ~" + mny(saving, s.Currency).Format(2) + "/yr",
			Detail: s.Name + " costs about " + mny(annual, s.Currency).Format(2) +
				"/yr monthly; many services are roughly two months cheaper on an annual plan.",
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(saving, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-SU14 — Cancellation-saved tally. A running scoreboard of how many
// subscriptions the user has cancelled, for positive reinforcement.
func su14CancellationTally(in Input) []smart.Insight {
	n := len(in.Subscriptions)
	if n == 0 {
		return nil
	}
	ins := smart.Insight{
		Feature:  "SMART-SU14",
		Page:     smart.PageSubscriptions,
		Key:      "SMART-SU14:tally",
		Title:    "You've cancelled " + plural(int64(n), "subscription"),
		Detail:   "Nice work trimming recurring costs — every cancellation keeps paying off each month.",
		Severity: smart.SeverityInfo,
	}
	return []smart.Insight{ins}
}

// --- subscription-engine helpers -----------------------------------------

// increasedNames returns the set of subscription names (lowercased) that have had
// a recent price increase, for the cancel-candidate signal.
func increasedNames(in Input) map[string]bool {
	out := map[string]bool{}
	changes, err := subscriptions.DetectPriceChanges(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return out
	}
	for _, c := range changes {
		if c.Increased() {
			out[strings.ToLower(strings.TrimSpace(c.Name))] = true
		}
	}
	return out
}

// joinReasons joins reason phrases into a natural-language clause.
func joinReasons(rs []string) string {
	switch len(rs) {
	case 0:
		return ""
	case 1:
		return rs[0]
	case 2:
		return rs[0] + " and " + rs[1]
	default:
		return strings.Join(rs[:len(rs)-1], ", ") + ", and " + rs[len(rs)-1]
	}
}
