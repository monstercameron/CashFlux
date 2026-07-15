// SPDX-License-Identifier: MIT

package smartengine

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-T19", t19NewMerchant)
	register("SMART-T20", t20NewSubscription)
}

const (
	// newMerchantRecentDays bounds how recently a merchant's first-ever charge
	// must have posted to be worth surfacing — an old merchant that happens to
	// have one historical charge is not news.
	newMerchantRecentDays = 30

	// newSubMinGap / newSubMaxGap bracket the spacing between the first two
	// charges that reads as a monthly subscription forming (a second charge
	// roughly a month after the first).
	newSubMinGap = 25
	newSubMaxGap = 35
	// newSubRecentDays bounds how recently the second charge must have posted so
	// the "new subscription" nudge is timely, not a retrospective note.
	newSubRecentDays = 40
	// newSubAmountTolBps is the band around the first charge within which the
	// second charge counts as "similar" — 20% (2000 basis points) absorbs the
	// tax/tier variation typical of a real subscription's first two bills.
	newSubAmountTolBps = 2000
)

// merchantCharge is one resolved expense used by the new-merchant detectors.
type merchantCharge struct {
	date     time.Time
	baseMag  int64  // magnitude in base minor units
	amount   int64  // raw signed amount, native currency
	currency string // native currency
}

// resolvedCharges groups a resolver's non-transfer expenses by the clean
// (alias-resolved) merchant name, each merchant's charges sorted ascending by
// date. Merchants that resolve to an empty name are skipped. The map key is the
// lower-cased resolved name (dismissal identity); labels carries the display
// casing.
func resolvedCharges(in Input) (charges map[string][]merchantCharge, labels map[string]string) {
	r := in.payeeResolver()
	charges = map[string][]merchantCharge{}
	labels = map[string]string{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		disp := strings.TrimSpace(r.Resolve(txnLabel(t)))
		if disp == "" {
			continue
		}
		key := strings.ToLower(disp)
		charges[key] = append(charges[key], merchantCharge{
			date:     t.Date,
			baseMag:  abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)),
			amount:   t.Amount.Amount,
			currency: t.Amount.Currency,
		})
		labels[key] = disp
	}
	for k := range charges {
		sort.Slice(charges[k], func(i, j int) bool { return charges[k][i].date.Before(charges[k][j].date) })
	}
	return charges, labels
}

// SMART-T19 — New-merchant awareness. Flags the first-ever charge at a resolved
// merchant when it posted recently: a fraud/awareness signal ("first time you've
// paid X"). Keyed on the clean merchant name so processor noise doesn't split one
// merchant into many false positives, and so dismissing one new merchant never
// silences a genuinely different one.
func t19NewMerchant(in Input) []smart.Insight {
	charges, labels := resolvedCharges(in)
	recentCut := in.Now.AddDate(0, 0, -newMerchantRecentDays)
	var out []smart.Insight
	for key, cs := range charges {
		first := cs[0] // earliest — the first-ever charge, by construction
		if first.date.Before(recentCut) || first.date.After(in.Now) {
			continue
		}
		label := labels[key]
		out = append(out, smart.Insight{
			Feature: "SMART-T19",
			Page:    smart.PageTransactions,
			Key:     "SMART-T19:" + key,
			Title:   "First time you've paid " + label,
			Detail: "This " + first.date.Format("Jan 2") + " charge of " + hmoneyc(first.baseMag, in.Base) +
				" is the first time " + label + " appears in your history. If you recognise it, all good — if not, it's worth a second look.",
			Severity: smart.SeverityInfo,
		}.WithAmount(mny(first.baseMag, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "View transactions", Route: "/transactions"}))
	}
	return out
}

// SMART-T20 — New-subscription detection. When a merchant's first two charges are
// a similar amount and land about a month apart (with no earlier history and no
// recurring already tracking it), it looks like a subscription forming. Offers a
// one-tap "track as recurring" that pre-fills a monthly recurring entry. Keyed on
// the clean merchant name for stable dismissal.
func t20NewSubscription(in Input) []smart.Insight {
	charges, labels := resolvedCharges(in)
	tracked := trackedRecurringLabels(in.Recurring)
	recentCut := in.Now.AddDate(0, 0, -newSubRecentDays)
	var out []smart.Insight
	for key, cs := range charges {
		// Exactly two charges so far = a subscription just forming, not an
		// established one already familiar to the user.
		if len(cs) != 2 {
			continue
		}
		if tracked[key] {
			continue // already tracked as recurring — nothing new to offer
		}
		first, second := cs[0], cs[1]
		if second.date.Before(recentCut) || second.date.After(in.Now) {
			continue
		}
		gap := int(second.date.Sub(first.date).Hours() / 24)
		if gap < newSubMinGap || gap > newSubMaxGap {
			continue
		}
		if !similarAmount(first.baseMag, second.baseMag) {
			continue
		}
		label := labels[key]
		out = append(out, smart.Insight{
			Feature: "SMART-T20",
			Page:    smart.PageTransactions,
			Key:     "SMART-T20:" + key,
			Title:   label + " looks like a new subscription",
			Detail: label + " has charged " + hmoneyc(second.baseMag, in.Base) + " twice, about a month apart (" +
				first.date.Format("Jan 2") + " and " + second.date.Format("Jan 2") +
				"). Track it as a recurring charge so it shows up in your plan.",
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(second.baseMag, in.Base)).
			WithAction(smart.Action{
				Kind:              smart.ActionCreateRecurring,
				Label:             "Track as recurring",
				RecurringLabel:    label,
				RecurringAmount:   -abs64(second.amount), // expense (negative), native currency
				RecurringCurrency: second.currency,
				RecurringCadence:  string(domain.CadenceMonthly),
			}))
	}
	return out
}

// trackedRecurringLabels returns the set of lower-cased recurring labels, so the
// new-subscription detector doesn't re-offer a merchant already being tracked.
func trackedRecurringLabels(rs []domain.Recurring) map[string]bool {
	out := make(map[string]bool, len(rs))
	for _, r := range rs {
		if l := strings.ToLower(strings.TrimSpace(r.Label)); l != "" {
			out[l] = true
		}
	}
	return out
}

// similarAmount reports whether two base-minor magnitudes are within
// newSubAmountTolBps of each other (measured against the larger, so the band is
// symmetric).
func similarAmount(a, b int64) bool {
	if a == 0 || b == 0 {
		return false
	}
	hi, lo := a, b
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi-lo)*10000 <= hi*newSubAmountTolBps
}
