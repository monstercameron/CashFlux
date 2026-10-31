// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
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
	// Tone is the fill color token ("bg-up","bg-warn","bg-down","bg-dim","bg-fg","bg-accent");
	// default "bg-up". "bg-accent" tracks the theme accent. Shared with ProgressBar's vocab.
	Tone string
	// Class holds extra spacing tokens for the track ("mt-1.5","mt-2").
	Class string
	// PaceMarker, when HasPaceMarker is set, draws a thin tick on the track at this
	// value, for example how far through a period the reading is. It is decorative;
	// the reading itself stays on the meter's ARIA values.
	PaceMarker    float64
	HasPaceMarker bool
	// TestID names the meter for tests. Empty adds no test id.
	TestID string
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
	pct := meterPercent(v, min, max)

	track := []css.Rule{tw.H2, tw.BgTrack, tw.RoundedFull, tw.OverflowHidden}
	switch props.Class {
	case "mt-1.5":
		track = append(track, tw.Mt15)
	case "mt-2":
		track = append(track, tw.Mt2)
	}
	children := []any{
		css.Class("cf-bar", css.New(track...)),
		Attr("role", "meter"),
		Attr("aria-label", props.Label),
		Attr("aria-valuemin", fmt.Sprintf("%g", min)),
		Attr("aria-valuemax", fmt.Sprintf("%g", max)),
		Attr("aria-valuenow", fmt.Sprintf("%g", v)),
		Div(css.Class(css.New(tw.HFull, meterToneRule(props.Tone))),
			Style(map[string]string{"width": fmt.Sprintf("%g%%", pct)})),
	}
	if props.TestID != "" {
		children = append(children, Attr("data-testid", props.TestID))
	}
	if props.HasPaceMarker {
		children = append(children, Div(css.Class("cf-bar-pace"), Attr("aria-hidden", "true"),
			Style(map[string]string{"left": fmt.Sprintf("%g%%", meterPercent(props.PaceMarker, min, max))})))
	}
	return Div(children...)
}

// meterPercent maps v onto the [min, max] scale as a percentage, 0 when the scale is empty.
func meterPercent(v, min, max float64) float64 {
	if max <= min {
		return 0
	}
	return (v - min) / (max - min) * 100
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
	case "bg-accent":
		return tw.BgAccent
	default:
		return tw.BgUp
	}
}
