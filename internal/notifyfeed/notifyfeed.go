// SPDX-License-Identifier: MIT

// Package notifyfeed bridges CashFlux's domain data to notification candidates
// for the notify catch-up engine (B19). Each generator turns one event's data
// into notify.Candidates, keeping the notify package itself free of any domain
// dependency. Pure Go, no syscall/js, table-tested. Rule gating (enabled /
// channels / frequency cap) is applied later by notify.CatchUp — the generators
// only produce the raw occurrences for an event.
package notifyfeed

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/copytext"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/payeebase"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
)

// defaultTaskStaleDays is how far past due a to-do keeps reminding before it is
// treated as backlog rather than a reminder.
const defaultTaskStaleDays = 30

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
	text func(name string, days int) (title, body copytext.Text),
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
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      notify.SeverityWarning,
		})
	}
	return out
}

// DigestCandidates returns a periodic summary as a one-element slice, keyed by
// the given period key (e.g. notify.WeekKey(now) or notify.MonthKey(now)) so the
// digest surfaces at most once per period. title and body are that period's
// summary as re-renderable copy — the caller computes the figures, keeping the
// localization and number formatting in the UI layer, and the key+args form lets
// the persisted archive be re-rendered in a language chosen later (C362). An
// empty title yields no candidate (nothing worth summarizing). Returned as a
// slice so callers append it uniformly alongside the other generators.
func DigestCandidates(ruleID, periodKey string, title, body copytext.Text, now time.Time) []notify.Candidate {
	if title.Empty() {
		return nil
	}
	return []notify.Candidate{{
		RuleID:        ruleID,
		Event:         notify.EventDigest,
		OccurrenceKey: "digest@" + periodKey,
		At:            now,
		Title:         title.String(),
		Body:          body.String(),
		TitleText:     title,
		BodyText:      body,
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
	text func(name string, daysUntil int) (title, body copytext.Text),
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
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      sev,
		})
	}
	return out
}

// TaskReminderCandidates produces a notify.Candidate for each open to-do that
// has reached its own reminder lead time (per taskrecur.ReminderDue) and is not
// yet more than staleAfterDays past due. Each occurrence is keyed by the task's
// due date, so a task reminds once per due date and again after a recurrence
// spawns the next one. A task due today or already overdue is critical;
// otherwise it is a warning. text renders the localized title and body from the
// task title and days until due (negative when overdue). Candidates are tagged
// with ruleID.
//
// C403: the per-task reminder offset existed and drove only the attention
// digest, so "remind me three days before" reminded nobody who wasn't already
// looking at the digest. The offset is the user's stated intent about WHEN to be
// told; this is the channel that finally tells them.
func TaskReminderCandidates(
	ruleID string,
	tasks []domain.Task,
	staleAfterDays int,
	now time.Time,
	text func(taskTitle string, daysUntil int) (title, body copytext.Text),
) []notify.Candidate {
	if staleAfterDays <= 0 {
		staleAfterDays = defaultTaskStaleDays
	}
	var out []notify.Candidate
	for _, t := range tasks {
		if !taskrecur.ReminderDue(t, now) {
			continue
		}
		days := daysBetween(now, t.Due)
		// A task months overdue is not a reminder, it is a backlog. Nagging about
		// it forever trains the reader to ignore the whole feed.
		if days < -staleAfterDays {
			continue
		}
		sev := notify.SeverityWarning
		if days <= 0 {
			sev = notify.SeverityCritical
		}
		title, body := text(t.Title, days)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventTaskReminder,
			OccurrenceKey: t.ID + "@" + t.Due.Format("2006-01-02"),
			At:            now,
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      sev,
		})
	}
	return out
}

// daysBetween is whole calendar days from now to due (negative when due has
// passed), computed on date boundaries so a due time of 09:00 and a now of 17:00
// on the same day is 0 days, not -1.
func daysBetween(now, due time.Time) int {
	n := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, now.Location())
	return int(d.Sub(n).Hours() / 24)
}

