//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

// CardProps configures a titled section card.
type CardProps struct {
	// Title is optional. When non-empty a styled <h2 class="card-title"> is rendered.
	Title string
	// HeaderAction is an optional node placed to the right of the title row
	// (e.g. an IconButton for a section-level action). Ignored when Title is empty.
	HeaderAction uic.Node
	// Body is the card's content area.
	Body uic.Node
}

// Card renders a titled section card (.card + optional .card-title + body).
// It matches the Section(ClassStr("card"), H2(ClassStr("card-title"), …), …) pattern
// repeated across screens so callers need only supply title and body.
func Card(props CardProps) uic.Node {
	args := []any{ClassStr("card")}
	if props.Title != "" {
		if props.HeaderAction != nil {
			args = append(args, Div(ClassStr("card-header"),
				H2(ClassStr("card-title"), props.Title),
				props.HeaderAction,
			))
		} else {
			args = append(args, H2(ClassStr("card-title"), props.Title))
		}
	}
	if props.Body != nil {
		args = append(args, props.Body)
	}
	return Section(args...)
}

// ---------------------------------------------------------------------------
// FormField
// ---------------------------------------------------------------------------

// FormField renders a labeled field — a visible caption above the control —
// matching the .labeled-field pattern from accounts.go's labeledField helper.
// The <label> element associates the caption with the control for a11y; callers
// should put an id on the control and pass a matching htmlFor when needed.
func FormField(label string, control uic.Node) uic.Node {
	return Label(ClassStr("labeled-field"),
		Span(ClassStr("t-caption text-dim"), label),
		control,
	)
}

// ---------------------------------------------------------------------------
// IconButton
// ---------------------------------------------------------------------------

// IconButtonProps configures an icon-only button.
type IconButtonProps struct {
	// Icon is the glyph to render inside the button.
	Icon icon.Name
	// Label is the accessible name (aria-label + title). Required — icon-only
	// buttons must always have an explicit label (WCAG 4.1.2).
	Label string
	// OnClick is the click handler. May be nil (button renders but does nothing).
	OnClick func()
	// Class is extra CSS classes to append to the base class string.
	Class string
	// Danger styles the button as a destructive action (.btn-del).
	Danger bool
}

// IconButton renders an accessible icon-only button. It is its own component
// so callers can safely use it inside variable-length loops (the On*-hooks-in-
// loops rule): each IconButton instance owns its click hook.
func IconButton(props IconButtonProps) uic.Node {
	return uic.CreateElement(iconButton, props)
}

func iconButton(props IconButtonProps) uic.Node {
	cls := "btn"
	if props.Danger {
		cls += " btn-del"
	}
	if props.Class != "" {
		cls += " " + props.Class
	}
	onClick := props.OnClick
	return Button(
		ClassStr(cls),
		Type("button"),
		Attr("aria-label", props.Label),
		Attr("title", props.Label),
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		Icon(props.Icon, ClassStr("w-4 h-4")),
	)
}

// ---------------------------------------------------------------------------
// EntityRow
// ---------------------------------------------------------------------------

// EntityRowProps configures a generic list row.
type EntityRowProps struct {
	// Leading is an optional node placed before the text block (e.g. avatar, color swatch).
	Leading uic.Node
	// Title is the primary row descriptor (.row-desc).
	Title string
	// Meta are secondary descriptor lines (.row-meta). Each entry is rendered as a
	// separate span so callers can pass formatted strings or node-like strings.
	Meta []string
	// Actions are pre-built interactive nodes placed in the trailing slot.
	// Because they are already-constructed nodes (not callbacks), EntityRow itself
	// registers no per-row hooks and is safe to call inside a MapKeyed loop.
	// Per-row interactive elements (IconButton, etc.) must be built as their own
	// components by the caller before being passed here.
	Actions []uic.Node
}

// EntityRow renders a generic list row (.row) with a leading slot, a .row-main
// text block (.row-desc + .row-meta lines), and a trailing actions slot.
// EntityRow owns no click hooks itself — all interactivity is passed as pre-built
// action nodes — so it is safe to call directly inside a variable-length loop.
func EntityRow(props EntityRowProps) uic.Node {
	mainArgs := []any{ClassStr("row-main")}
	mainArgs = append(mainArgs, Span(ClassStr("row-desc"), props.Title))
	for _, m := range props.Meta {
		mainArgs = append(mainArgs, Span(ClassStr("row-meta"), m))
	}

	rowArgs := []any{ClassStr("row")}
	if props.Leading != nil {
		rowArgs = append(rowArgs, props.Leading)
	}
	rowArgs = append(rowArgs, Div(mainArgs...))
	for _, a := range props.Actions {
		rowArgs = append(rowArgs, a)
	}
	return Div(rowArgs...)
}

// ---------------------------------------------------------------------------
// StatGrid
// ---------------------------------------------------------------------------

// Stat is a single figure in a stat grid: a label, a formatted value, and an
// optional tone class (e.g. "pos", "neg", "dim") applied to the value element.
type Stat struct {
	Label string
	Value string
	// Tone is an optional CSS class appended to the .stat-value span
	// (e.g. "pos", "neg", "dim"). Leave empty for the default colour.
	Tone string
}

// StatGrid renders a .stat-grid of figures, generalising the stat() helper
// repeated across screens (accounts, transactions, goals, etc.).
func StatGrid(stats []Stat) uic.Node {
	children := make([]any, 0, len(stats))
	for _, s := range stats {
		valueCls := "stat-value"
		if s.Tone != "" {
			valueCls += " " + s.Tone
		}
		children = append(children, Div(ClassStr("stat"),
			Div(ClassStr("stat-label"), s.Label),
			Div(ClassStr(valueCls), s.Value),
		))
	}
	return Div(append([]any{ClassStr("stat-grid")}, children...)...)
}
