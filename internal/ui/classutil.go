// SPDX-License-Identifier: MIT

// Package ui provides shared UI components for the CashFlux wasm frontend.
// This file contains pure (non-wasm) class-name utilities that can be tested
// on native Go without a browser.
package ui

import "strings"

// JoinClass concatenates CSS class tokens, omitting empty strings and
// de-duplicating adjacent duplicates. It is useful when building a class
// string from a fixed base plus optional modifiers so callers never have
// to handle leading/trailing/double spaces themselves.
//
//	JoinClass("btn", "btn-del", "")  →  "btn btn-del"
//	JoinClass("", "row", "")         →  "row"
func JoinClass(tokens ...string) string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, " ")
}
