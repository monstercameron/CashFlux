// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type importPlacementProps struct {
	Accounts  []domain.Account
	AccountID string
	OnChange  ui.Handler
	Disabled  bool
	Rows      []extract.Row
	Summary   importdraft.Summary
	Currency  string
}

func importAccountPlacement(p importPlacementProps) ui.Node {
	first, last := importdraft.DateRange(p.Rows)
	return Div(css.Class(importStatementHeader), Attr("data-testid", "import-account-placement"),
		Div(css.Class(importStack),
			Label(For("import-review-account"), uistate.T("importux.importInto")),
			Select(css.Class(importAccountSelect), Attr("id", "import-review-account"), Attr("aria-label", uistate.T("documents.importAccount")), Disabled(p.Disabled), OnChange(p.OnChange), uiw.FieldValue(p.AccountID),
				If(p.AccountID == "", Option(Value(""), SelectedIf(true), Disabled(true), uistate.T("transactions.chooseAccount"))),
				draftAccountOptions(p.Accounts, p.AccountID)),
			Div(css.Class(importMuted), Attr("data-testid", "import-statement-dates"),
				Span(uistate.T("importux.transactions", len(p.Rows))),
				If(!first.IsZero(), P(Style(map[string]string{"margin": "8px 0 0"}), uistate.T("importux.dateRange", first.Format("Jan 2, 2006"), last.Format("Jan 2, 2006"))))),
		),
		Div(css.Class(importFacts), Attr("data-testid", "import-statement-totals"),
			IfElse(p.Summary.Overflow, P(css.Class(importMuted), uistate.T("importux.totalUnavailable")),
				Fragment(importStatementFigure(uistate.T("importux.totalIn"), fmtMoney(money.New(p.Summary.Income, p.Currency)), "amount-income"),
					importStatementFigure(uistate.T("importux.totalOut"), fmtMoney(money.New(p.Summary.Expense, p.Currency)), "amount-expense"))),
			If(p.Summary.Duplicates > 0, P(css.Class(importMuted), uistate.T("importux.duplicates", p.Summary.Duplicates))),
		),
	)
}

func importStatementFigure(label, amount, tone string) ui.Node {
	return Div(css.Class(importStack), Style(map[string]string{"gap": "8px", "text-align": "right"}),
		Span(css.Class(importMuted), label),
		Span(css.Class(tone), Style(map[string]string{"font-variant-numeric": "tabular-nums", "font-size": "20px", "font-weight": "500"}), amount))
}

func importProgress(current string) ui.Node {
	step := func(key, name string) ui.Node {
		color := "var(--text-dim)"
		value := "false"
		if current == name {
			color, value = "var(--text)", "step"
		}
		return Span(Attr("aria-current", value), Style(map[string]string{"color": color}), uistate.T(key))
	}
	return Nav(css.Class(importMuted), Style(map[string]string{"display": "flex", "gap": "8px"}), Attr("aria-label", uistate.T("importux.steps")), Attr("data-testid", "import-progress"),
		step("importux.source", "source"), Span(Attr("aria-hidden", "true"), "—"), step("importux.read", "read"), Span(Attr("aria-hidden", "true"), "—"), step("importux.review", "review"))
}

