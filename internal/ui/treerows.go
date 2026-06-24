// SPDX-License-Identifier: MIT

// Package ui — pure (non-wasm) tree / indent helpers.
// This file has no build tag so it compiles on all platforms and can be
// unit-tested with plain "go test ./internal/ui/..." on native Go.

package ui

import "strings"

// ---------------------------------------------------------------------------
// TreeRows — indent helpers for category-tree and nested-task rows
// ---------------------------------------------------------------------------

// IndentPx returns the pixel-width CSS padding-left string appropriate for a
// given nesting depth using the standard 16 px-per-level rule used across
// CashFlux category and task rows. Depth 0 returns "0px".
//
//	IndentPx(0) → "0px"
//	IndentPx(1) → "16px"
//	IndentPx(2) → "32px"
func IndentPx(depth int) string {
	if depth <= 0 {
		return "0px"
	}
	// Use repeated string building to avoid importing strconv for a one-liner.
	// 16 px per level is the established visual rhythm (categories, tasks).
	const pxPerLevel = 16
	n := depth * pxPerLevel
	// itoa-style conversion without importing strconv so this file stays
	// dependency-free and importable from any package.
	return itoa(n) + "px"
}

// itoa converts a non-negative integer to its decimal string representation.
// It is a package-private helper so this file has no external dependencies.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// IndentLabel returns a depth-proportional non-breaking-space prefix for use
// in <option> text where CSS padding is unavailable. It mirrors the private
// indentLabel helper in screens/categories.go and screens/categoryaddform.go,
// consolidating the pattern into a single tested location.
//
// The prefix uses U+00A0 NO-BREAK SPACE repeated 3 times per depth level.
// Browsers do not collapse non-breaking spaces in <option> text (unlike normal
// leading spaces), giving a clean visual hierarchy.
//
//	IndentLabel(0) → ""
//	IndentLabel(1) → "   " (3 NBSPs)
//	IndentLabel(2) → "      " (6 NBSPs)
func IndentLabel(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("   ", depth)
}

// MaxIndentDepth is the recommended cap on visual indentation. Beyond this
// depth, rows are rendered at the maximum indent to avoid overflowing narrow
// viewports. Callers may enforce this cap themselves.
const MaxIndentDepth = 6

// ClampDepth returns depth clamped to [0, MaxIndentDepth].
//
//	ClampDepth(-1) → 0
//	ClampDepth(7)  → 6
func ClampDepth(depth int) int {
	if depth < 0 {
		return 0
	}
	if depth > MaxIndentDepth {
		return MaxIndentDepth
	}
	return depth
}
