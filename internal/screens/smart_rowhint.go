// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// smart_rowhint.go is the shared surface for the SM series' item-scoped
// affordances — the "✦ one sentence → do it / not now" line that sits on ONE row
// or card (SM-3, SM-5, SM-6, SM-8…SM-13, SM-16).
//
// It exists because those features differ only in what they detect. Every one of
// them wants the same thing on screen: a quiet sentence, at most one action, and
// a way to make it go away. Ten bespoke versions of that would drift in tone,
// spacing, and dismissal behavior within a release — the SmartFieldAssist chip
// and the smartBadge dot are the same idea at two other sizes, and this is the
// third size, not a fourth idea.
//
// Everything is gated the same way, in one place (smartRowHintFor): the feature
// must be enabled and unmuted, the density dial must permit that KIND of
// affordance, and the hint must not have been dismissed. A caller that forgets a
// gate is the failure mode this consolidation removes.

// smartRowHintProps is one hint. Text is a finished sentence — the caller builds
// it through uistate.T, so no copy is assembled here (screenlint stays at zero).
type smartRowHintProps struct {
	// Key is the stable dismissal key, unique per feature AND per record
	// (e.g. "sm8:" + txnID). Dismissal persists under it.
	Key string
	// TestID is the data-testid suffix, e.g. "sm8-dupe".
	TestID string
	// Text is the hint's one sentence.
	Text string
	// Detail is an optional quieter second line (the evidence behind the sentence).
	Detail string
	// ActionLabel and OnAction are the single "do it" affordance. An empty label
	// renders no button — some hints only need to be said.
	ActionLabel string
	OnAction    func()
	// Tone selects the accent. "warn" for something that costs money if ignored;
	// the default reads as a suggestion, not an alarm.
	Tone string
	// Dismissible adds the "not now" control. Off for hints that are already
	// transient (they stop rendering when the condition clears on its own).
	Dismissible bool
}

// smartRowHintInner holds the hooks. It must be its own component so UseEvent
// sits at a stable render position even when the caller renders hints inside a
// variable-length list of rows.
func smartRowHintInner(props smartRowHintProps) ui.Node {
	act := ui.UseEvent(func() {
		if props.OnAction != nil {
			props.OnAction()
		}
	})
	dismiss := ui.UseEvent(func() {
		uistate.DismissSmartInsight(props.Key)
		uistate.BumpDataRevision()
	})

	cls := "smart-rowhint"
	if props.Tone == "warn" {
		cls += " is-warn"
	}

	var action ui.Node = Fragment()
	if props.ActionLabel != "" && props.OnAction != nil {
		action = Button(css.Class("btn btn-sm smart-rowhint-act"), Type("button"),
			Attr("data-testid", "smart-rowhint-act-"+props.TestID),
			OnClick(act), props.ActionLabel)
	}
	var not ui.Node = Fragment()
	if props.Dismissible {
		not = Button(css.Class("btn-icon-bare smart-rowhint-x"), Type("button"),
			Attr("data-testid", "smart-rowhint-dismiss-"+props.TestID),
			Attr("aria-label", uistate.T("smartHint.dismissAria")),
			Title(uistate.T("smartHint.dismiss")),
			OnClick(dismiss),
			uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3)))
	}

	return Div(css.Class(cls), Attr("data-testid", "smart-rowhint-"+props.TestID),
		smartGlyph(false, tw.Fold(tw.W35, tw.H35, tw.ShrinkO)),
		Div(css.Class("smart-rowhint-body"),
			Div(css.Class("smart-rowhint-text"), props.Text),
			If(props.Detail != "", Div(css.Class("smart-rowhint-detail", tw.Text12, tw.TextDim), props.Detail)),
		),
		action,
		not,
	)
}

// smartRowHintFor renders a hint subject to every gate at once: the feature is
// enabled and not muted, the density dial permits this affordance kind, and the
// hint has not been dismissed. Returns Fragment when any gate closes, so a caller
// can build the props unconditionally and let this decide.
//
// affordance selects which density tier applies: AffordanceFieldAssist for a
// suggest-this-value hint (a split, an amount, a pace), AffordanceBadge for a
// flag about something already true (a duplicate, a spike, a missed charge).
func smartRowHintFor(settings smart.Settings, code string, affordance smart.Affordance, props smartRowHintProps) ui.Node {
	if props.Text == "" || props.Key == "" {
		return Fragment()
	}
	if !settings.IsEnabled(code) || settings.IsMuted(code) {
		return Fragment()
	}
	if !settings.DensityOrDefault().Shows(affordance) {
		return Fragment()
	}
	if settings.IsDismissed(props.Key) {
		return Fragment()
	}
	return ui.CreateElement(smartRowHintInner, props)
}
