//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/css"
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
	// TestID, when non-empty, is rendered as data-testid on the section root so a
	// hand-rolled Section(.card) carrying a test id can be ported without losing it.
	TestID string
	// Attrs holds any extra attributes/nodes to place on the section root (e.g. an
	// aria-label or a second data-* attribute) — appended right after the class.
	Attrs []any
	// Body is the card's content area.
	Body uic.Node
}

// Card renders a titled section card (.card + optional .card-title + body).
// It matches the Section(ClassStr("card"), H2(ClassStr("card-title"), …), …) pattern
// repeated across screens so callers need only supply title and body.
func Card(props CardProps) uic.Node {
	args := []any{css.Class("card")}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	if len(props.Attrs) > 0 {
		args = append(args, props.Attrs...)
	}
	if props.Title != "" {
		if props.HeaderAction != nil {
			args = append(args, Div(css.Class("card-header"),
				H2(css.Class("card-title"), props.Title),
				props.HeaderAction,
			))
		} else {
			args = append(args, H2(css.Class("card-title"), props.Title))
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
	return Label(css.Class("labeled-field"),
		Span(css.Class("t-caption", tw.TextDim), label),
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
		Icon(props.Icon, css.Class(tw.W4, tw.H4)),
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
	mainArgs := []any{css.Class("row-main")}
	mainArgs = append(mainArgs, Span(css.Class("row-desc"), props.Title))
	for _, m := range props.Meta {
		mainArgs = append(mainArgs, Span(css.Class("row-meta"), m))
	}

	rowArgs := []any{css.Class("row")}
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
// DeleteButton
// ---------------------------------------------------------------------------

// DeleteButtonProps configures a DeleteButton.
type DeleteButtonProps struct {
	// AriaLabel is the accessible name of the button (required — WCAG 4.1.2).
	// Example: "Delete transaction".
	AriaLabel string
	// Title is the tooltip text shown on hover. Defaults to AriaLabel when empty.
	Title string
	// OnClick is the click handler. May be nil (button renders but does nothing).
	OnClick func()
	// TestID is an optional data-testid attribute for e2e selectors.
	TestID string
}

// DeleteButton renders a destructive icon-only button (.btn-del + Close/Trash icon).
// It consolidates the 18× hand-rolled `.btn-del` + `icon.Close` + `aria-label` pattern
// scattered across screens into a single owned component so callers can safely use it
// inside variable-length loops (each DeleteButton owns its click hook).
func DeleteButton(props DeleteButtonProps) uic.Node {
	return uic.CreateElement(deleteButton, props)
}

func deleteButton(props DeleteButtonProps) uic.Node {
	title := props.Title
	if title == "" {
		title = props.AriaLabel
	}
	onClick := props.OnClick
	args := []any{
		css.Class("btn-del"),
		Type("button"),
		Attr("aria-label", props.AriaLabel),
		Attr("title", title),
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
	}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	args = append(args, Icon(icon.Close, css.Class(tw.W4, tw.H4)))
	return Button(args...)
}

// ---------------------------------------------------------------------------
// ExportButton
// ---------------------------------------------------------------------------

// ExportButtonProps configures an ExportButton.
type ExportButtonProps struct {
	// Label is the visible button text.
	Label string
	// Title is the tooltip / aria-label. Defaults to Label when empty.
	Title string
	// OnClick is called when the button is clicked. The caller is responsible for
	// triggering the download (e.g. via downloadBytes in the screens package).
	// This keeps the primitive free of syscall/js and the screen-private downloadBytes.
	OnClick func()
	// TestID is an optional data-testid attribute for e2e selectors.
	TestID string
}

// ExportButton renders a labeled export/download button. It consolidates the 14×
// hand-rolled inline-flex export button pattern. OnClick is caller-supplied so the
// primitive stays free of the screen-private downloadBytes helper and syscall/js.
// Each ExportButton is its own component and owns its click hook — safe in loops.
func ExportButton(props ExportButtonProps) uic.Node {
	return uic.CreateElement(exportButton, props)
}

func exportButton(props ExportButtonProps) uic.Node {
	title := props.Title
	if title == "" {
		title = props.Label
	}
	onClick := props.OnClick
	args := []any{
		css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
		Type("button"),
		Attr("title", title),
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
	}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	args = append(args, Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)))
	if props.Label != "" {
		args = append(args, Span(props.Label))
	}
	return Button(args...)
}

// ---------------------------------------------------------------------------
// EntityListSection
// ---------------------------------------------------------------------------

// EntityListSectionProps configures an EntityListSection.
type EntityListSectionProps struct {
	// Title is the section heading (.card-title). Required.
	Title string
	// HeaderAction is an optional node placed to the right of the title (e.g. add button).
	HeaderAction uic.Node
	// TestID, when non-empty, lands as data-testid on the section root (preserves a
	// hand-rolled card's test id during a port).
	TestID string
	// Attrs holds extra attributes/nodes for the section root (e.g. aria-label).
	Attrs []any
	// Empty is rendered when Body is nil and EmptyState is non-nil. If both Body
	// and EmptyState are nil, nothing is rendered inside the card body area.
	EmptyState uic.Node
	// Body is the list content (typically a Div(.rows) with mapped rows).
	// When non-nil it is rendered; EmptyState is ignored.
	Body uic.Node
}

// EntityListSection renders the canonical Card + title + (empty-state OR rows body)
// scaffold that appears on every CRUD screen. It absorbs the
// Section(.card) > H2(.card-title) + optional Div(.card-head) + (empty | Div(.rows))
// pattern so screens only supply title, header action, and list body.
func EntityListSection(props EntityListSectionProps) uic.Node {
	var content uic.Node
	if props.Body != nil {
		content = props.Body
	} else if props.EmptyState != nil {
		content = props.EmptyState
	}
	return Card(CardProps{
		Title:        props.Title,
		HeaderAction: props.HeaderAction,
		TestID:       props.TestID,
		Attrs:        props.Attrs,
		Body:         content,
	})
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
		children = append(children, Div(css.Class("stat"),
			Div(css.Class("stat-label"), s.Label),
			Div(ClassStr(valueCls), s.Value),
		))
	}
	return Div(append([]any{css.Class("stat-grid")}, children...)...)
}
