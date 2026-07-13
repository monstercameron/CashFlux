// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// accounts_tiles.go holds the Native widget bodies the /accounts surface host
// composes (see accounts_widget.go). Each is a self-contained engine tile: it reads
// the live store (props.App) and the shared accounts-page atoms (filter / transfer
// sub-view), never surface-local closures, so the host can place each tile through
// the same spec/render pipeline the dashboard and /transactions use. The rich
// per-account row (AccountRow) is reused verbatim — these tiles only restructure the
// page around it.

// --- shared tile props ----------------------------------------------------------

type acctSummaryProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctToolbarProps struct{ App *appstate.App }

// AccountPageTransferProps drives the page-level transfer form rendered inside the
// shell-root flip modal (see internal/app AccountTransferHost). OnDone closes it.
type AccountPageTransferProps struct {
	App    *appstate.App
	OnDone func()
}

type acctListProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctArchivedProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
}

type acctWelcomeProps struct{ App *appstate.App }

type acctFormulaProps struct{ App *appstate.App }

// --- shared helpers -------------------------------------------------------------

// acctActiveMemberID resolves the legacy single-owner filter from the top-bar active
// scope: when exactly one owner is scoped, return its id (rows are filtered to that
// member); otherwise "" (all members). It calls the UseActiveScope hook, so invoke it
// at a stable position near the top of a tile body.
func acctActiveMemberID() string {
	if s := uistate.UseActiveScope().Get(); len(s.Owners) == 1 {
		return s.Owners[0]
	}
	return ""
}

// partitionAssetAccounts splits the account set into the visible asset rows and the
// archived asset rows, applying owner scoping and excluding liabilities (which live
// on /debt). Assets are sorted by base-converted balance, largest first, so the
// accounts that move net worth most sit at the top. Pure (no hooks) so any tile can
// call it after fetching its data.
func partitionAssetAccounts(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, base, activeMemberID string) (assets, archived []domain.Account) {
	for _, ac := range accounts {
		if !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		if ac.Class == domain.ClassLiability {
			continue
		}
		if ac.Archived {
			archived = append(archived, ac)
		} else {
			assets = append(assets, ac)
		}
	}
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		if c, err := rates.Convert(bal, base); err == nil {
			return c.Amount
		}
		return bal.Amount
	}
	sort.SliceStable(assets, func(i, j int) bool { return convBal(assets[i]) > convBal(assets[j]) })
	return assets, archived
}

// acctRowCallbacks bundles the per-row mutation handlers the asset/archived list
// tiles hand to each AccountRow. They are plain funcs (no hooks) that mutate the
// store and bump the shared data revision so the whole surface re-renders in step;
// OnView is supplied separately by the tile (it closes over the router + tx filter).
type acctRowCallbacks struct {
	OnDelete     func(string)
	OnArchive    func(domain.Account)
	OnRefresh    func(domain.Account)
	OnSave       func(domain.Account)
	OnSetBalance func(ac domain.Account, current money.Money, newBalStr, catID string)
	OnTransfer   func(fromID, toID, amountStr, dateStr, desc string)
}

