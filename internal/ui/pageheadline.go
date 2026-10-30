// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// PageHeadlineProps configures the summary that opens a list page: one large figure
// with a plain sentence beside it, then an optional gauge, a row of supporting
// figures, and an optional attention line. Budgets, Transactions and Accounts open
// with this shape, so the reading order is the same on every page.
//
// Every part is a node the caller builds, so each keeps its own test id, tone and
// behaviour. Pass Fragment() for a part that is absent; nil is not allowed.
type PageHeadlineProps struct {
	// Figure is the large value, usually built with PageHeadlineFigure.
	Figure uic.Node
	// Sentence is the line beside the figure. It may carry a delta or a tooltip.
	Sentence uic.Node
	// Visual is an optional picture on the right of the copy, usually PageHeadlineTrend.
	Visual uic.Node
	// Meter is an optional gauge under the figure line, such as a MeterBar.
	Meter uic.Node
	// Facts are the supporting figures, shown in one quiet row (see PageHeadlineFact).
	Facts []uic.Node
	// Attention is an optional line of things that need a look, such as a filter link.
	Attention uic.Node
	// Live announces the figure and sentence to assistive technology when they change.
	Live bool
	// TestID names the headline region for tests and e2e lanes.
	TestID string
	// State is the headline's reading state (for example "ok" or "unknown"), set as
	// data-state so styles and tests can tell an unknown figure from a healthy one.
	State string
}

// PageHeadline renders the shared page summary. It is display-only (no hooks), so a
// page can build it from any render position.
func PageHeadline(props PageHeadlineProps) uic.Node {
	copyArgs := []any{css.Class("page-headline-copy"), props.Figure, Span(css.Class("page-headline-secondary"), props.Sentence)}
	if props.Live {
		copyArgs = append(copyArgs, Attr("role", "status"))
	}
	facts := make([]any, 0, len(props.Facts)+2)
	facts = append(facts, css.Class("page-headline-facts"))
	for _, f := range props.Facts {
		facts = append(facts, f)
	}
	args := []any{css.Class("page-headline"), Attr("data-testid", props.TestID),
		Div(css.Class("page-headline-top"),
			Div(copyArgs...),
			Div(css.Class("page-headline-visual"), props.Visual),
		),
		props.Meter,
		Div(facts...),
		Div(css.Class("page-headline-attention"), props.Attention),
	}
	if props.State != "" {
		args = append(args, Attr("data-state", props.State))
	}
	return Div(args...)
}

// PageHeadlineTrendProps configures the trend picture beside a PageHeadline.
type PageHeadlineTrendProps struct {
	// Values is the series, oldest first. Fewer than two points draws nothing.
	Values []float64
	// Label is the accessible description of the trend.
	Label string
	// GradientID keeps the gradient unique when a page shows more than one trend.
	GradientID string
}

// PageHeadlineTrend draws a trend picture for the right of a PageHeadline. It is the
// shared AreaChart at headline size, with both ends faded into the card so the line
// reads as a shape rather than a box.
func PageHeadlineTrend(props PageHeadlineTrendProps) uic.Node {
	if len(props.Values) < 2 {
		return Fragment()
	}
	gid := props.GradientID
	if gid == "" {
		gid = "page-headline-trend"
	}
	return AreaChart(AreaChartProps{
		Values: props.Values, GradientID: gid, Width: 420, Height: 96, Label: props.Label,
	})
}

// PageHeadlineFigureProps configures the large figure of a PageHeadline.
type PageHeadlineFigureProps struct {
	// Text is the formatted value, for example "$143,459.22".
	Text string
	// Tone is an optional colour class from the text vocabulary ("text-up", "text-down").
	Tone string
	// TestID names the figure for tests.
	TestID string
	// Block renders the figure as a block element, for pages that address it as one.
	Block bool
}

// PageHeadlineFigure renders the large value at the start of a PageHeadline.
func PageHeadlineFigure(props PageHeadlineFigureProps) uic.Node {
	class := css.Class("page-headline-figure", props.Tone)
	if props.Block {
		return Div(class, Attr("data-testid", props.TestID), props.Text)
	}
	return Span(class, Attr("data-testid", props.TestID), props.Text)
}

// PageHeadlineFactProps configures one supporting figure in the headline's fact row.
type PageHeadlineFactProps struct {
	// Label names the figure, for example "Assets".
	Label string
	// Value is the formatted figure.
	Value string
	// Tone is an optional colour class for the value.
	Tone string
	// Href makes the whole fact a link, such as to the page that explains it. Empty
	// keeps it as plain text.
	Href string
	// TestID names the value for tests.
	TestID string
}

// PageHeadlineFact renders one supporting figure: a dim label and a bold value.
func PageHeadlineFact(props PageHeadlineFactProps) uic.Node {
	body := []any{
		Span(css.Class("page-headline-fact-label"), props.Label),
		Span(css.Class("page-headline-fact-value", props.Tone), Attr("data-testid", props.TestID), props.Value),
	}
	if props.Href != "" {
		return A(append([]any{css.Class("page-headline-fact page-headline-fact-link"), Href(props.Href)}, body...)...)
	}
	return Span(append([]any{css.Class("page-headline-fact")}, body...)...)
}
