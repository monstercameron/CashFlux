// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

// mny is a short constructor for a money.Money in a given (possibly non-base)
// currency, used where an engine quotes a transaction in its native currency.
func mny(minor int64, cur string) money.Money { return money.New(minor, cur) }

func init() {
	register("SMART-T2", t2Duplicates)
	register("SMART-T6", t6SpendingSpike)
	register("SMART-T7", t7MissingTxn)
	register("SMART-T13", t13RefundMatch)
}

const (
	spikeFactor      = 4     // a txn this many× its category average is a spike
	spikeMinMean     = 10_00 // ignore categories whose average is under $10
	spikeMinSamples  = 4     // need this many prior txns to trust the average
	spikeRecentDays  = 35    // only flag spikes in this recent window
	missingGraceDays = 3     // days past expected before calling a charge missing
	missingWindow    = 35    // ignore charges overdue longer than this (likely cancelled)
	refundWindowDays = 60    // a refund matches a charge within this many days back
	refundMinAmount  = 1_00  // ignore tiny refunds ($1)
)

// SMART-T2 — Smart duplicate detection. Surfaces groups of transactions that
// share a date, amount, and description — the signature of a double entry.
func t2Duplicates(in Input) []smart.Insight {
	groups := dedupe.FindDuplicates(in.Transactions)
	var out []smart.Insight
	for _, g := range groups {
		extra := len(g.IDs) - 1
		out = append(out, smart.Insight{
			Feature: "SMART-T2",
			Page:    smart.PageTransactions,
			Key:     "SMART-T2:" + g.Date + ":" + strings.ToLower(g.Description) + ":" + itoa64(g.Amount),
			Title:   plural(int64(extra), "possible duplicate") + " of " + g.Description,
			Detail: itoa64(int64(len(g.IDs))) + " identical entries on " + g.Date + " for " +
				mny(abs64(g.Amount), g.Currency).Format(2) + " — merge or remove the extras.",
			Severity: smart.SeverityWarn,
		}.WithAmount(mny(abs64(g.Amount), g.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review transactions", Route: "/transactions"}))
	}
	return out
}

// SMART-T6 — Spending-spike alerts. Flags a recent transaction that is unusually
// large versus its category's own average.
func t6SpendingSpike(in Input) []smart.Insight {
	names := categoryNames(in.Categories)
	// Build each category's prior expense magnitudes (in base units).
	mags := map[string][]int64{}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() || t.CategoryID == "" {
			continue
		}
		mags[t.CategoryID] = append(mags[t.CategoryID], in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency))
	}
	recentCut := in.Now.AddDate(0, 0, -spikeRecentDays)
	// For each recent expense, compare against its category mean excluding itself.
	var out []smart.Insight
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() || t.CategoryID == "" {
			continue
		}
		if t.Date.Before(recentCut) || t.Date.After(in.Now) {
			continue
		}
		all := mags[t.CategoryID]
		if len(all) <= spikeMinSamples {
			continue
		}
		mag := in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency)
		mean := meanExcluding(all, mag)
		if mean < spikeMinMean || mag < mean*spikeFactor {
			continue
		}
		cat := names[t.CategoryID]
		if cat == "" {
			cat = "this category"
		}
		out = append(out, smart.Insight{
			Feature: "SMART-T6",
			Page:    smart.PageTransactions,
			Key:     "SMART-T6:" + t.ID,
			Title:   mny(mag, in.Base).Format(2) + " in " + cat + " is unusually large",
			Detail: txnLabel(t) + " on " + t.Date.Format("Jan 2") + " is about " +
				itoa64(mag/maxInt64(mean, 1)) + "× the typical " + cat + " charge (" +
				mny(mean, in.Base).Format(2) + ").",
			Severity: smart.SeverityWarn,
		}.WithAmount(mny(mag, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "View transaction",
				Route: "/transactions", RelatedType: "transaction", RelatedID: t.ID}))
	}
	return out
}

