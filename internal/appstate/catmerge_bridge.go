// SPDX-License-Identifier: MIT

package appstate

import (
	"github.com/monstercameron/CashFlux/internal/catmerge"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
)

// The two category-reference classes a merge could not reach from here (C548).
//
// Everything else a category can hide in — transactions, splits, budgets, goals,
// rules, recurring templates — lives in the store, so categoryRefs can just read
// it. These two do not:
//
//   - the learn-from-corrections tally is a UI-layer KV blob (uistate), so the
//     wasm layer lends it in through the bridge below;
//   - saved widget specs live INSIDE custom pages (CustomPage.Widgets[].Spec),
//     which is far enough off the obvious path that the original sweep missed
//     them entirely.
//
// Both survived a merge pointing at a retired id. Neither failed loudly: the
// tally made SMART keep proposing a category that no longer exists, and a saved
// chart reported zero without saying why.

// learnTallyBridge is how the wasm layer lends its persisted tally to a merge.
//
// A package-level bridge rather than a parameter on MergeCategories, for the
// same reason widgetcatalog and smartengine take one: this is a cross-layer
// detail of ONE operation, and threading it through every caller would make
// "remember to also sweep the tally" the caller's problem — which is precisely
// how it came to be missed. Unset (native tests, headless tooling), the merge
// simply has no tally to sweep.
var learnTallyBridge struct {
	load func() learntally.Tally
	save func(learntally.Tally)
}

// SetLearnTallyBridge installs the UI layer's tally accessors. Call once, at
// boot, before any merge can run.
func SetLearnTallyBridge(load func() learntally.Tally, save func(learntally.Tally)) {
	learnTallyBridge.load, learnTallyBridge.save = load, save
}

// loadLearnTally returns the persisted tally, or nil when no bridge is installed.
func loadLearnTally() learntally.Tally {
	if learnTallyBridge.load == nil {
		return nil
	}
	return learnTallyBridge.load()
}

// pageWidgetSpecs collects every persisted widget spec across the household's
// custom pages, so a merge can rewrite a category filter saved into a chart.
func (a *App) pageWidgetSpecs() []domain.WidgetSpec {
	var out []domain.WidgetSpec
	for _, p := range a.CustomPages() {
		for _, w := range p.Widgets {
			if w.Spec != nil {
				out = append(out, *w.Spec)
			}
		}
	}
	return out
}

// saveSweptExtras persists whatever the merge changed in the two out-of-store
// reference classes. It is called by MergeCategories after the store writes, and
// reports the first failure rather than swallowing it — a half-swept merge is
// exactly the state C548 was about.
func (a *App) saveSweptExtras(after catmerge.Refs) error {
	if learnTallyBridge.save != nil && after.Tally != nil {
		learnTallyBridge.save(after.Tally)
	}
	if len(after.Widgets) == 0 {
		return nil
	}
	// Index the rewritten specs by id, then write back only the pages that
	// actually changed — a merge of one chart must not rewrite every page.
	byID := make(map[string]domain.WidgetSpec, len(after.Widgets))
	for _, w := range after.Widgets {
		byID[w.ID] = w
	}
	for _, p := range a.CustomPages() {
		changed := false
		widgets := make([]domain.PageWidget, len(p.Widgets))
		copy(widgets, p.Widgets)
		for i := range widgets {
			if widgets[i].Spec == nil {
				continue
			}
			next, ok := byID[widgets[i].Spec.ID]
			if !ok || sameSeriesFilter(*widgets[i].Spec, next) {
				continue
			}
			spec := next
			widgets[i].Spec = &spec
			changed = true
		}
		if !changed {
			continue
		}
		p.Widgets = widgets
		if err := a.PutCustomPage(p); err != nil {
			return err
		}
	}
	return nil
}

// sameSeriesFilter reports whether two specs select the same transactions — the
// only field a merge rewrites on a widget.
func sameSeriesFilter(a, b domain.WidgetSpec) bool {
	af, bf := "", ""
	if a.Pipeline != nil {
		af = a.Pipeline.Source.Series.Filter
	}
	if b.Pipeline != nil {
		bf = b.Pipeline.Source.Series.Filter
	}
	return af == bf
}
