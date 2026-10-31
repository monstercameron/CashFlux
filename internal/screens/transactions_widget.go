// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
	"github.com/monstercameron/CashFlux/internal/txnreview"
	"github.com/monstercameron/CashFlux/internal/txntable"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	"github.com/monstercameron/CashFlux/internal/txnview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

const (
	// txnRowHeight is the fixed ledger row height in px the virtual window measures
	// against (the table rows are uniform: nowrap + table-layout:fixed).
	txnRowHeight = 44
)

var txnDerivedViewCache txnview.Cache

func txnDerivedScope(app *appstate.App, criteria txnfilter.Criteria) string {
	return fmt.Sprintf("%p|%q|%s", app, activeImportProfileWorkspaceID(), txnview.Scope(criteria))
}

// txnTableSortKey hides sortable affordances when register mode fixes the ledger
// to chronological order. An empty key makes DataTable render a plain header.
func txnTableSortKey(registerActive bool, key string) string {
	return txntable.SortKey(registerActive, key)
}

// Transactions is the widgetized global ledger. Per the "everything on the page is
// a widget" rule, the page is a thin SURFACE HOST: it builds one engine RenderCtx
// over the filtered ledger and renders a fixed set of widget specs through the same
// spec/render pipeline the dashboard uses (safeRenderSpec). Every visible block is
// its own engine widget tile —
//
//   - txn-toolbar   (Native): search, filters, chips, add/export, import & duplicates (both flip modals)
//   - txn-bulkbar   (Native): bulk recategorize / clear / export / delete (when a selection exists)
//   - txn-table     (Table) : the engine-hydrated ledger frame, paginated, with row drill-edit
//
// (Both import and duplicates review open as shell-root flip modals over the ledger —
// ImportPanelHost and DuplicatesHost — so the ledger is always the main slot; there are
// no in-place sub-views left.)
//
// The tiles share their interaction state (filter, selection, undo, receipt preview)
// through atoms in uistate, so no tile embeds another — the host just decides which
// specs are present and the engine renders each. The receipt
// preview is a modal overlay (like the edit host), not a bento tile.
func Transactions() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: uiw.Callout(uiw.CalloutProps{Tone: uiw.CalloutWarn, Body: Span(uistate.T("common.notReady"))})})
	}
	return ui.CreateElement(transactionsSurface, app)
}

func transactionsSurface(app *appstate.App) ui.Node {
	defer txnPerfStart("page")()
	_ = uistate.UseDataRevision().Get()
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	previewAtom := uistate.UseTxnPreview()
	filterAtom := uistate.UseTxFilter()
	f, _ := filterAtom.Get().WithOwnerLens(uistate.UseMemberLens())
	useTxnSelectionScope(f, selAtom, anchorAtom)
	_, single := f.SingleAccount()
	registerMode := uistate.UseTxnRegisterMode().Get()
	register := single && registerMode
	f = txntable.EffectiveCriteria(f, register, hiddenSortKeys(uistate.UseTxnCols().Get()))
	rctx := useTxnSurfaceContext(app, f)
	selected := len(selAtom.Get()) > 0
	specs := txnSurfaceSpecs(txntable.SurfaceIDs(selected, uistate.UseTxnViewMode().Get() == uistate.TxnViewCalendar))
	bento := Div(css.Class(txntable.SurfaceClass(selected)),
		MapKeyed(specs, func(sp domain.WidgetSpec) any { return sp.ID }, func(sp domain.WidgetSpec) ui.Node {
			c := rctx
			c.Spec = sp
			return ui.CreateElement(txnPlacement, txnPlacementProps{Context: c, Revision: app.Rev(), Scope: fmt.Sprintf("%p|%s|%#v", app, activeImportProfileWorkspaceID(), f)})
		}),
	)
	return Div(txnReceiptPreviewOverlay(app, previewAtom),
		A(css.Class("txn-skip-link"), Attr("href", "#txn-ledger-anchor"), uistate.T("txw1.skipToTransactions")), bento)
}

func useTxnSelectionScope(f txnfilter.Criteria, selAtom state.Atom[map[string]bool], anchorAtom state.Atom[string]) {
	scope := txntable.SelectionScope(f)
	previous := ui.UseState(scope)
	ui.UseEffect(func() func() {
		if previous.Get() != scope {
			previous.Set(scope)
			if len(selAtom.Get()) > 0 {
				selAtom.Set(map[string]bool{})
				anchorAtom.Set("")
			}
		}
		return nil
	}, scope)
}

func txnSurfaceSpecs(ids []string) []domain.WidgetSpec { return txntable.SurfaceSpecs(ids) }

func useTxnSurfaceContext(app *appstate.App, f txnfilter.Criteria) widgetrender.RenderCtx {
	accounts := ui.UseMemo(func() []domain.Account { return app.Accounts() }, revKey(app))
	categories := ui.UseMemo(func() []domain.Category { return app.Categories() }, revKey(app))
	transactions := ui.UseMemo(func() []domain.Transaction { return app.Transactions() }, revKey(app))
	rates := ui.UseMemo(func() currency.Rates {
		settings := app.Settings()
		return currency.Rates{Base: txntable.BaseCurrency(settings.BaseCurrency), Rates: settings.FXRates}
	}, revKey(app))
	base := rates.Base
	index := ui.UseMemo(func() txntable.Index {
		return txntable.BuildIndex(nil, accounts, categories, nil, nil, nil)
	}, revKey(app))
	accName, catName := index.AccountNames, index.CategoryNames
	resolver := app.PayeeResolver()
	derivedFilter := f
	derivedFilter.Page, derivedFilter.PageSize = 0, 0
	month := dateutil.MonthStart(time.Now())
	derived := ui.UseMemo(func() *txntoolbar.Snapshot {
		labels := txnfilter.Labels{
			Account: accName, Category: catName, Payee: memoTxnLabel(resolver.Resolve),
			BaseCurrency: base, ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, base) },
		}
		return &txntoolbar.Snapshot{Revision: app.Rev(), Scope: txnDerivedScope(app, derivedFilter),
			Value: deriveTxnScope(app, transactions, derivedFilter, labels, rates, base)}
	}, revKey(app), derivedFilter, activeImportProfileWorkspaceID(), month)
	shown := ui.UseMemo(func() []domain.Transaction {
		rows := txntable.OrderDescriptions(derived.Value.Rows, derivedFilter, resolver.HasLearned, resolver.Resolve)
		return txntable.VisibleTransfers(rows, derivedFilter, rates)
	}, derived)

	return widgetrender.RenderCtx{App: app, Accounts: accounts, Txns: transactions, ScopedAccounts: accounts, ScopedTxns: shown,
		TxnDerived: derived, Rates: rates, Base: base, Start: time.Time{}, End: dateutil.CalendarDayUTC(time.Now())}
}

// deriveTxnScope is the single checked ledger derivation boundary. Independent
// widget previews use it too; surface children reuse the host snapshot instead.
func deriveTxnScope(app *appstate.App, rows []domain.Transaction, f txnfilter.Criteria, labels txnfilter.Labels, rates currency.Rates, base string) txntoolbar.Derived {
	defer txnPerfStart("scope-derive")()
	defer txnPerfStart("filter-sort")()
	value, err := txntoolbar.Derive(rows, f, labels, rates, base, time.Now())
	if err != nil {
		slog.Error("Could not total the transaction scope", "error", err)
	}
	return value
}

type txnPlacementProps struct {
	Context  widgetrender.RenderCtx
	Revision uint64
	Scope    string
}

func txnPlacement(props txnPlacementProps) ui.Node {
	c := props.Context
	return ui.UseMemo(func() ui.Node {
		defer txnPerfStart("placement-" + c.Spec.ID)()
		if node, ok := safeRenderSpec(c.Spec, c); ok {
			return node
		}
		return Fragment()
	}, c.App, props.Revision, props.Scope, c.Spec.ID, c.TxnDerived)
}

// init registers the transactions-surface widget bodies with the engine render
// registry, keyed by NativeID (Native tiles) and id (the data-driven table). The
// surface host dispatches each placement through this registry — bodies read the
// shared atoms + the RenderCtx, never surface locals, which is what makes them
// engine widgets rather than embedded nodes.
func init() {
	R := widgetrender.Register

	R("txn-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: c.App, Base: c.Base, Rates: c.Rates, Shown: c.ScopedTxns, Derived: c.TxnDerived})
	})
	R("txn-bulkbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnBulkBarWidget, txnBulkBarProps{App: c.App})
	})
	R("txn-calendar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnCalendarWidget, txnCalendarProps{App: c.App, Base: c.Base, Shown: c.ScopedTxns})
	})

	widgetrender.RegisterFrame("txn-table", func(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnTableWidget, txnTableProps{Frame: fr, Shown: c.ScopedTxns, App: c.App, Base: c.Base, Derived: c.TxnDerived})
	})
}

// setTxFilterOn applies a mutation to the shared transaction filter atom, resetting
// the page if the scope changed and persisting the result. Shared by the tiles that
// write the filter (the toolbar's field handlers, the table's sort/pagination).
func setTxFilterOn(atom state.Atom[uistate.TxFilter], mut func(*uistate.TxFilter)) {
	prev := atom.Get()
	nf := prev
	mut(&nf)
	nf = nf.ResetPageIfScopeChanged(prev).Normalize()
	atom.Set(nf)
	uistate.PersistTxFilter(nf)
}