// BackupCandidates returns a gentle "back up your data" reminder as a one-element
// slice when a backup is due for the given cadence (per the pure backup package),
// or nil otherwise. The occurrence is keyed by the cadence's natural period
// (ISO-week for Weekly, year-month for Monthly) so the nudge surfaces at most once
// per period regardless of how often the app is reopened. text renders the
// localized title and body from the days since the last backup (0 when never
// backed up). Severity is informational — backups are encouraged, never alarming.
func BackupCandidates(
	ruleID string,
	cadence backup.Cadence,
	lastBackupAt time.Time,
	now time.Time,
	text func(daysSince int) (title, body copytext.Text),
) []notify.Candidate {
	if !backup.Due(cadence, lastBackupAt, now) {
		return nil
	}
	periodKey := notify.MonthKey(now)
	if cadence == backup.Weekly {
		periodKey = notify.WeekKey(now)
	}
	title, body := text(backup.DaysSince(lastBackupAt, now))
	return []notify.Candidate{{
		RuleID:        ruleID,
		Event:         notify.EventBackupDue,
		OccurrenceKey: "backup@" + periodKey,
		At:            now,
		Title:         title.String(),
		Body:          body.String(),
		TitleText:     title,
		BodyText:      body,
		Severity:      notify.SeverityInfo,
	}}
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
	text func(name string, over bool) (title, body copytext.Text),
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
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      sev,
		})
	}
	return out
}

// LowBalanceCandidates produces a notify.Candidate for each asset account whose
// current balance (opening balance plus all transactions) falls strictly below
// floor (minor units). Liability accounts are never checked — their balances
// represent money owed, not a safety buffer. A non-positive floor disables the
// alert entirely, matching the zero-disables convention of LargeTransactionCandidates.
// Each occurrence is keyed per account per ISO-week so the nudge fires at most
// once per week regardless of how often the app is reopened. text renders the
// localized title and body from the account name and its current balance.
// Candidates are tagged with ruleID.
func LowBalanceCandidates(
	ruleID string,
	accounts []domain.Account,
	txns []domain.Transaction,
	floor int64,
	now time.Time,
	text func(name string, balanceMinor int64) (title, body copytext.Text),
) ([]notify.Candidate, error) {
	if floor <= 0 {
		return nil, nil
	}
	var out []notify.Candidate
	for _, a := range accounts {
		if a.Archived || a.IsLiability() {
			continue
		}
		bal, err := balance(a, txns)
		if err != nil {
			return nil, err
		}
		if bal >= floor {
			continue
		}
		title, body := text(a.Name, bal)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventLowBalance,
			OccurrenceKey: "lowbal:" + a.ID + "@" + notify.WeekKey(now),
			At:            now,
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      notify.SeverityWarning,
		})
	}
	return out, nil
}

// balance returns the net balance of account a in minor units (sum of opening
// balance plus all transactions belonging to a). It is a thin helper so
// LowBalanceCandidates does not import the full ledger package.
func balance(a domain.Account, txns []domain.Transaction) (int64, error) {
	total := a.OpeningBalance.Amount
	for _, t := range txns {
		if t.AccountID != a.ID {
			continue
		}
		if t.Amount.Currency != "" && a.Currency != "" && t.Amount.Currency != a.Currency {
			return 0, fmt.Errorf("notifyfeed: account %s currency mismatch: txn %s", a.ID, t.ID)
		}
		total += t.Amount.Amount
	}
	return total, nil
}

// PaycheckLandedCandidates produces a notify.Candidate for each income transaction
// that looks like a paycheck: a positive (inflow) non-transfer transaction at or
// above threshold (minor units) whose date falls within the recent window ending at
// now (e.g. the last 3 days). Each is keyed by transaction id so a given paycheck
// fires exactly once. A non-positive threshold disables the alert entirely.
// text renders the localized title and body from the payee/description and amount.
// Candidates are tagged with ruleID and carry SeverityInfo — a paycheck is good news.
func PaycheckLandedCandidates(
	ruleID string,
	txns []domain.Transaction,
	threshold int64,
	windowDays int,
	now time.Time,
	text func(desc string, amount int64) (title, body copytext.Text),
) []notify.Candidate {
	if threshold <= 0 {
		return nil
	}
	if windowDays <= 0 {
		windowDays = 3
	}
	cutoff := now.AddDate(0, 0, -windowDays)
	var out []notify.Candidate
	for _, t := range txns {
		if !t.IsIncome() {
			continue
		}
		if !t.Date.After(cutoff) || t.Date.After(now) {
			continue
		}
		if t.Amount.Amount < threshold {
			continue
		}
		title, body := text(t.Desc, t.Amount.Amount)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventPaycheckLanded,
			OccurrenceKey: "paycheck:" + t.ID,
			At:            t.Date,
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      notify.SeverityInfo,
		})
	}
	return out
}

// unusualChargeFactor is how many times a payee's typical (median) expense a
// charge must exceed to count as "unusual" — a heads-up that a merchant you know
// billed far more than usual (a price hike, a mistaken double-charge, or fraud).
const unusualChargeFactor = 3.0

