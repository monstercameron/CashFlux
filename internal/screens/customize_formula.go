// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// FormulaCalculator is the formula-calculator screen body used by Customize() and the
// Studio "Formulas" tab. It is now a thin wrapper over the reusable, embeddable
// FormulaBuilder component (formula_builder.go) with the saved-formulas list shown —
// so the exact same builder can be embedded on any other page.
func FormulaCalculator() ui.Node {
	return FormulaBuilder(FormulaBuilderProps{ShowSaved: true})
}

// savedFormulasCard lists the user's saved formulas, each evaluated live against
// the current figures, with load-into-editor and delete actions. Hidden when
// there are none.
func savedFormulasCard(formulas []domain.Formula, vars map[string]float64, onLoad func(domain.Formula), onDelete func(string)) ui.Node {
	if len(formulas) == 0 {
		return Fragment()
	}
	rows := make([]ui.Node, 0, len(formulas))
	for _, f := range formulas {
		rows = append(rows, ui.CreateElement(SavedFormulaRow, savedFormulaRowProps{
			Formula: f, Result: evalFormulaDisplay(f.Expr, vars), OnLoad: onLoad, OnDelete: onDelete,
		}))
	}
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("customize.savedTitle"),
		Rows:  rows,
	})
}

// evalFormulaDisplay evaluates an expression against the live vars and returns the
// formatted result, or the error text on failure.
func evalFormulaDisplay(expr string, vars map[string]float64) string {
	v, err := formula.Eval(expr, formula.Env{Vars: vars})
	if err != nil {
		return uistate.T("customize.evalError")
	}
	return formatFormulaValue(v)
}

type savedFormulaRowProps struct {
	Formula  domain.Formula
	Result   string
	OnLoad   func(domain.Formula)
	OnDelete func(string)
}

// SavedFormulaRow renders one saved formula with its live result, a button to
// load it into the editor, and a delete button. It owns its handlers (per the
// no-hooks-in-loops rule).
func SavedFormulaRow(props savedFormulaRowProps) ui.Node {
	f := props.Formula
	load := ui.UseEvent(Prevent(func() { props.OnLoad(f) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(f.ID) }))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), f.Name),
			Span(css.Class("row-meta"), f.Expr),
		),
		Span(css.Class("amount fig"), props.Result),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("customize.loadTitle")), OnClick(load), uistate.T("customize.load")),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("customize.deleteTitle")), Title(uistate.T("customize.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// groupThousands renders a float with thousands separators and up to two
// decimals (trailing zeros trimmed), so formula results and variable values read
// like the rest of the app's figures (354,070 not 354070) instead of raw floats
// (C61, matching the C2 money-formatting style).
func groupThousands(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	var b strings.Builder
	n := len(intPart)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	out := b.String() + frac
	if neg {
		out = "-" + out
	}
	return out
}

// formatFormulaValue renders a formula result (number, bool, or string).
func formatFormulaValue(v formula.Value) string {
	switch x := v.(type) {
	case float64:
		return groupThousands(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}