// txnReceiptPreviewOverlay renders the receipt preview modal (L29) while the shared
// preview atom holds an attachment. The table's row paperclip opens it; the close
// button, Escape and a backdrop click clear the atom. It is a modal dialog named by
// its heading: focus is trapped inside while it is open and returns to the control
// that opened it on close. It is a dialog overlay, not a bento tile, mirroring how
// the edit form is a modal host rather than a widget.
//
// The overlay hooks run on every render, open or closed, so focus is restored when the
// atom is cleared.
func txnReceiptPreviewOverlay(app *appstate.App, previewAtom state.Atom[domain.AttachmentRef]) ui.Node {
	dismiss := func() { previewAtom.Set(domain.AttachmentRef{}) }
	closePreview := ui.UseEvent(Prevent(dismiss))
	closeOption := OnClick(closePreview)
	ref := previewAtom.Get()
	open := ref.ArtifactID != ""
	art := domain.Artifact{}
	if found := txntable.PreviewArtifact(ref.ArtifactID, app.Artifacts()); open && found != nil {
		art = *found
	}
	url := useArtifactImageURL(app, art)
	var dialog ui.Node
	if open {
		var body ui.Node
		if url != "" {
			body = Img(Attr("src", url), Attr("alt", uistate.T("transactions.previewAlt", ref.Name)), css.Class(tw.MaxWFull))
		} else {
			body = uiw.Callout(uiw.CalloutProps{Tone: uiw.CalloutWarn, Body: Span(uistate.T("transactions.previewMissing"))})
		}
		dialog = uiw.Card(uiw.CardProps{
			Header: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
				H2(css.Class("card-title"), Attr("id", "receipt-preview-title"), uistate.T("transactions.previewReceipt")),
				Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.previewClose")), Attr("data-testid", "receipt-preview-close"), closeOption, uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
			),
			Body: body,
		})
	}
	return ui.AccessibleOverlay(ui.AccessibleOverlayProps{
		Open: open, Target: ui.PortalTarget{Selector: "body"}, SurfaceID: "receipt-preview-surface",
		Kind: ui.OverlayKindDialog, Role: "dialog", LabelledBy: "receipt-preview-title", Modal: true,
		TrapFocus: true, RestoreFocus: true, CloseOnEscape: true, CloseOnOutsideClick: true,
		LockScroll: true, BackgroundInert: true, Backdrop: true, BaseZIndex: 3000,
		BackdropClass:        "receipt-preview-overlay",
		InitialFocusSelector: `[data-testid="receipt-preview-close"]`,
		Child:                dialog, OnDismiss: dismiss,
	})
}

// txnTableProps carries the engine-hydrated ledger frame and the data the table tile
// needs to present it: the filtered transactions (for attachments + empty states),
// the app, and the base currency.
type txnTableProps struct {
	Frame   domain.Frame
	Shown   []domain.Transaction
	App     *appstate.App
	Base    string
	Derived *txntoolbar.Snapshot
}

type txnTableController struct {
	props                                txnTableProps
	filter                               txnfilter.Criteria
	filterAtom                           state.Atom[uistate.TxFilter]
	selection                            state.Atom[map[string]bool]
	anchor                               state.Atom[string]
	preview                              state.Atom[domain.AttachmentRef]
	columns                              uistate.TxnCols
	register                             bool
	registerID                           string
	lensName                             string
	searchPending                        bool
	page                                 txntable.Page
	frame                                txntable.FrameColumns
	index                                txntable.Index
	rowContext                           txntable.RowContext
	smart                                txnRowFlags
	suggestionsReady, suggestionsEnabled bool
	followUps                            map[string]txntable.FollowUpInfo
	visible                              []string
	actions                              txnTableActions
}

type txnTableActions struct {
	openEdit, openSplit, createRule, confirmCategory, toggleExclude, deleteRow func(string)
	pairRefund, unpair, ungroup, unlinkBill                                    func(string)
	openLink                                                                   func(string, string)
	viewReceipt                                                                func(domain.AttachmentRef)
	openFollowUps                                                              func()
	tagFilter                                                                  func(string)
	toggleSelect                                                               func(string, bool)
}

// txnTableWidget is the txn-table FrameRenderer body: the engine-hydrated ledger
// frame, paginated and painted as a sortable, selectable DataTable inside a widget
// tile. It owns its sort/pagination/selection hooks (the surface host passes only
// data), reading and writing the shared filter + selection atoms so the toolbar and
// bulk tiles stay in step. Rows drill into the edit modal; the leading checkbox
// toggles bulk selection; the paperclip opens a receipt.
func txnTableWidget(props txnTableProps) ui.Node {
	defer txnPerfStart("table")()
	c := useTxnTableController(props)
	useTxnTableData(c)
	useTxnTableActions(c)
	category := useTxnTableCategoryTarget(props.App)
	renderRow := useTxnTableRows(c, category.Choose)
	body := ui.CreateElement(txnTableBody, txnTableBodyProps{App: props.App, Filter: c.filter, Columns: c.columns,
		Page: c.page, Total: props.Frame.Rows, Visible: c.visible, ShowBalance: c.rowContext.Balances != nil, Register: c.register,
		SearchPending: c.searchPending, RenderRow: renderRow, IDAt: func(i int) any { return c.frame.ID.Str(i) },
		OnSort: func(key string) {
			setTxFilterOn(c.filterAtom, func(f *uistate.TxFilter) { *f = txntable.SortBy(*f, key, c.register) })
		},
		OnPage: func(p int) { setTxFilterOn(c.filterAtom, func(f *uistate.TxFilter) { f.Page = p }) },
		OnPageSize: func(size int) {
			setTxFilterOn(c.filterAtom, func(f *uistate.TxFilter) { f.PageSize, f.Page = size, 1 })
		},
	})
	return ui.CreateElement(txnTableRegion, txnTableRegionProps{App: props.App, Body: body, Filter: c.filter, Shown: props.Shown, Base: props.Base, Category: category, Derived: props.Derived, Lens: c.lensName})
}

func useTxnTableController(props txnTableProps) *txnTableController {
	c := &txnTableController{props: props}
	c.filterAtom = uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	var lensOn bool
	c.filter, lensOn = c.filterAtom.Get().WithOwnerLens(memberLens)
	memberNames := txntoolbar.BuildIndexes(nil, nil, props.App.Members(), nil).Members
	c.lensName = txntoolbar.LensName(memberLens, lensOn, memberNames, func(n int) string { return plural(n, "member") })
	c.searchPending = uistate.UseTxnSearchPending().Get()
	c.selection = uistate.UseTxnSelection()
	c.anchor = uistate.UseTxnSelAnchor()
	c.preview = uistate.UseTxnPreview()
	c.columns = uistate.UseTxnCols().Get()
	id, single := c.filter.SingleAccount()
	registerMode := uistate.UseTxnRegisterMode().Get()
	c.registerID, c.register = id, single && registerMode
	c.filter = txntable.EffectiveCriteria(c.filter, c.register, hiddenSortKeys(c.columns))
	c.page = txntable.Paginate(c.filter.Page, c.filter.PageSize, props.Frame.Rows)
	c.frame = txntable.ReadFrame(props.Frame)
	return c
}

func useTxnTableData(c *txnTableController) {
	props := c.props
	c.index = ui.UseMemo(func() txntable.Index {
		return txntable.BuildIndex(props.Shown, props.App.Accounts(), props.App.Categories(), props.App.Members(), props.App.TxnLinks(), props.App.Events())
	}, props.App, props.App.Rev(), c.filter, props.Shown, len(props.Shown))
	balances := ui.UseMemo(func() map[string]money.Money {
		return txntable.RegisterBalances(c.register, c.registerID, props.App.Accounts(), props.App.Transactions())
	}, props.App, props.App.Rev(), c.registerID, c.register)
	groups := txnlinks.GroupsByTxn(props.App.TxnLinks())
	resolver := props.App.PayeeResolver()
	c.rowContext = txntable.RowContext{Index: c.index, Filter: c.filter, Now: time.Now(),
		Labels: txntable.Labels{T: uistate.T, Ledger: fmtLedgerAmount, Money: fmtMoney}, Learned: resolver.HasLearned, Resolve: resolver.Resolve,
		Balances: balances, Groups: groups, GroupTransactions: txnGroupMembersMemo(props.App, int(props.App.Rev()), groups),
		Rates: currency.Rates{Base: props.Base, Rates: props.App.Settings().FXRates},
	}
	settings := uistate.LoadSmartSettings()
	finish := txnPerfStart("flags")
	c.smart = useTxnRowFlags(props.App, int(props.App.Rev()), settings, uistate.LoadPrefs().WeekStartWeekday())
	finish()
	c.suggestionsReady = useAfterSettle("txn-suggestions")
	c.suggestionsEnabled = settings.IsEnabled(catSuggestFreeCode)
	useTxnFocusRow(txnFocusRowHint{Virtual: c.page.Virtual, RowHeight: txnRowHeight, Scroller: "main.cf-scroll", IndexOf: c.frame.IndexOf})
	c.followUps = txntable.FollowUps(props.App.Tasks(), uistate.UsePrefs().Get().FormatDate)
	c.visible = ui.UseMemo(func() []string { return txntable.IDs(c.frame.ID, c.page.SelectionStart, c.page.SelectionEnd) },
		props.App, props.App.Rev(), c.filter, c.frame.ID, len(c.frame.ID.Values), c.page.SelectionStart, c.page.SelectionEnd)
}

