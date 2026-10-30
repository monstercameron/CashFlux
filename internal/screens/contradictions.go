// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/contradict"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// contradictionsMax is how many findings the strip shows before folding the
// rest behind a count. Three is enough to establish that something is wrong
// without turning the strip into the page.
const contradictionsMax = 3

// contradictionsDismissedKV is the dataset key holding the finding keys the
// household has dismissed. It is stored in the dataset rather than the browser so
// that the undo stack records dismissing and bringing back like any other change.
const contradictionsDismissedKV = "contradictions.dismissed"

// contradictionStripProps scopes the strip to one surface.
type contradictionStripProps struct {
	// Only findings about these entity kinds are shown ("account",
	// "transaction", "budget", "task"). Empty shows all of them.
	Kinds []string
}

// contradictionStrip surfaces places where the app's own data disagrees with
// itself, on the page they concern (E3).
//
// It is deliberately not a destination. A "run a consistency check" page is one
// people visit twice: once out of curiosity and once after something has already
// gone wrong. An invariant that has to be remembered is broken most of the time,
// which is why this renders inline, always, and disappears entirely when the
// data agrees with itself.
//
// Every row states BOTH sides. "Something is wrong with this transfer" is a
// complaint; "money left this account, but nothing arrived in the other one" is
// a contradiction the reader can act on, because they can see which half to fix.
//
// A reader can dismiss a row they have already dealt with, or know is intentional.
// Dismissals are keyed on the finding, so they survive the amounts changing and end
// when the contradiction is gone. Dismissing and "Bring back" post an undoable
// notice, so a mistaken click is one Undo away.
func contradictionStrip(props contradictionStripProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()
	expanded := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	goTo := ui.UseEvent(func(e ui.Event) {
		route := e.JSValue().Get("currentTarget").Call("getAttribute", "data-route").String()
		if route != "" && route != "<null>" {
			nav.Navigate(uistate.RoutePath(route))
		}
	})
	dismiss := ui.UseEvent(func(e ui.Event) {
		key := e.JSValue().Get("currentTarget").Call("getAttribute", "data-key").String()
		app := appstate.Default
		if app == nil || key == "" || key == "<null>" {
			return
		}
		next := dismissedContradictions(app)
		next[key] = true
		setDismissedContradictions(app, next, uistate.T("contra.dismissedNotice"))
	})
	restore := ui.UseEvent(func(_ ui.Event) {
		app := appstate.Default
		if app == nil {
			return
		}
		n := len(dismissedContradictions(app))
		setDismissedContradictions(app, map[string]bool{}, uistate.T("contra.restoredNotice", n))
	})

	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	all := contradict.Check(contradict.Data{
		Accounts:     app.Accounts(),
		Transactions: app.Transactions(),
		Holdings:     app.Holdings(),
		Budgets:      app.Budgets(),
		Tasks:        app.Tasks(),
		Goals:        app.Goals(),
		Categories:   app.Categories(),
		Now:          time.Now(),
	})
	scoped := filterContradictions(all, props.Kinds)
	if len(scoped) == 0 {
		// Nothing at all when the data agrees. A detector that always shows
		// something is one people learn to scroll past.
		return Fragment()
	}
	found := contradict.Visible(scoped, dismissedContradictions(app))
	hidden := len(scoped) - len(found)
	if len(found) == 0 {
		// Everything was dismissed on purpose, so there is nothing left to show. Undo on the
		// dismissal toast brings the last one back.
		return Fragment()
	}

	shown := found
	if !expanded.Get() && len(shown) > contradictionsMax {
		shown = shown[:contradictionsMax]
	}
	rows := make([]any, 0, len(shown))
	for _, f := range shown {
		rows = append(rows, contradictionRow(f, goTo, dismiss))
	}

	s := contradict.Summarize(found)
	return Div(css.Class("contra-strip"), Attr("data-testid", "contra-strip"),
		Attr("role", "region"), Attr("aria-label", uistate.T("contra.title")),
		Div(css.Class("contra-head"),
			Span(css.Class("contra-title"), uistate.T("contra.title")),
			Span(css.Class("muted"), Attr("data-testid", "contra-count"),
				uistate.T("contra.count", s.Total())),
			If(hidden > 0, Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "contra-restore"),
				OnClick(restore), uistate.T("contra.restore", hidden)))),
		Div(css.Class("contra-rows"), rows),
		If(len(found) > contradictionsMax, Button(css.Class("btn-link"), Type("button"),
			Attr("data-testid", "contra-more"), Attr("aria-expanded", ariaBool(expanded.Get())),
			OnClick(toggle), contradictionsMoreLabel(expanded.Get(), len(found)-contradictionsMax))),
	)
}

