// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

const (
	// txnVirtualizeThreshold is the row count above which the "All" view switches to a
	// windowed (virtualized) body — below it, rendering every row is cheap enough.
	txnVirtualizeThreshold = 100
	// txnRowHeight is the fixed ledger row height in px the virtual window measures
	// against (the table rows are uniform: nowrap + table-layout:fixed).
	txnRowHeight = 44
)

// txnTableSortKey hides sortable affordances when register mode fixes the ledger
// to chronological order. An empty key makes DataTable render a plain header.
func txnTableSortKey(registerActive bool, key string) string {
	if registerActive {
		return ""
	}
	return key
}

// Transactions is the widgetized global ledger. Per the "everything on the page is
// a widget" rule, the page is a thin SURFACE HOST: it builds one engine RenderCtx
// over the filtered ledger and renders a fixed set of widget specs through the same
// spec/render pipeline the dashboard uses (safeRenderSpec). Every visible block is
// its own engine widget tile —
//
//   - txn-toolbar   (Native): search, filters, chips, add/export, import & duplicates (both flip modals)
//   - txn-bulkbar   (Native): bulk recategorize / clear / export / delete (when a selection exists)
//   - txn-undobar   (Native): undo the last bulk op (when one is pending)
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
	defer txnPerfStart("page")()
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Re-render the surface on any data mutation or shared-state change: a bulk op,
	// a row edit/delete, a selection toggle, a sub-view switch, a pending undo, or a
	// receipt preview opening all flow through these atoms.
	dataRevision := uistate.UseDataRevision().Get()
	selAtom := uistate.UseTxnSelection()
	selAnchorAtom := uistate.UseTxnSelAnchor()
	undoAtom := uistate.UseTxnUndo()
	previewAtom := uistate.UseTxnPreview()
	filterAtom := uistate.UseTxFilter()

	accounts := app.Accounts()
	categories := app.Categories()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	f := filterAtom.Get()
	// Honour the top-bar member perspective (L21): scope to the lensed member(s)
	// only when the per-screen filter has no member of its own. The persisted filter
	// is never mutated. Read through UseMemberLens, not the retired UseActiveMember
	// atom — nothing has written that atom since the app moved to ActiveScope, so
	// this layering was silently a no-op and the switcher did not scope the ledger
	// at all (C574).
	f, _ = f.WithOwnerLens(uistate.UseMemberLens())

	// C13: a bulk selection must not outlive the filter/search it was made under. A
	// quick-filter (or search, saved view, or member switch) that hides the selected
	// rows would otherwise leave the "N selected" bulk bar — Delete included — acting
	// on rows the user can no longer see. So drop the selection whenever the effective
	// result-set scope changes (its filter fields + search text), while leaving sort
	// and pagination alone: those reorder/paginate the SAME set without hiding rows.
	selScope := f
	selScope.Page, selScope.PageSize = 0, 0
	selScope.Sort, selScope.Dir = "", ""
	lastSelScope := ui.UseState(selScope)
	ui.UseEffect(func() func() {
		if lastSelScope.Get() != selScope {
			lastSelScope.Set(selScope)
			if len(selAtom.Get()) > 0 {
				selAtom.Set(map[string]bool{})
				selAnchorAtom.Set("")
			}
		}
		return nil
	}, selScope)

	// Register mode (TX12): when the filter scopes to exactly one account, the
	// running-balance column is available; while it's ON the ledger is forced into
	// chronological order (date ascending) so each row's running figure reads down
	// the column. The override lives here so the frame the table hydrates is already
	// in register order; the table restores the user's sort on exit (register off).
	_, singleAcct := f.SingleAccount()
	registerActive := singleAcct && uistate.UseTxnRegisterMode().Get()
	if registerActive {
		f.Sort, f.Dir = "date", txnfilter.Asc
	}
	// A hidden column cannot order the table. Four of the six sortable columns can
	// be switched off in Columns, and hiding the one you had sorted by used to leave
	// its order in force with NO header showing a sort indicator — the rows were
	// arranged by something that was not on screen, and it survived a reload. The
	// STORED preference is untouched, so putting the column back restores the sort
	// rather than discarding it the moment the user freed up some width.
	f.Sort, f.Dir = txnfilter.EffectiveSort(f, hiddenSortKeys(uistate.UseTxnCols().Get()))

	// The filtered + sorted set drives both the engine frame (the table) and the
	// duplicate/selection affordances in the toolbar/bulk tiles. RichTransactions
	// preserves this order, so the filter's sort flows straight through the frame.
	accName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		accName[a.ID] = a.Name
	}
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	// Pass the payee-alias resolver so text search matches the CLEANED merchant name
	// (TX1/SM-1), not just the raw payee/desc — otherwise a renamed merchant's charges
	// wouldn't surface when you search the clean name you now see in the ledger.
	transactions := app.Transactions()
	resolver := app.PayeeResolver()
	derivedFilter := f
	derivedFilter.Page, derivedFilter.PageSize = 0, 0
	shown := ui.UseMemo(func() []domain.Transaction {
		return deriveTxnRows(transactions, derivedFilter, txnfilter.Labels{
			Account: accName, Category: catName, Payee: resolver.Resolve,
			BaseCurrency: base, ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, base) },
		}, resolver.HasLearned, resolver.Resolve)
	}, app, app.Rev(), dataRevision, derivedFilter)

	// The engine render context: the live data every tile body reads from (§6). The
	// SCOPED slices are the filtered ledger, so the table frame and the toolbar/bulk
	// tiles all operate on the same set. Each tile is a registered renderer dispatched
	// by spec below — there are no surface-local closures.
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: transactions,
		ScopedAccounts: accounts, ScopedTxns: shown,
		Rates: rates, Base: base,
		Start: time.Time{}, End: time.Now(),
	}

	// The fixed placement set for the transactions surface. The toolbar is always
	// present; the bulk and undo tiles appear with selection / a pending undo; the main
	// slot is always the ledger table. Import and duplicates review both open as shell-
	// root flip modals over the ledger now, so there are no in-place sub-views left.
	specs := []domain.WidgetSpec{txnNativeSpec("txn-toolbar")}
	if len(selAtom.Get()) > 0 {
		specs = append(specs, txnNativeSpec("txn-bulkbar"))
	}
	if len(undoAtom.Get().Prior) > 0 {
		specs = append(specs, txnNativeSpec("txn-undobar"))
	}
	// The main slot is the ledger table, or the month calendar (TX8) when the view
	// mode is set to it. The calendar is a Native tile projecting the same filtered
	// set (ScopedTxns) — so active filter chips scope it exactly like the table.
	if uistate.UseTxnViewMode().Get() == uistate.TxnViewCalendar {
		specs = append(specs, txnNativeSpec("txn-calendar"))
	} else {
		specs = append(specs, txnTableSpec())
	}

	// Render each spec through the engine's per-widget error boundary. Keyed on the
	// spec id so inserting the bulk/undo tiles never shifts the table's identity (its
	// hooks stay aligned across renders).
	bentoClass := "bento bento-ledger"
	if len(selAtom.Get()) > 0 {
		bentoClass += " txn-has-selection"
	}
	bento := Div(css.Class(bentoClass),
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
	)

	return Div(txnReceiptPreviewOverlay(app, previewAtom),
		A(css.Class("txn-skip-link"), Attr("href", "#txn-ledger-anchor"), uistate.T("txw1.skipToTransactions")),
		// Pending-vs-posted: the month's still-unposted scheduled charges as
		// ghost rows above the ledger (parity scan; billmatch-suppressed).
		bento)
}

// init registers the transactions-surface widget bodies with the engine render
// registry, keyed by NativeID (Native tiles) and id (the data-driven table). The
// surface host dispatches each placement through this registry — bodies read the
// shared atoms + the RenderCtx, never surface locals, which is what makes them
// engine widgets rather than embedded nodes.
func init() {
	R := widgetrender.Register

	R("txn-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: c.App, Base: c.Base, Rates: c.Rates, Shown: c.ScopedTxns})
	})
	R("txn-bulkbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnBulkBarWidget, txnBulkBarProps{App: c.App})
	})
	R("txn-undobar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnUndoBarWidget, txnUndoBarProps{App: c.App})
	})
	R("txn-calendar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnCalendarWidget, txnCalendarProps{App: c.App, Base: c.Base, Shown: c.ScopedTxns})
	})

	widgetrender.RegisterFrame("txn-table", func(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(txnTableWidget, txnTableProps{Frame: fr, Shown: c.ScopedTxns, App: c.App, Base: c.Base})
	})
}

// txnNativeSpec builds the seed spec for a Native transactions tile. The surface is
// fixed (not user-reconfigurable or persisted), so the spec is constructed inline
// rather than catalogued in widgetregistry (which would surface these tiles in the
// dashboard's add-widget picker).
func txnNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}

// txnTableSpec builds the data-driven table spec: a Kind==Table widget whose
// Pipeline sources the rich all-transactions frame, hydrated by the engine over the
// host's filtered ledger (RenderCtx.ScopedTxns) and painted by the txn-table
// FrameRenderer.
func txnTableSpec() domain.WidgetSpec {
	return domain.WidgetSpec{
		SchemaVersion: domain.WidgetSpecVersion, ID: "txn-table", Kind: domain.KindTable,
		Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions-full"}},
	}
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

// txnReceiptPreviewOverlay renders the receipt preview modal (L29) when the shared
// preview atom holds an attachment, and an empty fragment otherwise. The table's row
// paperclip opens it; the close button clears the atom. It is a dialog overlay, not
// a bento tile, mirroring how the edit form is a modal host rather than a widget.
func txnReceiptPreviewOverlay(app *appstate.App, previewAtom state.Atom[domain.AttachmentRef]) ui.Node {
	closePreview := ui.UseEvent(Prevent(func() { previewAtom.Set(domain.AttachmentRef{}) }))
	ref := previewAtom.Get()
	if ref.ArtifactID == "" {
		return Fragment()
	}
	var art *domain.Artifact
	for i := range app.Artifacts() {
		if app.Artifacts()[i].ID == ref.ArtifactID {
			a := app.Artifacts()[i]
			art = &a
			break
		}
	}
	var body ui.Node
	if art != nil && len(art.Bytes) > 0 {
		body = Img(Attr("src", artifacts.DataURL(art.MIME, art.Bytes)), Attr("alt", uistate.T("transactions.previewAlt", ref.Name)), css.Class(tw.MaxWFull))
	} else {
		body = P(css.Class("empty"), uistate.T("transactions.previewMissing"))
	}
	return Div(css.Class("receipt-preview-overlay"), Attr("role", "dialog"), Attr("aria-label", uistate.T("transactions.previewReceipt")),
		uiw.Card(uiw.CardProps{
			Header: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
				H2(css.Class("card-title"), uistate.T("transactions.previewReceipt")),
				Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.previewClose")), Attr("data-testid", "receipt-preview-close"), OnClick(closePreview), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
			),
			Body: body,
		}),
	)
}

