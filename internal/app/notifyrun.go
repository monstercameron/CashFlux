//go:build js && wasm

package app

import (
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/notifyfeed"
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
	defer func() { _ = recover() }()

	app := appstate.Default
	if app == nil {
		return
	}
	now := time.Now()
	accounts := app.Accounts()

	var cands []notify.Candidate
	cands = append(cands, notifyfeed.StaleBalanceCandidates("default-stale", accounts, app.FreshnessWindows(), now,
		func(name string, days int) (title, body string) {
			return uistate.T("notify.staleTitle", name), uistate.T("notify.staleBody", days)
		})...)
	cands = append(cands, notifyfeed.BillDueCandidates("default-bill-due", bills.Upcoming(accounts, now), 7, now,
		func(name string, days int) (title, body string) {
			return uistate.T("notify.billTitle", name), uistate.T("notify.billBody", days)
		})...)

	log := loadDeliveredLog()
	out := notify.CatchUp(notify.DefaultRules(), cands, now, log)
	saveDeliveredLog(log)
	if len(out) == 0 {
		return
	}

	// One unobtrusive summary toast: the single reminder's title, or a count.
	msg := out[0].Title
	if len(out) > 1 {
		msg = uistate.T("notify.summary", len(out))
	}
	notice := uistate.UseNotice()
	notice.Set(notice.Get().With(msg, false))
}

// loadDeliveredLog reads the delivered-notification keys from localStorage.
func loadDeliveredLog() notify.DeliveredLog {
	log := notify.NewDeliveredLog()
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return log
	}
	v := ls.Call("getItem", notifyDeliveredKey)
	if !v.Truthy() {
		return log
	}
	var keys []string
	if err := json.Unmarshal([]byte(v.String()), &keys); err != nil {
		return log
	}
	for _, k := range keys {
		log.Mark(k)
	}
	return log
}

// saveDeliveredLog writes the delivered-notification keys back to localStorage.
func saveDeliveredLog(log notify.DeliveredLog) {
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return
	}
	data, err := json.Marshal(log.Keys())
	if err != nil {
		return
	}
	ls.Call("setItem", notifyDeliveredKey, string(data))
}
