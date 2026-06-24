// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// SkeletonProps configures a Skeleton placeholder.
type SkeletonProps struct {
	// Lines is the number of shimmer bars to render (default 3). The last bar is
	// rendered short to mimic a trailing line of text.
	Lines int
	// Class holds extra class names for the wrapper (e.g. spacing utilities).
	Class string
	// AriaLabel overrides the default "Loading…" label announced to screen readers.
	AriaLabel string
}

// Skeleton renders a shimmer placeholder for content that is still loading — a
// stack of animated bars wrapped in a polite, busy live region. Display-only (no
// hooks), so it's safe anywhere, including inside lists. The shimmer animation is
// defined in web/index.html (.cf-skeleton-bar) and is disabled under
// prefers-reduced-motion to respect the app's a11y stance.
func Skeleton(props SkeletonProps) uic.Node {
	n := props.Lines
	if n <= 0 {
		n = 3
	}
	label := props.AriaLabel
	if label == "" {
		label = "Loading…"
	}
	bars := make([]any, 0, n+1)
	cls := "cf-skeleton"
	if props.Class != "" {
		cls += " " + props.Class
	}
	bars = append(bars, ClassStr(cls), Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-busy", "true"), Attr("aria-label", label))
	for i := 0; i < n; i++ {
		barCls := "cf-skeleton-bar"
		if i == n-1 && n > 1 {
			barCls += " cf-skeleton-bar--short"
		}
		bars = append(bars, Div(css.Class(barCls), Attr("aria-hidden", "true")))
	}
	return Div(bars...)
}