func useTxnTableActions(c *txnTableController) {
	useTxnTableNavigation(c)
	useTxnTableSelection(c)
	useTxnTableMutations(c)
	useTxnTableLinks(c)
}

func useTxnTableSelection(c *txnTableController) {
	c.actions.toggleSelect = ui.UseCallback(func(id string, shift bool) {
		var order []string
		if shift && c.anchor.Get() != "" && c.anchor.Get() != id {
			order = txntable.IDs(c.frame.ID, c.page.SelectionStart, c.page.SelectionEnd)
		}
		c.selection.Set(txntable.ToggleSelection(c.selection.Get(), c.anchor.Get(), id, shift, order))
		c.anchor.Set(id)
	}, c.filter, c.props.App, c.props.App.Rev(), c.frame.ID, len(c.frame.ID.Values), c.page.SelectionStart, c.page.SelectionEnd)
	c.actions.openEdit = ui.UseCallback(func(id string) { uistate.SetTxnEdit(id) }, c.props.App)
	c.actions.openSplit = ui.UseCallback(func(id string) { uistate.SetTxnSplit(id) }, c.props.App)
	c.actions.viewReceipt = ui.UseCallback(func(ref domain.AttachmentRef) { c.preview.Set(ref) }, c.preview)
}

func useTxnTableNavigation(c *txnTableController) {
	nav := router.UseNavigate()
	memberLens := uistate.UseMemberLens()
	c.actions.openFollowUps = ui.UseCallback(func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkTransaction)
		txnLeaveFor("/todo")
		nav.Navigate(uistate.RoutePath("/todo"))
	}, nav)
	c.actions.createRule = ui.UseCallback(func(id string) {
		row, ok := c.props.App.TransactionByID(id)
		if !ok {
			return
		}
		uistate.SetRuleDraft(txntable.RulePhrase(row), row.CategoryID)
		txnLeaveForRow("/rules", id)
		nav.Navigate(uistate.RoutePath("/rules"))
	}, c.props.App, nav)
	c.actions.tagFilter = ui.UseCallback(func(tag string) {
		prev := c.filterAtom.Get()
		setTxFilterOn(c.filterAtom, func(f *uistate.TxFilter) { *f = f.DrillToTag(tag) })
		next := c.filterAtom.Get()
		effective, _ := next.WithOwnerLens(memberLens)
		rates := currency.Rates{Base: c.props.Base, Rates: c.props.App.Settings().FXRates}
		labels := txnfilter.Labels{Payee: c.props.App.PayeeResolver().Resolve, BaseCurrency: c.props.Base, ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, c.props.Base) }}
		if !txntable.TagFilterNotice(prev, next, effective, c.props.App.Transactions(), labels, len(c.props.Shown)) {
			return
		}

		uistate.PostNotice(uistate.T("transactions.tagFilterApplied", tag), false)
	}, c.props.App, c.props.App.Rev(), c.filter, memberLens, c.props.Shown, len(c.props.Shown), c.props.Base)
}

func useTxnTableMutations(c *txnTableController) {
	confirmCategory := func(id string) {
		t, ok := c.props.App.TransactionByID(id)
		if !ok {
			return
		}
		confirmed := txnreview.Confirm(t, t.CategoryID)
		_ = commitUndoable(uistate.T("transactions.confirmedCategory"), []string{id}, func() error { return c.props.App.PutTransactionsAtomic([]domain.Transaction{confirmed}) })
	}
	c.actions.confirmCategory = ui.UseCallback(confirmCategory, c.props.App)
	c.actions.toggleExclude = ui.UseCallback(txnTableExcludeAction(c.props.App), c.props.App)
	c.actions.deleteRow = ui.UseCallback(txnTableDeleteAction(c.props.App), c.props.App)
}

func txnTableExcludeAction(app *appstate.App) func(string) {
	applyExclude := func(t domain.Transaction, exclude bool) {
		live, ok := app.TransactionByID(t.ID)
		if !ok {
			return
		}
		live.ExcludeFromReports = exclude
		key := txntable.ExcludeNoticeKey(exclude)
		_ = commitUndoable(uistate.T(key, txnShortLabel(live)), []string{t.ID}, func() error { return app.PutTransactionsAtomic([]domain.Transaction{live}) })
	}
	toggleExclude := func(id string) {
		rows, err := app.TransactionsChecked()
		if err != nil {
			uistate.PostNotice(uistate.T("transactions.readFailed"), true)
			return
		}
		for _, t := range rows {
			if t.ID != id {
				continue
			}
			if t.ExcludeFromReports {
				applyExclude(t, false)
				return
			}
			row := t
			uistate.ConfirmModalLabeled(
				uistate.T("transactions.excludeConfirm", txnShortLabel(row)),
				uistate.T("transactions.excludeConfirmBtn"),
				true,
				func(ok bool) {
					if ok {
						applyExclude(row, true)
					}
				},
			)
			return
		}
	}
	return toggleExclude
}

func txnTableDeleteAction(app *appstate.App) func(string) {
	deleteRow := func(id string) {
		rows, err := app.TransactionsChecked()
		if err != nil {
			uistate.PostNotice(uistate.T("transactions.readFailed"), true)
			return
		}
		var txn domain.Transaction
		found := false
		for _, t := range rows {
			if t.ID == id {
				txn, found = t, true
				break
			}
		}
		if !found {
			return
		}
		linked := txntable.OpenTaskCount(app.Tasks(), id)

		what := txntable.DeletePrompt(app.ExpandTransferPairs([]string{txn.ID}), rows, app.Accounts(), linked, txntable.Labels{T: uistate.T, Money: fmtMoney})
		uistate.ConfirmModal(what, true, func(ok bool) {
			if !ok {
				return
			}
			ids := app.ExpandTransferPairs([]string{id})
			_ = commitUndoable(uistate.T("toast.txnDeleted"), ids, func() error { return app.DeleteTransactionWithTransferPair(id) })
		})
	}
	return deleteRow
}

func useTxnTableLinks(c *txnTableController) {
	linkAtom := uistate.UseTxnLinkTarget()
	refundAtom := uistate.UseRefundPairTarget()
	c.actions.openLink = ui.UseCallback(func(id, mode string) { linkAtom.Set(uistate.TxnLinkTarget{TxnID: id, Mode: mode}) }, linkAtom)
	pairRefundRow := func(id string) { refundAtom.Set(id) }
	ungroupRow := func(id string) {
		if l, ok := txnlinks.GroupOf(id, c.props.App.TxnLinks()); ok {
			if err := c.props.App.DeleteTxnLink(l.ID); err != nil {
				uistate.PostNotice(uistate.T("txnlinks.groupErr", err.Error()), true)
				return
			}
			uistate.PostNotice(uistate.T("txnlinks.ungrouped"), false)
			uistate.BumpDataRevision()
		}
	}
	unpairRow := func(id string) {
		if l, ok := txnlinks.PairOf(id, c.props.App.TxnLinks()); ok {
			if err := c.props.App.DeleteTxnLink(l.ID); err != nil {
				uistate.PostNotice(uistate.T("txnlinks.pairErr", err.Error()), true)
				return
			}
			uistate.PostNotice(uistate.T("txnlinks.unpaired"), false)
			uistate.BumpDataRevision()
		}
	}
	unlinkBillRow := func(id string) {
		if err := c.props.App.UnlinkBill(id); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.PostNotice(uistate.T("billmatch.unlinkLogged"), false)
		uistate.BumpDataRevision()
	}
	c.actions.pairRefund = ui.UseCallback(pairRefundRow, c.props.App)
	c.actions.ungroup = ui.UseCallback(ungroupRow, c.props.App)
	c.actions.unpair = ui.UseCallback(unpairRow, c.props.App)
	c.actions.unlinkBill = ui.UseCallback(unlinkBillRow, c.props.App)
}

func useTxnTableRows(c *txnTableController, choose func(string)) func(int) ui.Node {
	cache := ui.UseRef(&txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)})
	return func(i int) ui.Node {
		in := c.frame.RowAt(i)
		row := c.index.Transactions[in.ID]
		in.SuggestionsReady = c.suggestionsReady
		in.Selected = c.selection.Get()[in.ID]
		in.Duplicate, in.Spike = c.smart.Duplicate[in.ID], c.smart.Spike[in.ID]
		in.Automatic = txnAutoByID(c.props.App, []domain.Transaction{row}, c.props.App.Rules(), int(c.props.App.Rev()))[in.ID].IsAutomatic()
		in.FollowUpOpen, in.FollowUpTotal = c.followUps[in.ID].Open, c.followUps[in.ID].Total
		in.SuggestedCategory = txnTableSuggestedCategory(c, row)
		derived := txntable.DeriveRow(c.rowContext, in)
		r := txnFramePropsFromRow(derived)
		r.Vis = txnColsForScope(c.columns, c.filter.Member, len(c.index.MemberNames))
		r.FollowUps = txnTableFollowUpItems(c.followUps[in.ID].Items)
		r.RowIndex = c.page.RowIndex(i)
		c.actions.bind(&r)
		r.OnChooseCategory = choose
		r.OnSuggestCategory = func(id string) { uistate.SetCatSuggest(id) }
		r.OnReceiptSplit = func(id string) { txnTableReceiptSplit(c.props.App, id) }
		return ui.CreateElement(txnFrameRow, cache.Get().reuse(r))
	}
}

