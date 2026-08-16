// SPDX-License-Identifier: MIT

// Package anomalyprobe gathers the evidence behind a flagged finding before
// anyone asks about it (AG8).
//
// The assistant's "Discuss" chip used to attach a flag's headline to the composer
// and leave the model to work out what it meant — which usually meant a first
// reply that asked what the user already knew ("which transaction do you mean?")
// or, worse, a confident answer built from the headline alone. Both waste the one
// exchange the user was willing to have.
//
// So the app does the looking first. This package assembles what a person would
// pull up to judge the flag — the transactions it points at, what that merchant
// normally costs, whether a recurring schedule explains it — and hands the model a
// grounded brief. The agent's job becomes reaching a verdict, not discovering the
// facts, and the facts it reasons over are the app's rather than its recollection.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package anomalyprobe

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Finding is the flag being investigated, reduced to what the probe needs.
type Finding struct {
	// Feature is the detector's code ("SMART-T2"), which decides what evidence is
	// worth gathering.
	Feature string
	// Title and Detail are the finding's own words.
	Title, Detail string
	// Payee narrows the search when the finding is about a merchant.
	Payee string
	// AccountID narrows it when the finding is about an account.
	AccountID string
	// AmountMinor is the figure the finding is about, when it has one.
	AmountMinor int64
}

// Evidence is what the probe found: the rows behind the flag and the context that
// makes them judgeable.
type Evidence struct {
	// Related are the transactions the finding points at, newest first.
	Related []Txn
	// MerchantHistory summarises what this merchant has cost before, so "£120 at
	// the vet" can be read against "usually £45".
	MerchantHistory *MerchantStats
	// RecurringMatch names a scheduled item that explains the charge, when one
	// does. A flag with a recurring explanation is usually not a problem, and
	// saying so up front is the single most useful thing the probe can find.
	RecurringMatch string
	// Truncated reports that more rows matched than were gathered, so the brief
	// can say so rather than implying it saw everything.
	Truncated bool
}

// Txn is one transaction in the evidence, reduced to what a brief needs.
type Txn struct {
	Date        time.Time
	Payee       string
	AmountMinor int64
	Category    string
}

// MerchantStats is what a merchant has cost historically.
type MerchantStats struct {
	// Payee is the merchant.
	Payee string
	// Count is how many charges were seen.
	Count int
	// TypicalMinor is the median charge — median rather than mean because one
	// unusual charge is exactly what is being investigated, and a mean would let
	// the outlier redefine "typical".
	TypicalMinor int64
	// MinMinor and MaxMinor bound the range seen.
	MinMinor, MaxMinor int64
	// First and Last are when the household first and last paid this merchant.
	First, Last time.Time
}

// maxRelated bounds how many rows a brief carries. Enough to see a pattern, few
// enough that the brief stays a brief — and the count says when more exist.
const maxRelated = 8

// historyWindowDays is how far back merchant history looks. A year covers annual
// renewals, which are the charges most often flagged as unfamiliar.
const historyWindowDays = 365

// Gather assembles the evidence for one finding.
//
// It never invents a connection: when the finding names no merchant and no
// account, the probe returns what it can (nothing) rather than guessing at
// relevance from the headline's wording. An empty brief is honest; a brief full of
// unrelated rows would make the model's verdict worse than no brief at all.
func Gather(f Finding, txns []domain.Transaction, recurring []domain.Recurring, categoryOf func(string) string, now time.Time) Evidence {
	var ev Evidence
	payee := strings.ToLower(strings.TrimSpace(f.Payee))
	account := strings.TrimSpace(f.AccountID)
	if payee == "" && account == "" {
		return ev
	}

	matches := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if payee != "" && !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), payee) {
			continue
		}
		if account != "" && t.AccountID != account {
			continue
		}
		matches = append(matches, t)
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Date.After(matches[j].Date) })

	related := matches
	if len(related) > maxRelated {
		related, ev.Truncated = related[:maxRelated], true
	}
	for _, t := range related {
		name := strings.TrimSpace(t.Payee)
		if name == "" {
			name = strings.TrimSpace(t.Desc)
		}
		cat := ""
		if categoryOf != nil {
			cat = categoryOf(t.CategoryID)
		}
		ev.Related = append(ev.Related, Txn{
			Date: t.Date, Payee: name, AmountMinor: abs64(t.Amount.Amount), Category: cat,
		})
	}

	if payee != "" {
		ev.MerchantHistory = merchantStats(payee, matches, now)
		ev.RecurringMatch = recurringMatch(payee, recurring)
	}
	return ev
}