// txnTableProps carries the engine-hydrated ledger frame and the data the table tile
// needs to present it: the filtered transactions (for attachments + empty states),
// the app, and the base currency.
type txnTableProps struct {
	Frame domain.Frame
	Shown []domain.Transaction
	App   *appstate.App
	Base  string
}

// txnTableWidget is the txn-table FrameRenderer body: the engine-hydrated ledger
// frame, paginated and painted as a sortable, selectable DataTable inside a widget
// tile. It owns its sort/pagination/selection hooks (the surface host passes only
// data), reading and writing the shared filter + selection atoms so the toolbar and
// bulk tiles stay in step. Rows drill into the edit modal; the leading checkbox
// toggles bulk selection; the paperclip opens a receipt.
func txnTableWidget(props txnTableProps) ui.Node {
	defer txnPerfStart("table")()
	categoryTarget := ui.UseState("")
	chooseCategory := ui.UseCallback(func(id string) { categoryTarget.Set(id) }, props.App)
	// Subscribe to the data revision: link changes (XC1/XC2 group/pair badges)
	// mutate no row content, so without this the memoized table body renders
	// stale badges after an ungroup/unpair.
	txnDataRev := uistate.UseDataRevision().Get()
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	memberLens := uistate.UseMemberLens()
	f, _ = f.WithOwnerLens(memberLens)
	// C628: true while a typed search is still inside its debounce window, i.e.
	// while the rows this table is about to render answer the previous query.
	searchPending := uistate.UseTxnSearchPending().Get()
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	previewAtom := uistate.UseTxnPreview()
	colVis := uistate.UseTxnCols().Get() // which optional columns are shown
	// Register mode (TX12) forces the rows into chronological order so each running
	// balance reads down the column. The HOST applies that force when it builds the
	// frame; this tile did not, so the headers went on advertising the user's stored
	// sort — caret, aria-sort and all — over rows that were in a different order
	// entirely. Whatever orders the rows has to order the header too, so the same
	// force is applied here, before the effective-sort fallback below.
	regID, singleAcct := f.SingleAccount()
	registerActive := singleAcct && uistate.UseTxnRegisterMode().Get()
	if registerActive {
		f.Sort, f.Dir = "date", txnfilter.Asc
	}
	// The headers must claim the order the ROWS are actually in. The surface host
	// applies the same fallback when it builds the frame, so reading it here too is
	// what stops a hidden column's sort from being announced by a header that is no
	// longer rendered — or, worse, by no header at all while the rows stay ordered.
	f.Sort, f.Dir = txnfilter.EffectiveSort(f, hiddenSortKeys(colVis))

	// A row's "N follow-ups" chip links to the To-do list pre-filtered to transaction-
	// linked tasks (the closest the shared link filter gets to "this charge's tasks").
	nav := router.UseNavigate()
	// Plain closure (the row component owns the click hook — never register On* here in
	// the row loop). Filters the To-do list to transaction-linked tasks, then navigates.
	openFollowUps := func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkTransaction)
		txnLeaveFor("/todo") // C581: leave a named way back to this working set
		nav.Navigate(uistate.RoutePath("/todo"))
	}
	// Clicking a tag chip on a row narrows the ledger to that single tag (replacing any
	// multi-tag selection). Plain closure — the tag chip component owns its click hook.
	//
	// C651: the action was reported as a no-op from a row the search had already
	// narrowed to one result — and on that view it genuinely cannot move the count,
	// because the one matching row is the one carrying the tag. What it can do, and
	// did not, is SAY it happened. The criteria change is now announced as well as
	// charted: a filter that changes no visible row and posts no word is
	// indistinguishable from a dead button.
	onTagFilter := func(tag string) {
		prev := filterAtom.Get()
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { *x = x.DrillToTag(tag) })
		next := filterAtom.Get()
		// Speak ONLY when the user would otherwise see nothing. A toast on every
		// drill would be noise for the ordinary case — where a chip appears and the
		// row count moves, which is how every other filter change in the app
		// communicates — to fix the case where neither does. So the test is not
		// "did the criteria change" but "did anything VISIBLE change", which is the
		// reported situation exactly: a search already narrowed to the one row that
		// carries the tag, so the chip is the only difference and the count is
		// identical. Reuse the current lensed result count and derive only the next
		// scope, using the same alias and currency context as the displayed rows.
		if txnfilter.ScopeChanged(prev, next) {
			effective, _ := next.WithOwnerLens(memberLens)
			settings := props.App.Settings()
			rates := currency.Rates{Base: props.Base, Rates: settings.FXRates}
			rows, _ := txnfilter.ApplyWithLabelsChecked(props.App.Transactions(), effective, txnfilter.Labels{
				Payee: props.App.PayeeResolver().Resolve, BaseCurrency: props.Base,
				ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, props.Base) },
			})
			if len(rows) != len(props.Shown) {
				return
			}
		}
		uistate.PostNotice(uistate.T("transactions.tagFilterApplied", tag), false)
	}

	// Register mode (TX12): when the ledger is scoped to exactly one account and the
	// toggle is on, compute each transaction's running balance from the account's FULL
	// chronological history (ledger.RegisterBalances), so a paginated/filtered slice
	// still shows the TRUE figure. A multi-currency account (whose fold errors) leaves
	// runBal nil, and the column is simply not shown. (regID/registerActive are
	// resolved above, where they also force the header's sort to match the rows.)
	runBal := ui.UseMemo(func() map[string]money.Money {
		if registerActive {
			for _, a := range props.App.Accounts() {
				if a.ID == regID {
					if m, err := ledger.RegisterBalances(a, props.App.Transactions()); err == nil {
						return m
					}
					break
				}
			}
		}
		return nil
	}, props.App, props.App.Rev(), regID, registerActive)
	showBalance := runBal != nil

	setPage := func(p int) { setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.Page = p }) }
	setPageSize := func(s int) { setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.PageSize, x.Page = s, 1 }) }
	// A plain sort handler — the spinner-while-sorting behaviour is the DataTable's
	// standard SortSpinner config (it defers this call so the spinner paints first).
	sortBy := func(key string) {
		if registerActive {
			return
		}
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) {
			if x.Sort == key {
				if x.Dir == txnfilter.Asc {
					x.Dir = txnfilter.Desc
				} else {
					x.Dir = txnfilter.Asc
				}
			} else {
				x.Sort, x.Dir = key, txnfilter.DefaultDir(key)
			}
		})
	}

	// visibleOrder is filled below (after pagination) and captured by toggleSelect so
	// the header can represent the current scope. Shift-range selection builds its
	// own ordered ids lazily, only when a shift-click actually asks for them.
	var visibleOrder []string
	var rangeOrder func() []string
	toggleSelect := func(txnID string, shift bool) {
		m := selAtom.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if shift && anchorAtom.Get() != "" && anchorAtom.Get() != txnID {
			if rangeOrder != nil {
				visibleOrder = rangeOrder()
			}
			ai, bi := -1, -1
			for i, id := range visibleOrder {
				if id == anchorAtom.Get() {
					ai = i
				}
				if id == txnID {
					bi = i
				}
			}
			if ai >= 0 && bi >= 0 {
				if ai > bi {
					ai, bi = bi, ai
				}
				for _, id := range visibleOrder[ai : bi+1] {
					nm[id] = true
				}
				selAtom.Set(nm)
				anchorAtom.Set(txnID)
				return
			}
		}
		if nm[txnID] {
			delete(nm, txnID)
		} else {
			nm[txnID] = true
		}
		selAtom.Set(nm)
		anchorAtom.Set(txnID)
	}
	openEdit := func(id string) { uistate.SetTxnEdit(id) }
	openSplit := func(id string) { uistate.SetTxnSplit(id) }
	// C363: author a rule from a row — prefill the Rules workbench add-form with
	// this transaction's cleaned merchant (falling back to its description) and
	// current category, then jump to /rules so it can be confirmed in one click.
	createRuleFromTxn := func(id string) {
		t, ok := props.App.TransactionByID(id)
		if !ok {
			return
		}
		phrase := strings.TrimSpace(firstNonEmpty(t.Payee, t.Desc))
		uistate.SetRuleDraft(phrase, t.CategoryID)
		// C581: writing a rule from a row is the deepest side trip the ledger
		// offers — a different page, a form to fill, and the filtered view you
		// were working through left behind. Leave a named way back — to THIS
		// row (C605), since that is the charge the trip was about.
		txnLeaveForRow("/rules", id)
		nav.Navigate(uistate.RoutePath("/rules"))
	}
	// C579: accept an automatic category as the user's own decision. Reviewed is
	// exactly that fact — the flag the review queue and the provenance mark both
	// read — so confirming here is one write and the "auto" mark disappears from the
	// row. It is restorative rather than destructive (nothing about the money
	// changes, only who owns the classification), so it applies immediately, but it
	// still captures an undo point and says what it did.
	confirmCategory := func(id string) {
		t, ok := props.App.TransactionByID(id)
		if !ok || t.Reviewed {
			return // missing or already the user's decision
		}
		t.Reviewed = true
		if err := props.App.PutTransaction(t); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		postUndoStory(uistate.T("transactions.confirmedCategory"))
	}
	// TXC-1 / C562: flip a transaction's exclude-from-reports flag from the row kebab.
	//
	// Excluding is a reporting change with no visible trace on the row's own figures —
	// balances are untouched — so an accidental click used to silently move money out
	// of every budget, spending total and report with nothing but a relabelled menu
	// entry to show for it. It now states that boundary and asks first. Re-INCLUDING
	// is restorative, so it applies straight away; both directions capture an undo
	// point and post an undoable toast, so either is reversible from Ctrl+Z/Activity.
	applyExclude := func(t domain.Transaction, exclude bool) {
		t.ExcludeFromReports = exclude
		if err := props.App.PutTransaction(t); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		key := "transactions.includedOne"
		if exclude {
			key = "transactions.excludedOne"
		}
		postUndoStory(uistate.T(key, txnShortLabel(t)))
	}
	toggleExclude := func(id string) {
		for _, t := range props.App.Transactions() {
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
	// deleteRow removes the transaction (and its transfer pair).
	//
	// C620: this used to commit on the single ⋯-menu click, on the theory that the
	// undoable toast was enough soft-safety. It was not, for two reasons. The toast
	// only renders an Undo button when the undo stack is non-empty, and this
	// handler never called auditview.CaptureNow() — the very function whose doc
	// says delete handlers must call it "right before showing an Undo toast so the
	// undo stack is ready when the user clicks Undo". So the one affordance the
	// no-confirm decision rested on was the one that wasn't wired. Meanwhile BULK
	// delete has had a confirm, a checkpoint and a snapshot all along, which left
	// the riskier per-row path as the unguarded one.
	//
	// C621: deleting also strands any follow-up task pointing at this transaction —
	// tasklink.EntityName returns ok=false for a missing id, so the task simply
	// stops showing its link. The count goes in the confirmation, so the effect on
	// other work is stated BEFORE the click rather than discovered later.
	deleteRow := func(id string) {
		var txn domain.Transaction
		found := false
		for _, t := range props.App.Transactions() {
			if t.ID == id {
				txn, found = t, true
				break
			}
		}
		if !found {
			return
		}
		linked := 0
		for _, tk := range props.App.Tasks() {
			if tk.RelatedType == domain.RelatedTransaction && tk.RelatedID == id && tk.Status != domain.StatusDone {
				linked++
			}
		}
		acctName := ""
		if a, ok := domain.AccountByID(props.App.Accounts(), txn.AccountID); ok {
			acctName = a.Name
		}
		// Name the row the way the user is looking at it: who, when, how much, and
		// out of which account. "Delete this transaction?" is not enough to catch a
		// misclick on the wrong row.
		what := uistate.T("transactions.deleteConfirmDetail",
			firstNonEmpty(txn.Payee, txn.Desc),
			dateutil.FormatDate(txn.Date),
			fmtMoney(txn.Amount),
			acctName)
		if linked > 0 {
			what += " " + uistate.T("transactions.deleteConfirmTasks", linked)
		}
		uistate.ConfirmModal(what, true, func(ok bool) {
			if !ok {
				return
			}
			// Capture BEFORE the write, so Undo has a point to return to.
			auditview.CaptureNow()
			if err := props.App.DeleteTransactionWithTransferPair(id); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostUndoable(uistate.T("toast.txnDeleted"))
		})
	}
	viewReceipt := func(ref domain.AttachmentRef) { previewAtom.Set(ref) }

	frame := props.Frame
	total := frame.Rows
	pageSize := f.PageSize
	if pageSize == 0 {
		pageSize = txnfilter.DefaultPageSize
	}
	curPage := 1
	start, end := 0, total
	if pageSize > 0 {
		curPage = pagination.Clamp(f.Page, total, pageSize)
		start = (curPage - 1) * pageSize
		end = start + pageSize
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
	}

	idCol, _ := frame.Column("id")
	dateCol, _ := frame.Column("date")
	amtCol, _ := frame.Column("amount")
	curCol, _ := frame.Column("currency")
	payeeCol, _ := frame.Column("payee")
	descFull, _ := frame.Column("desc")
	accCol, _ := frame.Column("account")
	catCol, _ := frame.Column("category")
	clearedCol, _ := frame.Column("cleared")
	srcCol, _ := frame.Column("source")

	// Resolve a transaction's attachments by id so the row paperclip can open a
	// preview (the frame doesn't carry attachment refs).
	txByID := ui.UseMemo(func() map[string]domain.Transaction {
		byID := make(map[string]domain.Transaction, len(props.Shown))
		for _, t := range props.Shown {
			byID[t.ID] = t
		}
		return byID
	}, props.App, props.App.Rev(), txnDataRev, f)
	// Payee-alias resolver (TX1): raw payee → clean display name (learned alias →
	// rule pack → raw). Applied at DISPLAY only; the raw payee stays on the txn.
	payeeResolver := props.App.PayeeResolver()
	// #63: per-account reconciled-through dates — a cleared row dated at or
	// before its account's newest reconciliation wears the stronger ✓✓ chip.
	reconThrough := make(map[string]time.Time)
	for _, ac := range props.App.Accounts() {
		if th, ok := reconcile.Through(ac.Reconciliations); ok {
			reconThrough[ac.ID] = th
		}
	}
	// Account id → name, so a transfer row can name its counterparty rather than
	// just claim to be a transfer (C675).
	accName := make(map[string]string)
	for _, a := range props.App.Accounts() {
		accName[a.ID] = a.Name
	}
	// Member id → name, for the optional "User" column (the frame carries no member).
	memberNames := make(map[string]string)
	memberColors := make(map[string]string)
	for _, m := range props.App.Members() {
		memberNames[m.ID] = m.Name
		memberColors[m.ID] = m.Color
	}
	// Category id → name, so a split row can list its per-line categories.
	catName := make(map[string]string)
	for _, c := range props.App.Categories() {
		catName[c.ID] = c.Name
	}
	// C672: rows that look like transfers but were never linked. They are counting
	// as income or spending right now, so the ledger marks them instead of leaving
	// the reader to notice that "Transfer to Savings" is sitting in their spending.
	// The mark never removes the row from a total; only a person can do that.
	//
	// R3P: the check runs per RENDERED row inside rowPropsAt, not as a scan over
	// every shown row on every render — decorating a 25-row page must not cost
	// a 3,292-row classification pass on each sort, keystroke or selection.
	// openLink opens the payment-link flip modal (shell-root host) for a transaction,
	// pre-set to Bill or Subscription mode. The modal owns the actual write, so the row
	// ⋯ menu just sets the shared target atom.
	linkAtom := uistate.UseTxnLinkTarget()
	openLink := func(txnID, linkMode string) {
		linkAtom.Set(uistate.TxnLinkTarget{TxnID: txnID, Mode: linkMode})
	}

	// XC1/XC2 transaction links: per-row badge data + the ⋯-menu actions. The atom
	// is captured during render (never inside a callback); handlers mutate through
	// appstate and bump the revision so every consumer re-reads.
	refundAtom := uistate.UseRefundPairTarget()
	links := props.App.TxnLinks()
	groupByTxn := txnlinks.GroupsByTxn(links)
	// C-fix: an order group's TOTAL has to be measured against the group, not
	// against whatever the current filter happens to show. txByID above is built
	// from props.Shown (the filtered, searched, lensed page), and
	// txnlinks.GroupMembers silently drops any id missing from the map it is
	// given — so filtering to a category that matched only some members left the
	// badge reading "3 items · $47.00" when the three items really totalled
	// $65.00: a count and a money figure printed side by side, measured against
	// different populations. The group's membership is filter-independent, so its
	// sum is resolved against the whole ledger.
	// R3P: resolve ONLY the group members (a handful of ids), memoised per app
	// and data revision. This used to allocate a Shown-sized map on every
	// render and, when any group existed, a second whole-ledger map — which
	// renderRow then never read, so the badge total was still measured
	// against the filtered page the comment above says it must not use.
	groupTxByID := txnGroupMembersMemo(props.App, txnDataRev, groupByTxn)
	refundSide, refundedSide := map[string]bool{}, map[string]bool{}
	billMatched := map[string]bool{} // TX9: rows that settle a recurring occurrence
	// TX10: map each transaction to the name of the event it belongs to, so the row
	// shows a small event chip. A transaction maps to at most one link per event; the
	// first event found wins the chip.
	eventName := map[string]string{}
	eventNameByID := map[string]string{}
	for _, e := range props.App.Events() {
		eventNameByID[e.ID] = e.Name
	}
	for _, l := range links {
		if l.Kind == domain.TxnLinkRefundPair && len(l.TxnIDs) == 2 {
			refundedSide[l.TxnIDs[0]] = true // the original purchase
			refundSide[l.TxnIDs[1]] = true   // the refund
		}
		if l.Kind == domain.TxnLinkBillMatch && len(l.TxnIDs) == 1 {
			billMatched[l.TxnIDs[0]] = true
		}
		if l.Kind == domain.TxnLinkEventTxn && len(l.TxnIDs) == 1 && l.EventID != "" {
			if _, ok := eventName[l.TxnIDs[0]]; !ok {
				eventName[l.TxnIDs[0]] = eventNameByID[l.EventID]
			}
		}
	}
	pairRefundRow := func(id string) { refundAtom.Set(id) }
	ungroupRow := func(id string) {
		if l, ok := txnlinks.GroupOf(id, props.App.TxnLinks()); ok {
			if err := props.App.DeleteTxnLink(l.ID); err != nil {
				uistate.PostNotice(uistate.T("txnlinks.groupErr", err.Error()), true)
				return
			}
			uistate.PostNotice(uistate.T("txnlinks.ungrouped"), false)
			uistate.BumpDataRevision()
		}
	}
	unpairRow := func(id string) {
		if l, ok := txnlinks.PairOf(id, props.App.TxnLinks()); ok {
			if err := props.App.DeleteTxnLink(l.ID); err != nil {
				uistate.PostNotice(uistate.T("txnlinks.pairErr", err.Error()), true)
				return
			}
			uistate.PostNotice(uistate.T("txnlinks.unpaired"), false)
			uistate.BumpDataRevision()
		}
	}
	// TX9: release a bill-match link so the occurrence reads unpaid again.
	unlinkBillRow := func(id string) {
		if err := props.App.UnlinkBill(id); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.PostNotice(uistate.T("billmatch.unlinkLogged"), false)
		uistate.BumpDataRevision()
	}

	// SM-8 / SM-10: resolve the duplicate and spike flags ONCE for the whole table.
	// Both answers cost a full ledger scan, so asking them per rendered row is how a
	// helpful flag becomes a scroll stutter. Each half self-gates on its feature, so
	// this is free for anyone who has neither turned on.
	//
	// R3P: "once for the whole table" still meant once per table RENDER — every
	// sort, selection toggle and page change re-ran duplicate detection and the
	// smart engine over the full ledger, though neither answer can change
	// without a data write. Memoised per app + data revision + the two
	// features' on/off state; the settings value is loaded once and shared
	// with the suggestion gate below (it was loaded twice per render).
	smartSettings := uistate.LoadSmartSettings()
	finishFlags := txnPerfStart("flags")
	smartFlags := useTxnRowFlags(props.App, txnDataRev, smartSettings, uistate.LoadPrefs().WeekStartWeekday())
	finishFlags()

	sel := selAtom.Get()

	// TX6b: per-merchant charge counts (memoized on the data revision), so each row can
	// decide in O(1) whether to show the spending-trend chip without rescanning the
	// ledger on every sort / select / pagination re-render (the full stats still compute
	// lazily only when a chip is opened).
	finishMerchants := txnPerfStart("merchants")
	merchantCounts := merchantChargeCountsMemo(props.App, int(props.App.Rev()))
	finishMerchants()
	// C579: which rows carry a category no person has confirmed, and which rule (if
	// any) accounts for it. Classified once for the whole view — rows re-render on
	// selection, sort and pagination, and none of those change the answer.
	provenanceRules := props.App.Rules()
	// The trend chips are a secondary affordance, so mount them just AFTER the table has
	// painted (useAfterSettle) — this keeps the interactive-row cost off the initial
	// route-settle so the ledger paints as fast as before, then the chips fade in.
	trendReady := useAfterSettle("txn-trend")
	// Category suggestions run rule matching and merchant history lookup. Let the
	// ledger's first paint settle before mounting those row components.
	suggestionsReady := useAfterSettle("txn-suggestions")
	suggestionsEnabled := smartSettings.IsEnabled(catSuggestFreeCode)
	// C605: if the user came back here from a side trip that started on a row, land
	// on that row. Unconditional hook, at a stable position.
	// Virtualize the heavy "All" view (only the rows near the viewport are rendered);
	// the bounded pages (25/50/100) are already small, so render the slice directly.
	// Computed here rather than beside the table body because the focus helper below
	// needs it too, and two copies of the rule would eventually disagree.
	virtualize := pageSize <= 0 && total > txnVirtualizeThreshold
	// C605: if the user came back here from a side trip that started on a row, land
	// on that row. Unconditional hook, at a stable position.
	//
	// The hint is what makes it work on the virtualized view: there, the row may not
	// be in the DOM at all, so the helper has to move the scroller to where the row
	// WILL render before it can look for it. IndexOf is the frame's own ordering, so
	// the position is the one the user would have scrolled to.
	useTxnFocusRow(txnFocusRowHint{
		Virtual:   virtualize,
		RowHeight: txnRowHeight,
		Scroller:  "main.cf-scroll",
		IndexOf: func(id string) (int, bool) {
			for i := 0; i < total; i++ {
				if idCol.Str(i) == id {
					return i, true
				}
			}
			return 0, false
		},
	})

	// Follow-up tasks linked to each transaction (open/total + the items behind them), so
	// a row can surface a chip + hover popover. Built once from the task list, read O(1)
	// per row.
	followUps := followUpInfoByTxn(props.App.Tasks(), uistate.UsePrefs().Get().FormatDate)
	rowPropsCache := ui.UseRef(&txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)})

	// Keep callback identities stable across selection/preview renders. These hooks
	// live at the table component's fixed level (never inside the variable row loop).
	openFollowUps = ui.UseCallback(openFollowUps, nav)
	onTagFilter = ui.UseCallback(onTagFilter, props.App, props.App.Rev(), txnDataRev, f, memberLens)
	toggleSelect = ui.UseCallback(toggleSelect, f, txnDataRev)
	openEdit = ui.UseCallback(openEdit, props.App)
	openSplit = ui.UseCallback(openSplit, props.App)
	createRuleFromTxn = ui.UseCallback(createRuleFromTxn, props.App, nav)
	confirmCategory = ui.UseCallback(confirmCategory, props.App)
	toggleExclude = ui.UseCallback(toggleExclude, props.App)
	deleteRow = ui.UseCallback(deleteRow, props.App)
	viewReceipt = ui.UseCallback(viewReceipt, previewAtom)
	openLink = ui.UseCallback(openLink, linkAtom)
	pairRefundRow = ui.UseCallback(pairRefundRow, props.App)
	unpairRow = ui.UseCallback(unpairRow, props.App)
	ungroupRow = ui.UseCallback(ungroupRow, props.App)
	unlinkBillRow = ui.UseCallback(unlinkBillRow, props.App)

	// rowPropsAt builds one row's display props from the frame on demand. Factored out
	// so the paginated body and the virtualized window build rows identically — and so
	// the window only materializes the slice it actually shows.
	rowPropsAt := func(i int) txnFrameRowProps {
		rid := idCol.Str(i)
		amt := money.New(amtCol.Int64(i), curCol.Str(i))
		desc := txnDisplayDescription(descFull.Str(i), payeeCol.Str(i), payeeResolver.HasLearned, payeeResolver.Resolve)
		cat := catCol.Str(i)
		if strings.TrimSpace(cat) == "" {
			cat = uistate.T("transactions.uncategorized")
		}
		// A split transaction lists its per-line categories (deduped, in split order)
		// so the breakdown reads at a glance without opening the editor.
		if t, ok := txByID[rid]; ok && t.HasSplits() {
			seen := make(map[string]bool, len(t.Splits))
			names := make([]string, 0, len(t.Splits))
			for _, s := range t.Splits {
				n := catName[s.CategoryID]
				if n == "" {
					n = uistate.T("transactions.uncategorized")
				}
				if !seen[n] {
					seen[n] = true
					names = append(names, n)
				}
			}
			if len(names) > 0 {
				cat = strings.Join(names, ", ")
			}
		}
		cleared := false
		if b, ok := clearedCol.Values[i].(bool); ok {
			cleared = b
		}
		var firstAtt domain.AttachmentRef
		nAtt := 0
		if t, ok := txByID[rid]; ok {
			nAtt = len(t.Attachments)
			if nAtt > 0 {
				firstAtt = t.Attachments[0]
			}
		}
		trendMerchant := ""
		if t, ok := txByID[rid]; ok && !t.IsTransfer() && t.Amount.IsNegative() {
			if m := strings.TrimSpace(payeeResolver.Resolve(firstNonEmpty(t.Payee, t.Desc))); m != "" &&
				merchantCounts[strings.ToLower(m)] >= minTrendChipCharges {
				trendMerchant = m
			}
		}
		ledgerAmount, ledgerTone := fmtLedgerAmount(amt, txByID[rid].IsTransfer())
		suggestedCategory := ""
		rowTxn := txByID[rid]
		if suggestionsReady && suggestionsEnabled && rowTxn.CategoryID == "" && !rowTxn.HasSplits() && !rowTxn.IsTransfer() {
			inputs := txnCatSuggestInputsFor(props.App, txnDataRev)
			if suggestion, ok := inputs.suggestion(props.App, rowTxn); ok &&
				suggestion.Confidence >= catsuggest.ConfMedium &&
				categorySuggestionMatchesDirection(rowTxn, suggestion.CategoryID, inputs.categories) {
				suggestedCategory = catName[suggestion.CategoryID]
			}
		}
		// C672 suspect-transfer mark, answered for this rendered row only (R3P).
		_, suspectTransfer := txnclassify.Suspected(rowTxn, catName[rowTxn.CategoryID])
		autoProv := txnAutoByID(props.App, []domain.Transaction{rowTxn}, provenanceRules, int(props.App.Rev()))
		autoWhy := autoMarkWhy(autoProv[rid], srcCol.Str(i))
		if match := autoProv[rid].RuleMatch; match != nil {
			if phrase := rulePhrase(*match); phrase != "" {
				autoWhy = uistate.T("r1b.autoFiledByRule", phrase)
			}
		}
		return txnFrameRowProps{
			ID:              rid,
			AmountMoney:     amt,
			TrendMerchant:   trendMerchant,
			ShowTrend:       trendReady,
			ShowSuggestions: suggestionsReady,
			// .UTC() is load-bearing: txn dates are UTC-midnight calendar dates
			// (dateutil), and time.Unix reconstructs in the LOCAL zone — west of
			// UTC that rendered every ledger date a day early (Jul 1 → "Jun 30")
			// while /reports showed Jul 1 for the same transaction (C339).
			Date:              ledgerDate(time.Unix(int64(dateCol.Num(i)), 0).UTC(), f.From, f.To),
			Amount:            ledgerAmount,
			AmtTone:           ledgerTone,
			Desc:              desc,
			Tags:              txByID[rid].Tags,
			Account:           accCol.Str(i),
			Category:          cat,
			CategoryID:        txByID[rid].CategoryID,
			SuggestedCategory: suggestedCategory,
			Source:            srcCol.Str(i),
			MemberNames:       txnOwnerNames(txByID[rid].Owners(), memberNames),
			MemberColors:      txnOwnerColors(txByID[rid].Owners(), memberColors),
			OwnersResolved:    true,
			Cleared:           cleared,
			Reconciled: cleared && !reconThrough[txByID[rid].AccountID].IsZero() &&
				!txByID[rid].Date.After(reconThrough[txByID[rid].AccountID]),
			Reviewed:            txByID[rid].Reviewed,
			Queued:              reviewqueue.Needs(txByID[rid]),
			Duplicate:           smartFlags.Duplicate[rid],
			SpikeWhy:            smartFlags.Spike[rid],
			Unfiled:             txByID[rid].CategoryID == "" && !txByID[rid].HasSplits(),
			AutoCategory:        autoProv[rid].IsAutomatic(),
			AutoCategoryWhy:     autoWhy,
			Selected:            sel[rid],
			Receipts:            nAtt,
			Attachment:          firstAtt,
			Vis:                 txnColsForScope(colVis, f.Member, len(memberNames)),
			BillAccountID:       txByID[rid].BillAccountID,
			SubscriptionName:    txByID[rid].SubscriptionName,
			ExcludedFromReports: txByID[rid].ExcludeFromReports,
			HasNote:             txByID[rid].Note != "",
			HasSplits:           txByID[rid].HasSplits(),
			IsTransfer:          txByID[rid].IsTransfer(),
			TransferWith:        accName[txByID[rid].TransferAccountID],
			IsDebtPayment:       txByID[rid].BillAccountID != "",
			SuspectTransfer:     suspectTransfer,
			IsIncome:            txByID[rid].IsIncome(),
			IsRefund:            refundSide[rid],
			IsRefunded:          refundedSide[rid],
			IsBillMatched:       billMatched[rid],
			EventName:           eventName[rid],
			ShowBalance:         showBalance,
			Balance:             balanceStr(runBal, rid),
			BalTone:             balanceTone(runBal, rid),
			FollowUpOpen:        followUps[rid].Open,
			FollowUpTotal:       followUps[rid].Total,
			FollowUps:           followUps[rid].Items,
		}
	}
	renderRow := func(i int) ui.Node {
		r := rowPropsAt(i)
		// C626: only the windowed body needs to state an absolute position — a
		// rendered page's DOM order already is one. +2 because ARIA counts the
		// header as row 1.
		if virtualize {
			r.RowIndex = i + 2
		}
		if g, ok := groupByTxn[r.ID]; ok {
			r.GroupSize = len(g.TxnIDs)
			r.GroupTotal = fmtMoney(txnlinks.GroupSum(txnlinks.GroupMembers(g, groupTxByID)))
		}
		r.OnOpen = openEdit
		r.OnTagClick = onTagFilter
		r.OnToggleSelect = toggleSelect
		r.OnViewReceipt = viewReceipt
		r.OnOpenLink = openLink
		r.OnOpenSplit = openSplit
		r.OnToggleExclude = toggleExclude
		r.OnConfirmCategory = confirmCategory
		r.OnSuggestCategory = func(id string) { uistate.SetCatSuggest(id) }
		// C564: resolve against the LIVE ledger, not the render-time page map. txByID
		// is built from the shown slice, so a row whose data moved underneath the
		// render (a rule applied, a sync landed) yielded a zero Transaction that the
		// flow then tried to split. The flow rejects an unresolved id out loud.
		r.OnReceiptSplit = func(id string) {
			for _, t := range props.App.Transactions() {
				if t.ID == id {
					startReceiptSplitFlow(props.App, t)
					return
				}
			}
			uistate.PostNotice(uistate.T("receiptsplit.noTxn"), true)
		}
		r.OnPairRefund = pairRefundRow
		r.OnUnpair = unpairRow
		r.OnUngroup = ungroupRow
		r.OnUnlinkBill = unlinkBillRow
		r.OnOpenFollowUps = openFollowUps
		r.OnCreateRule = createRuleFromTxn
		r.OnDelete = deleteRow
		r.OnChooseCategory = chooseCategory
		return ui.CreateElement(txnFrameRow, rowPropsCache.Get().reuse(r))
	}

	// visibleOrder (for the select-all header) spans the rows in the current view:
	// the whole list when virtualized ("All"), otherwise the current page. Range
	// selection has a separate lazy order builder so ordinary renders do no full-ledger
	// ID walk just to preserve a future shift-click.
	vStart, vEnd := start, end
	if virtualize {
		vStart, vEnd = 0, total
	}
	rangeOrder = func() []string {
		ids := make([]string, 0, vEnd-vStart)
		for i := vStart; i < vEnd; i++ {
			ids = append(ids, idCol.Str(i))
		}
		return ids
	}
	visibleOrder = ui.UseMemo(func() []string {
		ids := make([]string, 0, vEnd-vStart)
		for i := vStart; i < vEnd; i++ {
			ids = append(ids, idCol.Str(i))
		}
		return ids
	}, props.App, props.App.Rev(), txnDataRev, f, virtualize, vStart, vEnd)

	var tableBody ui.Node
	switch {
	case len(props.App.Transactions()) == 0:
		tableBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("transactions.empty"), CTALabel: uistate.T("transactions.addFirst"), AddTarget: "transaction", Icon: icon.Transactions, ImportLink: true})
	default:
		// Header columns, built to match the row cells' conditional set exactly (same
		// order): Select + Date + Description are always shown; the rest follow the
		// user's column-visibility choice.
		// The select column's header is a real control, not a screen-reader-only
		// caption: one checkbox that selects or clears every row in view, sitting
		// directly above the per-row boxes it commands. Its own component, so its
		// hooks stay isolated from this variable-length column list.
		cols := []uiw.Column{
			{Head: ui.CreateElement(txnSelectAllHeader, txnSelectAllHeaderProps{Visible: visibleOrder}), Class: "td-select"},
			{Label: uistate.T("transactions.colDate"), SortKey: txnTableSortKey(registerActive, "date"), Class: "td-date"},
		}
		// row-desc-col: the fixed-layout ledger sizes columns from the header row, so
		// the width-priority rule (2026-07-17 audit — description reads first) must
		// live on the th, not the td.
		cols = append(cols, uiw.Column{Label: uistate.T("transactions.colDescription"), SortKey: txnTableSortKey(registerActive, "payee"), Class: "row-desc-col"})
		// The secondary columns carry their td-* class on the header too, so the
		// fixed-layout ledger sizes each column by class (not by fragile positional
		// nth-child that misaligns when a column's visibility toggles).
		if colVis.Category {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colCategory"), SortKey: txnTableSortKey(registerActive, "category"), Class: "td-cat"})
		}
		if colVis.Account {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colAccount"), SortKey: txnTableSortKey(registerActive, "account"), Class: "td-acct"})
		}
		// "Who" is useful only for an Everyone view in a real household.
		members := props.App.Members()
		showUser := txnUserColumnVisible(colVis.User, f.Member, len(members))
		if showUser {
			cols = append(cols, uiw.Column{Head: txnWhoHeader(), Class: "td-user"})
		}
		if colVis.Amount {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colAmount"), SortKey: txnTableSortKey(registerActive, "amount"), Class: "td-amount"})
		}
		// Register mode (TX12): a running-balance column follows Amount. No SortKey.
		if showBalance {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colBalance"), Class: "td-amount"})
		}
		if colVis.Source {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colSource"), SortKey: txnTableSortKey(registerActive, "source"), Class: "td-source"})
		}
		cols = append(cols, uiw.Column{Head: Span(css.Class(tw.SrOnly), uistate.T("transactions.colActions")), Class: "td-actions"})
		dtp := uiw.DataTableProps{
			Class:       "txn-table",
			StickyHead:  true,
			Columns:     cols,
			Sort:        f.Sort,
			Dir:         f.Dir,
			OnSort:      nil,
			SortSpinner: true,
			// C628: while a typed search is still inside its debounce the rows below
			// answer the PREVIOUS query. The toolbar already said so in words; the
			// table now dims them and drops their pointer events for the same window,
			// so the stale rows cannot be acted on — which is what "explicitly marked
			// as pending" has to mean for a row whose Edit button still worked.
			Busy:       searchPending,
			BusyLabel:  uistate.T("filters.searchPending"),
			Page:       curPage,
			Total:      total,
			PageSize:   pageSize,
			PageSizes:  txnfilter.PageSizes,
			OnPage:     setPage,
			OnPageSize: setPageSize,
			// Keep the interactive pager below the table to reduce tab stops before rows.
			TopPager: false,
			// Paging from the bottom pager scrolls the list back to the top, so the user
			// sees the new page's first rows instead of being stranded at the bottom.
			AnchorID: "txn-ledger-anchor",
		}
		if !registerActive {
			dtp.OnSort = sortBy
		}
		if total == 0 {
			futurePeriod, _ := txnFutureScheduleCount(f, time.Now())
			if futurePeriod {
				items := txnFutureScheduledItems(f, time.Now())
				dtp.Body = Fragment(txnScheduledTableRows(items,
					func(date time.Time) string { return ledgerDate(date, f.From, f.To) },
					colVis, showUser, showBalance)...)
				tableBody = Fragment(ui.CreateElement(txnNoMatch, txnNoMatchProps{
					Search: strings.TrimSpace(f.Text), FuturePeriod: true, InTable: true,
				}), uiw.DataTable(dtp))
			} else {
				tableBody = ui.CreateElement(txnNoMatch, txnNoMatchProps{Search: strings.TrimSpace(f.Text)})
			}
		} else if virtualize {
			dtp.Virtual = &uiw.VirtualSpec{
				Count:     total,
				RowHeight: txnRowHeight,
				ColSpan:   len(cols),
				Scroller:  "main.cf-scroll",
				RowAt:     renderRow,
				KeyAt:     func(i int) any { return idCol.Str(i) },
			}
			tableBody = uiw.DataTable(dtp)
		} else {
			idxs := make([]int, 0, end-start)
			for i := start; i < end; i++ {
				idxs = append(idxs, i)
			}
			dtp.Body = MapKeyed(idxs, func(i int) any { return idCol.Str(i) }, renderRow)
			tableBody = uiw.DataTable(dtp)
		}
	}

	// When the whole filtered set fits within the smallest page size, no rows-per-page
	// choice could ever create a second page — so the pager (range + size buttons +
	// disabled prev/next + "Page 1 of 1") is pure noise. Wrap the table in a marker
	// class that hides it (July 2026 review: hide unnecessary pagination). Above that
	// threshold the pager stays, so a user on "All" can always page back down.
	body := tableBody
	if total > 0 && total <= txnfilter.PageSizes[0] {
		body = Div(css.Class("txn-onepage"), tableBody)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-table", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(Attr("role", "region"), Attr("aria-label", uistate.T("txw1.tableRegion")), body,
			ui.CreateElement(txnPageCategoryPicker, txnPageCategoryPickerProps{App: props.App, ID: categoryTarget.Get(), Close: func() { categoryTarget.Set("") }})),
	})
}