func txnTableSuggestedCategory(c *txnTableController, row domain.Transaction) string {
	if !txntable.SuggestionEligible(row, c.suggestionsReady, c.suggestionsEnabled) {
		return ""
	}
	inputs := txnCatSuggestInputsFor(c.props.App, int(c.props.App.Rev()))
	suggestion, ok := inputs.suggestion(c.props.App, row)
	return txntable.SuggestedCategory(row, suggestion, ok, inputs.categories, c.index.CategoryNames)
}

func txnTableReceiptSplit(app *appstate.App, id string) {
	row, ok := app.TransactionByID(id)
	if !ok {
		uistate.PostNotice(uistate.T("receiptsplit.noTxn"), true)
		return
	}
	startReceiptSplitFlow(app, row)
}

func (a txnTableActions) bind(r *txnFrameRowProps) {
	r.OnOpen, r.OnOpenSplit, r.OnCreateRule = a.openEdit, a.openSplit, a.createRule
	r.OnConfirmCategory, r.OnToggleExclude, r.OnDelete = a.confirmCategory, a.toggleExclude, a.deleteRow
	r.OnPairRefund, r.OnUnpair, r.OnUngroup, r.OnUnlinkBill = a.pairRefund, a.unpair, a.ungroup, a.unlinkBill
	r.OnOpenLink, r.OnViewReceipt, r.OnOpenFollowUps = a.openLink, a.viewReceipt, a.openFollowUps
	r.OnTagClick, r.OnToggleSelect = a.tagFilter, a.toggleSelect
}

func txnFramePropsFromRow(r txntable.Row) txnFrameRowProps {
	return txnFrameRowProps{
		ID:                  r.ID,
		AmountMoney:         r.AmountMoney,
		ShowSuggestions:     r.ShowSuggestions,
		Date:                r.Date,
		Amount:              r.Amount,
		AmtTone:             r.AmtTone,
		Desc:                r.Desc,
		Tags:                r.Tags,
		Account:             r.Account,
		Category:            r.Category,
		CategoryTooltip:     r.CategoryTooltip,
		CategoryID:          r.CategoryID,
		SuggestedCategory:   r.SuggestedCategory,
		Source:              r.Source,
		MemberNames:         r.MemberNames,
		MemberColors:        r.MemberColors,
		OwnersResolved:      r.OwnersResolved,
		Cleared:             r.Cleared,
		Reconciled:          r.Reconciled,
		Reviewed:            r.Reviewed,
		Queued:              r.Queued,
		Duplicate:           r.Duplicate,
		SpikeWhy:            r.SpikeWhy,
		Unfiled:             r.Unfiled,
		AutoCategory:        r.AutoCategory,
		Selected:            r.Selected,
		Receipts:            r.Receipts,
		Attachment:          r.Attachment,
		BillAccountID:       r.BillAccountID,
		SubscriptionName:    r.SubscriptionName,
		ExcludedFromReports: r.ExcludedFromReports,
		HasNote:             r.HasNote,
		Note:                r.Note,
		HasSplits:           r.HasSplits,
		IsTransfer:          r.IsTransfer,
		TransferWith:        r.TransferWith,
		IsDebtPayment:       r.IsDebtPayment,
		SuspectTransfer:     r.SuspectTransfer,
		IsIncome:            r.IsIncome,
		IsRefund:            r.IsRefund,
		IsRefunded:          r.IsRefunded,
		IsBillMatched:       r.IsBillMatched,
		EventName:           r.EventName,
		ShowBalance:         r.ShowBalance,
		Balance:             r.Balance,
		BalTone:             r.BalTone,
		FollowUpOpen:        r.FollowUpOpen,
		FollowUpTotal:       r.FollowUpTotal,
		InfoText:            r.InfoText,
		FlashKey:            r.FlashKey,
		GroupSize:           r.GroupSize,
		GroupTotal:          r.GroupTotal,
	}
}

type txnTableBodyProps struct {
	App                                  *appstate.App
	Filter                               txnfilter.Criteria
	Columns                              uistate.TxnCols
	Page                                 txntable.Page
	Total                                int
	Visible                              []string
	ShowBalance, Register, SearchPending bool
	RenderRow                            func(int) ui.Node
	IDAt                                 func(int) any
	OnSort                               func(string)
	OnPage, OnPageSize                   func(int)
}

func txnTableColumns(props txnTableBodyProps) []uiw.Column {
	cols := []uiw.Column{
		{Head: ui.CreateElement(txnSelectAllHeader, txnSelectAllHeaderProps{Visible: props.Visible}), Class: "td-select"},
		{Label: uistate.T("transactions.colDate"), SortKey: txnTableSortKey(props.Register, "date"), Class: "td-date"},
	}
	cols = append(cols, uiw.Column{Label: uistate.T("transactions.colDescription"), SortKey: txnTableSortKey(props.Register, "payee"), Class: "row-desc-col txn-col-description"})
	if props.Columns.Category {
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colCategory"), SortKey: txnTableSortKey(props.Register, "category"), Class: "td-cat"})
	}
	if props.Columns.Account {
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colAccount"), SortKey: txnTableSortKey(props.Register, "account"), Class: "td-acct txn-col-account"})
	}
	members := props.App.Members()
	showUser := txnUserColumnVisible(props.Columns.User, props.Filter.Member, len(members))
	if showUser {
		cols = append(cols, uiw.Column{Head: txnWhoHeader(), Class: "td-user txn-col-who"})
	}
	if props.Columns.Amount {
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colAmount"), SortKey: txnTableSortKey(props.Register, "amount"), Class: "td-amount txn-col-amount"})
	}
	if props.ShowBalance {
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colBalance"), Class: "td-amount"})
	}
	if props.Columns.Source {
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colSource"), SortKey: txnTableSortKey(props.Register, "source"), Class: "td-source"})
	}
	cols = append(cols, uiw.Column{Head: Span(css.Class(tw.SrOnly), uistate.T("transactions.colActions")), Class: "td-actions txn-col-actions"})
	return cols
}

func txnTableBody(props txnTableBodyProps) ui.Node {
	if len(props.App.Transactions()) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("transactions.empty"), CTALabel: uistate.T("transactions.addFirst"), AddTarget: "transaction", Icon: icon.Transactions, ImportLink: true})
	}
	cols := txnTableColumns(props)
	dtp := uiw.DataTableProps{Class: "txn-table", StickyHead: true, Columns: cols, Sort: props.Filter.Sort, Dir: props.Filter.Dir, SortSpinner: true,
		Busy: props.SearchPending, BusyLabel: uistate.T("filters.searchPending"), Page: props.Page.Number, Total: props.Total,
		PageSize: props.Page.Size, PageSizes: txnfilter.PageSizes, OnPage: props.OnPage, OnPageSize: props.OnPageSize, TopPager: false, AnchorID: "txn-ledger-anchor"}
	if !props.Register {
		dtp.OnSort = props.OnSort
	}
	var body ui.Node
	switch {
	case props.Total == 0:
		body = txnTableEmptyBody(props, dtp)
	case props.Page.Virtual:
		dtp.Virtual = &uiw.VirtualSpec{Count: props.Total, RowHeight: txnRowHeight, ColSpan: len(cols), Scroller: "main.cf-scroll", RowAt: props.RenderRow, KeyAt: props.IDAt}
		body = uiw.DataTable(dtp)
	default:
		dtp.Body = MapKeyed(txntable.Indices(props.Page.Start, props.Page.End), props.IDAt, props.RenderRow)
		body = uiw.DataTable(dtp)
	}
	if props.Page.OnePage {
		return Div(css.Class("txn-onepage"), body)
	}
	return body
}

func txnTableEmptyBody(props txnTableBodyProps, dtp uiw.DataTableProps) ui.Node {
	future, _ := txnFutureScheduleCount(props.Filter, time.Now())
	if !future {
		return ui.CreateElement(txnNoMatch, txnNoMatchProps{Search: strings.TrimSpace(props.Filter.Text)})
	}
	items := txnFutureScheduledItems(props.Filter, time.Now())
	showUser := txnUserColumnVisible(props.Columns.User, props.Filter.Member, len(props.App.Members()))
	dtp.Body = Fragment(txnScheduledTableRows(items, func(date time.Time) string { return ledgerDate(date, props.Filter.From, props.Filter.To) }, props.Columns, showUser, props.ShowBalance)...)
	return Fragment(ui.CreateElement(txnNoMatch, txnNoMatchProps{Search: strings.TrimSpace(props.Filter.Text), FuturePeriod: true, InTable: true}), uiw.DataTable(dtp))
}

type txnTableCategoryTarget struct {
	ID       string
	Choose   func(string)
	Shortcut func(string)
	Close    func()
}

func useTxnTableCategoryTarget(app *appstate.App) txnTableCategoryTarget {
	target := ui.UseState("")
	opener := ui.UseRef(js.Null())
	choose := ui.UseCallback(func(id string) {
		opener.Set(js.Global().Get("document").Get("activeElement"))
		target.Set(id)
		scheduleTxnFocus(js.Global().Get("document"), "input[data-testid='catpick-search']")
	}, app)
	return txnTableCategoryTarget{ID: target.Get(), Choose: choose, Shortcut: func(id string) {
		opener.Set(js.Global().Get("document").Get("activeElement"))
		target.Set(id)
	}, Close: func() { target.Set(""); focusTxnOpener(opener.Get()) }}
}

