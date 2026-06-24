//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// MeterBarProps configures a MeterBar.
type MeterBarProps struct {
	// Value is the current measurement, clamped to [Min, Max] for the fill width.
	Value float64
	// Min and Max bound the scale. Max defaults to 100 when zero so a plain
	// percentage Meter needs only Value set.
	Min, Max float64
	// Label is the accessible name (aria-label) describing what is measured
	// (e.g. "Savings rate"). Required for screen-reader clarity.
	Label string
	// Tone is the fill color token ("bg-up","bg-warn","bg-down","bg-dim","bg-fg");
	// default "bg-up". Shared with ProgressBar's tone vocabulary.
	Tone string
	// Class holds extra spacing tokens for the track ("mt-1.5","mt-2").
	Class string
}

// MeterBar renders a static measurement gauge — disk-usage / savings-rate / "how
// full" — semantically distinct from ProgressBar (which represents task
// progress). It carries the ARIA `meter` role with aria-valuemin/max/now so
// assistive tech announces the reading, and reuses the .cf-bar track styling for
// visual parity with ProgressBar. Display-only (no hooks), safe inside loops.
// (Named MeterBar, not Meter, to avoid clashing with the shorthand <meter> tag.)
func MeterBar(props MeterBarProps) uic.Node {
	min, max := props.Min, props.Max
	if max == 0 {
		max = 100
	}
	v := props.Value
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	pct := 0.0
	if max > min {
		pct = (v - min) / (max - min) * 100
	}

	track := []css.Rule{tw.H2, tw.BgLine, tw.RoundedFull, tw.OverflowHidden}
	switch props.Class {
	case "mt-1.5":
		track = append(track, tw.Mt15)
	case "mt-2":
		track = append(track, tw.Mt2)
	}
	return Div(css.Class("cf-bar", css.New(track...)),
		Attr("role", "meter"),
		Attr("aria-label", props.Label),
		Attr("aria-valuemin", fmt.Sprintf("%g", min)),
		Attr("aria-valuemax", fmt.Sprintf("%g", max)),
		Attr("aria-valuenow", fmt.Sprintf("%g", v)),
		Div(css.Class(css.New(tw.HFull, meterToneRule(props.Tone))),
			Style(map[string]string{"width": fmt.Sprintf("%g%%", pct)})),
	)
}

// meterToneRule maps a tone token to its typed fill-color rule (mirrors
// ProgressBar's toneRule so the two primitives share a color vocabulary).
func meterToneRule(tone string) css.Rule {
	switch tone {
	case "bg-warn":
		return tw.BgWarn
	case "bg-down":
		return tw.BgDown
	case "bg-dim":
		return tw.BgDim
	case "bg-fg":
		return tw.BgFg
	default:
		return tw.BgUp
	}
}