// Scoped sheets avoid the legacy import card typography and decorative motion.
// Only the statement's account placement uses the existing display font.
var importSourceCard = css.New(
	css.Raw("border", "1px solid var(--border)"), css.Raw("border-radius", "var(--radius-lg)"),
	css.Raw("background", "var(--bg-elev)"), css.Raw("padding", "24px"), css.Raw("min-width", "0"),
)
var importStack = css.New(css.Raw("display", "flex"), css.Raw("flex-direction", "column"), css.Raw("gap", "16px"), css.Raw("min-width", "0"))
var importActionRow = css.New(css.Raw("display", "flex"), css.Raw("justify-content", "flex-end"), css.Raw("align-items", "center"), css.Raw("flex-wrap", "wrap"), css.Raw("gap", "8px"))
var importDropZone = css.New(
	css.Raw("display", "flex"), css.Raw("align-items", "center"), css.Raw("justify-content", "space-between"),
	css.Raw("flex-wrap", "wrap"), css.Raw("gap", "16px"), css.Raw("padding", "24px"),
	css.Raw("border", "1px dashed var(--border)"), css.Raw("border-radius", "var(--radius-lg)"),
	css.Raw("background", "var(--bg)"), css.Raw("min-width", "0"),
)
var importMuted = css.New(css.Raw("font-size", "13px"), css.Raw("line-height", "1.5"), css.Raw("color", "var(--text-dim)"), css.Raw("max-width", "72ch"), css.Raw("margin", "0"))
var importBackLink = css.New(css.Raw("padding", "0"), css.Raw("justify-self", "start"), css.Raw("align-self", "flex-start"), css.Raw("font-size", "13px"))
var importFocus = css.New(css.FocusVisible(css.Raw("outline", "2px solid var(--accent)"), css.Raw("outline-offset", "3px"))...)
var importStatementHeader = css.New(
	css.Raw("display", "grid"), css.Raw("grid-template-columns", "minmax(0, 1.4fr) minmax(0, 1fr)"),
	css.Raw("align-items", "end"), css.Raw("gap", "24px"), css.Raw("padding", "0 0 24px"), css.Raw("border-bottom", "1px solid var(--border)"),
)
var importAccountSelect = css.New(css.Raw("width", "100%"), css.Raw("min-width", "0"), css.Raw("font-family", "var(--font-display)"),
	css.Raw("font-size", "clamp(24px, 3vw, 32px)"), css.Raw("line-height", "1.25"), css.Raw("color", "var(--text)"),
	css.Raw("background", "var(--bg-elev)"), css.Raw("border", "1px solid var(--border)"), css.Raw("border-radius", "var(--radius-lg)"), css.Raw("padding", "8px"))
var importFacts = css.New(css.Raw("display", "flex"), css.Raw("justify-content", "flex-end"), css.Raw("flex-wrap", "wrap"), css.Raw("gap", "24px"), css.Raw("align-items", "end"))
var importSourceGrid = css.New(css.Raw("display", "grid"), css.Raw("grid-template-columns", "repeat(2,minmax(0,1fr))"), css.Raw("gap", "16px"))
var importSourceTile = css.New(css.Rules(
	css.Raw("display", "flex"), css.Raw("align-items", "flex-start"), css.Raw("gap", "16px"), css.Raw("padding", "16px"),
	css.Raw("min-width", "0"), css.Raw("text-align", "left"), css.Raw("color", "var(--text)"), css.Raw("background", "var(--bg-elev)"),
	css.Raw("border", "1px solid var(--border)"), css.Raw("border-radius", "var(--radius-lg)"), css.Raw("cursor", "pointer"),
	css.Hover(css.Raw("border-color", "var(--accent)")), css.FocusVisible(css.Raw("outline", "2px solid var(--accent)"), css.Raw("outline-offset", "3px")),
)...)
var importTier = css.New(css.Raw("font-size", "12px"), css.Raw("font-weight", "500"), css.Raw("color", "var(--text-dim)"), css.Raw("white-space", "nowrap"))
var importStatementColumns = css.New(css.Raw("display", "grid"), css.Raw("grid-template-columns", "100px minmax(120px,1fr) minmax(100px,.7fr) minmax(80px,auto) 64px"), css.Raw("gap", "16px"), css.Raw("align-items", "center"), css.Raw("min-width", "560px"))

var importPanelStyle = css.New(css.Rules(
	css.Raw("font-family", "var(--font-body, Inter, sans-serif)"), css.Raw("color", "var(--text)"),
	css.Descendant(css.Selector(":is(button, select, summary, textarea, input):focus-visible"),
		css.Raw("outline", "2px solid var(--accent)"), css.Raw("outline-offset", "3px")),
)...)

func importDisclosure(full string) ui.Node {
	return importDisclosureFor("importux.disclosureShort", full)
}

func importDisclosureFor(short, full string) ui.Node {
	return Div(css.Class(importMuted), Attr("data-testid", "import-ai-disclosure"),
		P(Style(map[string]string{"margin": "0 0 8px"}), uistate.T(short)),
		Details(Attr("data-testid", "import-ai-details"),
			Summary(css.Class(importFocus), Style(map[string]string{"cursor": "pointer", "color": "var(--text)"}), uistate.T("importux.whatSent")),
			P(Style(map[string]string{"margin": "8px 0 0"}), full),
			P(Style(map[string]string{"margin": "8px 0 0"}), uistate.T("importux.costNote")),
		),
	)
}