type txnTableRegionProps struct {
	App      *appstate.App
	Body     ui.Node
	Filter   txnfilter.Criteria
	Shown    []domain.Transaction
	Base     string
	Category txnTableCategoryTarget
	Derived  *txntoolbar.Snapshot
	Lens     string
}

func txnTableRegion(props txnTableRegionProps) ui.Node {
	keyDown := ui.UseEvent(func(e ui.KeyboardEvent) {
		if handleTxnTableShortcut(e.GetKey(), e.JSValue().Get("target"), js.Global().Get("document"), props.Category.Shortcut, func() { SetTxnKeyHelpOpen(true) }) {
			e.PreventDefault()
			e.StopPropagation()
		}
	})
	return uiw.Widget(uiw.WidgetProps{ID: "txn-table", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(Attr("role", "region"), Attr("aria-label", uistate.T("txw1.tableRegion")), OnKeyDown(keyDown),
			Div(css.Class("txn-table-head"),
				H2(css.Class("card-title"), uistate.T("nav.transactions")),
				txnTableScopedCountFacts(props.App, props.Filter, props.Shown, props.Base, props.App.Settings().FXRates, txnTableFactsContext{Snapshot: props.Derived, Lens: props.Lens})),
			props.Body,
			ui.CreateElement(txnPageCategoryPicker, txnPageCategoryPickerProps{App: props.App, ID: props.Category.ID, Close: props.Category.Close})),
	})
}

type txnTableFactsContext struct {
	Snapshot *txntoolbar.Snapshot
	Lens     string
}

func txnTableScopedCountFacts(app *appstate.App, criteria txnfilter.Criteria, rows []domain.Transaction, base string, fxRates map[string]float64, contexts ...txnTableFactsContext) ui.Node {
	if app == nil {
		return Fragment()
	}
	var context txnTableFactsContext
	if len(contexts) > 0 {
		context = contexts[0]
	}
	snapshot := context.Snapshot
	if !snapshot.Matches(app.Rev(), txnDerivedScope(app, criteria)) {
		// Standalone previews must derive from the current complete ledger,
		// never from collapsed or previous-render display rows.
		index := txntable.BuildIndex(nil, app.Accounts(), app.Categories(), nil, nil, nil)
		value := deriveTxnScope(app, app.Transactions(), criteria, txnLabelsForApp(app, index.AccountNames, index.CategoryNames), currency.Rates{Base: base, Rates: fxRates}, base)
		snapshot = &txntoolbar.Snapshot{Revision: app.Rev(), Scope: txnDerivedScope(app, criteria), Value: value}
	}
	facts := txntable.CountFacts(rows, base, fxRates, []txnview.View{snapshot.Value.View}, txntable.Labels{T: uistate.T, Money: fmtMoney}, txnCountTransactionCount)
	extra := txntoolbar.Presentation{Text: uistate.T, Money: fmtMoney}.SummaryFacts(txntoolbar.Facts{Issues: snapshot.Value.Issues, Lens: context.Lens})
	for _, text := range extra[1:] {
		facts = append(facts, txntable.Fact{Class: "txn-count-fact", Text: text})
	}
	return txnTableFactsBand(facts)
}

func txnTableCountFacts(rows []domain.Transaction, base string, fxRates map[string]float64, cached ...txnview.View) ui.Node {
	facts := txntable.CountFacts(rows, base, fxRates, cached, txntable.Labels{T: uistate.T, Money: fmtMoney}, txnCountTransactionCount)
	return txnTableFactsBand(facts)
}

func txnTableFactsBand(facts []txntable.Fact) ui.Node {
	nodes := make([]ui.Node, 0, len(facts))
	for _, fact := range facts {
		args := []any{css.Class(fact.Class)}
		if fact.TestID != "" {
			args = append(args, Attr("data-testid", fact.TestID))
		}
		nodes = append(nodes, Span(append(args, fact.Text)...))
	}
	return Div(css.Class("txn-count-secondary"), Attr("data-testid", "txn-table-count-facts"), nodes)
}

// txnRowFlagsMemo is the synchronous compatibility cache. Like the sliced job,
// it reads the complete ledger, never Shown, and keys all detector settings and
// workspace identity. Search cannot narrow either flag index.
var txnRowFlagsMemoState struct {
	key   txnFlagsKey
	value txnRowFlags
	valid bool
}

func txnRowFlagsMemo(app *appstate.App, rev int, settings smart.Settings, weekStart time.Weekday) txnRowFlags {
	key := makeTxnFlagsKey(app, rev, settings, weekStart)
	m := &txnRowFlagsMemoState
	if m.valid && m.key == key {
		return m.value
	}
	value := buildTxnRowFlags(app, settings, weekStart)
	m.key, m.value, m.valid = key, value, true
	return value
}

// txnGroupMembersMemo resolves the order-group member transactions against
// the whole ledger (a group's total is a fact about the group, not about
// the filtered page), collecting only the ids some group actually names.
// Memoised per app and data revision; link writes bump the revision.
var txnGroupMembersMemoState struct {
	app      *appstate.App
	rev      int
	storeRev uint64
	value    map[string]domain.Transaction
	valid    bool
}

func txnGroupMembersMemo(app *appstate.App, rev int, groupByTxn map[string]domain.TxnLink) map[string]domain.Transaction {
	m := &txnGroupMembersMemoState
	storeRev := uint64(0)
	if app != nil {
		storeRev = app.Rev()
	}
	if m.valid && m.app == app && m.rev == rev && m.storeRev == storeRev {
		return m.value
	}
	var all []domain.Transaction
	if app != nil {
		all = app.Transactions()
	}
	value := collectTxnGroupMembers(groupByTxn, all)
	m.app, m.rev, m.value, m.valid = app, rev, value, true
	m.storeRev = storeRev
	return value
}

// collectTxnGroupMembers picks the named group members out of all in one
// pass. The wanted set is the union of every group's TxnIDs — a handful of
// ids, versus a whole-ledger index map.
func collectTxnGroupMembers(groupByTxn map[string]domain.TxnLink, all []domain.Transaction) map[string]domain.Transaction {
	return txntable.GroupMembers(groupByTxn, all)
}

// txnRowActionName gives a row action enough visible context to distinguish
// repeated merchants while navigating the ledger with a screen reader.
func txnRowActionName(action, description, date, amount string) string {
	return txntable.ActionName(action, description, date, amount, uistate.T)
}

