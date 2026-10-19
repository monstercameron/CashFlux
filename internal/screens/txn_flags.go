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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	spikeFlagCode = "SMART-U1"
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
	defer txnPerfStart("flag-recompute")()
	if app == nil {
		return txnRowFlags{}
	}
	work := newTxnFlagsWork(txnFlagsInput(app, weekStart), settings)
	for !work.step() {
	}
	return work.value
}

// txnFlagsInput gathers only the data consumed by Unusual. Other Smart-page
// inputs (tasks, cancellations and paid occurrences) are irrelevant to row flags.
// Read directly from this App to avoid cross-store reuse at equal revisions.
func txnFlagsInput(app *appstate.App, weekStart time.Weekday) smartengine.Input {
	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return smartengine.Input{Now: time.Now(), Base: base, WeekStart: weekStart,
		Rates:    currency.Rates{Base: base, Rates: settings.FXRates},
		Accounts: app.Accounts(), Transactions: app.Transactions(), Categories: app.Categories(),
		Budgets: app.Budgets(), Goals: app.Goals(), Recurring: app.Recurring()}
}

// txnFlagsWork uses the duplicate detector's canonical key and the complete
// cooperative Unusual adapter. Nothing partial escapes while either is working.
type txnFlagsWork struct {
	input    smartengine.Input
	settings smart.Settings
	unusual  *smartengine.UnusualJob
	first    map[string]string
	position int
	value    txnRowFlags
	done     bool
}

func newTxnFlagsWork(input smartengine.Input, settings smart.Settings) *txnFlagsWork {
	w := &txnFlagsWork{input: input, settings: settings}
	if settings.IsEnabled(dupeFlagCode) && !settings.IsMuted(dupeFlagCode) {
		w.first = map[string]string{}
		w.value.Duplicate = map[string]bool{}
	} else {
		w.position = len(input.Transactions)
	}
	if settings.IsEnabled(spikeFlagCode) && !settings.IsMuted(spikeFlagCode) {
		w.unusual = smartengine.NewUnusualJob(input)
	}
	return w
}

func (w *txnFlagsWork) step() bool {
	if w.done {
		return true
	}
	if w.position < len(w.input.Transactions) {
		end := min(w.position+8, len(w.input.Transactions))
		for ; w.position < end; w.position++ {
			row := w.input.Transactions[w.position]
			if row.IsTransfer() {
				continue
			}
			key := dedupe.Key(row)
			if first, ok := w.first[key]; ok {
				w.value.Duplicate[first] = true
				w.value.Duplicate[row.ID] = true
			} else {
				w.first[key] = row.ID
			}
		}
		return false
	}
	if w.unusual != nil {
		if !w.unusual.Step(8) {
			return false
		}
		w.value.Spike = map[string]string{}
		for _, ins := range w.unusual.ActiveInsights(w.settings) {
			if ins.Feature == spikeFlagCode && ins.Action != nil && ins.Action.RelatedID != "" {
				w.value.Spike[ins.Action.RelatedID] = ins.Detail
			}
		}
	}
	w.done = true
	return true
}

func txnFlagEngineSettings(settings smart.Settings) smart.Settings {
	// Do not mutate the persisted settings' maps through a value copy.
	enabled := settings.IsEnabled(spikeFlagCode)
	settings.Enabled = map[string]bool{spikeFlagCode: enabled}
	settings.ExplicitOff = map[string]bool{spikeFlagCode: !enabled}
	for _, feature := range smart.Catalog() {
		if feature.Code != spikeFlagCode {
			settings.ExplicitOff[feature.Code] = true
		}
	}
	return settings
}

// txnFlagChipProps carries one row's flags.
type txnFlagChipProps struct {
	TxnID string
	// Duplicate marks a charge sharing a signature with another.
	Duplicate bool
	// SpikeWhy is the spike detector's reason, or "" when the row is not a spike.
	SpikeWhy string
}

type txnBadgePriority uint8

const (
	txnBadgeNone txnBadgePriority = iota
	txnBadgeSpike
	txnBadgeReview
	txnBadgeDuplicate
)

func txnDescriptionBadge(duplicate, review, spike bool) txnBadgePriority {
	switch {
	case duplicate:
		return txnBadgeDuplicate
	case review:
		return txnBadgeReview
	case spike:
		return txnBadgeSpike
	default:
		return txnBadgeNone
	}
}

func txnRowInfoText(openFollowUps, followUpTotal, receipts int, hasNote bool) string {
	var items []string
	if followUpTotal > 0 {
		key := "uxt5.openFollowUps"
		if openFollowUps == 1 {
			key = "uxt5.openFollowUp"
		}
		items = append(items, uistate.T(key, openFollowUps))
	}
	if receipts > 0 {
		key := "uxt5.receipts"
		if receipts == 1 {
			key = "uxt5.receipt"
		}
		items = append(items, uistate.T(key, receipts))
	}
	if hasNote {
		items = append(items, uistate.T("uxt5.hasNote"))
	}
	return strings.Join(items, ". ")
}

