// SPDX-License-Identifier: MIT

package styles

import (
	"strconv"
	"strings"
	"testing"
)

func TestExpenseToneUsesAccessibleNegativeToken(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerTxnCalendar()
	css := Build()
	for _, want := range []string{
		".txn-table .txn-amount-expense{color:var(--money-negative);}",
		".txn-cal-net.text-down{color:var(--money-negative);}",
		".text-down{color:var(--money-negative, #d8716f);}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("expense tone stylesheet missing %q", want)
		}
	}

	// --money-negative aliases --down: #d8716f on the dark #121214 card and
	// #b3322f on the light white card. Check both defaults against WCAG's 4.5:1.
	for _, tc := range []struct {
		name, foreground, background string
	}{
		{name: "dark", foreground: "#d8716f", background: "#121214"},
		{name: "light", foreground: "#b3322f", background: "#ffffff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ratio := contrastRatio(tc.foreground, tc.background); ratio < 4.5 {
				t.Errorf("negative tone contrast %.2f:1, want at least 4.5:1", ratio)
			}
		})
	}
}

func contrastRatio(foreground, background string) float64 {
	fore := luminance(foreground)
	back := luminance(background)
	if fore < back {
		fore, back = back, fore
	}
	return (fore + 0.05) / (back + 0.05)
}

func luminance(hex string) float64 {
	var channels [3]float64
	for i := range channels {
		value, err := strconv.ParseInt(hex[1+i*2:3+i*2], 16, 64)
		if err != nil {
			panic("invalid fixed test color: " + hex)
		}
		linear := float64(value) / 255
		if linear <= 0.04045 {
			channels[i] = linear / 12.92
		} else {
			channels[i] = ((linear + 0.055) / 1.055) * ((linear + 0.055) / 1.055) * ((linear + 0.055) / 1.055)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
}