// unusualMinHistory is how many OTHER expenses at the same payee are needed before
// its baseline is trustworthy — below this there's no established "normal" to
// deviate from, so nothing is flagged.
const unusualMinHistory = payeebase.MinHistory

// payeeKey groups a transaction by merchant. The rule lives in payeebase, which
// the workflow condition surface also uses, so the two can never drift into
// disagreeing about which charges belong to the same merchant.
func payeeKey(t domain.Transaction) string { return payeebase.Key(t) }

// medianMinor is the shared baseline statistic — see payeebase.Median for why it
// is a median rather than a mean.
func medianMinor(v []int64) int64 { return payeebase.Median(v) }

// UnusualChargeCandidates flags expenses that are unusually large versus the SAME
// payee's own history — the local "did this merchant just bill me way more than
// normal?" alert (a price hike, a duplicate charge, or fraud), computed entirely
// on-device with no external service. A charge is flagged when its base-currency
// magnitude is at least unusualChargeFactor × the median of that payee's OTHER
// expenses, is at least `floor` minor units (skip trivial amounts), and the payee
// has at least unusualMinHistory other expenses to establish a baseline. Only
// charges dated on/after `since` are flagged (the recent window), but the whole
// ledger builds each payee's baseline. Each is keyed by txn id so it fires once.
// A non-positive floor disables the alert. text renders the localized title/body
// from the payee, the charge amount, and the payee's typical (median) amount.
func UnusualChargeCandidates(
	ruleID string,
	txns []domain.Transaction,
	floor int64,
	since time.Time,
	rates currency.Rates,
	text func(payee string, amount, typical int64) (title, body copytext.Text),
) ([]notify.Candidate, error) {
	if floor <= 0 {
		return nil, nil
	}
	type exp struct {
		t   domain.Transaction
		mag int64
	}
	byPayee := map[string][]exp{}
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		key := payeeKey(t)
		if key == "" {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
		if err != nil {
			return nil, err
		}
		byPayee[key] = append(byPayee[key], exp{t: t, mag: conv.Amount})
	}
	var out []notify.Candidate
	for _, list := range byPayee {
		if len(list) <= unusualMinHistory {
			continue // not enough history for anyone in this group to be a baseline of >= minHistory OTHERS
		}
		for i, e := range list {
			if e.t.Date.Before(since) {
				continue // only recent charges are flagged
			}
			if e.mag < floor {
				continue
			}
			others := make([]int64, 0, len(list)-1)
			for j, o := range list {
				if j != i {
					others = append(others, o.mag)
				}
			}
			if len(others) < unusualMinHistory {
				continue
			}
			typical := medianMinor(others)
			if typical <= 0 || float64(e.mag) < unusualChargeFactor*float64(typical) {
				continue
			}
			label := e.t.Payee
			if strings.TrimSpace(label) == "" {
				label = e.t.Desc
			}
			title, body := text(label, e.mag, typical)
			out = append(out, notify.Candidate{
				RuleID:        ruleID,
				Event:         notify.EventUnusualCharge,
				OccurrenceKey: "unusual:" + e.t.ID,
				At:            e.t.Date,
				Title:         title.String(),
				Body:          body.String(),
				TitleText:     title,
				BodyText:      body,
				Severity:      notify.SeverityWarning,
			})
		}
	}
	return out, nil
}

// LargeTransactionCandidates produces a notify.Candidate for each expense whose
// base-currency magnitude meets or exceeds threshold (minor units) — the "a big
// charge just hit your account" alert. Only expenses on or after since are
// considered (a zero since means no lower bound), so the caller scopes it to the
// gap since the app was last open; each is keyed by transaction id so a given
// charge fires exactly once. A non-positive threshold yields nothing. text
// renders the localized title and body from the description and amount.
func LargeTransactionCandidates(
	ruleID string,
	txns []domain.Transaction,
	threshold int64,
	since time.Time,
	rates currency.Rates,
	text func(desc string, amount int64) (title, body copytext.Text),
) ([]notify.Candidate, error) {
	if threshold <= 0 {
		return nil, nil
	}
	var out []notify.Candidate
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		if !since.IsZero() && t.Date.Before(since) {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
		if err != nil {
			return nil, err
		}
		if conv.Amount < threshold {
			continue
		}
		title, body := text(t.Desc, conv.Amount)
		out = append(out, notify.Candidate{
			RuleID:        ruleID,
			Event:         notify.EventLargeTransaction,
			OccurrenceKey: "txn:" + t.ID,
			At:            t.Date,
			Title:         title.String(),
			Body:          body.String(),
			TitleText:     title,
			BodyText:      body,
			Severity:      notify.SeverityWarning,
		})
	}
	return out, nil
}
