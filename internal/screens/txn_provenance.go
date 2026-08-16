// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/txnprov"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnAutoMark is what a row shows beside a category no person has confirmed.
//
// C579: a rule-written category, an import's guess and a decision someone made by
// hand all rendered as the same plain word in the same cell, so the ledger gave a
// user no way to tell a settled file from a machine's suggestion. The mark is
// deliberately small — most rows carry it, and the ledger's job is still scanning
// — but it is a WORD, not a colour or a dot, and it explains itself on hover and
// to assistive tech.
type txnAutoMarkProps struct {
	// Why is the full sentence: what filed this row, and whether a rule accounts
	// for it. It is both the tooltip and the accessible name.
	Why string
}

func txnAutoMark(props txnAutoMarkProps) ui.Node {
	return Span(css.Class("txn-auto-mark", tw.ShrinkO),
		Attr("data-testid", "txn-auto-mark"),
		Attr("title", props.Why),
		Attr("aria-label", props.Why),
		uistate.T("transactions.autoMark"))
}

// autoMarkWhy builds the sentence the mark carries. Naming the rule is the part
// that turns "a machine did this" into something the user can act on, so a row
// whose category no current rule explains says THAT rather than implying a rule
// exists somewhere.
func autoMarkWhy(p txnprov.Provenance, sourceLabel string) string {
	if p.RuleMatch != nil {
		if phrase := rulePhrase(*p.RuleMatch); phrase != "" {
			return uistate.T("transactions.autoWhyRule", phrase)
		}
		return uistate.T("transactions.autoWhyRuleUnnamed")
	}
	if s := strings.TrimSpace(sourceLabel); s != "" && s != "—" {
		return uistate.T("transactions.autoWhySource", s)
	}
	return uistate.T("transactions.autoWhyUnknown")
}

// rulePhrase names a rule the way its author would recognise it: the match text
// for a substring rule, or a short rendering of the first structured condition.
// Empty when the rule can't be described in a few words, so the caller falls back
// to a sentence that doesn't pretend to quote one.
func rulePhrase(r rules.Rule) string {
	if m := strings.TrimSpace(r.Match); m != "" {
		return m
	}
	if len(r.Conditions) > 0 {
		c := r.Conditions[0]
		if v := strings.TrimSpace(c.Value); v != "" {
			return string(c.Field) + " " + string(c.Op) + " " + v
		}
	}
	return ""
}

// txnAutoByID classifies every transaction in the current view against the
// current rules, once per render rather than once per row-render.
//
// Rows re-render on selection, sort and pagination; classification depends only
// on the data, so doing it per row would re-run the rule engine for every one of
// those. The map is keyed by transaction id and read O(1) while building a row.
func txnAutoByID(txns []domain.Transaction, rs []rules.Rule) map[string]txnprov.Provenance {
	out := make(map[string]txnprov.Provenance, len(txns))
	for _, t := range txns {
		if p := txnprov.Of(t, rs); p.IsAutomatic() {
			out[t.ID] = p
		}
	}
	return out
}
