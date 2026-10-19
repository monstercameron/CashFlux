// SPDX-License-Identifier: MIT

package staging

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/statement"
)

// ParseAmount normalizes a statement/model amount using integer minor units.
// Currency symbols, grouping, parentheses, DR/CR and a Unicode minus are
// supported. Conflicting or repeated signs are rejected before sign application
// so a malformed amount can never overflow through negation of MinInt64.
func ParseAmount(value string, decimals int) (int64, error) {
	text := strings.ReplaceAll(strings.TrimSpace(value), "\u2212", "-")
	negative := false
	decorated := false
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		text = strings.TrimSpace(text[1 : len(text)-1])
		negative, decorated = true, true
	}
	upper := strings.ToUpper(text)
	if strings.HasSuffix(upper, "DR") || strings.HasSuffix(upper, "CR") {
		if decorated {
			return 0, fmt.Errorf("amount has conflicting signs")
		}
		negative, decorated = strings.HasSuffix(upper, "DR"), true
		text = strings.TrimSpace(text[:len(text)-2])
	}
	text = strings.TrimLeft(text, "$\u00a3\u20ac\u00a5 ")
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		if decorated {
			return 0, fmt.Errorf("amount has conflicting signs")
		}
		negative = text[0] == '-'
		text = strings.TrimSpace(text[1:])
	} else if strings.HasSuffix(text, "-") {
		if decorated {
			return 0, fmt.Errorf("amount has conflicting signs")
		}
		negative = true
		text = strings.TrimSpace(strings.TrimSuffix(text, "-"))
	}
	if strings.ContainsAny(text, "+-()") {
		return 0, fmt.Errorf("amount has conflicting signs")
	}
	// Reuse the statement parser for grouping/symbol normalization. The remaining
	// value has no sign, so its checked parser cannot negate a signed minimum.
	magnitude, err := statement.ParseAmount(text, decimals)
	if err != nil {
		return 0, fmt.Errorf("parse amount: %w", err)
	}
	if magnitude < 0 {
		return 0, fmt.Errorf("amount magnitude must be positive")
	}
	if negative {
		return money.Sub(0, magnitude)
	}
	return magnitude, nil
}