// SMART-T7 — Missing-transaction detection. Notices when an expected recurring
// charge is overdue and hasn't posted.
func t7MissingTxn(in Input) []smart.Insight {
	subs, err := subscriptions.Detect(in.Transactions, in.Rates, recurringMinCount)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, s := range subs {
		expected := s.NextRenewal
		overdue := int(in.Now.Sub(expected).Hours() / 24)
		if overdue < missingGraceDays || overdue > missingWindow {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-T7",
			Page:    smart.PageTransactions,
			Key:     "SMART-T7:" + strings.ToLower(s.Name) + ":" + expected.Format("2006-01"),
			Title:   s.Name + " hasn't posted yet",
			Detail: s.Name + " usually charges about " + mny(s.Amount, s.Currency).Format(2) +
				" by " + expected.Format("Jan 2") + ", but no charge is recorded — check for a forgotten entry or a failed payment.",
			Severity: smart.SeverityWarn,
		}.WithAmount(mny(s.Amount, s.Currency)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Check transactions", Route: "/transactions"}))
	}
	return out
}

// SMART-T13 — Refund / reversal matching. Pairs a refund (a positive entry) with
// a recent matching charge of the same merchant and magnitude.
func t13RefundMatch(in Input) []smart.Insight {
	var out []smart.Insight
	for _, r := range in.Transactions {
		if r.IsTransfer() || !r.Amount.IsPositive() {
			continue
		}
		if r.Amount.Amount < refundMinAmount {
			continue
		}
		charge, ok := matchingCharge(in.Transactions, r, refundWindowDays)
		if !ok {
			continue
		}
		out = append(out, smart.Insight{
			Feature: "SMART-T13",
			Page:    smart.PageTransactions,
			Key:     "SMART-T13:" + r.ID,
			Title:   "Refund of " + r.Amount.Format(2) + " from " + txnLabel(r),
			Detail: "This " + r.Date.Format("Jan 2") + " credit looks like a refund of your " +
				charge.Date.Format("Jan 2") + " charge — they net out, so it won't distort category totals.",
			Severity: smart.SeverityInfo,
		}.WithAmount(r.Amount).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "View transaction",
				Route: "/transactions", RelatedType: "transaction", RelatedID: r.ID}))
	}
	return out
}

// --- transaction-engine helpers ------------------------------------------

// matchingCharge finds a non-transfer expense with the same merchant label and
// magnitude as the refund r, dated within windowDays before r.
func matchingCharge(txns []domain.Transaction, r domain.Transaction, windowDays int) (domain.Transaction, bool) {
	cut := r.Date.AddDate(0, 0, -windowDays)
	want := txnLabel(r)
	for _, c := range txns {
		if c.ID == r.ID || c.IsTransfer() || !c.Amount.IsNegative() {
			continue
		}
		if -c.Amount.Amount != r.Amount.Amount || c.Amount.Currency != r.Amount.Currency {
			continue
		}
		if c.Date.After(r.Date) || c.Date.Before(cut) {
			continue
		}
		if !strings.EqualFold(txnLabel(c), want) {
			continue
		}
		return c, true
	}
	return domain.Transaction{}, false
}

// txnLabel is the display label for a transaction — its payee, else description.
func txnLabel(t domain.Transaction) string {
	if s := strings.TrimSpace(t.Payee); s != "" {
		return s
	}
	return strings.TrimSpace(t.Desc)
}

// categoryNames maps category id → display name.
func categoryNames(cats []domain.Category) map[string]string {
	m := make(map[string]string, len(cats))
	for _, c := range cats {
		m[c.ID] = c.Name
	}
	return m
}

// meanExcluding returns the integer mean of xs with one occurrence of `one`
// removed (the candidate transaction shouldn't inflate its own baseline).
func meanExcluding(xs []int64, one int64) int64 {
	var sum, n int64
	removed := false
	for _, x := range xs {
		if !removed && x == one {
			removed = true
			continue
		}
		sum += x
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
