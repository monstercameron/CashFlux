// SPDX-License-Identifier: MIT

// Package explainseed builds the pre-seeded assistant prompt behind
// explain-anything (AG7). A KPI/figure surface anywhere in the app can turn a
// named engine variable into a grounded "explain this number" request: this
// package formats the engineenv.Derivation (molecule → atoms, or an atom's
// source) into a short natural-language message the chat host drops in as the
// user's opening turn, so the agent walks the derivation instead of guessing.
//
// Pure Go, no syscall/js — unit-tested natively. The wasm layer computes the
// Derivation (engineenv.Explain over the live vars) and the seed text here, then
// hands the text to the assistant.
package explainseed

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/engineenv"
)

// Label returns a friendly, human title for an engine variable name, so a seed
// or affordance reads "net worth" rather than "net_worth".
func Label(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), "_", " ")
}

// SeedText builds the assistant's opening user message for explaining the named
// figure. It embeds the derivation so the agent answers from the grounded
// context: for a molecule, the formula plus each input's live value; for an atom,
// its fundamental source. formatValue formats a resolved variable value for
// display (money vs count vs percent is the caller's concern) — pass nil to fall
// back to a plain number. The returned text is plain English, safe to show and to
// send as the first turn.
func SeedText(d engineenv.Derivation, formatValue func(float64) string) string {
	fv := formatValue
	if fv == nil {
		fv = func(v float64) string { return fmt.Sprintf("%g", v) }
	}
	label := Label(d.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "Explain how my %s of %s is calculated, and what's driving it.", label, fv(d.Value))
	switch d.Kind {
	case "molecule":
		if strings.TrimSpace(d.Formula) != "" {
			fmt.Fprintf(&b, "\n\nIt's derived as %s = %s.", d.Name, d.Formula)
		}
		if len(d.Inputs) > 0 {
			names := make([]string, 0, len(d.Inputs))
			for k := range d.Inputs {
				names = append(names, k)
			}
			sort.Strings(names)
			parts := make([]string, 0, len(names))
			for _, k := range names {
				parts = append(parts, fmt.Sprintf("%s = %s", k, fv(d.Inputs[k])))
			}
			b.WriteString(" Its inputs right now: " + strings.Join(parts, ", ") + ".")
		}
	case "atom":
		if strings.TrimSpace(d.Source) != "" {
			fmt.Fprintf(&b, "\n\nIt's an atom: %s.", d.Source)
		}
	case "custom":
		if strings.TrimSpace(d.Source) != "" {
			fmt.Fprintf(&b, "\n\nIt comes from a %s.", d.Source)
		}
	}
	b.WriteString(" Use list_formula_metrics and evaluate_formula to trace it down to the transactions if I ask.")
	return b.String()
}