func appendTxnInfo(current, item string) string {
	if strings.TrimSpace(item) == "" {
		return current
	}
	if current == "" {
		return item
	}
	return current + ". " + item
}

func txnCategoryTooltip(txn domain.Transaction, counterparty string) string {
	if txn.HasSplits() {
		_, tooltip := splitCellLabel(txn)
		return tooltip
	}
	if txn.IsTransfer() {
		props := txnFrameRowProps{IsTransfer: true, TransferWith: counterparty, AmountMoney: txn.Amount}
		return uistate.T(transferCategoryKey(props), counterparty)
	}
	return ""
}

func txnRowFlashKey(txn domain.Transaction) string {
	encoded, err := json.Marshal(txn)
	if err != nil {
		return txn.ID
	}
	digest := sha256.Sum256(encoded)
	return txn.ID + ":" + hex.EncodeToString(digest[:])
}

type txnRowInfoProps struct {
	Text         string
	HasReceipt   bool
	HasFollowUps bool
	OnReceipt    func()
	OnFollowUps  func()
}

func txnRowInfo(props txnRowInfoProps) ui.Node {
	open := ui.UseState(false)
	popoverID := ui.UseId()
	hasPopup := "false"
	if props.HasReceipt && props.HasFollowUps {
		hasPopup = "true"
	}
	toggle := ui.UseEvent(Prevent(func() {
		if props.HasReceipt && props.HasFollowUps {
			open.Set(!open.Get())
			return
		}
		if props.HasReceipt && props.OnReceipt != nil {
			props.OnReceipt()
		} else if props.HasFollowUps && props.OnFollowUps != nil {
			props.OnFollowUps()
		}
	}))
	viewReceipt := ui.UseEvent(Prevent(func() {
		open.Set(false)
		if props.OnReceipt != nil {
			props.OnReceipt()
		}
	}))
	openFollowUps := ui.UseEvent(Prevent(func() {
		open.Set(false)
		if props.OnFollowUps != nil {
			props.OnFollowUps()
		}
	}))
	closePopover := func() { open.Set(false) }
	viewReceiptClick := OnClick(viewReceipt)
	openFollowUpsClick := OnClick(openFollowUps)
	toggleClick := OnClick(toggle)
	var popover ui.Node = Fragment()
	var content ui.Node
	if !props.HasReceipt && !props.HasFollowUps {
		if strings.TrimSpace(props.Text) == "" {
			content = Fragment()
		} else {
			content = Span(css.Class("txn-row-info-text"), Attr("data-testid", "txn-row-info-text"), props.Text)
		}
	} else {
		if props.HasReceipt && props.HasFollowUps {
			popover = uiw.Overlay(uiw.OverlayProps{
				Open: open.Get(), Kind: uiw.OverlayPopover,
				Title: uistate.T("uxt5.info", props.Text), AnchorID: popoverID,
				OnClose: closePopover,
				TestID:  "txn-row-info-popover",
				Body: Div(css.Class("txn-row-info-popover"),
					Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "txn-info-receipt"), viewReceiptClick, uistate.T("uxt5.viewReceipt")),
					Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "txn-info-followups"), openFollowUpsClick, uistate.T("uxt5.openFollowUpsAction"))),
			})
		}
		content = Div(css.Class("txn-row-info-wrap"),
			Button(css.Class("txn-row-info"), ID(popoverID), Type("button"),
				Attr("data-testid", "txn-row-info"), Attr("aria-label", uistate.T("uxt5.info", props.Text)),
				Attr("aria-haspopup", hasPopup),
				Attr("aria-expanded", ariaBool(open.Get())), Title(props.Text), toggleClick, "i"),
			popover,
		)
	}
	return content
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
	openDupesClick := OnClick(openDupes)
	openTxnClick := OnClick(openTxn)

	settings := uistate.LoadSmartSettings()
	if !settings.DensityOrDefault().Shows(smart.AffordanceBadge) {
		return Fragment()
	}

	if props.Duplicate {
		return Button(css.Class("txn-flag is-dupe", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "txn-flag-dupe"),
			Attr("aria-label", uistate.T("sm8.hint")+": "+uistate.T("sm8.chip")),
			Title(uistate.T("sm8.detail")),
			openDupesClick,
			uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
			Span(uistate.T("sm8.chip")))
	}
	if props.SpikeWhy != "" {
		return Button(css.Class("txn-flag is-spike", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "txn-flag-spike"),
			Attr("aria-label", props.SpikeWhy+": "+uistate.T("sm10.chip")),
			Title(props.SpikeWhy),
			openTxnClick,
			uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
			Span(uistate.T("sm10.chip")))
	}
	return Fragment()
}
