// SPDX-License-Identifier: MIT

package smartengine

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func init() {
	register("SMART-SU1", su1CancelCandidates)
	register("SMART-SU3", su3TrialConversion)
	register("SMART-SU4", su4AnnualSavings)
	register("SMART-SU6", su6CostCreep)
	register("SMART-SU8", su8Forgotten)
	register("SMART-SU7", su7UsageVsCost)
	register("SMART-SU9", su9RenewalReminders)
	register("SMART-SU11", su11Zombie)
	register("SMART-SU12", su12Attribution)
	register("SMART-SU14", su14CancellationTally)
	register("SMART-SU15", su15Pause)
}

const su15MinCharges = 3 // need this many charges to judge a seasonal pattern

// SMART-SU15 — Pause-instead-of-cancel. Detects a merchant charged only part of
// the year (gaps between charges) and suggests pausing in off-months rather than
// cancelling outright.
func su15Pause(in Input) []smart.Insight {
	type info struct {
		months map[int]bool // year*12+month buckets that had a charge
		amt    int64        // a representative amount (base)
		label  string
	}
	byMerchant := map[string]*info{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(txnLabel(t)))
		if key == "" {
			continue
		}
		m := byMerchant[key]
		if m == nil {
			m = &info{months: map[int]bool{}, label: txnLabel(t)}
			byMerchant[key] = m
		}
		m.months[t.Date.Year()*12+int(t.Date.Month())-1] = true
		m.amt = abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency))
	}
	var out []smart.Insight
	for key, m := range byMerchant {
		if len(m.months) < su15MinCharges {
			continue
		}
		lo, hi := 1<<30, -1
		for k := range m.months {
			if k < lo {
				lo = k
			}
			if k > hi {
				hi = k
			}
		}
		span := hi - lo + 1
		// A seasonal pattern spans more months than it actually charged in — i.e.
		// there are off-months with no charge between the first and last.
		if span <= len(m.months) {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU15",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU15:" + key,
			Title:   m.label + " looks seasonal",
			Detail: m.label + " charges only part of the year. Pausing it in the off-months — rather than cancelling " +
				"and re-subscribing — keeps your settings and saves the gap months.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(m.amt, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-SU7 — Usage-vs-cost flag. When a subscription's category shows no other
// engagement (e.g. a gym membership but no other fitness spend), flags it as
// "paying but maybe not using."
func su7UsageVsCost(in Input) []smart.Insight {
	// Total non-transfer expense count per category.
	catCount := map[string]int{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() || t.CategoryID == "" {
			continue
		}
		catCount[t.CategoryID]++
	}
	names := categoryNames(in.Categories)
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		cat := categoryForMerchant(in, s.Name)
		if cat == "" {
			continue
		}
		// If the category's only activity is the subscription itself, there's no
		// other engagement to justify it.
		if catCount[cat] > s.Count {
			continue
		}
		catName := names[cat]
		if catName == "" {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU7",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU7:" + strings.ToLower(s.Name),
			Title:   "You may be paying for " + s.Name + " without using it",
			Detail: s.Name + " is the only activity in " + catName + " — " +
				hmoneyc(s.Amount, s.Currency) + " a month with nothing else in that category. Worth a look.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-SU12 — Shared/household sub attribution. In a multi-member household,
// flags subscriptions whose charges aren't attributed to any member.
func su12Attribution(in Input) []smart.Insight {
	if len(in.Members) < 2 {
		return nil // only relevant for multi-member households
	}
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		if merchantHasMember(in, s.Name) {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU12",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU12:" + strings.ToLower(s.Name),
			Title:   s.Name + " isn't assigned to anyone",
			Detail: s.Name + " (" + hmoneyc(s.Amount, s.Currency) +
				") isn't attributed to a household member. Assign it so everyone's share is clear.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// categoryForMerchant returns the category id of the most recent transaction
// matching the merchant name, or "".
func categoryForMerchant(in Input, name string) string {
	target := strings.ToLower(strings.TrimSpace(name))
	var best time.Time
	cat := ""
	for _, t := range in.Transactions {
		if strings.ToLower(strings.TrimSpace(txnLabel(t))) != target || t.CategoryID == "" {
			continue
		}
		if cat == "" || t.Date.After(best) {
			best, cat = t.Date, t.CategoryID
		}
	}
	return cat
}

// merchantHasMember reports whether any transaction for the merchant carries a
// member attribution.
func merchantHasMember(in Input, name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, t := range in.Transactions {
		if strings.ToLower(strings.TrimSpace(txnLabel(t))) == target && t.MemberID != "" {
			return true
		}
	}
	return false
}

const su9RenewalWindow = 7 // remind this many days before a renewal

// SMART-SU9 — Renewal-timed reminder automation. For a subscription renewing
// soon, offers a one-tap "should I keep this?" to-do a few days ahead.
func su9RenewalReminders(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		days := int(s.NextRenewal.Sub(in.Now).Hours() / 24)
		if days < 0 || days > su9RenewalWindow {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU9",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU9:" + strings.ToLower(s.Name) + ":" + s.NextRenewal.Format("2006-01-02"),
			Title:   s.Name + " renews " + s.NextRenewal.Format("Jan 2"),
			Detail: s.Name + " (" + hmoneyc(s.Amount, s.Currency) + ") renews on " + s.NextRenewal.Format("Jan 2") +
				". Decide whether to keep it before it charges again.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionCreateTask, Label: "Add a to-do",
				TaskTitle: "Keep " + s.Name + "? Renews " + s.NextRenewal.Format("Jan 2"),
				TaskNotes: s.Name + " renews " + s.NextRenewal.Format("Jan 2") + " for " + hmoneyc(s.Amount, s.Currency) + "."}))
	}
	return out
}

// SMART-SU6 — Per-subscription cost-creep history. Surfaces how much a
// subscription's price has crept up over time, making silent walk-ups visible.
func su6CostCreep(in Input) []smart.Insight {
	changes, err := subscriptions.DetectPriceChanges(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, c := range changes {
		if !c.Increased() || c.PercentChange < priceMinIncrease {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU6",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU6:" + strings.ToLower(strings.TrimSpace(c.Name)),
			Title:   c.Name + " costs " + itoa64(int64(c.PercentChange)) + "% more than before",
			Detail: c.Name + " has crept from " + hmoneyc(c.OldAmount, in.Base) + " to " +
				hmoneyc(c.NewAmount, in.Base) + " — a silent price walk-up worth noticing.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(c.Delta, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

// SMART-SU8 — "Forgotten since" surfacing. Ranks subscriptions whose last charge
// is overdue past their cadence, surfacing the truly out-of-mind ones.
func su8Forgotten(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		if !subscriptions.NeedsReview(s, in.Now) {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU8",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU8:" + strings.ToLower(s.Name),
			Title:   s.Name + " — no charge since " + s.Last.Format("Jan 2"),
			Detail: s.Name + " hasn't charged in a while. If it lapsed that's fine; if it's still active, " +
				"it's an easy one to forget you're paying for.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

const (
	zombieMinCount   = 6     // a charge running this many periods is well-established
	zombieMaxMonthly = 10_00 // … but small ($10/mo) — easy to forget
)

// SMART-SU11 — Zombie-charge detection. Flags small, long-running recurring
// charges that are easy to forget and worth a periodic "still using it?" check.
func su11Zombie(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		if s.Count < zombieMinCount || s.MonthlyAmount() > zombieMaxMonthly {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-SU11",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU11:" + strings.ToLower(s.Name),
			Title:   s.Name + " has quietly charged for " + plural(int64(s.Count), "period"),
			Detail: "A small recurring charge (" + hmoneyc(s.Amount, s.Currency) + ") that's been running a long time — " +
				"easy to forget. Worth a check that you still use it.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
	}
	return out
}

const (
	trialMaxIntro    = 1_00 // a charge at or below this counts as a $0/intro trial
	trialMinReal     = 3_00 // the first real charge must be at least this
	trialRecentDays  = 35   // only warn about a conversion this recent
	trialLookbackDay = 120  // a trial charge must be within this far back of the real one
)

// SMART-SU3 — Free-trial → paid conversion watch. Detects a merchant's first
// real charge following a $0/intro amount and warns at conversion.
func su3TrialConversion(in Input) []smart.Insight {
	// Group non-transfer expenses by merchant label.
	type charge struct {
		date   time.Time
		amount int64 // base minor, magnitude
	}
	byMerchant := map[string][]charge{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(txnLabel(t)))
		if name == "" {
			continue
		}
		byMerchant[name] = append(byMerchant[name], charge{date: t.Date, amount: abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency))})
	}
	recentCut := in.Now.AddDate(0, 0, -trialRecentDays)
	var out []smart.Insight
	for name, charges := range byMerchant {
		sort.Slice(charges, func(i, j int) bool { return charges[i].date.Before(charges[j].date) })
		// Find a recent first real charge preceded by an intro/$0 charge.
		for i, c := range charges {
			if c.amount < trialMinReal || c.date.Before(recentCut) || c.date.After(in.Now) {
				continue
			}
			// Is there an earlier intro charge within the lookback window?
			introOK := false
			for j := 0; j < i; j++ {
				if charges[j].amount <= trialMaxIntro && !charges[j].date.Before(c.date.AddDate(0, 0, -trialLookbackDay)) {
					introOK = true
					break
				}
			}
			if !introOK {
				continue
			}
			label := displayMerchant(in, name)
			out = append(out, smart.Insight{
				Feature: "SMART-SU3",
				Page:    smart.PageSubscriptions,
				Key:     "SMART-SU3:" + name + ":" + c.date.Format("2006-01-02"),
				Title:   label + " just converted to a paid charge",
				Detail: "After a free or intro period, " + label + " posted its first real charge of " +
					hmoneyc(c.amount, in.Base) + " on " + c.date.Format("Jan 2") + " — cancel now if you're not using it.",
				Severity: smart.SeverityWarn,
			}.WithAmount(mny(c.amount, in.Base)).
				WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review subscriptions", Route: "/subscriptions"}))
			break // one conversion warning per merchant
		}
	}
	return out
}

// displayMerchant recovers a nicer-cased merchant label from the original
// transactions for a lowercased key, falling back to the key.
func displayMerchant(in Input, lowerName string) string {
	for _, t := range in.Transactions {
		if strings.ToLower(strings.TrimSpace(txnLabel(t))) == lowerName {
			if l := txnLabel(t); l != "" {
				return l
			}
		}
	}
	return lowerName
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
			Title:    "Consider cutting " + s.Name + " — save " + hmoneyc(annual, s.Currency) + "/yr",
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
			Feature: "SMART-SU4",
			Page:    smart.PageSubscriptions,
			Key:     "SMART-SU4:" + strings.ToLower(s.Name),
			Title:   "Pay " + s.Name + " annually to save " + hmoneyc(saving, s.Currency) + "/yr",
			Detail: s.Name + " costs about " + hmoneyc(annual, s.Currency) +
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
