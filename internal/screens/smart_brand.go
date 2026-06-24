// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// The SMART layer's brand mark is the sparkle glyph (icon.Sparkles), used
// consistently everywhere SMART surfaces: the inline page strips, the /smart hub
// sections, and the AI controls. The Free (deterministic) variant tones the glyph
// neutral; the AI variant tones it with the up/accent color, so "Smart" and
// "Smart AI" share one brand family but read distinctly at a glance.

// smartGlyph renders the SMART brand glyph at a given size token. ai selects the
// AI-accent tone over the neutral Free tone.
func smartGlyph(ai bool, sizeTone ...string) ui.Node {
	tone := "text-dim"
	if ai {
		tone = "text-up"
	}
	cls := tw.ColorClass(tone)
	for _, s := range sizeTone {
		cls += " " + s
	}
	return uiw.Icon(icon.Sparkles, css.Class(cls), Attr("aria-hidden", "true"))
}

// smartBrandHeader builds a branded card header: the sparkle brand glyph + the
// title (matching the auto `.card-head`/`.card-title` styling), with an optional
// trailing action node on the right. Pass ai=true for the AI ("Smart AI") tone.
func smartBrandHeader(title string, ai bool, action ui.Node) ui.Node {
	titleNode := H2(ClassStr("card-title "+tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
		smartGlyph(ai, tw.Fold(tw.W4, tw.H4)),
		Span(title),
	)
	if action == nil {
		return Div(ClassStr("card-head"), titleNode)
	}
	return Div(ClassStr("card-head "+tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2)),
		titleNode,
		action,
	)
}
