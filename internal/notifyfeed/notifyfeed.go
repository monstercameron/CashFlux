// Package notifyfeed bridges CashFlux's domain data to notification candidates
// for the notify catch-up engine (B19). Each generator turns one event's data
// into notify.Candidates, keeping the notify package itself free of any domain
// dependency. Pure Go, no syscall/js, table-tested. Rule gating (enabled /
// channels / frequency cap) is applied later by notify.CatchUp — the generators
// only produce the raw occurrences for an event.
package notifyfeed

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/notify"
)

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
