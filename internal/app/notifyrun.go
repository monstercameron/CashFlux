// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"log/slog"
	"runtime/debug"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/notifyfeed"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// notifyDeliveredKey is the localStorage key holding the delivered-notification
// log, so catch-up is idempotent across reloads (B19).
const notifyDeliveredKey = "cashflux:notify:delivered"

// runNotifyCatchUp surfaces the "while you were away" reminders once on load: it
// gathers the current stale-balance and bill-due occurrences, runs them through
// the pure notify catch-up engine against the persisted delivered log (so each
// fires at most once per its natural period), and shows a single summary toast.
// It is wrapped in a recover so a notification hiccup can never break app boot.
func runNotifyCatchUp() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("runNotifyCatchUp panicked; boot continues",
				"panic", r,
				"stack", string(debug.Stack()),
			)
			// C272: surface a quiet, non-alarming notice instead of failing silently.
			uistate.PostNotice(uistate.T("notify.catchUpError"), true)
		}
	}()

	app := appstate.Default
	if app == nil {
		return
	}
	now := time.Now()
	accounts := app.Accounts()

	ruleCfg := notify.UnmarshalRuleConfig(uistate.SettingKVGet(notify.RuleConfigKey()))

	// Resolve bill-due lead days: user override or rule default.
	var billLeadDays int
	for _, r := range notify.DefaultRules() {
		if r.ID == "default-bill-due" {
			billLeadDays = int(notify.EffectiveThreshold("default-bill-due", ruleCfg, int64(r.Threshold)))
		}
	}

	var cands []notify.Candidate
	cands = append(cands, notifyfeed.StaleBalanceCandidates("default-stale", accounts, app.FreshnessWindows(), now,
		func(name string, days int) (title, body string) {
			return uistate.T("notify.staleTitle", name), uistate.T("notify.staleBody", days)
		})...)
	cands = append(cands, notifyfeed.BillDueCandidates("default-bill-due", bills.UpcomingAll(accounts, app.Recurring(), now), billLeadDays, now,
		func(name string, days int) (title, body string) {
			return uistate.T("notify.billTitle", name), uistate.T("notify.billBody", days)
		})...)
	cands = append(cands, notifyfeed.BudgetCandidates("default-budget", currentBudgetStatuses(app, now), now,
		func(name string, over bool) (title, body string) {
			if over {
				return uistate.T("notify.budgetOverTitle", name), uistate.T("notify.budgetOverBody")
			}
			return uistate.T("notify.budgetNearTitle", name), uistate.T("notify.budgetNearBody")
		})...)
	cands = append(cands, weeklyDigestCandidates(app, now)...)
	cands = append(cands, largeTransactionCandidates(app, now, ruleCfg)...)
	cands = append(cands, backupReminderCandidates(app, now)...)
	cands = append(cands, lowBalanceCandidates(app, now, ruleCfg)...)
	cands = append(cands, paycheckLandedCandidates(app, now, ruleCfg)...)

	log := loadDeliveredLog()
	out := notify.CatchUp(notify.EnabledRules(notify.DefaultRules(), ruleCfg), cands, now, log)
	saveDeliveredLog(log)

	// Self-heal: clear stale "balance is low" alerts for accounts that no longer
	// qualify. Runs every boot, before the no-new-notifications early return, so a
	// warning left over from before the user marked an account a liability (where a
	// zero balance is good — you owe nothing) or archived it, disappears on its own.
	reconcileLowBalanceFeed(app)

	if len(out) == 0 {
		return
	}

	// Record each notification in the persisted Notification Center feed (C75).
	// Severity is mapped from the notify.Severity int to the canonical string
	// used by the UI (C267):
	//   - budget-over / large-txn / bill-due-tomorrow → SeverityCritical → "critical"
	//   - stale-balance / bill-due-soon / budget-near → SeverityWarning   → "warning"
	//   - digest / backup / others                    → SeverityInfo       → "info"
	// The mapping is deterministic — it lives in the notify package's Candidate
	// producers (notifyfeed.*Candidates) and flows unchanged through CatchUp.
	feed := make([]uistate.FeedItem, len(out))
	for i, n := range out {
		feed[i] = uistate.FeedItem{
			ID:       n.ID,
			Title:    n.Title,
			Body:     n.Body,
			At:       n.At.Unix(),
			Severity: severityString(n.Severity),
		}
	}
	uistate.PrependNotifyFeed(feed)
	postBrowserNotifications(out)

	// One unobtrusive summary toast: the single reminder's title, or a count.
	msg := out[0].Title
	if len(out) > 1 {
		msg = uistate.T("notify.summary", len(out))
	}
	// PostNotice (not the UseNotice hook) — boot context, see CurrentPrefs note above.
	uistate.PostNotice(msg, false)
}

