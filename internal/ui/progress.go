// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// ProgressBarProps configures a ProgressBar.
type ProgressBarProps struct {
	Percent int    // fill percentage; clamped to [0, 100] for the width
	Tone    string // fill color token ("bg-up","bg-warn","bg-down","bg-dim","bg-fg"); default "bg-up"
	Class   string // extra track spacing token ("mt-1.5","mt-2"); typed below
}

// toneRule maps a ProgressBar tone token to its typed fill-color rule.
func toneRule(tone string) css.Rule {
	switch tone {
	case "bg-warn":
		return tw.BgWarn
	case "bg-down":
		return tw.BgDown
	case "bg-dim":
		return tw.BgDim
	case "bg-fg":
		return tw.BgFg
	default: // "bg-up" / ""
		return tw.BgUp
	}
}

// appendTrackSpacing adds the optional extra-spacing token as a typed rule.
func appendTrackSpacing(rules []css.Rule, c string) []css.Rule {
	switch c {
	case "mt-1.5":
		return append(rules, tw.Mt15)
	case "mt-2":
		return append(rules, tw.Mt2)
	default:
		return rules
	}
}

// ProgressBar renders the candidate-C bento progress bar: a thin rounded track
// with a colored fill. Display-only (no hooks), so it's a plain helper reused by
// budgets, goals, savings rate, and anywhere a ratio is shown. Its utility styling
// is folded into hashed classes via the css engine (no Tailwind class strings).
func ProgressBar(props ProgressBarProps) uic.Node {
	w := props.Percent
	if w < 0 {
		w = 0
	}
	if w > 100 {
		w = 100
	}
	track := appendTrackSpacing([]css.Rule{tw.H2, tw.BgLine, tw.RoundedFull, tw.OverflowHidden}, props.Class)
	return Div(css.Class(css.New(track...)),
		Div(css.Class(css.New(tw.HFull, toneRule(props.Tone))), Style(map[string]string{"width": fmt.Sprintf("%d%%", w)})),
	)
}
