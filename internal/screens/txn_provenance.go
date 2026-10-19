// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
		if strings.EqualFold(s, "Recurring") {
			return uistate.T("transactions.autoWhySource")
		}
		return uistate.T("transactions.autoWhySourceOther", s)
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

// provenance memo: the classification depends only on the ledger and the rule set,
// which move together with the data revision — so it is recomputed when the
// revision moves, not on every sort / select / pagination re-render of the table.
//
// The cache is not an optimisation to be added later. Classifying one row runs the
// rule engine over the whole rule set, and each rule's legacy match calls
// payeeclean.Suggest, which is several regex passes. Unmemoized, ticking ONE row's
// checkbox re-ran that for every visible row — and "All" renders 3,000+ of them.
// merchantChargeCountsMemo solves the identical problem two files over; this
// follows it rather than inventing a second answer.
var (
	provCache txnProvenanceCache
)

type txnProvenanceCache struct {
	app      *appstate.App
	revision int
	values   map[string]txnprov.Provenance
	seen     map[string]bool
	valid    bool
}

// txnAutoByID returns the rows whose category is automatic and unconfirmed, keyed
// by transaction id and read O(1) while building a row.
//
// The SCOPE is part of the cache key, not just the data revision: filtering
// changes which rows are classified without touching the data, so a key of
// revision alone would hand a narrowed view the previous view's answers. Row count
// alone is not enough either — two different filters can leave the same number of
// rows at the same revision, and then the second one gets the first one's map. The
// exact ordered ID list is part of the key so equal-sized views with shared
// boundary rows cannot reuse one another's classifications.
func txnAutoByID(app *appstate.App, txns []domain.Transaction, rs []rules.Rule, dataRev int) map[string]txnprov.Provenance {
	if !provCache.valid || provCache.app != app || provCache.revision != dataRev {
		provCache = txnProvenanceCache{
			app: app, revision: dataRev,
			values: make(map[string]txnprov.Provenance),
			seen:   make(map[string]bool), valid: true,
		}
	}
	for _, t := range txns {
		if !provCache.seen[t.ID] {
			p := txnprov.Of(t, rs)
			if p.IsAutomatic() {
				provCache.values[t.ID] = p
			}
			provCache.seen[t.ID] = true
		}
	}
	return provCache.values
}

// provenanceKey identifies a scoped view by the data revision and every row ID in
// order. Length-prefixing IDs makes the encoding unambiguous even when an ID
// contains punctuation used by the key itself.
func provenanceKey(txns []domain.Transaction, dataRev int) string {
	var key strings.Builder
	key.WriteString(strconv.Itoa(dataRev))
	key.WriteByte(':')
	key.WriteString(strconv.Itoa(len(txns)))
	for _, txn := range txns {
		key.WriteByte(':')
		key.WriteString(strconv.Itoa(len(txn.ID)))
		key.WriteByte(':')
		key.WriteString(txn.ID)
	}
	return key.String()
}
