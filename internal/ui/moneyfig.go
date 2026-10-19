// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

var (
	moneyFigBaseRule     css.Rule = css.Raw("font-variant-numeric", "tabular-nums")
	moneyFigPositiveRule css.Rule = tw.TextUp
	moneyFigNeutralRule  css.Rule = tw.TextDim
)

// MoneyFigProps configures the amount and its presentation convention.
type MoneyFigProps struct {
	// Money is the exact integer-minor-unit value to display.
	Money money.Money
	// Style selects ledger, signed, accounting, or magnitude presentation.
	Style MoneyStyle
	// IsTransfer keeps ledger amounts unsigned and neutral for transfers.
	IsTransfer bool
}

// MoneyFig renders a formatted amount as a static span. Its accessible name
// spells out the sign so assistive technology does not have to infer it from a
// typographic symbol.
func MoneyFig(props MoneyFigProps) uic.Node {
	text, tone := FormatMoneyFig(props.Money, props.Style, props.IsTransfer)
	label := moneyFigAccessibleLabel(props.Money, normalizeMoneyStyle(props.Style), props.IsTransfer, text)
	rules := []css.Rule{moneyFigBaseRule}
	if tone == "text-up" {
		rules = append(rules, moneyFigPositiveRule)
	} else if tone == "txn-amount-expense" {
		rules = append(rules, moneyFigNeutralRule)
	}
	return Span(css.Class("cf-money-fig", tone, css.New(rules...)), Attr("aria-label", label), text)
}
