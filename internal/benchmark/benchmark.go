// SPDX-License-Identifier: MIT

// Package benchmark formats a "your figure vs. a typical range" comparison behind
// web-grounded benchmarks (AG9). The assistant already has web_search to find a
// current external range and the finance tools to read the user's own figure; the
// discipline AG9 demands is the RESPONSE SHAPE — a local figure, an external range
// WITH its source, an explicit list of assumptions, and a verdict that is never
// vibes. This package is that shape: a pure comparator + formatter, so the verdict
// (below / within / above the range) is deterministic and testable.
//
// Money is base-currency minor units throughout. No syscall/js, no network — the
// wasm tool gathers the figure and the range, then calls Compare/Format here.
package benchmark

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// fmtMoney formats a money value as "12.34 USD" using the currency's decimals —
// a plain, edge-only formatter (the package holds no locale/symbol table).
func fmtMoney(m money.Money) string {
	return m.Format(currency.Decimals(m.Currency)) + " " + m.Currency
}

// Verdict classifies where the user's figure falls relative to the typical range.
type Verdict string

const (
	// VerdictBelow means the figure is under the low end (typically good for a cost).
	VerdictBelow Verdict = "below"
	// VerdictWithin means the figure sits inside the typical range.
	VerdictWithin Verdict = "within"
	// VerdictAbove means the figure exceeds the high end (worth a closer look).
	VerdictAbove Verdict = "above"
)

// Comparison is the structured result of benchmarking one figure against a range.
type Comparison struct {
	Category    string
	FigureMinor int64
	LowMinor    int64
	HighMinor   int64
	Base        string
	Verdict     Verdict
	// DeltaMinor is the signed distance to the nearest range edge: negative when
	// below the low end, positive when above the high end, zero when within.
	DeltaMinor  int64
	Assumptions []string
}

// Compare classifies figure against [low, high] (base-currency minor units). If
// low and high are given out of order they are swapped so the range is always
// well-formed. assumptions carries the caller's stated caveats (region, coverage,
// household size) and is preserved verbatim.
func Compare(category string, figureMinor, lowMinor, highMinor int64, base string, assumptions []string) Comparison {
	if lowMinor > highMinor {
		lowMinor, highMinor = highMinor, lowMinor
	}
	c := Comparison{
		Category: strings.TrimSpace(category), FigureMinor: figureMinor,
		LowMinor: lowMinor, HighMinor: highMinor, Base: base,
		Assumptions: cleanAssumptions(assumptions),
	}
	switch {
	case figureMinor < lowMinor:
		c.Verdict = VerdictBelow
		c.DeltaMinor = figureMinor - lowMinor // negative
	case figureMinor > highMinor:
		c.Verdict = VerdictAbove
		c.DeltaMinor = figureMinor - highMinor // positive
	default:
		c.Verdict = VerdictWithin
	}
	return c
}

// Format renders the comparison in the AG9 response shape: the local figure, the
// external range, the verdict with the distance to the range, and an explicit
// assumptions block. source is the range's citation (a URL or publication) and is
// always shown so the number is never presented as fact-free. Format never
// invents a source: when source is empty it says so, keeping the honesty contract.
func (c Comparison) Format(source string) string {
	m := func(minor int64) string { return fmtMoney(money.New(minor, c.Base)) }
	cat := c.Category
	if cat == "" {
		cat = "this figure"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Your %s: %s\n", cat, m(c.FigureMinor))
	fmt.Fprintf(&b, "Typical range: %s–%s\n", m(c.LowMinor), m(c.HighMinor))
	switch c.Verdict {
	case VerdictBelow:
		fmt.Fprintf(&b, "Verdict: below typical — %s under the low end.\n", m(abs(c.DeltaMinor)))
	case VerdictAbove:
		fmt.Fprintf(&b, "Verdict: above typical — %s over the high end.\n", m(c.DeltaMinor))
	default:
		b.WriteString("Verdict: within the typical range.\n")
	}
	if s := strings.TrimSpace(source); s != "" {
		fmt.Fprintf(&b, "Source for the range: %s\n", s)
	} else {
		b.WriteString("Source for the range: none cited — treat the range as a rough estimate.\n")
	}
	if len(c.Assumptions) > 0 {
		b.WriteString("Assumptions:\n")
		for _, a := range c.Assumptions {
			b.WriteString("- " + a + "\n")
		}
	} else {
		b.WriteString("Assumptions: none stated — the range may not match your region, coverage, or household.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func cleanAssumptions(in []string) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
