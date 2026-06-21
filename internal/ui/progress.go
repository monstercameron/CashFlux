//go:build js && wasm

package ui

import (
	"fmt"

	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ProgressBarProps configures a ProgressBar.
type ProgressBarProps struct {
	Percent int    // fill percentage; clamped to [0, 100] for the width
	Tone    string // fill color class (e.g. "bg-up", "bg-warn", "bg-down", "bg-dim", "bg-fg"); default "bg-up"
	Class   string // extra classes for the track (e.g. spacing like "mt-1.5")
}

// ProgressBar renders the candidate-C bento progress bar: a thin rounded track
// with a colored fill. Display-only (no hooks), so it's a plain helper reused by
// budgets, goals, savings rate, and anywhere a ratio is shown.
func ProgressBar(props ProgressBarProps) uic.Node {
	w := props.Percent
	if w < 0 {
		w = 0
	}
	if w > 100 {
		w = 100
	}
	tone := props.Tone
	if tone == "" {
		tone = "bg-up"
	}
	track := "h-2 bg-line rounded-full overflow-hidden"
	if props.Class != "" {
		track += " " + props.Class
	}
	return Div(ClassStr(track),
		Div(ClassStr("h-full "+tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", w)})),
	)
}
