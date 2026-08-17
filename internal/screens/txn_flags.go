// SPDX-License-Identifier: MIT

//go:build js && wasm

// Per-charge flags on the ledger row: a likely duplicate (SM-8) and a charge that
// is unusually large for its category (SM-10).
//
// Both detectors already exist and are tested — dedupe.FindDuplicates behind
// SMART-T2, and the SMART-T6 engine — and both already surface on the /smart hub.
// What they could not do is tell you about the row you are looking at, which is
// the whole point of an item-scoped affordance: a duplicate you notice while
// scanning the ledger costs one click to resolve, and the same duplicate found on
// a hub costs a hunt back through the ledger to find it again.
//
// The flags render as a compact chip in the row rather than as a block under it.
// A ledger row lives in a table; a full-width aside beneath one means a colspan
// row, which breaks the row-per-record rhythm the whole page is scanned by.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The catalog codes behind the two flags. Both are Free engines that already
// ship, so there is no new free/AI pairing to introduce here.
const (
	dupeFlagCode  = "SMART-T2"
	spikeFlagCode = "SMART-T6"
)

// txnRowFlags is the per-render index the table hands to its rows: which charges
// are part of a duplicate group, and which are spikes (with the engine's own
// sentence). Computing it ONCE per render is the point — resolving either
// question per row would re-scan the ledger for every row on screen.
type txnRowFlags struct {
	// Duplicate holds the ids of charges that share a signature with another.
	Duplicate map[string]bool
	// Spike maps a charge id to the detector's reason sentence.
	Spike map[string]string
}

// Any reports whether the index found anything at all, so the caller can skip
// threading it through the row props when there is nothing to show.
func (f txnRowFlags) Any() bool { return len(f.Duplicate) > 0 || len(f.Spike) > 0 }

// buildTxnRowFlags computes the flag index for the current ledger. Each half is
// gated on its own feature: a user who wants duplicate detection and not spike
// alerts pays for neither the other scan nor the other chip.
//
// The density gate is applied by the CALLER's chip, not here — the index is also
// what decides whether the props are worth threading at all, and recomputing it
// on a density change would be wasted work either way.
func buildTxnRowFlags(app *appstate.App, settings smart.Settings, weekStart time.Weekday) txnRowFlags {
	out := txnRowFlags{}
	if app == nil {
		return out
	}
	if settings.IsEnabled(dupeFlagCode) && !settings.IsMuted(dupeFlagCode) {
		out.Duplicate = map[string]bool{}
		for _, g := range dedupe.FindDuplicates(app.Transactions()) {
			for _, id := range g.IDs {
				out.Duplicate[id] = true
			}
		}
	}
	if settings.IsEnabled(spikeFlagCode) && !settings.IsMuted(spikeFlagCode) {
		out.Spike = map[string]string{}
		in := buildSmartInput(app, weekStart)
		for _, ins := range smartengine.RunPage(in, settings, smart.PageTransactions) {
			if ins.Feature != spikeFlagCode || ins.Action == nil || ins.Action.RelatedID == "" {
				continue
			}
			// The detector's own sentence, not a second one written here. It already
			// states the multiple and the category's typical charge, and a paraphrase
			// on the row would be a second place for that wording to drift.
			out.Spike[ins.Action.RelatedID] = ins.Detail
		}
	}
	return out
}

// txnFlagChipProps carries one row's flags.
type txnFlagChipProps struct {
	TxnID string
	// Duplicate marks a charge sharing a signature with another.
	Duplicate bool
	// SpikeWhy is the spike detector's reason, or "" when the row is not a spike.
	SpikeWhy string
}

// txnFlagChip renders the row's strongest flag as a compact chip. Its own
// component: it owns a click hook and renders inside the table's row loop.
//
// Only ONE chip renders even when both flags are set. A duplicate outranks a
// spike, because a duplicate is a data error with a concrete fix while a spike is
// an observation — and two warning chips on one row halve the weight of both.
func txnFlagChip(props txnFlagChipProps) ui.Node {
	dupOpen := uistate.UseDuplicatesModalOpen()

	openDupes := ui.UseEvent(func() { dupOpen.Set(true) })
	openTxn := ui.UseEvent(func() { uistate.SetTxnEdit(props.TxnID) })

	settings := uistate.LoadSmartSettings()
	if !settings.DensityOrDefault().Shows(smart.AffordanceBadge) {
		return Fragment()
	}

	if props.Duplicate {
		return Button(css.Class("txn-flag is-dupe", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "txn-flag-dupe"),
			Attr("aria-label", uistate.T("sm8.hint")),
			Title(uistate.T("sm8.detail")),
			OnClick(openDupes),
			uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
			Span(uistate.T("sm8.chip")))
	}
	if props.SpikeWhy != "" {
		return Button(css.Class("txn-flag is-spike", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "txn-flag-spike"),
			Attr("aria-label", props.SpikeWhy),
			Title(props.SpikeWhy),
			OnClick(openTxn),
			uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
			Span(uistate.T("sm10.chip")))
	}
	return Fragment()
}
