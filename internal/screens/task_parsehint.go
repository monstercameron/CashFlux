// SPDX-License-Identifier: MIT

//go:build js && wasm

// Reading a date out of a typed to-do (SM-14).
//
// As soon as the title line contains something the app can date — "pay rent
// friday", "review subscriptions every month on the 1st" — a hint under the field
// offers the parse: the cleaned title, the due date, and any repeat. Clicking it
// fills all three fields at once.
//
// It offers rather than applies. Silently rewriting a field somebody is still
// typing in is the single most annoying thing a form can do, and a wrong silent
// parse is worse than no parse: the user has to notice a date they never typed
// before they can correct it.
//
// The free tier is internal/duedate — no key, nothing leaves the device. Smart+
// is offered only for the lines duedate could not read, which is the tail it is
// worth paying for.
package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/duedate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The free and paid halves of SM-14.
const (
	taskParseFreeCode = "SMART-D4F"
	taskParseAICode   = "SMART-D4"
)

// taskParseHintProps carries the raw title line and the callback that fills the
// form when a parse is accepted.
type taskParseHintProps struct {
	// Raw is the current contents of the title field.
	Raw string
	// OnApply fills title, due (zero = leave alone) and repeat ("" = none).
	OnApply func(title string, due time.Time, repeat string)
}

// taskParseHint renders the parse offer under the title field. Its own component:
// it owns click and request hooks, and it re-renders on every keystroke above it.
func taskParseHint(props taskParseHintProps) ui.Node {
	pr := uistate.UsePrefs().Get()
	app := appstate.Default

	aiTitle := ui.UseState("")
	aiDue := ui.UseState(time.Time{})
	aiRepeat := ui.UseState("")
	loading := ui.UseState(false)
	errText := ui.UseState("")

	settings := uistate.LoadSmartSettings()
	raw := strings.TrimSpace(props.Raw)
	local := duedate.Parse(raw, time.Now())

	backendAI := app != nil && aiProviderConfigured(app, pr.Normalize().BackendActive())
	aiEnabled := backendAI && settings.IsEnabled(taskParseAICode) && !settings.IsMuted(taskParseAICode)
	aiConn := smartAIConn{}
	if app != nil {
		aiConn = resolveAIConn(app, pr.Normalize().BackendActive(), pr.ServerURL, pr.ServerToken)
	}

	applyLocal := func() {
		if props.OnApply != nil {
			props.OnApply(local.Title, local.Due, string(local.Cadence))
		}
	}
	applyAI := func() {
		if props.OnApply != nil && aiTitle.Get() != "" {
			props.OnApply(aiTitle.Get(), aiDue.Get(), aiRepeat.Get())
		}
	}
	ask := ui.UseEvent(Prevent(func() {
		if loading.Get() || raw == "" {
			return
		}
		loading.Set(true)
		errText.Set("")
		runSmartAI(aiConn, smartai.TaskParseStructured(raw, time.Now()),
			func(text string) {
				loading.Set(false)
				d, ok := smartai.ParseTaskDraft(strings.TrimSpace(text))
				if !ok {
					errText.Set(uistate.T("sm14.aiNoAnswer"))
					return
				}
				aiTitle.Set(d.Title)
				aiDue.Set(d.Due)
				aiRepeat.Set(d.Repeat)
			},
			func(e string) { loading.Set(false); errText.Set(e) })
	}))

	// Nothing typed yet, or too short to be a sentence: stay out of the way.
	if len(raw) < 4 {
		return Fragment()
	}

	// The local parse. Offered only when it actually changes something — a line the
	// parser read as "no date and the same title" has nothing to apply, and a hint
	// that does nothing when clicked is worse than no hint.
	if local.Found() && local.Title != raw {
		return smartRowHintFor(settings, taskParseFreeCode, smart.AffordanceFieldAssist, smartRowHintProps{
			// Keyed on the parse, not the raw line: dismissing one suggestion should
			// not suppress the next one for a different date.
			Key:         "sm14:" + local.Matched,
			TestID:      "sm14",
			Text:        taskParseSummary(local.Title, local.Due, string(local.Cadence), pr.FormatDate),
			Detail:      uistate.T("sm14.readFrom", local.Matched),
			ActionLabel: uistate.T("sm14.action"),
			OnAction:    applyLocal,
			Dismissible: true,
		})
	}

	if !aiEnabled {
		return Fragment()
	}
	if t := aiTitle.Get(); t != "" {
		return smartRowHintFor(settings, taskParseAICode, smart.AffordanceFieldAssist, smartRowHintProps{
			Key:         "sm14ai:" + t,
			TestID:      "sm14-ai",
			Text:        taskParseSummary(t, aiDue.Get(), aiRepeat.Get(), pr.FormatDate),
			Detail:      uistate.T("catSuggest.aiSource"),
			ActionLabel: uistate.T("sm14.action"),
			OnAction:    applyAI,
			Dismissible: true,
		})
	}
	label := uistate.T("sm14.aiAction")
	if loading.Get() {
		label = uistate.T("sm14.aiPending")
	}
	return Div(css.Class("sm14-ask", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
		Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "sm14-ai-btn"), Attr("aria-disabled", ariaBool(loading.Get())),
			OnClick(ask), smartGlyph(true, tw.Fold(tw.W3, tw.H3)), Span(label)),
		If(errText.Get() != "", Span(css.Class(tw.Text12, tw.TextDim), errText.Get())),
	)
}

// taskRepeatLabels are the ONLY cadences the add form's repeat select offers, and
// therefore the only ones a parse may put in it. duedate can also read "daily",
// "biweekly" and "semimonthly"; writing one of those into the select would set a
// value the control cannot display, leaving the field looking empty while the
// saved task repeats. Those parses keep their date and drop their repeat.
var taskRepeatLabels = map[string]string{
	string(domain.CadenceWeekly):    "todo.repeatWeekly",
	string(domain.CadenceMonthly):   "todo.repeatMonthly",
	string(domain.CadenceQuarterly): "todo.repeatQuarterly",
	string(domain.CadenceYearly):    "todo.repeatYearly",
}

// taskParseSummary renders the parse as the one line the hint shows: the title,
// the due date in the household's own date format, and any repeat the form can
// actually represent.
func taskParseSummary(title string, due time.Time, repeat string, fmtDate func(time.Time) string) string {
	s := title
	if !due.IsZero() && fmtDate != nil {
		s += uistate.T("sm14.dueSuffix", fmtDate(due))
	}
	if key, ok := taskRepeatLabels[repeat]; ok {
		s += uistate.T("sm14.repeatSuffix", uistate.T(key))
	}
	return s
}

// applyTaskParse is the form-side handler: it fills the title, the due field (in
// the app's own date format, so the field the user sees round-trips through the
// same parser the Save button uses), and the repeat select.
func applyTaskParse(setTitle func(string), setDue func(string), setRepeat func(string)) func(string, time.Time, string) {
	return func(title string, due time.Time, repeat string) {
		if title != "" {
			setTitle(title)
		}
		if !due.IsZero() {
			setDue(dateutil.FormatDate(due))
		}
		if _, ok := taskRepeatLabels[repeat]; ok {
			setRepeat(repeat)
		}
	}
}