// buildAcctRowCallbacks wires the store mutations for AccountRow. Errors surface
// through the global notice/toast (PostNotice) rather than a screen-local error
// state, matching the rest of the widgetized surfaces.
func buildAcctRowCallbacks(app *appstate.App) acctRowCallbacks {
	return acctRowCallbacks{
		OnDelete: func(accountID string) {
			// Refuse to delete an account that still has transactions (including the far
			// leg of a transfer): deleting it would orphan them. Archive keeps history.
			for _, t := range app.Transactions() {
				if t.AccountID == accountID || t.TransferAccountID == accountID {
					uistate.PostNotice(uistate.T("accounts.deleteHasTxns"), true)
					return
				}
			}
			restoreFocus := captureRowFocus(".rows", ".row")
			if err := app.DeleteAccount(accountID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			restoreFocus()
			auditview.CaptureNow()
			uistate.PostUndoable(uistate.T("toast.accountDeleted"))
		},
		OnArchive: func(ac domain.Account) {
			ac.Archived = !ac.Archived
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
		},
		OnRefresh: func(ac domain.Account) {
			ac.BalanceAsOf = time.Now()
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
		},
		OnSave: func(ac domain.Account) {
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
		},
		OnSetBalance: func(ac domain.Account, currentBal money.Money, newStr, catID string) {
			dec := currency.Decimals(ac.Currency)
			target, err := money.ParseMinor(strings.TrimSpace(newStr), dec)
			if err != nil {
				uistate.PostNotice(uistate.T("accounts.invalidBalance"), true)
				return
			}
			// Post an adjustment transaction for the difference so the computed balance
			// equals the figure entered (e.g. matching a statement); the optional catID
			// attaches a category so it doesn't land as an uncategorized spike (L57/L30).
			if amount, ok := ledger.AdjustmentToTarget(currentBal, target); ok {
				adj := domain.Transaction{
					ID: id.New(), AccountID: ac.ID, Date: time.Now(), Desc: uistate.T("accounts.balanceAdjustment"),
					Amount: amount, Cleared: true, CategoryID: catID, Source: domain.TxnSourceManual,
				}
				if err := app.PutTransaction(adj); err != nil {
					uistate.PostNotice(err.Error(), true)
					return
				}
			}
			ac.BalanceAsOf = time.Now()
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(target, ac.Currency))), false)
		},
		OnTransfer: func(fromID, toID, amountStr, dateStr, desc string) {
			doAccountTransfer(app, fromID, toID, amountStr, dateStr, desc)
		},
	}
}

// doAccountTransfer creates a transfer pair from the page/row transfer forms. Pure
// (no hooks): validates the amount, defaults the date/desc, and bumps the revision.
func doAccountTransfer(app *appstate.App, fromID, toID, amountStr, dateStr, desc string) {
	dec := currency.Decimals("")
	for _, ac := range app.Accounts() {
		if ac.ID == fromID {
			dec = currency.Decimals(ac.Currency)
			break
		}
	}
	amtMinor, err := money.ParseMinor(strings.TrimSpace(amountStr), dec)
	if err != nil || amtMinor <= 0 {
		uistate.PostNotice(uistate.T("accounts.transferInvalidAmount"), true)
		return
	}
	var when time.Time
	if d, e := time.Parse("2006-01-02", strings.TrimSpace(dateStr)); e == nil {
		when = d
	}
	d := strings.TrimSpace(desc)
	if d == "" {
		d = uistate.T("accounts.transferDefaultDesc")
	}
	if _, _, err := app.CreateTransferPair(appstate.TransferParams{
		FromAccountID: fromID, ToAccountID: toID, AmountMinor: amtMinor, Date: when, Desc: d,
	}); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.BumpDataRevision()
	uistate.PostNotice(uistate.T("accounts.transferDone"), false)
}

// acctViewTransactions returns an OnView handler that pins the ledger filter to one
// account and navigates to /transactions. nav + txFilter are hooks, so the caller
// resolves them at a stable position and passes them in.
func acctViewTransactions(nav router.Navigator, txFilter state.Atom[uistate.TxFilter]) func(string) {
	return func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
}

