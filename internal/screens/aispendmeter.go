// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// AISpendMeter shows what the AI features have actually cost this month, split by
// the feature that spent it, with an optional monthly ceiling (EC-15).
//
// Every AI surface already showed an estimate before a call and a token line after
// it, but nothing added them up — so the honest answer to "what is this costing
// me?" was "look at your provider's bill", which is the wrong place and a month
// late. This is that answer, in the app, while the month is still running.
//
// Three things it is careful about. A month with nothing recorded says so rather
// than showing $0.00, because "nothing yet" and "nothing spent" are different
// statements. A total that includes calls on an unpriced model reads as "at least
// $X", because it is a floor. And the projection refuses to extrapolate from the
// first day or two of a month, since multiplying one day by thirty-one produces a
// number that is alarming and meaningless in equal measure.
func AISpendMeter() ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	editing := ui.UseState(false)
	draft := ui.UseState("")
	startEdit := ui.UseEvent(Prevent(func() {
		if app != nil && app.AISpendCap() > 0 {
			draft.Set(strconv.FormatFloat(app.AISpendCap(), 'f', -1, 64))
		}
		editing.Set(true)
	}))
	onDraft := ui.UseEvent(func(e ui.Event) { draft.Set(e.GetValue()) })
	saveCap := ui.UseEvent(Prevent(func() {
		editing.Set(false)
		if app == nil {
			return
		}
		// An unparseable or empty box clears the cap rather than erroring: the
		// most likely reason somebody emptied it is that they want no cap.
		usd, _ := strconv.ParseFloat(strings.TrimSpace(draft.Get()), 64)
		if err := app.SetAISpendCap(usd); err == nil {
			uistate.BumpDataRevision()
			uistate.RequestPersist()
		}
	}))
	clearCap := ui.UseEvent(Prevent(func() {
		editing.Set(false)
		if app == nil {
			return
		}
		if err := app.SetAISpendCap(0); err == nil {
			uistate.BumpDataRevision()
			uistate.RequestPersist()
		}
	}))

	if app == nil {
		return Fragment()
	}
	summary := app.AISpendThisMonth()
	capUSD := app.AISpendCap()

	if !summary.Recorded {
		return Div(css.Class("ai-meter"), Attr("data-testid", "ai-spend-meter"),
			P(css.Class("ai-meter-title"), uistate.T("aispend.title")),
			P(css.Class("ai-meter-empty"), Attr("data-testid", "ai-spend-empty"), uistate.T("aispend.none")),
		)
	}

	rows := make([]any, 0, len(summary.ByFeature))
	for _, f := range summary.ByFeature {
		rows = append(rows, Li(css.Class("ai-meter-row"),
			Span(css.Class("ai-meter-feature"), uistate.T(aiSpendFeatureKey(f.Feature))),
			Span(css.Class("ai-meter-calls"), uistate.T("aispend.calls", f.Calls)),
			Span(css.Class("ai-meter-cost"), aiSpendCostText(f.CostUSD, f.UnpricedCalls)),
		))
	}
	list := append([]any{css.Class("ai-meter-list"), Attr("data-testid", "ai-spend-by-feature")}, rows...)

	return Div(css.Class("ai-meter"), Attr("data-testid", "ai-spend-meter"),
		P(css.Class("ai-meter-title"), uistate.T("aispend.title")),
		Div(css.Class("ai-meter-total"),
			Span(css.Class("ai-meter-figure"), Attr("data-testid", "ai-spend-total"),
				aiSpendCostText(summary.CostUSD, summary.UnpricedCalls)),
			Span(css.Class("ai-meter-sub"), uistate.T("aispend.thisMonth",
				ai.FormatTokens(summary.Tokens), summary.Calls)),
		),
		aiSpendPaceNote(summary, capUSD),
		Ul(list...),
		aiSpendCapControl(aiSpendCapProps{
			CapUSD: capUSD, Editing: editing.Get(), Draft: draft.Get(),
			OnStart: startEdit, OnDraft: onDraft, OnSave: saveCap, OnClear: clearCap,
		}),
	)
}