// txnRowFlagsMemo caches buildTxnRowFlags per app and data revision. Both
// halves (duplicate scan, smart-engine page run) read only ledger data and
// the two features' enabled/muted state plus the week start, so those are
// the key; any transaction write bumps the revision and rebuilds. Toggling a
// smart feature happens on the Settings surface, and returning here remounts
// the table — but the toggles are in the key anyway, so even a same-mount
// change is honoured.
var txnRowFlagsMemoState struct {
	app       *appstate.App
	rev       int
	storeRev  uint64
	dupeOn    bool
	spikeOn   bool
	weekStart time.Weekday
	value     txnRowFlags
	valid     bool
}

func txnRowFlagsMemo(app *appstate.App, rev int, settings smart.Settings, weekStart time.Weekday) txnRowFlags {
	dupeOn := settings.IsEnabled(dupeFlagCode) && !settings.IsMuted(dupeFlagCode)
	spikeOn := settings.IsEnabled(spikeFlagCode) && !settings.IsMuted(spikeFlagCode)
	m := &txnRowFlagsMemoState
	storeRev := uint64(0)
	if app != nil {
		storeRev = app.Rev()
	}
	if m.valid && m.app == app && m.rev == rev && m.storeRev == storeRev && m.dupeOn == dupeOn && m.spikeOn == spikeOn && m.weekStart == weekStart {
		return m.value
	}
	value := buildTxnRowFlags(app, settings, weekStart)
	m.app, m.rev, m.dupeOn, m.spikeOn, m.weekStart, m.value, m.valid = app, rev, dupeOn, spikeOn, weekStart, value, true
	m.storeRev = storeRev
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
	out := make(map[string]domain.Transaction)
	if len(groupByTxn) == 0 {
		return out
	}
	wanted := make(map[string]bool)
	for _, link := range groupByTxn {
		for _, id := range link.TxnIDs {
			wanted[id] = true
		}
	}
	for _, t := range all {
		if wanted[t.ID] {
			out[t.ID] = t
		}
	}
	return out
}