// contradictionRow renders one finding: both sides, the size of the
// disagreement where it is a number, a way to go look, and a way to dismiss it.
func contradictionRow(f contradict.Finding, goTo, dismiss ui.Handler) ui.Node {
	tone := "contra-notice"
	switch f.Severity {
	case contradict.SeverityCritical:
		tone = "contra-critical"
	case contradict.SeverityWarning:
		tone = "contra-warning"
	}
	// The delta is the reader's first question — "how wrong is it?" — so it sits
	// beside the sentence rather than behind a click.
	var delta ui.Node = Fragment()
	if f.HasDelta {
		delta = Span(css.Class("contra-delta"),
			fmtMoney(money.New(absMinor(f.DeltaMinor), baseCurrencyOrUSD())))
	}
	route := contradictionRoute(f.EntityKind)
	return Div(ClassStr("contra-row "+tone), Attr("data-testid", "contra-"+f.Key),
		Span(css.Class("contra-text"), contradict.Describe(f)),
		delta,
		If(route != "", Button(css.Class("btn-link"), Type("button"),
			Attr("data-route", route), Attr("data-testid", "contra-go-"+f.Key),
			OnClick(goTo), uistate.T("contra.look"))),
		Button(css.Class("btn-link"), Type("button"),
			Attr("data-key", f.Key), Attr("data-testid", "contra-dismiss-"+f.Key),
			OnClick(dismiss), uistate.T("contra.dismiss")),
	)
}

// dismissedContradictions returns the finding keys the household has dismissed. A
// missing or unreadable value means nothing is dismissed.
func dismissedContradictions(app *appstate.App) map[string]bool {
	out := map[string]bool{}
	raw, ok := app.GetKV(contradictionsDismissedKV)
	if !ok || raw == "" {
		return out
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return out
	}
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// setDismissedContradictions stores the dismissed set as one dataset write, so a
// single Undo reverses it, and posts the undoable notice.
func setDismissedContradictions(app *appstate.App, dismissed map[string]bool, notice string) {
	keys := make([]string, 0, len(dismissed))
	for k := range dismissed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b, err := json.Marshal(keys)
	if err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	if err := app.SetKV(contradictionsDismissedKV, string(b)); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	uistate.PostUndoable(notice)
}

// filterContradictions narrows findings to the entity kinds a surface owns, so
// the accounts page does not report a budget problem it cannot help with.
func filterContradictions(fs []contradict.Finding, kinds []string) []contradict.Finding {
	if len(kinds) == 0 {
		return fs
	}
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	out := make([]contradict.Finding, 0, len(fs))
	for _, f := range fs {
		if want[f.EntityKind] {
			out = append(out, f)
		}
	}
	return out
}

// contradictionRoute maps an entity kind to where the reader can go look at it.
func contradictionRoute(kind string) string {
	switch kind {
	case "transaction":
		return "/transactions"
	case "account":
		return "/accounts"
	case "budget":
		return "/budgets"
	case "task":
		return "/todo"
	case "goal":
		return "/goals"
	}
	return ""
}

func contradictionsMoreLabel(expanded bool, hidden int) string {
	if expanded {
		return uistate.T("contra.less")
	}
	return uistate.T("contra.more", hidden)
}

// baseCurrencyOrUSD is the household base currency, defaulting to USD so a
// delta always renders as money rather than as a bare number.
func baseCurrencyOrUSD() string {
	if app := appstate.Default; app != nil {
		if c := app.Settings().BaseCurrency; c != "" {
			return c
		}
	}
	return "USD"
}