// postBrowserNotifications posts OS/browser notifications for the emitted items
// when the user has enabled them and granted permission (C75 browser channel).
func postBrowserNotifications(out []notify.Notification) {
	if !uistate.BrowserNotifyEnabled() {
		return
	}
	N := js.Global().Get("Notification")
	if !N.Truthy() {
		return
	}
	post := func() {
		for _, n := range out {
			N.New(n.Title, map[string]any{"body": n.Body, "tag": n.DedupeKey})
		}
	}
	switch N.Get("permission").String() {
	case "granted":
		post()
	case "denied":
		return
	default:
		var cb js.Func
		cb = js.FuncOf(func(_ js.Value, args []js.Value) any {
			cb.Release()
			if len(args) > 0 && args[0].String() == "granted" {
				post()
			}
			return nil
		})
		N.Call("requestPermission").Call("then", cb)
	}
}

// weeklyDigestCandidates emits a once-per-ISO-week summary of the previous
// completed week's income and spending (keyed by the current week, so the first
// open each week shows last week's recap). It produces nothing when there was no
// activity, so a quiet week doesn't nag.
func weeklyDigestCandidates(app *appstate.App, now time.Time) []notify.Candidate {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// CurrentPrefs (not the UsePrefs hook) — runNotifyCatchUp runs at boot, outside
	// any component render, where calling a hook panics with "GoUseAtom called
	// outside component context" (caught by recover, but it silently kills catch-up).
	weekStart := uistate.CurrentPrefs().WeekStartWeekday()
	prev := period.NewWindow(period.Week, now, weekStart).Shift(-1)
	ps, pe := prev.Range()
	flow, err := reports.IncomeVsExpense(app.Transactions(), ps, pe, rates)
	if err != nil || (flow.Income == 0 && flow.Expense == 0) {
		return nil
	}
	title := uistate.T("notify.digestTitle")
	body := uistate.T("notify.digestBody", fmtBaseMoney(flow.Income, base), fmtBaseMoney(flow.Expense, base))
	return notifyfeed.DigestCandidates("default-digest", notify.WeekKey(now), title, body, now)
}