// txnRowActionName gives a row action enough visible context to distinguish
// repeated merchants while navigating the ledger with a screen reader.
func txnRowActionName(action, description, date, amount string) string {
	return uistate.T("txw1.rowAction", action, description, date, amount)
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
	// nobody has confirmed (C579). AutoCategoryWhy is the sentence explaining what
	// filed it — naming the rule when one accounts for it — shown on the mark's
	// tooltip and as its accessible name.
	AutoCategory    bool
	AutoCategoryWhy string
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
	if previous, ok := c.rows[next.ID]; ok && sameTxnFrameRowValues(*previous, next) {
		return previous
	}
	if _, ok := c.rows[next.ID]; !ok {
		c.order = append(c.order, next.ID)
	}
	current := new(txnFrameRowProps)
	*current = next
	c.rows[next.ID] = current
	for len(c.order) > txnRowPropsCacheLimit {
		delete(c.rows, c.order[0])
		c.order = c.order[1:]
	}
	return current
}

func sameTxnFrameRowValues(a, b txnFrameRowProps) bool {
	// Function values are deliberately excluded: callbacks are kept stable with
	// UseCallback, while reflect.DeepEqual considers any non-nil function unequal.
	a.OnTagClick, b.OnTagClick = nil, nil
	a.OnConfirmCategory, b.OnConfirmCategory = nil, nil
	a.OnSuggestCategory, b.OnSuggestCategory = nil, nil
	a.OnOpenLink, b.OnOpenLink = nil, nil
	a.OnToggleExclude, b.OnToggleExclude = nil, nil
	a.OnOpenFollowUps, b.OnOpenFollowUps = nil, nil
	a.OnOpenSplit, b.OnOpenSplit = nil, nil
	a.OnReceiptSplit, b.OnReceiptSplit = nil, nil
	a.OnOpen, b.OnOpen = nil, nil
	a.OnToggleSelect, b.OnToggleSelect = nil, nil
	a.OnViewReceipt, b.OnViewReceipt = nil, nil
	a.OnCreateRule, b.OnCreateRule = nil, nil
	a.OnDelete, b.OnDelete = nil, nil
	a.OnPairRefund, b.OnPairRefund = nil, nil
	a.OnUnpair, b.OnUnpair = nil, nil
	a.OnUngroup, b.OnUngroup = nil, nil
	a.OnUnlinkBill, b.OnUnlinkBill = nil, nil
	a.OnChooseCategory, b.OnChooseCategory = nil, nil
	return reflect.DeepEqual(a, b)
}

