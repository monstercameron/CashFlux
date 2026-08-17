// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// smart_explain.go is the shared "explain this one thing" affordance behind SM-5
// (why did this balance move?), SM-7 (what does this alert mean?) and SM-13 (what
// would land this goal on its date?).
//
// Both are the same shape: the app has already detected something and stated it
// deterministically, and the paid tier adds context and a next step for THAT one
// item. Neither asks the model to detect anything — the finding is handed over
// finished, which is what keeps the answer anchored to figures the app can defend.
//
// It is one component rather than two because the only real difference is the
// prompt, and two copies of a request/loading/error/answer cycle drift.

// notifyExplainCode gates SM-7 on the notification feed. It is a Hub-page code
// because a notification is not owned by any one page's feature set — the same
// feed carries budget, bill, account and goal alerts.
const notifyExplainCode = "SMART-EXPLAIN"

// explainKind selects which prompt frames the ask.
type explainKind string

const (
	// explainAlert glosses a notification (SM-7).
	explainAlert explainKind = "alert"
	// explainBalance explains an unusual balance move (SM-5).
	explainBalance explainKind = "balance"
	// explainGoalPace adds one line to a goal's trajectory (SM-13).
	explainGoalPace explainKind = "goalpace"
)

// smartExplainProps configures one explain affordance.
type smartExplainProps struct {
	// ID is the data-testid suffix, unique on the page.
	ID string
	// Code is the AI catalog code that gates it (SMART-EXPLAIN / SMART-A12).
	Code string
	Kind explainKind
	// Title and Body are the thing being explained, in the app's own words.
	Title, Body string
	// Context is any extra figures worth grounding the answer in. Optional.
	Context string
	// Label overrides the button text. Empty uses the shared default.
	Label string
}

// smartExplainBlock renders the ask button and, once answered, the answer. Its
// own component: it owns the request hooks and frequently renders inside a list.
//
// It renders NOTHING when the feature is off, muted, or no provider is
// configured. A control that cannot work is worse than an absent one — it reads
// as broken rather than as unconfigured, and the place to configure it is a page
// away in Settings.
func smartExplainBlock(props smartExplainProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default

	answer := ui.UseState("")
	loading := ui.UseState(false)
	errText := ui.UseState("")

	settings := uistate.LoadSmartSettings()
	backendAI := pr.Normalize().BackendActive()
	enabled := app != nil && aiProviderConfigured(app, backendAI) &&
		settings.IsEnabled(props.Code) && !settings.IsMuted(props.Code)
	aiConn := smartAIConn{}
	if app != nil {
		aiConn = resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	}

	ask := ui.UseEvent(Prevent(func() {
		if loading.Get() {
			return
		}
		loading.Set(true)
		errText.Set("")
		var req smartai.Request
		switch props.Kind {
		case explainBalance:
			req = smartai.BalanceAnomaly(strings.TrimSpace(props.Title + "\n" + props.Body + "\n" + props.Context))
		case explainGoalPace:
			req = smartai.GoalPace(strings.TrimSpace(props.Title + "\n" + props.Body + "\n" + props.Context))
		default:
			req = smartai.ExplainAlert(props.Title, props.Body, props.Context)
		}
		runSmartAI(aiConn, req,
			func(text string) { loading.Set(false); answer.Set(strings.TrimSpace(text)) },
			func(e string) { loading.Set(false); errText.Set(e) })
	}))

	if !enabled || strings.TrimSpace(props.Title) == "" {
		return Fragment()
	}

	label := props.Label
	if label == "" {
		label = uistate.T("smartExplain.action")
	}
	if loading.Get() {
		label = uistate.T("smartExplain.pending")
	}

	return Div(css.Class("smart-explain"), Attr("data-testid", "smart-explain-"+props.ID),
		Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "smart-explain-btn-"+props.ID),
			Attr("aria-disabled", ariaBool(loading.Get())),
			OnClick(ask),
			smartGlyph(true, tw.Fold(tw.W3, tw.H3)), Span(label)),
		If(answer.Get() != "", P(css.Class("smart-explain-answer", tw.Text13),
			Attr("data-testid", "smart-explain-answer-"+props.ID), answer.Get())),
		If(errText.Get() != "", P(css.Class("err", tw.Text12), Attr("role", "alert"), errText.Get())),
	)
}