// largeTransactionCandidates flags recent unusually large expenses (B19), over
// the last 30 days so the first open doesn't replay the whole history; each is
// keyed by transaction id so a given charge surfaces once. The threshold comes
// from the user's override in cfg (if set and positive), otherwise the default-large
// rule's built-in value; a zero/absent threshold disables the alert.
func largeTransactionCandidates(app *appstate.App, now time.Time, cfg notify.RuleConfig) []notify.Candidate {
	var ruleDefault int64
	for _, r := range notify.DefaultRules() {
		if r.ID == "default-large" {
			ruleDefault = int64(r.Threshold)
		}
	}
	threshold := notify.EffectiveThreshold("default-large", cfg, ruleDefault)
	if threshold <= 0 {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	since := now.AddDate(0, 0, -30)
	out, err := notifyfeed.LargeTransactionCandidates("default-large", app.Transactions(), threshold, since, rates,
		func(desc string, amount int64) (title, body string) {
			label := desc
			if label == "" {
				label = uistate.T("notify.largeNoDesc")
			}
			return uistate.T("notify.largeTitle", fmtBaseMoney(amount, base)), uistate.T("notify.largeBody", label)
		})
	if err != nil {
		return nil
	}
	return out
}

// lastBackupKey holds the timestamp (RFC3339) of the user's most recent JSON
// export, so the backup reminder knows how long it's been (B28).
const lastBackupKey = "cashflux:lastBackupAt"

// backupCadenceKey holds the user's chosen backup-reminder cadence (B28); unset
// defaults to the gentle monthly cadence.
const backupCadenceKey = "cashflux:backupCadence"

// loadBackupCadence reads the chosen reminder cadence, defaulting to the gentle
// monthly cadence when unset (an explicit "off" disables reminders).
func loadBackupCadence() backup.Cadence {
	raw := uistate.SettingKVGet(backupCadenceKey)
	if raw == "" {
		return backup.DefaultCadence
	}
	return backup.ParseCadence(raw)
}

// saveBackupCadence persists the chosen reminder cadence (Settings → Data).
func saveBackupCadence(c backup.Cadence) {
	uistate.SettingKVSet(backupCadenceKey, string(c))
}

// recordBackupNow stamps the current time as the last backup — called after a
// successful data export.
func recordBackupNow() {
	uistate.SettingKVSet(lastBackupKey, time.Now().Format(time.RFC3339))
}

// loadLastBackup reads the last-backup timestamp, or the zero time when never set
// or unparseable (which reads as "never backed up").
func loadLastBackup() time.Time {
	raw := uistate.SettingKVGet(lastBackupKey)
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

// lastBackupSummary is the plain-English "Last backed up <date>" line for the
// Settings → Data section (C299), or a never-backed-up nudge. Formats with the
// user's preferred date format (the non-hook formatter, safe anywhere).
func lastBackupSummary() string {
	t := loadLastBackup()
	if t.IsZero() {
		return uistate.T("settings.lastBackupNever")
	}
	return uistate.T("settings.lastBackup", uistate.LoadPrefs().FormatDate(t))
}

// backupReminderCandidates produces a gentle "back up your data" nudge when a
// backup is due for the default cadence (B28). It's suppressed when there are no
// transactions yet (nothing worth backing up), so a fresh install isn't nagged.
func backupReminderCandidates(app *appstate.App, now time.Time) []notify.Candidate {
	if len(app.Transactions()) == 0 {
		return nil
	}
	return notifyfeed.BackupCandidates("default-backup", loadBackupCadence(), loadLastBackup(), now,
		func(daysSince int) (title, body string) {
			if daysSince <= 0 {
				return uistate.T("notify.backupTitle"), uistate.T("notify.backupBodyNever")
			}
			return uistate.T("notify.backupTitle"), uistate.T("notify.backupBody", daysSince)
		})
}

// severityString maps a notify.Severity int to the canonical pill label used by
// the Notification Center UI (C267). The mapping is the single source of truth:
// edit here to change how any severity level is labelled.
func severityString(s notify.Severity) string {
	switch s {
	case notify.SeverityWarning:
		return "warning"
	case notify.SeverityCritical:
		return "critical"
	default:
		return "info"
	}
}

// fmtBaseMoney formats a base-currency minor-units value in the app's accounting
// style (symbol + grouping), for notification text.
func fmtBaseMoney(v int64, base string) string {
	return money.FormatAccounting(v, currency.Decimals(base), currency.Symbol(base))
}

// lowBalanceCandidates flags asset accounts whose current balance is below the
// user-configured floor (or the default-low-balance rule's built-in value when
// no override is set). A zero/absent threshold disables the alert.
func lowBalanceCandidates(app *appstate.App, now time.Time, cfg notify.RuleConfig) []notify.Candidate {
	var ruleDefault int64
	for _, r := range notify.DefaultRules() {
		if r.ID == "default-low-balance" {
			ruleDefault = int64(r.Threshold)
		}
	}
	floor := notify.EffectiveThreshold("default-low-balance", cfg, ruleDefault)
	if floor <= 0 {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	out, err := notifyfeed.LowBalanceCandidates("default-low-balance", app.Accounts(), app.Transactions(), floor, now,
		func(name string, balMinor int64) (title, body string) {
			return uistate.T("notify.lowBalTitle", name),
				uistate.T("notify.lowBalBody", fmtBaseMoney(balMinor, base))
		})
	if err != nil {
		return nil
	}
	return out
}

// reconcileLowBalanceFeed removes any existing "balance is low" notifications for
// accounts that no longer qualify for the alert: liabilities (a low/zero balance
// there is good — you owe nothing) and archived accounts. New low-balance
// candidates already skip these (notifyfeed.LowBalanceCandidates checks
// IsLiability + Archived), but an alert raised BEFORE the user marked the account
// a liability would otherwise linger; this clears it on the next boot.
//
// Low-balance feed IDs are DedupeKey("default-low-balance", "lowbal:<acctID>@<week>")
// = "default-low-balance@lowbal:<acctID>@<week>", so the account id is recoverable.
func reconcileLowBalanceFeed(app *appstate.App) {
	resolved := map[string]bool{}
	for _, a := range app.Accounts() {
		if a.IsLiability() || a.Archived {
			resolved[a.ID] = true
		}
	}
	if len(resolved) == 0 {
		return
	}
	const prefix = "default-low-balance@lowbal:"
	uistate.RemoveFeedItems(func(it uistate.FeedItem) bool {
		if !strings.HasPrefix(it.ID, prefix) {
			return false
		}
		rest := strings.TrimPrefix(it.ID, prefix)
		acctID := rest
		if i := strings.LastIndex(rest, "@"); i >= 0 { // strip the "@<week>" suffix
			acctID = rest[:i]
		}
		return resolved[acctID]
	})
}

// paycheckLandedCandidates flags income transactions that look like a paycheck
// arriving in the last 3 days (the short recent window where a paycheck is "fresh
// news"). The threshold comes from the user's override in cfg (if set and
// positive), otherwise the default-paycheck rule's built-in value; a zero/absent
// threshold disables the alert.
func paycheckLandedCandidates(app *appstate.App, now time.Time, cfg notify.RuleConfig) []notify.Candidate {
	var ruleDefault int64
	for _, r := range notify.DefaultRules() {
		if r.ID == "default-paycheck" {
			ruleDefault = int64(r.Threshold)
		}
	}
	threshold := notify.EffectiveThreshold("default-paycheck", cfg, ruleDefault)
	if threshold <= 0 {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	out := notifyfeed.PaycheckLandedCandidates("default-paycheck", app.Transactions(), threshold, 3, now,
		func(desc string, amount int64) (title, body string) {
			label := desc
			if label == "" {
				label = uistate.T("notify.largeNoDesc")
			}
			return uistate.T("notify.paycheckTitle", fmtBaseMoney(amount, base)),
				uistate.T("notify.paycheckBody", label)
		})
	return out
}

// currentBudgetStatuses evaluates every budget over its own current period (as
// of now), mirroring the Budgets screen, so the budget-threshold notifications
// reflect the same near/over state the user sees. Parent budgets roll up their
// sub-categories' spend (D5). Budgets that fail to evaluate are skipped.
func currentBudgetStatuses(app *appstate.App, now time.Time) []budgeting.Status {
	budgets := app.Budgets()
	if len(budgets) == 0 {
		return nil
	}
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// CurrentPrefs (not the UsePrefs hook) — runNotifyCatchUp runs at boot, outside
	// any component render, where calling a hook panics with "GoUseAtom called
	// outside component context" (caught by recover, but it silently kills catch-up).
	weekStart := uistate.CurrentPrefs().WeekStartWeekday()
	cats := app.Categories()

	out := make([]budgeting.Status, 0, len(budgets))
	for _, b := range budgets {
		bs, be := budgeting.PeriodRange(b.Period, now, weekStart)
		st, err := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs()))
		if err != nil {
			continue
		}
		out = append(out, st)
	}
	return out
}

// loadDeliveredLog reads the delivered-notification keys from localStorage.
func loadDeliveredLog() notify.DeliveredLog {
	log := notify.NewDeliveredLog()
	raw := uistate.KVGet(notifyDeliveredKey)
	if raw == "" {
		return log
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return log
	}
	for _, k := range keys {
		log.Mark(k)
	}
	return log
}

// saveDeliveredLog writes the delivered-notification keys into the SQLite-backed
// app KV (single source of truth; cleared by a wipe like all other data).
func saveDeliveredLog(log notify.DeliveredLog) {
	data, err := json.Marshal(log.Keys())
	if err != nil {
		return
	}
	uistate.KVSet(notifyDeliveredKey, string(data))
}