// txnFrameRow renders one transaction row. It owns its edit/select/
// view hooks (per the GWC rule: On* handlers live inside a per-row component,
// never in a loop). The description and pencil open Edit; the leading
// checkbox toggles bulk selection (its cell stops click propagation so toggling
// does not also open the modal); the paperclip opens the first receipt.
// txnTagChipProps configure one clickable tag chip in the description column.
type txnTagChipProps struct {
	Tag     string
	OnClick func(tag string)
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
		Attr("aria-label", uistate.T("transactions.tagFilterTitle", props.Tag)),
		OnClick(onClick), "#"+props.Tag)
}

func txnFrameRow(props *txnFrameRowProps) ui.Node {
	defer txnPerfStart("row")()
	pickerOpen := ui.UseState(false)
	openCategoryPicker := func(ui.Event) {
		if props.OnChooseCategory != nil {
			props.OnChooseCategory(props.ID)
			return
		}
		pickerOpen.Set(true)
	}
	applyManualCategory := func(categoryID string) {
		pickerOpen.Set(false)
		applyTxnManualCategory(props.ID, categoryID)
	}
	// Description and pencil share one explicit edit action. The row itself is
	// not interactive, so selecting text or clicking a date cannot open the editor.
	editRow := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		props.OnOpen(props.ID)
	})
	selToggle := ui.UseEvent(func(e ui.Event) {
		// Read shiftKey defensively: a `change` event has no shiftKey property, so
		// .Bool() on the undefined value PANICS (aborting the handler — selection then
		// silently never registers). Truthy() is undefined-safe; the handler is wired to
		// OnClick (which carries shiftKey) so shift-range select works.
		shift := e.JSValue().Get("shiftKey").Truthy()
		props.OnToggleSelect(props.ID, shift)
	})
	stop := ui.UseEvent(func(e ui.Event) { e.StopPropagation() })
	view := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnViewReceipt != nil {
			props.OnViewReceipt(props.Attachment)
		}
	})

	rowClass := "row txn-ledger-row"
	if props.Selected {
		rowClass += " selected"
	}
	if props.Cleared {
		rowClass += " cleared"
	}
	if props.ExcludedFromReports {
		rowClass += " txn-excluded" // TXC-1: muted, marked out of budgets/reports
	}

	// Explicit state markers (beyond the row tint), inline beside the description.
	//
	// C578 asked for text labels here, and the first attempt put the word in the
	// badge — which made 8 rows in 10 read "• NEEDS REVIEW" in a pill next to the
	// merchant name, crowding the description into an ellipsis. The word belongs in
	// the ledger, but not repeated inside the row's most-read cell: it is the Status
	// COLUMN (below) that carries it, where a repeated value is what a column is FOR
	// and it costs the description nothing.
	//
	// What stays here is the glyph — and, new, an accessible name for it. A bare
	// "✓✓" reads as "check check" to a screen reader and a bare "•" reads as nothing
	// at all, so the state was previously unavailable to anyone not looking at it.
	// role=img makes the name authoritative rather than a hint an AT may drop on a
	// generic span.
	// …and when the Status column IS showing, the glyph is dropped entirely. It would
	// be the same fact twice on one row, once in code and once in words, with the
	// coded copy sitting inside the description — the cell the ledger can least
	// afford to spend. The word wins; the glyph is what you get when the column is off.
	var stateBadge ui.Node = Fragment()
	switch {
	// C618 said this branch order mirrors rowStatusWord exactly. It did not: the
	// two settlement cases sat FIRST here and LAST there, so a row that was both
	// cleared and flagged wore "✓" and said
	// nothing about the review queue — exactly the C601 defect the column's word
	// order was rewritten to fix, re-introduced in the glyph. Needs-review outranks
	// settlement in both places now, and the settled state is still carried in the
	// badge's tooltip and accessible name below.
	case props.Queued && !props.IsTransfer:
		// Review is displayed beside the category, where a person makes the decision.
		stateBadge = Fragment()
	case props.Reconciled:
		// #63: the strongest state — this cleared row falls at or before its
		// account's reconciled-through date, so a statement has vouched for it.
		stateBadge = Span(css.Class("badge"), Attr("data-testid", "txn-reconciled-badge"),
			Attr("role", "img"), Attr("title", uistate.T("transactions.reconciledBadgeTitle")),
			Attr("aria-label", uistate.T("acctxn.legendReconciled")), "✓✓")
	case props.Cleared:
		stateBadge = Span(css.Class("badge"), Attr("data-testid", "txn-cleared-badge"),
			Attr("role", "img"), Attr("title", uistate.T("transactions.clearedBadgeTitle")),
			Attr("aria-label", uistate.T("acctxn.legendCleared")), "✓")
	}

	// C675: what this row IS, said in words beside the description.
	//
	// A transfer's amount is signed account-relative — the very same move reads
	// negative on the leg it left and positive on the leg it reached — so the
	// ledger's income-green / spending-red is not merely unhelpful here, it argues
	// the opposite of the truth on one of the two rows. The badge is what carries
	// the meaning instead of the colour, and it names the far side, because "this
	// is a transfer" leaves the obvious next question unanswered.
	//
	// Debt payment is a SEPARATE badge from Transfer, not a stronger flavour of it
	// (C677): pointing at a card says the money did not leave the household, while
	// counting it as a payment is a claim about which debt it settles, and a row
	// can be the first without being the second.
	var classifyBadge ui.Node = Fragment()
	switch {
	case props.IsDebtPayment:
		label := uistate.T("transactions.badgeDebtPayment")
		classifyBadge = Span(css.Class("badge txn-badge-debt"), Attr("data-testid", "txn-debtpay-badge"),
			Attr("title", uistate.T("transactions.badgeDebtPaymentTitle", firstNonEmpty(props.TransferWith, label))),
			"⇄ "+label)
	case props.IsTransfer:
		classifyBadge = Fragment()
	case props.SuspectTransfer:
		// C672: counting right now, and probably should not be. Phrased as a
		// question because the app does not know — only the person does.
		classifyBadge = Span(css.Class("badge txn-badge-suspect"), Attr("data-testid", "txn-suspect-transfer-badge"),
			Attr("title", uistate.T("transactions.badgeSuspectTransferTitle")),
			"? "+uistate.T("transactions.badgeSuspectTransfer"))
	}

	// XC1/XC2: link badges beside the description, mirroring the classic view.
	var linkBadge ui.Node = Fragment()
	switch {
	case props.GroupSize > 1:
		title := uistate.T("txnlinks.groupBadgeTitle", props.GroupSize, props.GroupTotal)
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-group-badge"), Attr("title", title),
			"◱ "+uistate.T("txnlinks.groupBadge", props.GroupSize))
	case props.IsRefund:
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-refund-badge"),
			Attr("title", uistate.T("txnlinks.refundBadge")), "↩ "+uistate.T("txnlinks.refundBadge"))
	case props.IsRefunded:
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-refunded-badge"),
			Attr("title", uistate.T("txnlinks.refundedBadge")), "↩ "+uistate.T("txnlinks.refundedBadge"))
	}

	// Tags appended after the description: up to 3 small chips in a shrinkable,
	// overflow-hidden group so they never widen the column or spill — extras collapse
	// to a "+N". The whole group can flex-shrink (min-width:0), so a tight row clips
	// the tags cleanly rather than stretching the cell.
	var tagsNode ui.Node = Fragment()
	if len(props.Tags) > 0 {
		const maxTags = 3
		kids := []any{ClassStr("txn-desc-tags"), Attr("data-testid", "txn-row-tags")}
		for i, tg := range props.Tags {
			if i >= maxTags {
				break
			}
			// Own component so each chip's click hook sits at a stable position (never an
			// On* option inside this variable-length loop).
			kids = append(kids, ui.CreateElement(txnTagChip, txnTagChipProps{Tag: tg, OnClick: props.OnTagClick}))
		}
		if extra := len(props.Tags) - maxTags; extra > 0 {
			kids = append(kids, Span(css.Class("txn-desc-tag txn-desc-tag-more"),
				Attr("title", uistate.T("transactions.tagsMoreTitle", extra)), "+"+strconv.Itoa(extra)))
		}
		tagsNode = Span(kids...)
	}

	// Untagged rows show a muted em dash so "where did this come from?" reads as
	// "not recorded" rather than a real source.
	srcClass := "td-source"
	if props.Source == "" || props.Source == "—" {
		srcClass += " text-dim"
	}

	// Cells are rendered in the same conditional order as the table header
	// (transactions_widget.go's `cols`): Select, Date, [Amount], Description,
	// [Account], [Category], [Source], [Who]. An unassigned member stays blank
	// while the cell keeps a useful accessible name.
	accountCellArgs := []any{ClassStr("td-acct")}
	if txnAccountNeedsTooltip(props.Account) {
		accountCellArgs = append(accountCellArgs, Attr("title", props.Account))
	}
	accountCellArgs = append(accountCellArgs, props.Account)
	accountCell := Td(accountCellArgs...)
	categoryAccessibleName := props.Category
	if props.Unfiled && props.SuggestedCategory != "" {
		categoryAccessibleName = uistate.T("r5w.suggestedCategory", props.Category, props.SuggestedCategory)
	}
	categoryCellArgs := []any{ClassStr("td-cat"), Attr("aria-label", categoryAccessibleName)}
	if props.Unfiled && props.SuggestedCategory != "" {
		categoryCellArgs = append(categoryCellArgs, Attr("title", categoryAccessibleName))
	}
	if !props.IsTransfer && txnCategoryNeedsTooltip(props.Category) {
		categoryCellArgs = append(categoryCellArgs, Attr("title", props.Category))
	}
	categoryCellArgs = append(categoryCellArgs,
		Span(css.Class("td-cat-inner"), Style(map[string]string{"min-height": "1.5rem", "width": "100%"}),
			If(props.IsTransfer, Span(css.Class("txn-transfer-category"),
				Attr("data-testid", "txn-transfer-category"),
				Attr("title", uistate.T("r4c.transferCounterparty", props.TransferWith)),
				uistate.T(transferCategoryKey(*props), props.TransferWith))),
			If(!props.IsTransfer && !(props.Unfiled && props.SuggestedCategory != ""),
				ui.CreateElement(txnCategoryChoose, txnCategoryChooseProps{Label: props.Category, OnChoose: openCategoryPicker})),
			If(props.Unfiled && props.SuggestedCategory != "", Span(css.Class("td-cat-name td-cat-uncategorized"), props.Category)),
			If(props.ShowSuggestions && props.Unfiled && !props.IsTransfer,
				ui.CreateElement(txnCatSuggestChip, txnCatSuggestChipProps{TxnID: props.ID, OnChoose: openCategoryPicker}),
			),
			If(props.Queued && !props.IsTransfer, Span(css.Class("txn-review-pill"),
				Attr("data-testid", "txn-review-pill"), Attr("role", "img"),
				Attr("title", statusDetail(*props)),
				Attr("aria-label", statusDetail(*props)),
				uistate.T("r4c.review"))),
			If(props.AutoCategory, Span(css.Class("txn-auto-mark"), Attr("data-testid", "txn-auto-mark"),
				Attr("title", props.AutoCategoryWhy), Attr("aria-label", props.AutoCategoryWhy), "⚙"))))
	categoryCell := Td(categoryCellArgs...)
	rowArgs := []any{ClassStr(rowClass), Attr("data-testid", "txn-row-"+props.ID)}
	// C626: this row's ABSOLUTE position in the match set, not its position in the
	// DOM. In the virtualized "All" view those two differ by however far the user
	// has scrolled, and without the absolute one assistive tech reads "row 3 of
	// 3284" for a row three thousand entries down. Set only when the caller is
	// windowing — a fully rendered page needs no correction, and asserting one
	// that matched the DOM anyway would be noise.
	if props.RowIndex > 0 {
		rowArgs = append(rowArgs, Attr("aria-rowindex", strconv.Itoa(props.RowIndex)))
	}
	// XC1: a grouped member reads as a physical grouping — a quiet accent tie-line
	// on the left rail shared by every member of the purchase.
	if props.GroupSize > 1 {
		rowArgs = append(rowArgs, Style(map[string]string{"box-shadow": "inset 3px 0 0 0 var(--accent)"}))
	}
	rowArgs = append(rowArgs,
		Td(ClassStr("td-select"), OnClick(stop),
			Input(Type("checkbox"), Attr("aria-label", uistate.T("transactions.selectRow", props.Desc)), CheckedIf(props.Selected), OnClick(selToggle))),
		Td(ClassStr("td-date"), props.Date),
		// The description cell is a flex row: the description text truncates (min-width:0),
		// while the badges and the follow-up pill after it stay at natural size so they're
		// never clipped by a long description.
		Td(ClassStr("row-desc-cell"),
			Div(css.Class("row-desc-inner"),
				If(props.ExcludedFromReports, Span(css.Class("badge txn-excluded-badge"), Attr("data-testid", "txn-excluded-badge"),
					Attr("title", uistate.T("transactions.excludeHint")), uistate.T("transactions.excludedBadge"))),
				If(props.HasNote, Span(css.Class("txn-note-glyph"), Attr("data-testid", "txn-row-note"),
					Attr("title", uistate.T("transactions.hasNote")), uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W35, tw.H35)))),
				// title carries the full text, because this cell truncates with an
				// ellipsis at every viewport width and nothing else on the row repeats
				// it. "CKE*DD DOORDASH WINGSTOP 855-431-0459" clipping to "CKE*DD
				// DOORDASH WINGSTOP 855-4" hides exactly the identifying tail a person
				// is squinting at the ledger to find, with no way to see the rest short
				// of opening the row.
				Button(css.Class("row-desc-text txn-description-edit"), Type("button"),
					Attr("data-testid", "txn-description-edit"), Title(props.Desc),
					Attr("aria-label", txnRowActionName(uistate.T("transactions.rowEdit"), props.Desc, props.Date, props.Amount)),
					OnClick(editRow), props.Desc),
				stateBadge,
				tagsNode,
				// Follow-up indicator, to the right of the description: a chip with the open/total
				// count that reveals a hover popover listing the linked to-dos, and links to the
				// filtered To-do list on click. Own component (owns its state + hover hooks); the
				// popover anchors via JS so it escapes the cell's clipping even when trailing.
				If(props.FollowUpTotal > 0, ui.CreateElement(txnFollowUpChip, txnFollowUpChipProps{
					TxnID: props.ID, Open: props.FollowUpOpen, Total: props.FollowUpTotal,
					Items: props.FollowUps, OnOpen: props.OnOpenFollowUps,
				})),
				If(props.Receipts > 0, Button(css.Class("btn btn-icon", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("aria-label", receiptCountLabel(props.Receipts)), Title(receiptCountLabel(props.Receipts)),
					Attr("data-testid", "txn-row-receipt"), OnClick(view),
					uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(strconv.Itoa(props.Receipts)))),
				classifyBadge,
				linkBadge,
				// SM-8 / SM-10: the row's own flag (a likely duplicate, or a charge
				// unusually large for its category). Own component — it owns a click hook.
				If(props.Duplicate || props.SpikeWhy != "",
					ui.CreateElement(txnFlagChip, txnFlagChipProps{
						TxnID: props.ID, Duplicate: props.Duplicate, SpikeWhy: props.SpikeWhy,
					})),
				If(props.EventName != "", Span(css.Class("badge"), Attr("data-testid", "txn-event-chip"),
					Attr("title", uistate.T("events.chipTitle", props.EventName)), "◈ "+props.EventName)),
				// TX6b: spending-trend chip when this merchant has history — opens the merchant
				// story (sparkline + this-vs-typical) that used to hide inside the edit modal.
				// Deferred past route-settle (ShowTrend) so it never slows the ledger paint.
				If(props.TrendMerchant != "" && props.ShowTrend, ui.CreateElement(merchantTrendChip, merchantTrendChipProps{
					Merchant: props.TrendMerchant, TxnID: props.ID, Amount: props.AmountMoney,
				})))),
		// td-acct/td-cat mark the secondary columns so the stylesheet can dim them
		// (2026-07-17 audit: the description column carries the reading priority).
		// The provenance mark sits IN the category cell, at the point of trust: the
		// question it answers ("did a person decide this?") is about that word, and a
		// badge parked over in the description column would not be read as being
		// about the category at all (C579).
		If(props.Vis.Category, categoryCell),
		If(props.Vis.Account, accountCell),
		If(props.Vis.User, txnWhoCellForRow(props)),
		If(props.Vis.Amount, Td(ClassStr("td-amount "+tw.ColorClass(props.AmtTone)), props.Amount)),
		If(props.ShowBalance, Td(ClassStr("td-amount "+tw.ColorClass(props.BalTone)), props.Balance)),
		If(props.Vis.Source, Td(ClassStr(srcClass), props.Source)),
		Td(ClassStr("td-actions"), OnClick(stop),
			// The testid deliberately avoids the `txn-row-` prefix: `[data-testid^=
			// "txn-row-"]` is how the suite selects ROWS, and a per-row child sharing
			// that prefix aliases into the row list — nth(7) stops being the seventh
			// row and starts being a button inside the third. The pre-existing
			// tags/note/receipt ids are conditional, so they only ever perturbed a few
			// rows; an Edit control on EVERY row would shift the whole index space.
			Button(css.Class("btn btn-icon txn-row-edit"), Type("button"),
				Attr("data-testid", "txn-rowedit"),
				Attr("aria-label", txnRowActionName(uistate.T("transactions.rowEdit"), props.Desc, props.Date, props.Amount)),
				Title(uistate.T("transactions.rowEditHint")),
				OnClick(editRow),
				uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(css.Class("txn-row-edit-label"), uistate.T("transactions.rowEdit"))),
			txnRowMenu(*props)),
	)
	row := Tr(rowArgs...)
	pickerTarget := props.CategoryPickerTarget
	if pickerTarget.Selector == "" && pickerTarget.Node == nil {
		pickerTarget = ui.PortalTarget{Selector: "body"}
	}
	var picker ui.Node
	if pickerOpen.Get() {
		picker = ui.Portal(ui.PortalProps{
			Target: pickerTarget,
			Child: ui.CreateElement(CategoryPicker, CategoryPickerProps{
				Kind:     txnCategoryKind(props.AmountMoney),
				Selected: props.CategoryID,
				OnPick:   applyManualCategory,
				OnClose:  func() { pickerOpen.Set(false) },
			}),
		})
	}
	return Fragment(row, picker)
}

