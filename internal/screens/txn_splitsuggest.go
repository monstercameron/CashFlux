// SPDX-License-Identifier: MIT

//go:build js && wasm

// Split suggestion for one charge (SM-3). It renders at the top of the split
// editor — the moment the work it saves is about to be done by hand — as a faint
// hint rather than a pre-filled form: proposing a breakdown is a guess, and a
// guess that silently fills the fields is one the user has to notice before they
// can disagree with it. Clicking the hint fills them; ignoring it costs nothing.
//
// The free tier reads the merchant's own past splits (internal/splitsuggest). The
// Smart+ tier is offered only when there is no history to copy, which is exactly
// the case a model is worth paying for.
package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/splitsuggest"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The free and paid halves of SM-3. As with SM-2, the gate on the local half must
// be the FREE code: an AI code defaults off and would hide a no-key suggestion.
const (
	splitSuggestFreeCode = "SMART-T21F"
	splitSuggestAICode   = "SMART-T21"
)

// splitSuggestProps carries the charge being split and the callback that fills
// the editor with an accepted proposal.
type splitSuggestProps struct {
	Txn domain.Transaction
	// OnUse hands the editor a complete set of lines that already sum to the
	// charge, so accepting a proposal can never leave a remainder to reconcile.
	OnUse func([]domain.CategorySplit)
}

// splitSuggestHint renders the proposal for one charge. Its own component: it
// owns click hooks and lives inside a modal that re-renders on every keystroke in
// the editor beneath it.
func splitSuggestHint(props splitSuggestProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default

	aiLines := ui.UseState([]domain.CategorySplit(nil))
	loading := ui.UseState(false)
	aiNote := ui.UseState("")

	settings := uistate.LoadSmartSettings()
	txn := props.Txn

	// The local proposal, from this merchant's own past splits.
	var local splitsuggest.Suggestion
	hasLocal := false
	if app != nil {
		local, hasLocal = splitsuggest.Suggest(splitsuggest.Input{
			AmountMinor: txn.Amount.Amount,
			History:     merchantSplitHistory(app, txn),
		})
	}

	backendAI := pr.Normalize().BackendActive()
	aiEnabled := app != nil && aiProviderConfigured(app, backendAI) &&
		settings.IsEnabled(splitSuggestAICode) && !settings.IsMuted(splitSuggestAICode)
	aiConn := smartAIConn{}
	if app != nil {
		aiConn = resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	}

	// Plain closures, NOT UseEvent handlers: these are handed to smartRowHintFor,
	// whose inner component registers the click hook itself. Wrapping them here
	// would put a second hook at this render position for a node that may not
	// render, which is exactly the instability the own-component rule prevents.
	useLocal := func() {
		if props.OnUse != nil && hasLocal {
			props.OnUse(splitsuggest.AsSplits(local, txn.Amount.Currency))
		}
	}
	useAI := func() {
		if props.OnUse != nil {
			if lines := aiLines.Get(); len(lines) > 0 {
				props.OnUse(lines)
			}
		}
	}
	askAI := ui.UseEvent(func() {
		a := appstate.Default
		if a == nil || loading.Get() {
			return
		}
		loading.Set(true)
		aiNote.Set("")
		catalog := smartCatalog(a.Categories())
		charge := txnLabelOf(txn) + " " + fmtMoney(txn.Amount)
		runSmartAI(aiConn, smartai.SplitSuggest(charge, catalog.Prompt()),
			func(text string) {
				loading.Set(false)
				shares := smartai.ParseSplitShares(strings.TrimSpace(text), catalog)
				if len(shares) == 0 {
					aiNote.Set(uistate.T("sm3.aiNoAnswer"))
					return
				}
				// The model proposed SHARES; the app turns them into money. The
				// same largest-remainder apportionment the local path uses, so an
				// accepted AI proposal sums to the charge exactly too.
				weights := make([]int64, len(shares))
				for i, s := range shares {
					weights[i] = int64(s.Percent)
				}
				amounts := splitsuggest.Distribute(txn.Amount.Amount, weights)
				lines := make([]domain.CategorySplit, 0, len(shares))
				for i, s := range shares {
					lines = append(lines, domain.CategorySplit{
						CategoryID: s.CategoryID,
						Amount:     money.New(amounts[i], txn.Amount.Currency),
					})
				}
				aiLines.Set(lines)
			},
			func(e string) { loading.Set(false); aiNote.Set(e) })
	})

	if app == nil || txn.ID == "" {
		return Fragment()
	}

	// A charge that already carries a breakdown needs no proposal — the user has
	// already answered the question this asks.
	if txn.HasSplits() {
		return Fragment()
	}

	// The local proposal wins when it exists: it is free, instant, and grounded in
	// what this household actually did, which is a better answer than a model's.
	if hasLocal {
		return smartRowHintFor(settings, splitSuggestFreeCode, smart.AffordanceFieldAssist, smartRowHintProps{
			Key:         "sm3:" + txn.ID,
			TestID:      "sm3",
			Text:        uistate.T("sm3.hint"),
			Detail:      uistate.T("sm3.detail", local.Precedents),
			ActionLabel: uistate.T("sm3.action"),
			OnAction:    useLocal,
			Dismissible: true,
		})
	}

	// No history to copy. Offer the model only if it is configured and enabled —
	// never a dead control.
	if !aiEnabled {
		return Fragment()
	}
	if lines := aiLines.Get(); len(lines) > 0 {
		return smartRowHintFor(settings, splitSuggestAICode, smart.AffordanceFieldAssist, smartRowHintProps{
			Key:         "sm3ai:" + txn.ID,
			TestID:      "sm3-ai",
			Text:        uistate.T("sm3.hint"),
			Detail:      uistate.T("catSuggest.aiSource"),
			ActionLabel: uistate.T("sm3.action"),
			OnAction:    useAI,
			Dismissible: true,
		})
	}
	label := uistate.T("sm3.aiAction")
	if loading.Get() {
		label = uistate.T("sm3.aiPending")
	}
	return Div(css.Class("sm3-ask", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb2),
		Attr("data-testid", "sm3-ask"),
		Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "sm3-ai-btn"), Attr("aria-disabled", ariaBool(loading.Get())),
			OnClick(askAI), smartGlyph(true, tw.Fold(tw.W35, tw.H35)), Span(label)),
		If(aiNote.Get() != "", Span(css.Class(tw.Text12, tw.TextDim), aiNote.Get())),
	)
}

// merchantSplitHistory returns the OTHER charges from the same merchant, resolved
// through the payee-alias table so processor noise collapses to one merchant.
//
// It hands back every charge rather than filtering to the split ones: splitsuggest
// ignores the unsplit entries itself, and doing the filtering in one place keeps
// the definition of "a usable precedent" with the package that decides it.
func merchantSplitHistory(app *appstate.App, txn domain.Transaction) []domain.Transaction {
	key := reviewqueue.MerchantKey(txn)
	if key == "" {
		return nil
	}
	var out []domain.Transaction
	for _, t := range app.Transactions() {
		if t.ID == txn.ID || t.IsTransfer() {
			continue
		}
		if reviewqueue.MerchantKey(t) == key {
			out = append(out, t)
		}
	}
	return out
}
