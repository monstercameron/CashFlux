// SPDX-License-Identifier: MIT

// Package lockquotes holds the curated, rotating lines shown on the lock screen.
// Rotation is deterministic (caller passes a day/index) — no randomness, so it's
// safe in logic packages (Math.random is banned) and stable for tests. No network.
package lockquotes

// quotes is the curated set: short, non-preachy finance/motivation lines.
var quotes = []string{
	"A budget is telling your money where to go instead of wondering where it went.",
	"Small, steady savings outlast the occasional big one.",
	"Spend less than you earn, and invest the difference.",
	"Every dollar you don't spend is a dollar that works for you.",
	"Wealth is built quietly, one good decision at a time.",
	"Pay yourself first — then live on the rest.",
	"The best time to start saving was yesterday. The next best is today.",
	"Track it to change it: what gets measured gets managed.",
	"Freedom is having choices, and choices are easier with savings.",
	"Future you is counting on the choices present you makes.",
	"A little planning beats a lot of worrying.",
	"Progress, not perfection — keep the streak going.",
}

// Count returns the number of curated quotes.
func Count() int { return len(quotes) }

// ForIndex returns a quote by index, wrapping so any int maps to a quote (negative
// indices wrap too). Deterministic — same index always yields the same quote.
func ForIndex(i int) string {
	n := len(quotes)
	if n == 0 {
		return ""
	}
	return quotes[((i%n)+n)%n]
}
