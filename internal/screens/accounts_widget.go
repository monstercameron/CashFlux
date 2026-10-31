// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// Accounts is the accounts page. Like /transactions, the page is a thin surface host:
// it builds one engine RenderCtx over the live account set and renders a list of
// widget specs through the same spec/render pipeline the dashboard uses
// (safeRenderSpec). Each visible block is its own engine tile:
//
//   - acct-welcome  (Native): first-run load-sample CTA (only when there are no accounts)
//   - acct-summary  (Native): net-worth hero + assets/liabilities + month-to-date trend
//   - acct-toolbar  (Native): search, type/archived filters, chips, transfer/mark-all/FX actions
//   - acct-list     (Native): the owner-scoped, filtered account rows (AccountRow), with an All/Assets/Liabilities toggle
//   - acct-archived (Native): archived accounts (when "show archived" is on and any exist)
//   - acct-formula  (Native): account metrics formulas (when the Formulas toggle is on)
//
// The page-level "Transfer money" action opens a shell-root flip modal
// (app.AccountTransferHost) rather than an inline tile, consistent with the account
// row editors. The tiles share the search/type filter through a uistate atom, so no
// tile embeds another. The host only decides which specs are present, and the engine
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

	// On entering /accounts, reconcile the document-expiry reminder tasks against the
	// current document set. This creates any newly due renewal nudge and resolves any
	// whose document has since been renewed. It runs on mount only; per-change
	// reconciles happen in accounts_docs.go when a document is attached or removed.
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
	_, archived := accountview.Assets(accounts, accountSnapshot(app).Balances, rates, base, activeMemberID)

	// The engine render context: the live data every tile body reads from.
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: accounts, ScopedTxns: txns,
		Rates: rates, Base: base,
		Start: time.Time{}, End: time.Now(),
	}

	// The placement set. Welcome appears only on an empty app, and the archived and
	// formula tiles only when their toggles are on. Summary, toolbar and list are
	// always present.
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

	// Each spec renders through the engine's per-widget error boundary, keyed on the
	// spec id so inserting the transfer or archived tile never shifts another tile's
	// identity (its hooks stay aligned across renders). The sweep proposal cards sit
	// above the bento, like the payday-waterfall card on /goals. They appear only when
	// a sweep rule is due.
	return Div(
		ui.CreateElement(accountsSweepCards),
		ui.CreateElement(surfaceTiles, surfaceTilesProps{Class: "bento-accounts", Specs: specs, Context: rctx}),
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
