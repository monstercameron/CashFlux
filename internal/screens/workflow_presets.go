// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/wfpreset"
	"github.com/monstercameron/CashFlux/internal/workflow"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// wfPresetGalleryProps carries the refresh callback the workflows deck uses to
// re-read the registry after a preset is added.
type wfPresetGalleryProps struct{ Refresh func() }

// wfPresetGallery renders the ready-made automation catalog (C405).
//
// The composer could already express all of these; what it could not do is tell
// someone which ones are worth having. Two templates were not enough runway over
// a blank trigger/condition/actions form, so the gallery ships six complete
// automations with the reasoning done, each stating in plain English when it
// fires and what it does before it is added.
func wfPresetGallery(props wfPresetGalleryProps) ui.Node {
	presets := wfpreset.All()
	cards := make([]any, 0, len(presets))
	for _, p := range presets {
		cards = append(cards, ui.CreateElement(wfPresetCard, wfPresetCardProps{
			Preset: p, Refresh: props.Refresh,
		}))
	}
	return Div(css.Class("wf-quick"), Attr("data-testid", "wf-preset-gallery"),
		H3(css.Class("wf-sec-title"), uistate.T("wfpreset.title")),
		P(css.Class("wf-sec-lede"), uistate.T("wfpreset.lede")),
		Div(css.Class("wf-preset-grid"), cards),
	)
}

type wfPresetCardProps struct {
	Preset  wfpreset.Preset
	Refresh func()
}

// wfPresetCard is one gallery entry. Each card is its own component because it
// owns a click hook — the framework's rule about interactive elements inside a
// variable-length list.
func wfPresetCard(props wfPresetCardProps) ui.Node {
	p := props.Preset
	added := ui.UseState(false)

	add := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		name := uistate.T(p.NameKey)
		wf := p.Instantiate(id.New(), name)
		// Validate before saving even though the catalog is tested: a preset can
		// reference a condition variable that a future refactor renames, and a
		// silent save of a workflow that never fires is the worst failure here.
		if errs := workflow.Validate(wf); len(errs) > 0 {
			uistate.PostNotice(errs[0], true)
			return
		}
		if err := app.PutWorkflow(wf); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		added.Set(true)
		uistate.PostNotice(uistate.T("wfpreset.added", name), false)
		if props.Refresh != nil {
			props.Refresh()
		}
	}))

	// Adding the same preset twice is legal — two "big charge" rules at different
	// thresholds is a reasonable thing to want — so the button stays live and only
	// its label acknowledges the first add.
	label := uistate.T("wfpreset.add")
	if added.Get() {
		label = uistate.T("wfpreset.addAgain")
	}

	return Div(css.Class("wf-preset-card"), Attr("data-testid", "wf-preset-"+p.ID),
		H4(css.Class("wf-preset-name"), uistate.T(p.NameKey)),
		P(css.Class("wf-preset-desc"), uistate.T(p.DescKey)),
		Div(css.Class("wf-preset-meta"),
			Span(css.Class("wf-chip"), triggerLabel(p.Trigger.Kind)),
			Span(css.Class("wf-chip"), actionsLabel(len(p.Actions))),
		),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "wf-preset-add-"+p.ID), OnClick(add), label),
	)
}
