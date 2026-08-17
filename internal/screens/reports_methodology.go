// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/methodology"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// rptaMethodDrawer renders a report section's "How this is computed" drawer
// (C385), or nothing when the section has no note.
//
// It is a native <details>, not a state-driven panel: it needs no hooks, so it
// can be called from anywhere in the report including inside loops, it survives
// print (an open drawer prints open, which is the right behaviour for a
// print-to-PDF appendix), and it is keyboard-operable without a line of code.
//
// It is closed by default. Methodology is reference material — the reader who
// wants it will look for it, and the reader who does not should not have to
// scroll past it eleven times.
func rptaMethodDrawer(sectionID string) ui.Node {
	note, ok := methodology.For(sectionID)
	if !ok || !note.HasContent() {
		return Fragment()
	}
	var blocks []any

	if len(note.FormulaKeys) > 0 {
		items := make([]any, 0, len(note.FormulaKeys))
		for _, k := range note.FormulaKeys {
			items = append(items, Li(uistate.T(k)))
		}
		blocks = append(blocks, Div(css.Class("rpta-method-block"),
			H4(css.Class("t-caption"), uistate.T("method.howTitle")),
			Ul(css.Class("rpta-method-list"), items)))
	}

	// Benchmarks carry their source in the same line as the value. Splitting them
	// — value here, attribution in a footnote — is how an unsourced-looking claim
	// happens even when the source exists.
	if len(note.Benchmarks) > 0 {
		rows := make([]any, 0, len(note.Benchmarks))
		for _, b := range note.Benchmarks {
			rows = append(rows, Li(
				Span(css.Class("rpta-method-bmk"), uistate.T(b.LabelKey)),
				Span(css.Class("rpta-method-val"), uistate.T(b.ValueKey)),
				Span(css.Class("rpta-method-src"), uistate.T("method.sourcePrefix", uistate.T(b.SourceKey))),
			))
		}
		blocks = append(blocks, Div(css.Class("rpta-method-block"),
			H4(css.Class("t-caption"), uistate.T("method.benchTitle")),
			Ul(css.Class("rpta-method-list", "rpta-method-bmks"), rows)))
	}

	if len(note.ExclusionKeys) > 0 {
		items := make([]any, 0, len(note.ExclusionKeys))
		for _, k := range note.ExclusionKeys {
			items = append(items, Li(uistate.T(k)))
		}
		blocks = append(blocks, Div(css.Class("rpta-method-block"),
			H4(css.Class("t-caption"), uistate.T("method.exclTitle")),
			Ul(css.Class("rpta-method-list"), items)))
	}

	return Details(css.Class("rpta-method"), Attr("data-testid", "rpta-method-"+sectionID),
		Summary(css.Class("rpta-method-sum"), uistate.T("method.drawerLabel")),
		Div(css.Class("rpta-method-body"), blocks),
	)
}