// txnFrameRowProps configures one rendered row of the transactions table tile.
// Display strings are pre-formatted; the callbacks are plain funcs (not hooks) so
// they pass safely through MapKeyed. The row owns its own interaction hooks.
type txnFrameRowProps struct {
	ID               string
	OnChooseCategory func(string)
	// CategoryPickerTarget is normally empty (the overlay portals to body); a
	// component test may supply its fixture root as the portal target.
	CategoryPickerTarget ui.PortalTarget
	// RowIndex is this row's 1-based ARIA position in the whole match set,
	// counting the header as row 1 — so the first data row is 2. Zero means "do
	// not claim a position", which is correct for a fully rendered page where the
	// DOM order already is the answer. See C626 in txnFrameRow.
	RowIndex int
	Date     string
	Amount   string
	AmtTone  string // color token for the amount figure (e.g. "text-down")
	// AmountMoney is the raw amount (for the merchant-trend delta); TrendMerchant is the
	// resolved merchant name when this row's merchant has enough history to show the
	// spending-trend chip (TX6b), else "".
	AmountMoney       money.Money
	TrendMerchant     string
	ShowTrend         bool // defer chip mount until after the table settles (perf)
	ShowSuggestions   bool
	Desc              string
	Tags              []string         // appended after the description as small chips (capped, non-stretching)
	OnTagClick        func(tag string) // click a tag chip → filter the ledger to that tag
	Account           string
	Category          string
	CategoryTooltip   string
	InfoText          string
	FlashKey          string
	CategoryID        string
	SuggestedCategory string   // resolved inline suggestion, for the category cell's accessible name
	Source            string   // provenance label ("Manual"/"Imported"/…, "—" if unset)
	Member            string   // compatibility for row callers that supply one owner name
	MemberColor       string   // compatibility for legacy row tests/callers; library avatars choose their palette
	MemberNames       []string // assigned household member names in stable owner order
	MemberColors      []string
	OwnersResolved    bool            // true when owner ids were resolved by the ledger frame
	Vis               uistate.TxnCols // which optional columns to render (must match the header)
	Cleared           bool
	// Reconciled marks a cleared row dated at or before its account's
	// reconciled-through date (#63) — vouched for by a statement, shown with a
	// stronger chip than plain cleared.
	Reconciled bool
	Reviewed   bool
	// Queued is reviewqueue.Needs(t): whether the Review inbox actually holds a
	// card for this row. It is NOT the same question as Reviewed — a charge a rule
	// categorized is unreviewed but unqueued — and conflating them is what let the
	// ledger say "Needs review" about rows the inbox had never heard of (C618).
	Queued bool
	// AutoCategory marks a row whose category was filed by automation and which
	// nobody has confirmed (C579).
	AutoCategory bool
	// Duplicate / SpikeWhy are the SM-8 / SM-10 row flags, resolved ONCE for the
	// whole table (buildTxnRowFlags) rather than per row — either question costs a
	// full ledger scan to answer, and asking it per rendered row is what turns a
	// helpful flag into a scroll stutter.
	Duplicate bool
	SpikeWhy  string
	// Unfiled marks a charge with no category AND no split breakdown — the set the
	// SM-2 suggestion chip offers itself on. It is a precomputed flag rather than a
	// test on Category, which is the DISPLAY string and reads "Uncategorized"
	// (localized) precisely when the id is empty.
	Unfiled bool
	// OnConfirmCategory accepts the automatic category as the user's own decision,
	// so the correction path for "the machine is right" costs one click and does not
	// require opening the Rules page (C579).
	OnConfirmCategory func(txnID string)
	OnSuggestCategory func(txnID string)
	Selected          bool
	// Payment linkage (the ⋯ row menu): the current bill / subscription links (if any),
	// shown as a ✓ on the menu items. OnOpenLink opens the payment-link flip modal for
	// this transaction, pre-set to the chosen mode (uistate.TxnLinkMode*).
	BillAccountID    string
	SubscriptionName string
	OnOpenLink       func(txnID, mode string)
	// TXC-1 / TXC-2: excluded-from-reports state (drives the kebab toggle label + a
	// muted row affordance) and whether the row carries a memo (drives a note glyph).
	ExcludedFromReports bool
	HasNote             bool
	Note                string
	OnToggleExclude     func(txnID string)
	// Follow-up tasks linked to this transaction: a row chip shows "open/total", opens a
	// hover popover listing them, and links to the filtered To-do list. Total 0 = no chip.
	FollowUpOpen    int
	FollowUpTotal   int
	FollowUps       []followUpItem
	OnOpenFollowUps func()
	// Split-into-categories (the ⋯ row menu): HasSplits shows a ✓ when the
	// transaction already carries a category breakdown; OnOpenSplit opens the split
	// flip modal. IsTransfer hides the entry — a transfer leg has no category to
	// split (mirroring the classic view, which gates every category action on it).
	HasSplits   bool
	IsTransfer  bool
	OnOpenSplit func(txnID string)
	// C675: what KIND of movement this row is, and who the far side is, so the
	// ledger can say it rather than leave the reader to infer it from a colour.
	// A transfer's amount is signed account-relative — the same move is negative
	// on one leg and positive on the other — so income-green and spending-red are
	// actively misleading on these rows, and the badge is what replaces them.
	//
	// TransferWith is the counterparty's display name ("" when it no longer
	// resolves); IsDebtPayment is true when BillAccountID is set, which is a
	// different claim from "the counterparty is a debt" — see C677.
	TransferWith  string
	IsDebtPayment bool
	// C672: this row is NOT structurally a transfer but looks like one — filed
	// under a transfer category, or described as a move. It is counting as income
	// or spending right now. Flagged rather than hidden: the category alone is not
	// evidence enough to remove someone's money from their totals.
	SuspectTransfer bool
	// OnReceiptSplit (XC11) opens the "Split from receipt…" flow: pick a receipt
	// image, vision reads its line items, and a proposed breakdown pre-fills the
	// split editor for review. Gated like OnOpenSplit (hidden on transfer legs).
	OnReceiptSplit func(txnID string)
	Receipts       int                  // attachment count (drives the paperclip)
	Attachment     domain.AttachmentRef // first attachment, opened by the paperclip
	OnOpen         func(id string)
	OnToggleSelect func(id string, shift bool)
	OnViewReceipt  func(domain.AttachmentRef)
	// OnCreateRule (C363) prefills the Rules workbench add-form from this row's
	// merchant + category and navigates to /rules, so a rule can be authored from
	// a transaction in one click. Hidden on transfer legs (no category to file).
	OnCreateRule func(txnID string)
	// OnDelete removes this transaction (and its transfer pair, if any). The single delete is
	// undoable via the toast, matching the card view — so no modal confirm, just the ⋯-menu action.
	OnDelete func(txnID string)
	// Transaction links (XC1 order groups / XC2 refund pairs): badge data plus the
	// ⋯-menu actions. GroupTotal is pre-formatted; IsIncome gates the pair action
	// (only money-in can be a refund).
	GroupSize    int
	GroupTotal   string
	IsRefund     bool // this row is the refund side of a pair
	IsRefunded   bool // this row is the original purchase of a pair
	IsIncome     bool
	OnPairRefund func(txnID string)
	OnUnpair     func(txnID string)
	OnUngroup    func(txnID string)
	// IsBillMatched (TX9) is true when this row settles a recurring occurrence via a
	// durable bill-match link; OnUnlinkBill releases that link.
	IsBillMatched bool
	OnUnlinkBill  func(txnID string)
	// EventName (TX10) is the name of the event this transaction belongs to, shown as
	// a small chip beside the description. Empty = not mapped to any event.
	EventName string
	// Register mode (TX12): ShowBalance adds a running-balance cell after Amount
	// (only in register mode, when the ledger is scoped to one account). Balance is
	// the pre-formatted running figure after this row; BalTone colours a negative
	// running balance so a dip into the red reads at a glance.
	ShowBalance bool
	Balance     string
	BalTone     string
}

// txnFrameRowPropsCache retains only recent row props so the reconciler can bail
// out of unchanged keyed rows. Its bound keeps a long virtualized scroll from
// retaining every row the user has ever visited.
type txnFrameRowPropsCache struct {
	rows  map[string]*txnFrameRowProps
	order []string
}

const txnRowPropsCacheLimit = 256

func (c *txnFrameRowPropsCache) reuse(next txnFrameRowProps) *txnFrameRowProps {
	row, rows, order := txntable.RetainRow(c.rows, c.order, next.ID, next, txnRowPropsCacheLimit)
	c.rows, c.order = rows, order
	return row
}

// txnFrameRow renders one transaction row. It owns its edit/select/
// view hooks (per the GWC rule: On* handlers live inside a per-row component,
// never in a loop). The description and pencil open Edit; the leading
// checkbox toggles bulk selection (its cell stops click propagation so toggling
// does not also open the modal); the paperclip opens the first receipt.
// txnTagChipProps configure one clickable tag chip in the description column.
type txnTagChipProps struct {
	Tag         string
	Description string
	OnClick     func(tag string)
}

// txnTagChip is a single "#tag" chip that, on click, filters the ledger to that tag. Its
// own component so the click hook stays at a stable position (never registered inside the
// row's variable-length tag loop). StopPropagation keeps the click from also opening the
// row's edit modal.
func txnTagChip(props txnTagChipProps) ui.Node {
	onClick := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnClick != nil {
			props.OnClick(props.Tag)
		}
	})
	return Button(ClassStr("txn-desc-tag txn-desc-tag-btn"), Type("button"),
		Attr("data-testid", "txn-tag-"+props.Tag),
		Attr("title", uistate.T("transactions.tagFilterTitle", props.Tag)),
		Attr("aria-label", uistate.T("v5row.tag", props.Tag, props.Description)),
		OnClick(onClick), "#"+props.Tag)
}

func txnFrameRow(props *txnFrameRowProps) ui.Node {
	saveStatus := useTxnSaveStatus(props.ID)
	defer txnPerfStart("row")()
	pickerOpen := ui.UseState(false)
	useTxnRowFlash(props.ID, props.FlashKey)
	choose := func(ui.Event) {
		if props.OnChooseCategory != nil {
			props.OnChooseCategory(props.ID)
			return
		}
		pickerOpen.Set(true)
	}
	apply := func(id string) { pickerOpen.Set(false); applyTxnManualCategory(props.ID, id) }
	var picker ui.Node
	if pickerOpen.Get() {
		target := props.CategoryPickerTarget
		if target.Selector == "" && target.Node == nil {
			target = ui.PortalTarget{Selector: "body"}
		}
		picker = ui.Portal(ui.PortalProps{Target: target, Child: ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind: txnCategoryKind(props.AmountMoney), Selected: props.CategoryID, OnPick: apply, OnClose: func() { pickerOpen.Set(false) },
		})})
	}
	return Fragment(ui.CreateElement(txnFrameRowCells, txnFrameRowCellsProps{Row: props, SaveStatus: saveStatus, Choose: choose}), picker)
}

func useTxnRowFlash(id, key string) {
	last := ui.UseRef(key)
	ui.UseEffect(func() func() {
		if last.Get() == key {
			return nil
		}
		last.Set(key)
		doc := js.Global().Get("document")
		if doc.Truthy() {
			return flashTransactionRow(doc, id)
		}
		return nil
	}, key)
}

type txnFrameRowCellsProps struct {
	Row        *txnFrameRowProps
	SaveStatus string
	Choose     func(ui.Event)
}

