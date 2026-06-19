// Package notifyfeed bridges CashFlux's domain data to notification candidates
// for the notify catch-up engine (B19). Each generator turns one event's data
// into notify.Candidates, keeping the notify package itself free of any domain
// dependency. Pure Go, no syscall/js, table-tested. Rule gating (enabled /
// channels / frequency cap) is applied later by notify.CatchUp — the generators
// only produce the raw occurrences for an event.
package notifyfeed

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/notify"
)

// defaultBillWindow is how many days ahead a bill is considered "due soon" when
// the rule doesn't specify its own window.
const defaultBillWindow = 7

// StaleBalanceCandidates produces a notify.Candidate for each account whose
// balance is currently stale (per freshness), deduped to one per account per
// ISO-week so it nudges weekly rather than on every app open. text renders the
// localized title and body from the account name and days since the balance was
// confirmed (-1 when it never has been). Candidates are tagged with ruleID.
func StaleBalanceCandidates(
	ruleID string,
	accounts []domain.Account,
	windows freshness.Windows,
	now time.Time,
	text func(name string, days int) (title, body string),
) []notify.Candidate {
	stale := freshness.StaleAccounts(accounts, windows, now)
	out := make([]notify.Candidate, 0, len(stale))
	for _, a := range stale {
		days := freshness.DaysSinceUpdate(a, now)
		title, body := text(a.Name, days)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventStaleBalance,
			OccurrenceKey: a.ID + "@" + notify.WeekKey(now),
			At:            now,
			Title:         title,
			Body:          body,
			Severity:      notify.SeverityWarning,
		})
	}
	return out
}

// DigestCandidates returns a periodic summary as a one-element slice, keyed by
// the given period key (e.g. notify.WeekKey(now) or notify.MonthKey(now)) so the
// digest surfaces at most once per period. title and body are the already-
// rendered summary of that period — the caller computes the figures, keeping the
// localization and number formatting in the UI layer. An empty title yields no
// candidate (nothing worth summarizing). Returned as a slice so callers append
// it uniformly alongside the other generators.
func DigestCandidates(ruleID, periodKey, title, body string, now time.Time) []notify.Candidate {
	if title == "" {
		return nil
	}
	return []notify.Candidate{{
		RuleID:        ruleID,
		Event:         notify.EventDigest,
		OccurrenceKey: "digest@" + periodKey,
		At:            now,
		Title:         title,
		Body:          body,
		Severity:      notify.SeverityInfo,
	}}
}

// BillDueCandidates produces a notify.Candidate for each upcoming bill due
// within withinDays (a non-positive withinDays falls back to a 7-day window).
// Each occurrence is keyed by its specific due date, so a bill fires once per
// due date (idempotent across opens) and again next cycle. A bill due today or
// tomorrow is critical; otherwise a warning. text renders the localized title
// and body from the bill name and days until due. Candidates are tagged with
// ruleID.
func BillDueCandidates(
	ruleID string,
	upcoming []bills.Bill,
	withinDays int,
	now time.Time,
	text func(name string, daysUntil int) (title, body string),
) []notify.Candidate {
	if withinDays <= 0 {
		withinDays = defaultBillWindow
	}
	var out []notify.Candidate
	for _, b := range upcoming {
		if b.DaysUntil < 0 || b.DaysUntil > withinDays {
			continue
		}
		sev := notify.SeverityWarning
		if b.DaysUntil <= 1 {
			sev = notify.SeverityCritical
		}
		title, body := text(b.Name, b.DaysUntil)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventBillDue,
			OccurrenceKey: b.AccountID + "@" + b.DueDate.Format("2006-01-02"),
			At:            now,
			Title:         title,
			Body:          body,
			Severity:      sev,
		})
	}
	return out
}

// BudgetCandidates produces a notify.Candidate for each budget that is near or
// over its limit (per the given budgeting statuses), deduped per budget + state
// per month — so a budget that goes from near to over fires a fresh, higher-
// severity alert rather than being silenced by the earlier "near" one. text
// renders the localized title and body from the budget name and whether it's
// over (vs merely near). Candidates are tagged with ruleID.
func BudgetCandidates(
	ruleID string,
	statuses []budgeting.Status,
	now time.Time,
	text func(name string, over bool) (title, body string),
) []notify.Candidate {
	var out []notify.Candidate
	for _, s := range statuses {
		if s.State != budgeting.StateNear && s.State != budgeting.StateOver {
			continue
		}
		over := s.State == budgeting.StateOver
		sev := notify.SeverityWarning
		if over {
			sev = notify.SeverityCritical
		}
		title, body := text(s.Budget.Name, over)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventBudgetThreshold,
			OccurrenceKey: s.Budget.ID + ":" + string(s.State) + "@" + notify.MonthKey(now),
			At:            now,
			Title:         title,
			Body:          body,
			Severity:      sev,
		})
	}
	return out
}