type txnCategoryChooseProps struct {
	Label    string
	OnChoose func(ui.Event)
}

func txnCategoryChoose(props txnCategoryChooseProps) ui.Node {
	choose := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnChoose != nil {
			props.OnChoose(e)
		}
	})
	return Button(ClassStr("txn-cat-choose"), Type("button"), Attr("aria-label", uistate.T("r10w.chooseCategory")),
		Attr("data-testid", "txn-category-choose"), OnClick(choose), props.Label)
}

func txnCategoryKind(amount money.Money) domain.CategoryKind {
	if amount.IsPositive() {
		return domain.KindIncome
	}
	return domain.KindExpense
}

func applyTxnManualCategory(txnID, categoryID string) {
	app := appstate.Default
	if app == nil || txnID == "" || categoryID == "" {
		return
	}
	txn, ok := txnByID(app, txnID)
	if !ok || txn.IsTransfer() || txn.CategoryID == categoryID {
		return
	}
	auditview.CaptureNow()
	txn = txnCategoryAfterManualChoice(txn)
	txn.CategoryID = categoryID
	if err := app.PutTransaction(txn); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	uistate.PostUndoable(uistate.T("r10w.categoryFiled", catname.LabelByID(app.Categories(), categoryID)))
}

func ledgerDate(date time.Time, from, to string) string {
	format := "Jan 2, 2006"
	start, startErr := time.Parse("2006-01-02", from)
	end, endErr := time.Parse("2006-01-02", to)
	if startErr == nil && endErr == nil && start.Year() == time.Now().Year() && end.Year() == start.Year() {
		format = "Jan 2"
	}
	return date.Format(format)
}