func txnFrameRowCells(props txnFrameRowCellsProps) ui.Node {
	r := props.Row
	rowClass := txntable.RowClass(r.Selected, r.Cleared, r.ExcludedFromReports)
	if r.GroupSize > 1 {
		rowClass += " txn-row-grouped"
	}
	args := []any{ClassStr(rowClass), Attr("data-testid", "txn-row-"+r.ID), Attr("aria-busy", strconv.FormatBool(props.SaveStatus == "Saving..."))}
	if r.RowIndex > 0 {
		args = append(args, Attr("aria-rowindex", strconv.Itoa(r.RowIndex)))
	}
	args = append(args,
		ui.CreateElement(txnRowSelectionCell, r),
		Td(ClassStr("td-date"), r.Date),
		ui.CreateElement(txnRowDescriptionCell, props),
		If(r.Vis.Category, ui.CreateElement(txnRowCategoryCell, props)),
		If(r.Vis.Account, txnRowAccountCell(r.Account)),
		If(r.Vis.User, txnWhoCellForRow(r)),
		If(r.Vis.Amount, Td(ClassStr("td-amount txn-col-amount "+tw.ColorClass(r.AmtTone)), r.Amount)),
		If(r.ShowBalance, Td(ClassStr("td-amount "+tw.ColorClass(r.BalTone)), r.Balance)),
		If(r.Vis.Source, Td(ClassStr(txntable.SourceClass(r.Source)), r.Source)),
		ui.CreateElement(txnRowActionsCell, r),
	)
	return Tr(args...)
}

func txnRowSelectionCell(props *txnFrameRowProps) ui.Node {
	stop := ui.UseEvent(func(e ui.Event) { e.StopPropagation() })
	toggle := ui.UseEvent(func(e ui.Event) { props.OnToggleSelect(props.ID, e.JSValue().Get("shiftKey").Truthy()) })
	return Td(ClassStr("td-select"), OnClick(stop),
		Input(Type("checkbox"), Attr("aria-label", uistate.T("v5row.select", props.Desc)), CheckedIf(props.Selected), OnClick(toggle)))
}

func txnRowDescriptionCell(props txnFrameRowCellsProps) ui.Node {
	r := props.Row
	edit := ui.UseEvent(func(e ui.Event) { e.StopPropagation(); r.OnOpen(r.ID) })
	return Td(ClassStr("row-desc-cell txn-col-description"),
		Div(css.Class("row-desc-inner"),
			Div(css.Class("txn-desc-mainline"),
				Button(css.Class("row-desc-text txn-description-edit"), Type("button"), Attr("data-testid", "txn-description-edit"), Title(r.Desc),
					Attr("aria-label", txnRowActionName(uistate.T("transactions.rowEdit"), r.Desc, r.Date, r.Amount)), OnClick(edit), r.Desc),
				Span(ClassStr("txn-desc-account"), r.Account),
				ui.CreateElement(txnRowTags, r),
				// The status glyphs share one fixed column at the right of the cell, so the same
				// marks line up down the ledger whatever the length of the description.
				Span(css.Class("txn-desc-status"),
					txnRowDescriptionBadge(r),
					If(r.Cleared, txnRowClearedGlyph()),
					If(r.HasNote, ui.CreateElement(txnRowNote, txnRowNoteProps{Note: r.Note})),
					If(props.SaveStatus != "", Span(ClassStr("muted"), Attr("role", "status"), props.SaveStatus)),
					If(r.InfoText != "", ui.CreateElement(txnRowInfo, txnRowInfoProps{Text: r.InfoText, HasReceipt: r.Receipts > 0, HasFollowUps: r.FollowUpTotal > 0, FollowUps: r.FollowUps,
						OnReceipt: func() {
							if r.OnViewReceipt != nil {
								r.OnViewReceipt(r.Attachment)
							}
						}, OnFollowUps: r.OnOpenFollowUps}))),
			),
		),
	)
}

func txnRowDescriptionBadge(props *txnFrameRowProps) ui.Node {
	switch txntable.DescriptionBadge(props.Duplicate, props.Queued && !props.IsTransfer, props.SpikeWhy != "") {
	case txntable.BadgeDuplicate:
		return ui.CreateElement(txnFlagChip, txnFlagChipProps{TxnID: props.ID, Duplicate: true})
	case txntable.BadgeReview:
		return Span(css.Class("txn-review-pill"), Attr("data-testid", "txn-review-pill"), Attr("role", "img"), Attr("title", txntable.ReviewLabel(props.Queued, props.IsTransfer, uistate.T)), Attr("aria-label", txntable.ReviewLabel(props.Queued, props.IsTransfer, uistate.T)), uistate.T("r4c.review"))
	case txntable.BadgeSpike:
		return ui.CreateElement(txnFlagChip, txnFlagChipProps{TxnID: props.ID, SpikeWhy: props.SpikeWhy})
	}
	return nil
}

func txnRowTags(props *txnFrameRowProps) ui.Node {
	if len(props.Tags) == 0 {
		return Fragment()
	}
	tags, extra := txntable.TagWindow(props.Tags)
	kids := []any{ClassStr("txn-desc-tags"), Attr("data-testid", "txn-row-tags")}
	for _, tag := range tags {
		kids = append(kids, ui.CreateElement(txnTagChip, txnTagChipProps{Tag: tag, Description: props.Desc, OnClick: props.OnTagClick}))
	}
	if extra > 0 {
		kids = append(kids, Span(css.Class("txn-desc-tag txn-desc-tag-more"), Attr("title", uistate.T("transactions.tagsMoreTitle", extra)), "+"+strconv.Itoa(extra)))
	}
	return Span(kids...)
}

func txnRowAccountCell(account string) ui.Node {
	args := []any{ClassStr("td-acct txn-col-account")}
	if txnAccountNeedsTooltip(account) {
		args = append(args, Attr("title", account))
	}
	return Td(append(args, account)...)
}

func txnRowCategoryCell(props txnFrameRowCellsProps) ui.Node {
	r := props.Row
	name, title := txntable.CategoryDisplay(r.Category, r.SuggestedCategory, r.CategoryTooltip, r.Unfiled, r.IsTransfer, uistate.T)
	args := []any{ClassStr("td-cat"), Attr("aria-label", name)}
	if title != "" {
		args = append(args, Attr("title", title))
	}
	args = append(args, Span(css.Class("td-cat-inner"),
		If(r.IsTransfer, Span(css.Class("txn-transfer-category"), Attr("data-testid", "txn-transfer-category"),
			Attr("title", txntable.TransferTooltip(r.CategoryTooltip, r.TransferWith, uistate.T)),
			txntable.TransferDisplay(r.Category, r.TransferWith, r.AmountMoney.Amount < 0, uistate.T))),
		If(!r.IsTransfer && !(r.Unfiled && r.SuggestedCategory != ""), ui.CreateElement(txnCategoryChoose, txnCategoryChooseProps{Label: r.Category, Description: r.Desc, OnChoose: props.Choose})),
		If(r.Unfiled && r.SuggestedCategory != "", Span(css.Class("td-cat-name td-cat-uncategorized"), r.Category)),
		If(r.ShowSuggestions && r.Unfiled && !r.IsTransfer, ui.CreateElement(txnCatSuggestChip, txnCatSuggestChipProps{TxnID: r.ID, OnChoose: props.Choose})),
	))
	return Td(args...)
}

func txnRowActionsCell(props *txnFrameRowProps) ui.Node {
	stop := ui.UseEvent(func(e ui.Event) { e.StopPropagation() })
	return Td(ClassStr("td-actions txn-col-actions"), OnClick(stop),
		Div(css.Class("txn-row-actions"), txnRowMenu(*props)))
}

func flashTransactionRow(doc js.Value, txnID string) func() {
	rows := doc.Call("querySelectorAll", `[data-testid^="txn-row-"]`)
	var target js.Value
	for i := 0; i < rows.Length(); i++ {
		row := rows.Index(i)
		if row.Call("getAttribute", "data-testid").String() == "txn-row-"+txnID {
			target = row
			break
		}
	}
	if !target.Truthy() {
		return nil
	}
	target.Get("classList").Call("add", "txn-row-flash")
	timer := time.AfterFunc(150*time.Millisecond, func() {
		if target.Truthy() {
			target.Get("classList").Call("remove", "txn-row-flash")
		}
	})
	return func() { timer.Stop(); target.Get("classList").Call("remove", "txn-row-flash") }
}

type txnCategoryChooseProps struct {
	Label       string
	Description string
	OnChoose    func(ui.Event)
}

func txnCategoryChoose(props txnCategoryChooseProps) ui.Node {
	choose := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnChoose != nil {
			props.OnChoose(e)
		}
	})
	return Button(ClassStr("txn-cat-choose"), Type("button"), Attr("aria-label", txnV5RowControlName(uistate.T("v5row.category"), props.Label, props.Description)),
		Attr("data-testid", "txn-category-choose"), OnClick(choose), props.Label)
}

func txnCategoryKind(amount money.Money) domain.CategoryKind {
	return txntable.CategoryKind(amount)
}

func applyTxnManualCategory(txnID, categoryID string) {
	app := appstate.Default
	if app == nil || txnID == "" || categoryID == "" {
		return
	}
	txn, ok := txnByID(app, txnID)
	if !ok || txn.IsTransfer() {
		return
	}
	confirmed := txnreview.Confirm(txn, categoryID)
	_ = commitUndoable(uistate.T("r10w.categoryFiled", catname.LabelByID(app.Categories(), categoryID)), []string{txnID}, func() error { return app.PutTransactionsAtomic([]domain.Transaction{confirmed}) })
}

func txnDeletePrompt(app *appstate.App, txn domain.Transaction) string {
	return txntable.DeletePrompt(app.ExpandTransferPairs([]string{txn.ID}), app.Transactions(), app.Accounts(), 0,
		txntable.Labels{T: uistate.T, Money: fmtMoney})
}

func ledgerDate(date time.Time, from, to string) string {
	return txntable.LedgerDate(date, from, to, time.Now())
}