// aiSpendCostText renders a cost, marking it as a floor when some of the calls it
// covers ran on a model whose price is unknown.
func aiSpendCostText(cost float64, unpriced int) string {
	if unpriced > 0 {
		return uistate.T("aispend.atLeast", ai.FormatCostUSD(cost))
	}
	return ai.FormatCostUSD(cost)
}

// aiSpendFeatureKey maps a recorded feature name to its copy key, falling back to
// the raw name so a surface that starts recording without adding copy still shows
// up in the split rather than vanishing from the total.
func aiSpendFeatureKey(feature string) string {
	switch feature {
	case "assistant", "smart", "receipt", "categorize", "other":
		return "aispend.feature." + feature
	default:
		return feature
	}
}

// aiSpendPaceNote says how the month is tracking against the cap. It renders
// nothing when there is no cap and nothing worth saying — a meter with no limit
// set is a report, not a warning.
func aiSpendPaceNote(summary aispend.Summary, capUSD float64) ui.Node {
	pace := summary.PaceAgainst(capUSD, time.Now())
	if pace == aispend.PaceNoCap || pace == aispend.PaceComfortable {
		return Fragment()
	}
	// A month containing calls on an unpriced model cannot be judged against a cap
	// at all: the recorded figure is a floor, and the real total could be anywhere
	// above it. Saying so is the only honest option — the alternative is a
	// reassuring verdict computed from a number that is knowingly incomplete.
	if pace == aispend.PaceUnknown {
		return P(css.Class("ai-meter-pace"), Attr("data-testid", "ai-spend-pace"), Attr("role", "status"),
			uistate.T("aispend.paceUnknown", ai.FormatCostUSD(capUSD)))
	}
	key, cls := "aispend.paceTight", "ai-meter-pace"
	switch pace {
	case aispend.PaceOverPace:
		key, cls = "aispend.paceOver", "ai-meter-pace is-warn"
	case aispend.PaceExceeded:
		key, cls = "aispend.paceExceeded", "ai-meter-pace is-over"
	}
	projected := ai.FormatCostUSD(capUSD)
	if p, ok := summary.ProjectSpend(time.Now()); ok {
		projected = ai.FormatCostUSD(p)
	}
	return P(ClassStr(cls), Attr("data-testid", "ai-spend-pace"), Attr("role", "status"),
		uistate.T(key, projected, ai.FormatCostUSD(capUSD)))
}

type aiSpendCapProps struct {
	CapUSD  float64
	Editing bool
	Draft   string
	OnStart ui.Handler
	OnDraft ui.Handler
	OnSave  ui.Handler
	OnClear ui.Handler
}

// aiSpendCapControl renders the monthly-ceiling control. It takes already-bound
// handlers so its own render allocates no hooks — the meter owns them, at stable
// positions.
func aiSpendCapControl(p aiSpendCapProps) ui.Node {
	if p.Editing {
		return Div(css.Class("ai-meter-cap"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Attr("step", "0.5"),
				Attr("data-testid", "ai-spend-cap-input"),
				Attr("aria-label", uistate.T("aispend.capAria")),
				Value(p.Draft), OnInput(p.OnDraft)),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"),
				Attr("data-testid", "ai-spend-cap-save"), OnClick(p.OnSave), uistate.T("action.save")),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "ai-spend-cap-clear"), OnClick(p.OnClear), uistate.T("aispend.capClear")),
		)
	}
	label := uistate.T("aispend.capNone")
	if p.CapUSD > 0 {
		label = uistate.T("aispend.capSet", ai.FormatCostUSD(p.CapUSD))
	}
	return Div(css.Class("ai-meter-cap"),
		Button(css.Class("btn-link", tw.Text12), Type("button"),
			Attr("data-testid", "ai-spend-cap"), OnClick(p.OnStart), label),
	)
}
