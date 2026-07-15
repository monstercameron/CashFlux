// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Accounts is the widgetized accounts surface. Like /transactions, the page is a
// thin SURFACE HOST: it builds one engine RenderCtx over the live account set and
// renders a fixed set of widget specs through the same spec/render pipeline the
// dashboard uses (safeRenderSpec). Every visible block is its own engine tile —
//
//   - acct-welcome  (Native): first-run load-sample CTA (only when there are no accounts)
//   - acct-summary  (Native): net-worth hero + assets/liabilities + month-to-date trend
//   - acct-toolbar  (Native): search, type/archived filters, chips, transfer/mark-all/FX actions
//   - acct-list     (Native): the owner-scoped, filtered account rows (AccountRow), with an All/Assets/Liabilities toggle
//   - acct-archived (Native): archived accounts (when "show archived" is on and any exist)
//
// The page-level "Transfer money" action opens a shell-root flip modal
// (app.AccountTransferHost) rather than an inline tile, consistent with the account
// row editors. The tiles share the search/type filter through a uistate atom, so no
// tile embeds another — the host just decides which specs are present and the engine
// renders each.
func Accounts() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Re-render the surface on any data mutation or shared-state change: a row
	// edit/delete/transfer, a filter change, or opening the transfer sub-view all
	// flow through these atoms.
	_ = uistate.UseDataRevision().Get()

	// AC17: on entering /accounts, reconcile the document-expiry reminder tasks
	// against the current document set — creates any newly-due renewal nudge and
	// auto-resolves any whose document has since been renewed. Runs on mount only
	// (per-mutation reconciles also happen from accounts_docs.go on attach/remove).
	ui.UseEffect(func() func() {
		if created, resolved, err := app.ReconcileDocExpiryTasks(time.Now(), docexpiry.DefaultLeadDays); err == nil && (created > 0 || resolved > 0) {
			uistate.BumpDataRevision()
		}
		return nil
	})

	filterAtom := uistate.UseAccountsFilter()
	formulasAtom := uistate.UseAcctShowFormulas()
	f := filterAtom.Get()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()
	activeMemberID := acctActiveMemberID()
	_, archived := partitionAssetAccounts(accounts, txns, rates, base, activeMemberID)

	// The engine render context: the live data every tile body reads from (§6).
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: accounts, ScopedTxns: txns,
		Rates: rates, Base: base,
		Start: time.Time{}, End: time.Now(),
	}

	// The placement set. Welcome appears only on an empty app; the transfer tile
	// appears with the sub-view open; the archived tile appears when revealed and
	// non-empty. Summary, toolbar, and list are always present.
	var specs []domain.WidgetSpec
	if len(accounts) == 0 {
		specs = append(specs, acctNativeSpec("acct-welcome"))
	}
	specs = append(specs, acctNativeSpec("acct-summary"), acctNativeSpec("acct-toolbar"))
	specs = append(specs, acctNativeSpec("acct-list"))
	if f.ShowArchived && len(archived) > 0 {
		specs = append(specs, acctNativeSpec("acct-archived"))
	}
	if formulasAtom.Get() {
		specs = append(specs, acctNativeSpec("acct-formula"))
	}

	// Render each spec through the engine's per-widget error boundary, keyed on the
	// spec id so inserting the transfer/archived tiles never shifts another tile's
	// identity (its hooks stay aligned across renders). AC7 sweep-proposal cards
	// sit above the bento (like the GL1 waterfall card on /goals) — a quiet
	// preview-approve moment, present only when a sweep rule is due.
	return Div(
		ui.CreateElement(accountsSweepCards),
		Div(css.Class("bento bento-accounts"),
			MapKeyed(specs,
				func(sp domain.WidgetSpec) any { return sp.ID },
				func(sp domain.WidgetSpec) ui.Node {
					c := rctx
					c.Spec = sp
					if node, ok := safeRenderSpec(sp, c); ok {
						return node
					}
					return Fragment()
				},
			),
		),
	)
}

// init registers the accounts-surface widget bodies with the engine render registry,
// keyed by NativeID. The surface host dispatches each placement through this registry;
// the bodies read the shared atoms + the RenderCtx, never surface locals.
func init() {
	R := widgetrender.Register

	R("acct-welcome", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctWelcomeWidget, acctWelcomeProps{App: c.App})
	})
	R("acct-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctSummaryWidget, acctSummaryProps{App: c.App, Base: c.Base, Rates: c.Rates})
	})
	R("acct-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctToolbarWidget, acctToolbarProps{App: c.App})
	})
	R("acct-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctListWidget, acctListProps{App: c.App, Base: c.Base, Rates: c.Rates})
	})
	R("acct-archived", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctArchivedWidget, acctArchivedProps{App: c.App, Base: c.Base, Rates: c.Rates})
	})
	R("acct-formula", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(acctFormulaWidget, acctFormulaProps{App: c.App})
	})
}

// acctNativeSpec builds the seed spec for a Native accounts tile. The surface is
// fixed (not user-reconfigurable or persisted), so the spec is constructed inline
// rather than catalogued in widgetregistry.
func acctNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
