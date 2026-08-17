// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/contradict"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// contradictionsMax is how many findings the strip shows before folding the
// rest behind a count. Three is enough to establish that something is wrong
// without turning the strip into the page.
const contradictionsMax = 3

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
	found := filterContradictions(all, props.Kinds)
	if len(found) == 0 {
		// Nothing at all when the data agrees. A detector that always shows
		// something is one people learn to scroll past.
		return Fragment()
	}

	shown := found
	if !expanded.Get() && len(shown) > contradictionsMax {
		shown = shown[:contradictionsMax]
	}
	rows := make([]any, 0, len(shown))
	for _, f := range shown {
		rows = append(rows, contradictionRow(f, goTo))
	}

	s := contradict.Summarize(found)
	return Div(css.Class("contra-strip"), Attr("data-testid", "contra-strip"),
		Attr("role", "region"), Attr("aria-label", uistate.T("contra.title")),
		Div(css.Class("contra-head"),
			Span(css.Class("contra-title"), uistate.T("contra.title")),
			Span(css.Class("muted"), Attr("data-testid", "contra-count"),
				uistate.T("contra.count", s.Total()))),
		Div(css.Class("contra-rows"), rows),
		If(len(found) > contradictionsMax, Button(css.Class("btn-link"), Type("button"),
			Attr("data-testid", "contra-more"), Attr("aria-expanded", ariaBool(expanded.Get())),
			OnClick(toggle), contradictionsMoreLabel(expanded.Get(), len(found)-contradictionsMax))),
	)
}

// contradictionRow renders one finding: both sides, the size of the
// disagreement where it is a number, and a way to go look.
func contradictionRow(f contradict.Finding, goTo ui.Handler) ui.Node {
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
	)
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