// acctViewBillPayments drills to the transactions the user marked as bill payments
// toward an account (Transaction.BillAccountID) — the proof behind the row's
// "last bill" line.
func acctViewBillPayments(nav router.Navigator, txFilter state.Atom[uistate.TxFilter]) func(string) {
	return func(accountID string) {
		f := uistate.TxFilter{BillAccount: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
}

// --- acct-summary ---------------------------------------------------------------

// acctSummaryWidget is the net-worth-dominant summary tile: a hero net-worth figure
// (with a smart explainer + month-to-date trend) beside assets and a liabilities
// total that links through to /debt, plus a missing-exchange-rate notice (L4).
func acctSummaryWidget(props acctSummaryProps) ui.Node {
	// Subscribe to the shared data revision so a mutation in any tile (a transfer, a
	// balance update, a delete) re-renders this tile — its props are the same *App
	// pointer across host renders, so without this the engine would memoize it stale.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	accounts := app.Accounts()
	txns := app.Transactions()

	nw, _ := ledger.NetWorthExplained(accounts, txns, props.Rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities

	// Month-to-date net-worth delta (G3 §3): the honest change since the 1st.
	// The boundary must be UTC midnight (dateutil.MonthStart), not the local
	// month start: txn dates are UTC-midnight calendar dates, so a local
	// boundary (Jul 1 00:00-04:00 = Jul 1 04:00Z) excluded first-of-month
	// transactions and this tile said "No change this month" while the
	// dashboard hero (already UTC) showed the real delta (C341).
	nowTS := time.Now()
	monthStart := dateutil.MonthStart(nowTS)
	var nwDelta money.Money
	haveDelta := false
	if series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{monthStart, nowTS.AddDate(0, 0, 1)}, props.Rates); err == nil && len(series) == 2 {
		if d, derr := series[1].Sub(series[0]); derr == nil {
			nwDelta, haveDelta = d, true
		}
	}

	smartSettings := uistate.LoadSmartSettings()

	body := Div(
		Div(css.Class("nw-summary"),
			Div(css.Class("stat stat-hero"),
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("dashboard.netWorth"),
					smartTooltipFor(smartSettings, "accounts-net", uistate.T("dashboard.netWorth"), uistate.T("smart.tipAccountsNet")),
				),
				Div(ClassStr("stat-value "+accentFor(net)), fmtMoney(net)),
				netWorthDeltaLine(nwDelta, haveDelta),
			),
			stat(uistate.T("accounts.assets"), fmtMoney(assets), "pos"),
			Div(css.Class("stat"),
				A(css.Class("stat-label"), Href(uistate.RoutePath("/debt")), uistate.T("dashboard.liabilities")),
				Div(ClassStr("stat-value neg"), fmtMoney(liabilities)),
			),
		),
		If(len(nw.MissingCurrencies) > 0, P(css.Class("err"), Attr("role", "alert"),
			uistate.T("accounts.nwExcludes", plural(len(nw.ExcludedAccounts), "account"), strings.Join(nw.MissingCurrencies, ", ")))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- acct-toolbar ---------------------------------------------------------------

// acctToolbarGlyph renders one accounts-toolbar action as a standard labeled button
// (the .btn-tool treatment shared with the transactions toolbar): a slightly-grayed
// leading glyph, an always-visible text label, and a small muted trailing badge that
// says what the button DOES — kind "modal" opens a flip modal (⧉), "nav" navigates to
// another page (↗), "action" acts in place (no badge). The visible label removes the
// hover-to-decode step of the old icon-only treatment. variant tints it ("" neutral,
// "primary" accent, "stale" amber); open keeps a modal opener highlighted while its
// modal is showing.
func acctToolbarGlyph(testID string, ic icon.Name, label, kind, variant string, open bool, onClick ui.Handler) ui.Node {
	cls := "btn btn-tool"
	switch variant {
	case "primary":
		cls += " btn-primary"
	case "stale":
		cls += " bt-stale"
	}
	if open {
		cls += " is-open"
	}
	// Muted trailing badge conveys the button's behaviour without a hover: a dialog
	// glyph for modal openers, a diagonal arrow for page navigations.
	var kindBadge ui.Node = Fragment()
	switch kind {
	case "modal":
		kindBadge = Span(css.Class("bt-kind"), Attr("aria-hidden", "true"), "⧉")
	case "nav":
		kindBadge = Span(css.Class("bt-kind"), Attr("aria-hidden", "true"), "↗")
	}
	args := []any{
		css.Class(cls), Type("button"), Attr("data-testid", testID),
		Attr("aria-label", label), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(label),
		kindBadge,
	}
	if kind == "modal" {
		args = append(args, Attr("aria-haspopup", "dialog"), Attr("aria-expanded", boolStr(open)))
	}
	return Button(args...)
}

// acctToolbarWidget is the filter toolbar tile, modelled on the /transactions
// toolbar: a search box, a Filters popover (account type + show-archived), active
// chips, and the page actions (Transfer money, Mark all updated, Manage exchange
// rates). It writes the shared accounts filter + transfer-open atoms so the list and
// transfer tiles react in step.
func acctToolbarWidget(props acctToolbarProps) ui.Node {
	// Subscribe to the data revision so the stale-count badge + FX affordance refresh
	// after a mark-all / balance update (the tile's props are revision-independent).
	_ = uistate.UseDataRevision().Get()
	app := props.App
	filterAtom := uistate.UseAccountsFilter()
	transferAtom := uistate.UseAcctTransferOpen()
	formulasAtom := uistate.UseAcctShowFormulas()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	setFilter := func(mut func(*uistate.AccountsFilter)) {
		nf := filterAtom.Get()
		mut(&nf)
		filterAtom.Set(nf.Normalize())
	}
	onSearch := func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.Search = v }) }

	onToggleArch := ui.UseEvent(Prevent(func() { setFilter(func(x *uistate.AccountsFilter) { x.ShowArchived = !x.ShowArchived }) }))
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	openTransfer := ui.UseEvent(Prevent(func() { transferAtom.Set(true) }))
	openFX := ui.UseEvent(Prevent(func() { uistate.OpenGlobalSettingsAt("household") }))
	markAll := ui.UseEvent(Prevent(func() {
		w := app.FreshnessWindows()
		now := time.Now()
		n := 0
		for _, ac := range app.Accounts() {
			if ac.Archived || !freshness.IsStale(ac, w, now) {
				continue
			}
			ac.BalanceAsOf = now
			if err := app.PutAccount(ac); err != nil {
				uistate.PostNotice(uistate.T("accounts.markErr", err.Error()), true)
				continue
			}
			n++
		}
		if n > 0 {
			uistate.PostNotice(uistate.T("accounts.markedUpdated", plural(n, "balance")), false)
		}
		uistate.BumpDataRevision()
	}))

	accounts := app.Accounts()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	// Type filter options: the asset types actually present in the scoped set.
	typeSet := map[string]bool{}
	hasForeign := false
	windows := app.FreshnessWindows()
	now := time.Now()
	staleCount := 0
	for _, ac := range accounts {
		if freshness.IsStale(ac, windows, now) {
			staleCount++
		}
		if ac.Currency != "" && ac.Currency != base {
			hasForeign = true
		}
		if !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		typeSet[string(ac.Type)] = true
	}
	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Strings(types)
	typeOpts := withAllOption(uistate.T("accounts.allTypes"),
		uiw.OptionsFrom(types, func(s string) string { return s }, func(s string) string { return humanizeType(s) }, ""))

	archLabel := uistate.T("accounts.showArchived")
	if f.ShowArchived {
		archLabel = uistate.T("accounts.hideArchived")
	}
	formulasLabel := uistate.T("accounts.showFormulas")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("accounts.hideFormulas")
	}
	filtersBody := Div(css.Class("filter-fields"),
		filterSelect(uistate.T("accounts.filterType"), f.Type, typeOpts, func(v string) { setFilter(func(x *uistate.AccountsFilter) { x.Type = v }) }),
		withFieldLabel(uistate.T("accounts.archived"),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(f.ShowArchived)),
				Attr("data-testid", "acct-toggle-archived"), OnClick(onToggleArch), Text(archLabel))),
		withFieldLabel(uistate.T("accounts.formulaTitle"),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "acct-toggle-formulas"), OnClick(onToggleFormulas), Text(formulasLabel))),
	)

	chips := []uiw.Chip{}
	if f.Search != "" {
		chips = append(chips, uiw.Chip{Key: "search", Label: uistate.T("accounts.chipSearch", f.Search)})
	}
	if f.Type != "" {
		chips = append(chips, uiw.Chip{Key: "type", Label: uistate.T("accounts.chipType", humanizeType(f.Type))})
	}
	removeChip := func(key string) { filterAtom.Set(f.Without(key).Normalize()) }
	clearAll := func() { filterAtom.Set(uistate.AccountsFilter{ShowArchived: f.ShowArchived}.Normalize()) }

	showFX := hasForeign || len(app.Settings().FXRates) > 0

	toolbar := uiw.FilterToolbar(uiw.FilterToolbarProps{
		Search:       f.Search,
		SearchLabel:  uistate.T("accounts.searchPlaceholder"),
		OnSearch:     onSearch,
		FiltersLabel: uistate.T("accounts.filters"),
		FiltersTitle: uistate.T("accounts.filtersTitle"),
		ActiveAriaLabel: func(n int) string {
			if n == 0 {
				return uistate.T("accounts.filters")
			}
			return uistate.T("accounts.filtersActiveAria", plural(n, "filter"))
		},
		FilterFields:  filtersBody,
		Chips:         chips,
		OnRemoveChip:  removeChip,
		OnClearAll:    clearAll,
		ClearAllLabel: uistate.T("accounts.clearFilters"),
		RemoveLabel:   uistate.T("accounts.removeFilter"),
		Actions: []ui.Node{
			// Labeled toolbar buttons (.btn-tool): a grayed leading glyph + a visible text
			// label, plus a small trailing badge for behaviour — Transfer opens a flip modal
			// (⧉), Manage exchange rates navigates to Settings (↗), and Mark-all is an
			// in-place bulk action (amber, no badge). All left-justified as one group, with
			// the primary "Transfer money" LAST so it anchors the right end of the group.
			If(staleCount > 0, acctToolbarGlyph("acct-markall-btn", icon.Refresh,
				uistate.T("accounts.markAll", plural(staleCount, "account")), "action", "stale", false, markAll)),
			If(showFX, acctToolbarGlyph("acct-fx-btn", icon.Scale,
				uistate.T("accounts.manageFXRates"), "nav", "", false, openFX)),
			// Primary action last → right end of the left-justified group.
			If(len(accounts) >= 2, acctToolbarGlyph("page-transfer-btn", icon.Repeat,
				uistate.T("accounts.transferMoney"), "modal", "primary", transferAtom.Get(), openTransfer)),
		},
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- page transfer form ---------------------------------------------------------

// AccountPageTransferForm is the page-level "Transfer money" editor: pick any two
// accounts and move money between them. It owns its own field hooks and reuses
// doAccountTransfer / CreateTransferPair, then calls OnDone. It renders inside the
// shell-root flip modal (AccountTransferHost) — not an inline tile — so it centers on
// the viewport like the other account editors. From/To option lists each exclude the
// other side so the same account can't be picked twice (C69).
func AccountPageTransferForm(props AccountPageTransferProps) ui.Node {
	app := props.App
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	fromS := ui.UseState("")
	toS := ui.UseState("")
	amtS := ui.UseState("")
	dateS := ui.UseState(time.Now().Format("2006-01-02"))
	descS := ui.UseState("")
	onAmt := ui.UseEvent(func(v string) { amtS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	cancel := ui.UseEvent(Prevent(func() { done() }))
	submit := ui.UseEvent(Prevent(func() {
		from, to := fromS.Get(), toS.Get()
		if from == "" || to == "" || from == to {
			return
		}
		doAccountTransfer(app, from, to, amtS.Get(), dateS.Get(), descS.Get())
		done()
	}))

	pfrom, pto := fromS.Get(), toS.Get()
	fromOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
	toOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
	for _, ac := range app.Accounts() {
		if ac.Archived {
			continue
		}
		lbl := ac.Name + " (" + ac.Currency + ")"
		if ac.ID != pto {
			fromOpts = append(fromOpts, uiw.SelectOption{Value: ac.ID, Label: lbl})
		}
		if ac.ID != pfrom {
			toOpts = append(toOpts, uiw.SelectOption{Value: ac.ID, Label: lbl})
		}
	}
	sameAcct := pfrom != "" && pto != "" && pfrom == pto
	submitDisabled := sameAcct || pfrom == "" || pto == ""

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "page-transfer-form"),
		Attr("aria-label", uistate.T("accounts.transferFormLabel")), OnSubmit(submit),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.transferFromLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: pfrom, OnChange: func(v string) { fromS.Set(v) },
					AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "page-xfer-from-select"})),
			labeledField(uistate.T("accounts.transferToLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: pto, OnChange: func(v string) { toS.Set(v) },
					AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "page-xfer-to-select"})),
			If(sameAcct, P(css.Class("err"), Attr("role", "alert"), uistate.T("accounts.transferSameAccountErr"))),
			labeledField(uistate.T("accounts.transferAmount"),
				Input(css.Class("field"), Attr("id", "page-xfer-amt"), Attr("data-testid", "page-xfer-amt"), Attr("autofocus", ""),
					Type("number"), Placeholder(uistate.T("accounts.transferAmount")), Value(amtS.Get()),
					Step("0.01"), Attr("min", "0.01"), OnInput(onAmt))),
			labeledField(uistate.T("accounts.transferDateLabel"),
				Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.transferDateLabel")),
					Value(dateS.Get()), OnInput(onDate))),
			labeledField(uistate.T("accounts.transferDescLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.transferDefaultDesc")),
					Value(descS.Get()), OnInput(onDesc))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// --- acct-list ------------------------------------------------------------------