func transferCategoryKey(props txnFrameRowProps) string {
	if props.AmountMoney.Amount < 0 {
		return "r4c.transferTo"
	}
	return "r4c.transferFrom"
}

func txnCategoryNeedsTooltip(category string) bool {
	return len([]rune(category)) > 23
}

func txnAccountNeedsTooltip(account string) bool {
	return len([]rune(account)) > 20
}

func txnWhoHeader() ui.Node {
	return Span(uistate.T("r4c.who"))
}

func txnOwnerNames(ownerIDs []string, memberNames map[string]string) []string {
	names := make([]string, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		name := strings.TrimSpace(memberNames[id])
		if name == "" {
			name = id
		}
		names = append(names, name)
	}
	return names
}

func txnWhoCell(names []string) ui.Node {
	return txnWhoCellWithColors(names, nil)
}

func txnWhoCellWithColors(names []string, colors []string) ui.Node {
	label := uistate.T("who2.unassigned")
	var avatar ui.Node = uiw.MemberUnassigned(label, "txn-who-unassigned")
	if len(names) == 1 {
		label = names[0]
		avatar = uiw.MemberAvatar(uiw.MemberAvatarProps{Name: label, Title: label, Color: ownerColor(colors, 0), TestID: "txn-who-avatar"})
	} else if len(names) > 1 {
		label = uistate.T("who2.sharedBy", whoNames(names))
		members := make([]uiw.MemberAvatarProps, 0, len(names))
		for i, name := range names {
			members = append(members, uiw.MemberAvatarProps{Name: name, Color: ownerColor(colors, i)})
		}
		avatar = uiw.AvatarStack(uiw.AvatarStackProps{Members: members, Label: label, Title: label, TestID: "txn-who-stack"})
	}
	return Td(ClassStr("td-user"), Attr("aria-label", label), Title(label), avatar)
}

