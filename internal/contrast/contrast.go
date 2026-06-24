// SPDX-License-Identifier: MIT

// Package contrast computes WCAG 2.x color-contrast metrics: the relative
// luminance of an sRGB color and the contrast ratio between two colors, plus the
// AA/AAA pass thresholds. It lets CashFlux check that text and UI colors — and
// especially a user's chosen accent — are legible against their background.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package contrast

import (
	"fmt"
	"math"
	"strings"
)

// WCAG contrast thresholds (ratio must be at least these values to pass).
const (
	AANormal  = 4.5 // AA, normal text
	AALarge   = 3.0 // AA, large text (>=18pt or 14pt bold) and UI components
	AAANormal = 7.0 // AAA, normal text
	AAALarge  = 4.5 // AAA, large text
)

// ParseHex parses a "#rgb" or "#rrggbb" color (the leading # is optional) into
// 0–255 channel values. It errors on a malformed string.
func ParseHex(hex string) (r, g, b uint8, err error) {
	s := strings.TrimSpace(hex)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3: // shorthand: expand each nibble (e.g. "f0a" -> "ff00aa")
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	default:
		return 0, 0, 0, fmt.Errorf("contrast: %q is not a 3- or 6-digit hex color", hex)
	}
	var v uint64
	for i := 0; i < 6; i++ {
		d, ok := hexDigit(s[i])
		if !ok {
			return 0, 0, 0, fmt.Errorf("contrast: %q has a non-hex digit", hex)
		}
		v = v<<4 | uint64(d)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}

func hexDigit(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// channel linearizes one 0–255 sRGB component to the 0–1 linear space WCAG uses.
func channel(c uint8) float64 {
	cs := float64(c) / 255
	if cs <= 0.03928 {
		return cs / 12.92
	}
	return math.Pow((cs+0.055)/1.055, 2.4)
}

// RelativeLuminance returns the WCAG relative luminance (0–1) of an sRGB hex
// color: 0 is black, 1 is white.
func RelativeLuminance(hex string) (float64, error) {
	r, g, b, err := ParseHex(hex)
	if err != nil {
		return 0, err
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b), nil
}

// Ratio returns the WCAG contrast ratio between two sRGB hex colors, in the range
// 1.0 (identical) to 21.0 (black vs white). Order doesn't matter.
func Ratio(hexA, hexB string) (float64, error) {
	la, err := RelativeLuminance(hexA)
	if err != nil {
		return 0, err
	}
	lb, err := RelativeLuminance(hexB)
	if err != nil {
		return 0, err
	}
	hi, lo := la, lb
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05), nil
}

// PassesAA reports whether a contrast ratio meets WCAG AA — 4.5 for normal text,
// 3.0 for large text and UI components.
func PassesAA(ratio float64, largeText bool) bool {
	if largeText {
		return ratio >= AALarge
	}
	return ratio >= AANormal
}

// PassesAAA reports whether a contrast ratio meets WCAG AAA — 7.0 for normal
// text, 4.5 for large text.
func PassesAAA(ratio float64, largeText bool) bool {
	if largeText {
		return ratio >= AAALarge
	}
	return ratio >= AAANormal
}