// acctListWidget is the accounts-list tile: the owner-scoped, search/type-filtered
// account rows rendered as AccountRow inside an EntityListSection. A segmented
// "All / Assets / Liabilities" toggle in the section header narrows by class so
// both sides of net worth can be spot-checked in one place (assets and liabilities
// used to be two separate tiles, which listed every asset twice). Rows sort by
// signed balance high to low, so the biggest holdings sit on top and the heaviest
// debts (most negative) at the bottom. The tile owns the per-row callbacks and the
// "view transactions" navigation.
func acctListWidget(props acctListProps) ui.Node {
	// Subscribe to the data revision so balances/rows re-render after any mutation.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	filterAtom := uistate.UseAccountsFilter()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	// Client-side navigation to /debt for the "Manage debts" link. Declared
	// unconditionally (stable hook position); it prevents the anchor's default full
	// page load — which otherwise re-booted the app and, with app-lock on, dropped
	// the user onto the lock screen.
	goToDebt := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/debt")) }))

	accounts := app.Accounts()
	txns := app.Transactions()

	// Base-converted net-worth contribution: assets positive, liabilities the
	// negative of their owed magnitude. Taking -Abs for liabilities keeps a debt
	// below the assets regardless of how its balance is signed at rest (the sample
	// stores debts negative; the "amount you owe" add form stores them positive).
	// Falls back to the raw amount for rate-less accounts.
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		m := bal.Amount
		if c, err := props.Rates.Convert(bal, props.Base); err == nil {
			m = c.Amount
		}
		if ac.Class == domain.ClassLiability {
			if m < 0 {
				m = -m
			}
			return -m
		}
		return m
	}

	// Active, owner-visible accounts split by class (archived live in their own tile).
	var active, liabs []domain.Account
	for _, ac := range accounts {
		if ac.Archived || !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		active = append(active, ac)
		if ac.Class == domain.ClassLiability {
			liabs = append(liabs, ac)
		}
	}
	hasLiab := len(liabs) > 0

	// The class view. With no liabilities the toggle is pointless, so force the
	// assets-only view (and hide the segment) — the page reads exactly as before.
	classView := f.Class
	if !hasLiab {
		classView = string(domain.ClassAsset)
	}
	assetsOnly := classView == string(domain.ClassAsset)

	var shown []domain.Account
	for _, ac := range active {
		if assetsOnly && ac.Class == domain.ClassLiability {
			continue
		}
		if classView == string(domain.ClassLiability) && ac.Class != domain.ClassLiability {
			continue
		}
		if f.Matches(ac.Name, string(ac.Type)) {
			shown = append(shown, ac)
		}
	}
	// Signed high to low across every view: the biggest holdings lead and the
	// heaviest debts (most negative) trail.
	sort.SliceStable(shown, func(i, j int) bool { return convBal(shown[i]) > convBal(shown[j]) })

	smartSettings := uistate.LoadSmartSettings()
	pr := uistate.UsePrefs().Get()
	smartIn := buildSmartInput(app, pr.WeekStartWeekday())
	accountInsights := smartengine.RunPage(smartIn, smartSettings, smart.PageAccounts)
	accountByEntity := insightsByEntity(accountInsights)

	windows := app.FreshnessWindows()
	now := time.Now()
	categories := app.Categories()
	members := app.Members()
	accDefs := app.CustomFieldDefsFor("account")
	cbs := buildAcctRowCallbacks(app)
	viewTxns := acctViewTransactions(nav, txFilter)
	viewBills := acctViewBillPayments(nav, txFilter)

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now),
			Members: members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			SmartSettings: smartSettings, SmartByEntity: accountByEntity, ValuationHistory: app.BalanceHistory(ac.ID),
			AccountDefs: accDefs,
			BillPayment: ledger.BillPaymentForAccount(ac.ID, txns), OnViewBills: viewBills,
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	var bodyContent ui.Node
	switch {
	case len(active) == 0:
		bodyContent = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("accounts.noAssets"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account", Icon: icon.Accounts, ImportLink: true})
	case len(shown) == 0 && f.HasNarrowing():
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noMatch"))
	case len(shown) == 0 && classView == string(domain.ClassLiability):
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noLiabilities"))
	case len(shown) == 0:
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noAssets"))
	default:
		bodyContent = Div(css.Class("rows"), MapKeyed(shown, keyOf, renderRow))
	}

	// Section title tracks the class view; the segmented toggle is the class picker
	// (only shown when there is something to toggle between).
	title := uistate.T("accounts.assets")
	if hasLiab {
		switch classView {
		case string(domain.ClassLiability):
			title = uistate.T("accounts.liabilitiesTitle")
		case string(domain.ClassAsset):
			title = uistate.T("accounts.assets")
		default:
			title = uistate.T("accounts.allAccounts")
		}
	}

	var classSeg ui.Node = Fragment()
	if hasLiab {
		classSeg = uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("accounts.classFilterLabel"),
			Selected: classView,
			Options: []uiw.SegOption{
				{Value: "", Label: uistate.T("accounts.classAll"), TestID: "acct-class-all"},
				{Value: string(domain.ClassAsset), Label: uistate.T("accounts.classAssets"), TestID: "acct-class-assets"},
				{Value: string(domain.ClassLiability), Label: uistate.T("accounts.classLiabilities"), TestID: "acct-class-liabilities"},
			},
			OnSelect: func(v string) {
				nf := filterAtom.Get()
				nf.Class = v
				filterAtom.Set(nf.Normalize())
				uistate.PersistAcctClass(v)
				// Flush now so a quick reload after toggling doesn't race the
				// autosave ticker and lose the choice (same guard as sample load, C2).
				uistate.RequestPersist()
			},
		})
	}

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: title,
		// The class filter (All / Assets / Liabilities) is the only header action; the
		// old "Smart" sparkle shortcut was dropped here as noise (the Smart hub is one
		// nav click away and the page already carries the inline Smart strip up top).
		HeaderAction: classSeg,
		Body:         bodyContent,
	})

	// A shortcut to /debt, where liabilities carry the richer payoff surface (min
	// payment, utilization, payoff order). When the assets-only view hides them, the
	// link names how many are tucked away (C346); otherwise it's a plain shortcut.
	// Left-click navigates client-side (goToDebt prevents the full reload); the href
	// stays for keyboard/middle-click. Built via If (not a Go `if`) so the OnClick
	// hook is always registered at a stable position.
	debtLinkText := uistate.T("accounts.manageDebtLink")
	debtLinkTestID := "acct-manage-debt"
	if assetsOnly {
		debtLinkText = uistate.T("accounts.liabilitiesStub", len(liabs))
		debtLinkTestID = "acct-liabilities-stub"
	}
	debtLink := If(hasLiab,
		Div(css.Class(tw.Mt3),
			A(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
				Href(uistate.RoutePath("/debt")),
				Attr("data-testid", debtLinkTestID),
				OnClick(goToDebt),
				uiw.Icon(icon.CreditCard, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				debtLinkText,
			),
		),
	)

	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(section, debtLink),
	})
}