// merchantStats summarises a merchant's charges over the history window. It
// returns nil below two charges: a "typical" derived from a single observation is
// not a fact about the merchant, it is that observation wearing a hat.
func merchantStats(payee string, matches []domain.Transaction, now time.Time) *MerchantStats {
	cutoff := now.AddDate(0, 0, -historyWindowDays)
	amounts := make([]int64, 0, len(matches))
	stats := MerchantStats{Payee: payee}
	for _, t := range matches {
		if t.Date.Before(cutoff) || !t.IsExpense() {
			continue
		}
		amount := abs64(t.Amount.Amount)
		amounts = append(amounts, amount)
		if stats.Count == 0 || amount < stats.MinMinor {
			stats.MinMinor = amount
		}
		if amount > stats.MaxMinor {
			stats.MaxMinor = amount
		}
		if stats.First.IsZero() || t.Date.Before(stats.First) {
			stats.First = t.Date
		}
		if t.Date.After(stats.Last) {
			stats.Last = t.Date
		}
		stats.Count++
	}
	if stats.Count < 2 {
		return nil
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })
	stats.TypicalMinor = amounts[len(amounts)/2]
	return &stats
}

// recurringMatch names a scheduled item whose label matches the merchant. A flag
// with a schedule behind it is usually not a problem at all.
func recurringMatch(payee string, recurring []domain.Recurring) string {
	for _, r := range recurring {
		if r.Paused {
			continue
		}
		label := strings.TrimSpace(r.Label)
		if label == "" {
			continue
		}
		low := strings.ToLower(label)
		if strings.Contains(low, payee) || strings.Contains(payee, low) {
			return label
		}
	}
	return ""
}

// Brief renders the evidence as the grounded opening the model reasons over.
//
// It is written as a briefing TO the assistant rather than as an answer: it states
// what was found, and then asks for a verdict and a proposed next step. The shape
// matters — a brief that only supplies facts gets a summary of the facts back,
// which the user can already see on the row they clicked.
//
// fmtMoney formats a minor-unit figure; fmtDate formats a date. Both come from the
// caller so this package stays free of currency and locale handling.
func (e Evidence) Brief(f Finding, fmtMoney func(int64) string, fmtDate func(time.Time) string) string {
	var b strings.Builder
	b.WriteString("A check on this household's data flagged the following:\n\n")
	b.WriteString(strings.TrimSpace(f.Title))
	if d := strings.TrimSpace(f.Detail); d != "" {
		b.WriteString(" — " + strings.TrimRight(d, ".") + ".")
	}
	b.WriteString("\n\n")

	if e.RecurringMatch != "" {
		b.WriteString("This household has a recurring schedule called \"" + e.RecurringMatch +
			"\", which may explain it.\n\n")
	}
	if s := e.MerchantHistory; s != nil {
		b.WriteString("What " + s.Payee + " usually costs them: " + fmtMoney(s.TypicalMinor) +
			" (" + itoa(s.Count) + " charges between " + fmtMoney(s.MinMinor) + " and " +
			fmtMoney(s.MaxMinor) + ", from " + fmtDate(s.First) + " to " + fmtDate(s.Last) + ").\n\n")
	}
	if len(e.Related) > 0 {
		b.WriteString("The transactions involved:\n")
		for _, t := range e.Related {
			line := "- " + fmtDate(t.Date) + " · " + t.Payee + " · " + fmtMoney(t.AmountMinor)
			if strings.TrimSpace(t.Category) != "" {
				line += " · " + t.Category
			}
			b.WriteString(line + "\n")
		}
		if e.Truncated {
			b.WriteString("(only the most recent " + itoa(len(e.Related)) + " are listed)\n")
		}
		b.WriteString("\n")
	}
	if e.RecurringMatch == "" && e.MerchantHistory == nil && len(e.Related) == 0 {
		// Saying the lookup found nothing is itself useful: it stops the model
		// answering as though it had evidence, and tells the user the flag is
		// standing on the detector's own reasoning alone.
		b.WriteString("Nothing else in their data relates to this flag.\n\n")
	}
	b.WriteString("Give your verdict first — is this a problem or not — then the reasoning in one or " +
		"two sentences, then one concrete thing they could do about it (or say plainly that nothing " +
		"needs doing). Use only the figures above.")
	return b.String()
}

// abs64 returns the magnitude of a signed minor-unit amount.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// itoa formats a small non-negative count without pulling in strconv at the call
// sites above.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// PayeeIn finds which of the household's merchants a finding is talking about, by
// looking for one of their actual payee names inside the finding's own words.
//
// This is deliberately grounded in the household's data rather than parsed out of
// the detector's phrasing. Detectors word their findings differently and change
// wording over time; the set of merchants they can possibly mean is known exactly.
// Matching against that set means the probe either finds a real merchant or finds
// nothing — it can never invent one from a sentence fragment.
//
// The LONGEST match wins, so a household with both "Amazon" and "Amazon Prime"
// gets the specific one rather than whichever happens to be checked first.
func PayeeIn(text string, payees []string) string {
	low := strings.ToLower(text)
	best := ""
	for _, p := range payees {
		name := strings.TrimSpace(p)
		// Two characters or fewer matches half the alphabet's worth of substrings
		// and would attach the probe to the wrong merchant constantly.
		if len(name) <= 2 {
			continue
		}
		if strings.Contains(low, strings.ToLower(name)) && len(name) > len(best) {
			best = name
		}
	}
	return best
}
