// SPDX-License-Identifier: MIT

// Package csvsafe contains helpers for safely exporting untrusted text to CSV.
package csvsafe

import "strings"

// NeedsGuard reports whether a CSV text cell begins with a character that
// spreadsheet applications may interpret as a formula.
func NeedsGuard(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return true
	default:
		return false
	}
}

// Guard prefixes formula-leading text with a single quote so spreadsheets treat
// the cell as text. Other values are returned unchanged.
func Guard(value string) string {
	if NeedsGuard(value) {
		return "'" + value
	}
	// Double an existing leading quote so Restore can distinguish an original
	// quote from the guard added above, without changing the text on round trip.
	if strings.HasPrefix(value, "'") {
		return "'" + value
	}
	return value
}

// Restore removes the guard added by Guard. Call it only for rows explicitly
// marked as CashFlux exports; arbitrary CSV input must retain its original text.
func Restore(value string) string {
	if strings.HasPrefix(value, "''") {
		return value[1:]
	}
	if len(value) > 1 && value[0] == '\'' && NeedsGuard(value[1:]) {
		return value[1:]
	}
	return value
}