// --- acct-archived --------------------------------------------------------------

// acctArchivedWidget is the archived-accounts tile, placed by the host only when the
// "show archived" toggle is on and archived accounts exist. Rows reuse AccountRow
// (so an archived account can still be restored, viewed, or edited).
func acctArchivedWidget(props acctArchivedProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	filterAtom := uistate.UseAccountsFilter()
	activeMemberID := acctActiveMemberID()
	f := filterAtom.Get()

	accounts := app.Accounts()
	txns := app.Transactions()
	_, archived := partitionAssetAccounts(accounts, txns, props.Rates, props.Base, activeMemberID)

	var shown []domain.Account
	for _, ac := range archived {
		if f.Matches(ac.Name, string(ac.Type)) {
			shown = append(shown, ac)
		}
	}

	windows := app.FreshnessWindows()
	now := time.Now()
	categories := app.Categories()
	members := app.Members()
	accDefs := app.CustomFieldDefsFor("account")
	cbs := buildAcctRowCallbacks(app)
	viewTxns := acctViewTransactions(nav, txFilter)
	viewBills := acctViewBillPayments(nav, txFilter)

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now),
			Members: members, Accounts: accounts, Categories: categories,
			OnDelete: cbs.OnDelete, OnArchive: cbs.OnArchive, OnRefresh: cbs.OnRefresh,
			OnSave: cbs.OnSave, OnView: viewTxns, OnSetBalance: cbs.OnSetBalance, OnTransfer: cbs.OnTransfer,
			ValuationHistory: app.BalanceHistory(ac.ID), AccountDefs: accDefs,
			BillPayment: ledger.BillPaymentForAccount(ac.ID, txns), OnViewBills: viewBills,
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	var bodyContent ui.Node
	if len(shown) == 0 {
		bodyContent = P(css.Class("empty"), uistate.T("accounts.noMatch"))
	} else {
		bodyContent = Div(css.Class("rows"), MapKeyed(shown, keyOf, renderRow))
	}
	section := uiw.EntityListSection(uiw.EntityListSectionProps{Title: uistate.T("accounts.archived"), Body: bodyContent})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-archived", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// --- acct-welcome ---------------------------------------------------------------

// acctWelcomeWidget is the first-run tile (no accounts yet): a load-sample CTA so a
// new user can populate the app and explore.
func acctWelcomeWidget(props acctWelcomeProps) ui.Node {
	app := props.App
	loadSample := ui.UseEvent(Prevent(func() {
		if err := app.LoadSample(); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.SetSampleActive(true)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))
	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("accounts.welcomeTitle"),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("accounts.welcomeDesc")),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(loadSample), uistate.T("accounts.loadSample")),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-welcome", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- acct-formula ---------------------------------------------------------------

// acctFormulaWidget is the opt-in "Account metrics" tile (revealed by the toolbar's
// Formulas toggle). It embeds the reusable FormulaBuilder, which evaluates against
// the live engine variable surface — account aggregates (assets, liabilities,
// net_worth, asset_accounts, …) plus every number-typed account custom field as a
// cf_acct_<key> variable — so it ties custom fields and formulas together: define a
// numeric custom field on your accounts, then compute over it here.
func acctFormulaWidget(props acctFormulaProps) ui.Node {
	body := Div(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("accounts.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("accounts.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "acct-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