func ownerColor(colors []string, index int) string {
	if index < 0 || index >= len(colors) {
		return ""
	}
	return colors[index]
}

func txnOwnerColors(ownerIDs []string, memberColors map[string]string) []string {
	colors := make([]string, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		colors = append(colors, memberColors[id])
	}
	return colors
}

func txnWhoCellForRow(props *txnFrameRowProps) ui.Node {
	if props.OwnersResolved {
		return txnWhoCellWithColors(props.MemberNames, props.MemberColors)
	}
	// Keep direct legacy row callers working while the ledger frame supplies the
	// complete resolved owner list on every production row.
	label := uistate.T("r4c.unassigned")
	var avatar ui.Node = Fragment()
	if strings.TrimSpace(props.Member) != "" {
		label = uistate.T("txu5.member", props.Member)
		avatar = Span(css.Class("txn-member-avatar"), Attr("style", "--txn-member-color: "+props.MemberColor), txnMemberInitials(props.Member))
	}
	return Td(ClassStr("td-user"), Attr("aria-label", label), avatar)
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
	if props.AutoCategory && props.OnConfirmCategory != nil {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("transactions.confirmCategory"), Icon: icon.Check, Section: organize,
			TestID:   "txn-confirm-category",
			OnSelect: func() { props.OnConfirmCategory(props.ID) },
		})
	}
	if props.OnSuggestCategory != nil && !props.IsTransfer {
		items = append(items, uiw.OverflowMenuItem{
			Label: uistate.T("r5w.suggestCategorySmart"), Icon: icon.Tag, Section: organize,
			TestID:   "txn-categorize-open",
			OnSelect: func() { props.OnSuggestCategory(props.ID) },
		})
	}
	// Split-into-categories: not offered on transfer legs (no category to split).
	if props.OnOpenSplit != nil && !props.IsTransfer {
		splitLabel := uistate.T("r4k.split")
		if props.HasSplits {
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
	if props.OnToggleExclude != nil {
		excLabel := uistate.T("transactions.kebabExclude")
		if props.ExcludedFromReports {
			excLabel = uistate.T("transactions.kebabInclude")
		}
		excIcon := icon.Ban
		if props.ExcludedFromReports {
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
	if props.OnDelete != nil {
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
	if props.OnOpenLink != nil {
		items = append(items,
			uiw.OverflowMenuItem{Label: uistate.T("transactions.markBill"), TestID: "txn-markbill-open", OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeBill) }},
			uiw.OverflowMenuItem{Label: uistate.T("transactions.markSub"), TestID: "txn-marksub-open", OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeSub) }},
		)
	}
	if props.OnPairRefund != nil && props.IsIncome && !props.IsRefund && !props.IsTransfer {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.pairAction"), TestID: "txn-pair-refund", OnSelect: func() { props.OnPairRefund(props.ID) }})
	}
	if props.OnUnpair != nil && (props.IsRefund || props.IsRefunded) {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.unpairAction"), TestID: "txn-unpair", OnSelect: func() { props.OnUnpair(props.ID) }})
	}
	if props.OnUnlinkBill != nil && props.IsBillMatched {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("billmatch.unlink"), TestID: "txn-unlink-bill", OnSelect: func() { props.OnUnlinkBill(props.ID) }})
	}
	if props.OnUngroup != nil && props.GroupSize > 1 {
		items = append(items, uiw.OverflowMenuItem{Label: uistate.T("txnlinks.ungroupAction"), TestID: "txn-ungroup", OnSelect: func() { props.OnUngroup(props.ID) }})
	}
	return items
}