func transferCategoryKey(props txnFrameRowProps) string {
	return txntable.TransferCategoryKey(props.AmountMoney)
}

func txnCategoryNeedsTooltip(category string) bool {
	return txntable.NeedsTooltip(category, 23)
}

func txnAccountNeedsTooltip(account string) bool {
	return txntable.NeedsTooltip(account, 20)
}

func txnWhoHeader() ui.Node {
	return Span(uistate.T("r4c.who"))
}

func txnWhoCell(names []string) ui.Node {
	return txnWhoCellWithColors(names, nil)
}

func txnWhoCellWithColors(names []string, colors []string) ui.Node {
	label := txntable.OwnerLabel(names, uistate.T)
	var avatar ui.Node = uiw.MemberUnassigned(label, "txn-who-unassigned")
	if len(names) == 1 {
		avatar = uiw.MemberAvatar(uiw.MemberAvatarProps{Name: label, Title: label, Color: ownerColor(colors, 0), TestID: "txn-who-avatar"})
	} else if len(names) > 1 {
		members := make([]uiw.MemberAvatarProps, 0, len(names))
		for i, name := range names {
			members = append(members, uiw.MemberAvatarProps{Name: name, Color: ownerColor(colors, i)})
		}
		avatar = uiw.AvatarStack(uiw.AvatarStackProps{Members: members, Label: label, Title: label, TestID: "txn-who-stack"})
	}
	return Td(ClassStr("td-user txn-col-who"), Attr("aria-label", label), Title(label), avatar)
}

func ownerColor(colors []string, index int) string {
	return txntable.OwnerColor(colors, index)
}

func txnWhoCellForRow(props *txnFrameRowProps) ui.Node {
	if props.OwnersResolved {
		return txnWhoCellWithColors(props.MemberNames, props.MemberColors)
	}
	// Keep direct legacy row callers working while the ledger frame supplies the
	// complete resolved owner list on every production row.
	label, assigned := txntable.LegacyOwnerLabel(props.Member, uistate.T)
	var avatar ui.Node = Fragment()
	if assigned {
		avatar = Span(css.Class("txn-member-avatar"), Attr("style", "--txn-member-color: "+props.MemberColor), txnMemberInitials(props.Member))
	}
	return Td(ClassStr("td-user txn-col-who"), Attr("aria-label", label), avatar)
}

// txnRowMenu is the row's ⋯ kebab: its Link to… submenu goes straight to the
// payment-link editor, and the split action opens the split editor. Built with
// OverflowMenu (loop-safe: it owns each item's click hook).
//
// The actions stay in the order of their workflow; Delete remains visually separated
// at the bottom and keeps its destructive styling. Section labels stay on each item
// for its accessible name, while the row menu hides the redundant visible headings.
const (
	txnMenuOrganize = "txnmenu.sectionOrganize"
	txnMenuLinks    = "txnmenu.sectionLinks"
	txnMenuReports  = "txnmenu.sectionReports"
	txnMenuRemove   = "txnmenu.sectionRemove"
)

func txnRowMenu(props txnFrameRowProps) ui.Node {
	items := txnRowMenuItems(props)
	if len(items) == 0 {
		return Fragment()
	}
	menu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		Items:         items,
		TriggerLabel:  txnRowActionName(uistate.T("transactions.rowActions"), props.Desc, props.Date, props.Amount),
		TriggerTestID: "txn-kebab-" + props.ID,
	})
	return Div(ClassStr("txn-row-menu"), menu)
}

func txnRowMenuItems(props txnFrameRowProps) []uiw.OverflowMenuItem {
	var items []uiw.OverflowMenuItem
	// Edit is the menu's first entry: the row's primary action, opened from its ⋯ trigger.
	items = append(items, uiw.OverflowMenuItem{
		Label: uistate.T("transactions.rowEdit"), Icon: icon.Pencil, TestID: "txn-rowedit",
		OnSelect: func() { props.OnOpen(props.ID) },
	})
	menu := txnMenuDecisions(props)
	organize := uistate.T(txnMenuOrganize)
	links := uistate.T(txnMenuLinks)
	reports := uistate.T(txnMenuReports)
	remove := uistate.T(txnMenuRemove)

	// --- Organize: reversible bookkeeping on this one charge --------------------
	// C579: when the category is automation's guess, the cheapest correct action is
	// to say "yes, that's right" — and it was the one action the page did not offer.
	// Accepting it here is one click and needs no trip to the Rules page; disagreeing
	// is the Edit button the row already has. Offered only on rows that actually read
	// as automatic, so it never appears as a no-op.
	if menu.Confirm {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("transactions.confirmCategory"), Icon: icon.Check, Section: organize,
			TestID:   "txn-confirm-category",
			OnSelect: func() { props.OnConfirmCategory(props.ID) },
		})
	}
	if menu.Suggest {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("r5w.suggestCategorySmart"), Icon: icon.Tag, Section: organize,
			TestID:   "txn-categorize-open",
			OnSelect: func() { props.OnSuggestCategory(props.ID) },
		})
	}
	// Split-into-categories: not offered on transfer legs (no category to split).
	if menu.Split {
		splitLabel := uistate.T("r4k.split")
		if menu.SplitChecked {
			splitLabel = "✓ " + splitLabel
		}
		items = append(items, uiw.OverflowMenuItem{
			Label: splitLabel, Icon: icon.Split, Section: organize,
			TestID:   "txn-split-open",
			OnSelect: func() { props.OnOpenSplit(props.ID) },
		})
	}
	// --- Links: what this charge is FOR, which other surfaces read -------------
	if linkItems := txnLinkMenuItems(props); len(linkItems) > 0 {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("r3k.linkTo"), Icon: icon.Bills, Section: links,
			TestID: "txn-link-open", Submenu: linkItems,
		})
	}

	// --- Reporting: changes what every budget and report counts ----------------
	// TXC-1: exclude / include this transaction in budgets & reports (still counts
	// toward account balances either way). The label states the action to perform.
	// Excluding wears the danger tier: it leaves this row looking untouched while
	// rewriting figures on four other surfaces (C562 makes it confirm as well).
	if menu.Exclude {
		excLabel := uistate.T("transactions.kebabExclude")
		if menu.Include {
			excLabel = uistate.T("transactions.kebabInclude")
		}
		excIcon := icon.Ban
		if menu.Include {
			excIcon = icon.Check
		}
		items = append(items, uiw.OverflowMenuItem{
			Label: excLabel, Icon: excIcon, Section: reports,
			Danger:   false,
			TestID:   "txn-toggle-exclude",
			OnSelect: func() { props.OnToggleExclude(props.ID) },
		})
	}

	// --- Remove: the charge stops existing -------------------------------------
	// Destructive action last: delete this transaction (and its transfer pair). Undoable via the
	// toast, so no modal — consistent with the card view's single delete.
	if menu.Delete {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("action.delete"), Icon: icon.Trash, Section: remove, Danger: true,
			TestID:   "txn-delete",
			OnSelect: func() { props.OnDelete(props.ID) },
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func txnLinkMenuItems(props txnFrameRowProps) []uiw.OverflowMenuItem {
	var items []uiw.OverflowMenuItem
	menu := txnMenuDecisions(props)
	if menu.Link {
		items = append(items,
			uiw.OverflowMenuItem{Label: uistate.T("transactions.markBill"), TestID: "txn-markbill-open", OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeBill) }},
			uiw.OverflowMenuItem{Label: uistate.T("transactions.markSub"), TestID: "txn-marksub-open", OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeSub) }},
		)
	}
	if menu.Pair {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.pairAction"), TestID: "txn-pair-refund", OnSelect: func() { props.OnPairRefund(props.ID) }})
	}
	if menu.Unpair {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.unpairAction"), TestID: "txn-unpair", OnSelect: func() { props.OnUnpair(props.ID) }})
	}
	if menu.UnlinkBill {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("billmatch.unlink"), TestID: "txn-unlink-bill", OnSelect: func() { props.OnUnlinkBill(props.ID) }})
	}
	if menu.Ungroup {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.ungroupAction"), TestID: "txn-ungroup", OnSelect: func() { props.OnUngroup(props.ID) }})
	}
	return items
}

func txnMenuDecisions(props txnFrameRowProps) txntable.Menu {
	return txntable.DecideMenu(txntable.MenuState{Automatic: props.AutoCategory, Transfer: props.IsTransfer, Split: props.HasSplits,
		Excluded: props.ExcludedFromReports, Income: props.IsIncome, Refund: props.IsRefund, Refunded: props.IsRefunded, BillMatched: props.IsBillMatched, GroupSize: props.GroupSize,
		Confirm: props.OnConfirmCategory != nil, Suggest: props.OnSuggestCategory != nil, OpenSplit: props.OnOpenSplit != nil,
		ToggleExclude: props.OnToggleExclude != nil, Delete: props.OnDelete != nil, OpenLink: props.OnOpenLink != nil,
		PairRefund: props.OnPairRefund != nil, Unpair: props.OnUnpair != nil, Ungroup: props.OnUngroup != nil, UnlinkBill: props.OnUnlinkBill != nil})
}

func txnTableFollowUpItems(items []txntable.FollowUpItem) []followUpItem {
	result := make([]followUpItem, 0, len(items))
	for _, item := range items {
		result = append(result, followUpItem{ID: item.ID, Title: item.Title, Done: item.Done, Due: item.Due, dueT: item.DueDate})
	}
	return result
}
