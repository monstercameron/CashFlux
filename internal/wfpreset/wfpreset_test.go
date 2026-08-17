// SPDX-License-Identifier: MIT

package wfpreset

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// Every preset must survive the same validation a hand-built workflow does. A
// gallery entry that cannot be saved is worse than no gallery entry: the user
// picks it, it fails, and the whole feature reads as broken.
func TestEveryPresetInstantiatesIntoAValidWorkflow(t *testing.T) {
	for _, p := range All() {
		wf := p.Instantiate("wf-"+p.ID, "Test "+p.ID)
		if errs := workflow.Validate(wf); len(errs) > 0 {
			t.Errorf("%s: %v", p.ID, errs)
		}
		if !wf.Enabled {
			t.Errorf("%s: instantiated disabled — a preset the user picked should run", p.ID)
		}
		if len(wf.Actions) == 0 {
			t.Errorf("%s: no actions", p.ID)
		}
	}
}

// The conditions are written by hand into the catalog, so they are exactly the
// kind of thing that rots silently. Parse them.
func TestPresetConditionsParse(t *testing.T) {
	for _, p := range All() {
		if p.Condition == "" {
			continue
		}
		if _, err := formula.Parse(p.Condition); err != nil {
			t.Errorf("%s: condition %q does not parse: %v", p.ID, p.Condition, err)
		}
	}
}

func TestCatalogIdsAndKeysAreUniqueAndPresent(t *testing.T) {
	seenID, seenName := map[string]bool{}, map[string]bool{}
	for _, p := range All() {
		if p.ID == "" || seenID[p.ID] {
			t.Errorf("id %q is empty or duplicated", p.ID)
		}
		seenID[p.ID] = true
		if p.NameKey == "" || seenName[p.NameKey] {
			t.Errorf("%s: name key %q is empty or duplicated", p.ID, p.NameKey)
		}
		seenName[p.NameKey] = true
		if p.DescKey == "" {
			t.Errorf("%s: no description key", p.ID)
		}
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("overdue-bill-task"); !ok {
		t.Error("a catalog id did not resolve")
	}
	if _, ok := Find("no-such-preset"); ok {
		t.Error("an unknown id resolved")
	}
}

// Instantiate must deep-copy: a saved workflow whose action slice aliases the
// catalog would let one household's edit rewrite the gallery for the next pick.
func TestInstantiateDoesNotAliasTheCatalog(t *testing.T) {
	p, _ := Find("overdue-bill-task")
	wf := p.Instantiate("wf-1", "Mine")
	wf.Actions[0].Title = "MUTATED"

	fresh, _ := Find("overdue-bill-task")
	if fresh.Actions[0].Title == "MUTATED" {
		t.Error("editing an instantiated workflow reached back into the catalog")
	}
}

// The price-change preset is the one that depends on a variable the condition
// surface only grew for it. Guard the coupling: if txn_vs_typical is ever
// renamed, this fails here rather than silently never firing in production.
func TestPriceChangePresetUsesTheBaselineVariable(t *testing.T) {
	p, ok := Find("subscription-price-change")
	if !ok {
		t.Fatal("the price-change preset is missing")
	}
	if !strings.Contains(p.Condition, "txn_vs_typical") {
		t.Errorf("condition = %q, want a txn_vs_typical comparison", p.Condition)
	}
	// The task body quotes the baseline, which is the second variable.
	if !strings.Contains(p.Actions[0].Notes, "{{txn_payee_typical}}") {
		t.Errorf("notes = %q, want the typical amount quoted", p.Actions[0].Notes)
	}
}
