// SPDX-License-Identifier: MIT

// Package insights — afford_parser.go
// ParseAffordQuery extracts a structured affordability question from free text.
// Pure Go; no syscall/js — safe to test on native Go and call from any layer.
package insights

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AffordQuery is the structured result of parsing an affordability question.
// MonthsAhead is 0 when no time target was specified (callers default to 1).
// TargetLabel is the human-readable form of the target ("Dec 2026", "3 months",
// etc.) or "" when no target was given.
type AffordQuery struct {
	Amount      int64  // purchase price in minor units (cents)
	MonthsAhead int    // months until the target date (0 = unspecified)
	TargetLabel string // display label for the time target, or ""
}

// Grammar (case-insensitive):
//
//	AFFORD     = "can" SUBJ "afford" PRICE TARGET?
//	SUBJ       = "I" | "we"
//	PRICE      = "$" DIGITS ("," DIGITS)*    e.g. $1,200 or $500
//	TARGET     = "by" MONTH_EXPR | "in" N "months"
//	MONTH_EXPR = MONTH_NAME YEAR?
//	MONTH_NAME = full or 3-letter English month name
//
// Amount is converted to minor units by multiplying by 100 (dollar → cents).
// Returns (nil, false) for any input that does not match the grammar.

// reAfford matches an affordability question (case-insensitive). Named groups via
// positional captures:
//
//	1 – raw digits of the dollar amount (commas included, stripped later)
//	2 – "by …" branch (whole phrase, matched or empty string)
//	3 – month name within "by …"
//	4 – optional 4-digit year within "by …"
//	5 – "in N months" branch (whole phrase)
//	6 – N from "in N months"
var reAfford = regexp.MustCompile(
	`(?i)\bcan\s+(?:i|we)\s+afford\s+\$([0-9][0-9,]*)` +
		`(?:\s+(by\s+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)` +
		`(?:\s+(\d{4}))?` +
		`|` +
		`(in\s+(\d+)\s+months?)))?`,
)

// monthNumbers maps normalised month-name prefix (lower, 3 chars) to
// time.Month values.
var monthNumbers = map[string]time.Month{
	"jan": time.January,
	"feb": time.February,
	"mar": time.March,
	"apr": time.April,
	"may": time.May,
	"jun": time.June,
	"jul": time.July,
	"aug": time.August,
	"sep": time.September,
	"oct": time.October,
	"nov": time.November,
	"dec": time.December,
}

// ParseAffordQuery parses text for an affordability question and returns the
// structured query. It returns (nil, false) for any non-affordability input so
// callers can safely route the message to the AI path unchanged.
func ParseAffordQuery(text string) (*AffordQuery, bool) {
	m := reAfford.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}

	// m[1]: raw dollar amount, e.g. "1,200" or "500"
	rawAmt := strings.ReplaceAll(m[1], ",", "")
	dollars, err := strconv.ParseInt(rawAmt, 10, 64)
	if err != nil || dollars < 0 {
		return nil, false
	}

	q := &AffordQuery{Amount: dollars * 100}

	switch {
	case m[3] != "": // "by <month> [year]"
		key := strings.ToLower(m[3])
		if len(key) > 3 {
			key = key[:3]
		}
		month, ok := monthNumbers[key]
		if !ok {
			return nil, false
		}
		now := time.Now()
		year := now.Year()
		if m[4] != "" {
			y, yerr := strconv.Atoi(m[4])
			if yerr == nil && y >= year {
				year = y
			}
		}
		// If the named month/year has already passed (or is the current month),
		// advance a year so "by Dec" in December means next December.
		target := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		curMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if !target.After(curMonth) && m[4] == "" {
			target = time.Date(year+1, month, 1, 0, 0, 0, 0, time.UTC)
		}
		months := monthsBetween(now, target)
		q.MonthsAhead = months
		if m[4] != "" {
			q.TargetLabel = month.String() + " " + m[4]
		} else {
			q.TargetLabel = month.String()
		}

	case m[6] != "": // "in N months"
		n, nerr := strconv.Atoi(m[6])
		if nerr != nil || n < 0 {
			return nil, false
		}
		q.MonthsAhead = n
		if n == 1 {
			q.TargetLabel = "1 month"
		} else {
			q.TargetLabel = strconv.Itoa(n) + " months"
		}
	}

	return q, true
}

// monthsBetween returns the number of whole calendar months from now until
// target (always ≥ 0).
func monthsBetween(now, target time.Time) int {
	y1, m1 := now.Year(), int(now.Month())
	y2, m2 := target.Year(), int(target.Month())
	months := (y2-y1)*12 + (m2 - m1)
	if months < 0 {
		return 0
	}
	return months
}
