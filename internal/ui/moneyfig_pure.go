// SPDX-License-Identifier: MIT

package ui

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MoneyStyle selects the presentation convention used for a money value.
type MoneyStyle int

const (
	// MoneyLedger shows income with a plus sign and expenses with a minus sign.
	MoneyLedger MoneyStyle = iota
	// MoneySigned shows an explicit plus or minus sign. Use it for net summaries.
	MoneySigned
	// MoneyAccounting shows negative values in parentheses. Reserve it for reports
	// and balances that use accounting notation.
	MoneyAccounting
	// MoneyMagnitude always shows the absolute value without a sign.
	MoneyMagnitude
)

// FormatMoneyFig formats a money value and returns its semantic tone class.
// Transfers in ledger style are displayed as unsigned magnitudes with neutral
// tone. Unknown styles safely use ledger formatting.
func FormatMoneyFig(m money.Money, style MoneyStyle, isTransfer bool) (text, toneClass string) {
	symbol := currency.Symbol(m.Currency)
	amount := moneyFigMagnitude(m.Amount, currency.Decimals(m.Currency))
	style = normalizeMoneyStyle(style)

	switch style {
	case MoneyLedger:
		if isTransfer || m.Amount == 0 {
			return symbol + amount, ""
		}
		if m.Amount > 0 {
			return "+" + symbol + amount, "text-up"
		}
		return "−" + symbol + amount, "text-down"
	case MoneySigned:
		switch {
		case m.Amount > 0:
			return "+" + symbol + amount, "text-up"
		case m.Amount < 0:
			return "−" + symbol + amount, "text-down"
		default:
			return symbol + amount, ""
		}
	case MoneyAccounting:
		if m.Amount < 0 {
			return "(" + symbol + amount + ")", ""
		}
		return symbol + amount, ""
	case MoneyMagnitude:
		return symbol + amount, ""
	default:
		return symbol + amount, ""
	}
}

func normalizeMoneyStyle(style MoneyStyle) MoneyStyle {
	if style < MoneyLedger || style > MoneyMagnitude {
		return MoneyLedger
	}
	return style
}

func moneyFigMagnitude(amount int64, decimals int) string {
	var magnitude uint64
	if amount < 0 {
		// amount+1 is representable even for MinInt64, so this negation is safe;
		// its result is non-negative before conversion to uint64.
		magnitude = uint64(-(amount + 1)) + 1 // #nosec G115 -- adding one before negation keeps MinInt64 representable; the negated value is non-negative and fits uint64.
	} else {
		// The branch proves amount is in [0, MaxInt64], within uint64's range.
		magnitude = uint64(amount) // #nosec G115 -- this branch restricts amount to non-negative int64, which is within uint64's range.
	}
	if decimals <= 0 {
		return money.Group(strconv.FormatUint(magnitude, 10))
	}
	divisor := uint64(1)
	for range decimals {
		divisor *= 10
	}
	whole := strconv.FormatUint(magnitude/divisor, 10)
	frac := strconv.FormatUint(magnitude%divisor, 10)
	if len(frac) < decimals {
		frac = strings.Repeat("0", decimals-len(frac)) + frac
	}
	return money.Group(whole + "." + frac)
}

func moneyFigAccessibleLabel(value money.Money, style MoneyStyle, transfer bool, text string) string {
	if transfer && style == MoneyLedger {
		return "Transfer " + strings.ReplaceAll(text, "−", "")
	}
	switch {
	case style == MoneySigned && value.Amount < 0:
		return "Minus " + strings.TrimPrefix(text, "−")
	case style == MoneyAccounting && value.Amount < 0:
		return "Minus " + strings.Trim(text, "()")
	case value.Amount < 0 && style == MoneyLedger:
		return "Minus " + strings.TrimPrefix(text, "−")
	case value.Amount > 0 && (style == MoneySigned || style == MoneyLedger):
		return "Plus " + strings.TrimPrefix(text, "+")
	default:
		return text
	}
}
