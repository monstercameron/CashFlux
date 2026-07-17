// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// keep the legacy full-page ledger referenced so it is not flagged as dead by
// linters during the transition (its handlers remain a useful reference).
var _ = transactionsLegacy

const (
	// txnVirtualizeThreshold is the row count above which the "All" view switches to a
	// windowed (virtualized) body — below it, rendering every row is cheap enough.
	txnVirtualizeThreshold = 100
	// txnRowHeight is the fixed ledger row height in px the virtual window measures
	// against (the table rows are uniform: nowrap + table-layout:fixed).
	txnRowHeight = 35
)

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
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Re-render the surface on any data mutation or shared-state change: a bulk op,
	// a row edit/delete, a selection toggle, a sub-view switch, a pending undo, or a
	// receipt preview opening all flow through these atoms.
	_ = uistate.UseDataRevision().Get()
	selAtom := uistate.UseTxnSelection()
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
	// Honour the top-bar active-member perspective (L21): scope to the active member
	// only when the per-screen filter has no member of its own. The persisted filter
	// is never mutated.
	if am := uistate.UseActiveMember().Get(); am != "" && f.Member == "" {
		f.Member = am
	}

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
	shown := txnfilter.ApplyWithLabels(app.Transactions(), f, txnfilter.Labels{
		Account: accName, Category: catName, Payee: app.PayeeResolver().Resolve,
	})

	// The engine render context: the live data every tile body reads from (§6). The
	// SCOPED slices are the filtered ledger, so the table frame and the toolbar/bulk
	// tiles all operate on the same set. Each tile is a registered renderer dispatched
	// by spec below — there are no surface-local closures.
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: app.Transactions(),
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
	bento := Div(css.Class("bento bento-ledger"),
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

	return Div(txnReceiptPreviewOverlay(app, previewAtom), bento)
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
	// Subscribe to the data revision: link changes (XC1/XC2 group/pair badges)
	// mutate no row content, so without this the memoized table body renders
	// stale badges after an ungroup/unpair.
	txnDataRev := uistate.UseDataRevision().Get()
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	previewAtom := uistate.UseTxnPreview()
	colVis := uistate.UseTxnCols().Get() // which optional columns are shown

	// A row's "N follow-ups" chip links to the To-do list pre-filtered to transaction-
	// linked tasks (the closest the shared link filter gets to "this charge's tasks").
	nav := router.UseNavigate()
	// Plain closure (the row component owns the click hook — never register On* here in
	// the row loop). Filters the To-do list to transaction-linked tasks, then navigates.
	openFollowUps := func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkTransaction)
		nav.Navigate(uistate.RoutePath("/todo"))
	}
	// Clicking a tag chip on a row narrows the ledger to that single tag (replacing any
	// multi-tag selection). Plain closure — the tag chip component owns its click hook.
	onTagFilter := func(tag string) {
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.Tag = tag; x.Tags = "" })
	}

	// Register mode (TX12): when the ledger is scoped to exactly one account and the
	// toggle is on, compute each transaction's running balance from the account's FULL
	// chronological history (ledger.RegisterBalances), so a paginated/filtered slice
	// still shows the TRUE figure. A multi-currency account (whose fold errors) leaves
	// runBal nil, and the column is simply not shown.
	regID, singleAcct := f.SingleAccount()
	registerActive := singleAcct && uistate.UseTxnRegisterMode().Get()
	var runBal map[string]money.Money
	if registerActive {
		for _, a := range props.App.Accounts() {
			if a.ID == regID {
				if m, err := ledger.RegisterBalances(a, props.App.Transactions()); err == nil {
					runBal = m
				}
				break
			}
		}
	}
	showBalance := runBal != nil

	setPage := func(p int) { setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.Page = p }) }
	setPageSize := func(s int) { setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.PageSize, x.Page = s, 1 }) }
	// A plain sort handler — the spinner-while-sorting behaviour is the DataTable's
	// standard SortSpinner config (it defers this call so the spinner paints first).
	sortBy := func(key string) {
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
	// a shift-click can resolve the anchor→target span in the order the user sees.
	var visibleOrder []string
	toggleSelect := func(txnID string, shift bool) {
		m := selAtom.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if shift && anchorAtom.Get() != "" && anchorAtom.Get() != txnID {
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
	// TXC-1: flip a transaction's exclude-from-reports flag from the row kebab.
	toggleExclude := func(id string) {
		for _, t := range props.App.Transactions() {
			if t.ID == id {
				t.ExcludeFromReports = !t.ExcludeFromReports
				_ = props.App.PutTransaction(t)
				uistate.BumpDataRevision()
				return
			}
		}
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
	txByID := make(map[string]domain.Transaction, len(props.Shown))
	for _, t := range props.Shown {
		txByID[t.ID] = t
	}
	// Payee-alias resolver (TX1): raw payee → clean display name (learned alias →
	// rule pack → raw). Applied at DISPLAY only; the raw payee stays on the txn.
	payeeResolver := props.App.PayeeResolver()
	// Member id → name, for the optional "User" column (the frame carries no member).
	memberName := make(map[string]string)
	for _, m := range props.App.Members() {
		memberName[m.ID] = m.Name
	}
	// Category id → name, so a split row can list its per-line categories.
	catName := make(map[string]string)
	for _, c := range props.App.Categories() {
		catName[c.ID] = c.Name
	}
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

	sel := selAtom.Get()

	// TX6b: per-merchant charge counts (memoized on the data revision), so each row can
	// decide in O(1) whether to show the spending-trend chip without rescanning the
	// ledger on every sort / select / pagination re-render (the full stats still compute
	// lazily only when a chip is opened).
	merchantCounts := merchantChargeCountsMemo(props.App, txnDataRev)
	// The trend chips are a secondary affordance, so mount them just AFTER the table has
	// painted (useAfterSettle) — this keeps the interactive-row cost off the initial
	// route-settle so the ledger paints as fast as before, then the chips fade in.
	trendReady := useAfterSettle("txn-trend")

	// Follow-up tasks linked to each transaction (open/total + the items behind them), so
	// a row can surface a chip + hover popover. Built once from the task list, read O(1)
	// per row.
	followUps := followUpInfoByTxn(props.App.Tasks(), uistate.UsePrefs().Get().FormatDate)

	// rowPropsAt builds one row's display props from the frame on demand. Factored out
	// so the paginated body and the virtualized window build rows identically — and so
	// the window only materializes the slice it actually shows.
	rowPropsAt := func(i int) txnFrameRowProps {
		rid := idCol.Str(i)
		amt := money.New(amtCol.Int64(i), curCol.Str(i))
		desc := descFull.Str(i)
		payee := payeeCol.Str(i)
		switch {
		case strings.TrimSpace(payee) != "" && payeeResolver.HasLearned(payee):
			// A learned merchant-cleanup alias (TX1/SM-1: "always show this merchant
			// as X") is a deliberate rename that must cascade to EVERY charge, so it
			// wins over the raw import description — cleaning a merchant once updates
			// all of its transaction titles at display time. The raw payee is preserved.
			desc = payeeResolver.Resolve(payee)
		case strings.TrimSpace(desc) == "":
			// No description: show the cleaned payee name, not the raw processor string.
			desc = payeeResolver.Resolve(payee)
		}
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
		return txnFrameRowProps{
			ID:            rid,
			AmountMoney:   amt,
			TrendMerchant: trendMerchant,
			ShowTrend:     trendReady,
			// .UTC() is load-bearing: txn dates are UTC-midnight calendar dates
			// (dateutil), and time.Unix reconstructs in the LOCAL zone — west of
			// UTC that rendered every ledger date a day early (Jul 1 → "Jun 30")
			// while /reports showed Jul 1 for the same transaction (C339).
			Date:                time.Unix(int64(dateCol.Num(i)), 0).UTC().Format("Jan 2, 2006"),
			Amount:              fmtMoney(amt),
			AmtTone:             figTone(amt),
			Desc:                desc,
			Tags:                txByID[rid].Tags,
			Account:             accCol.Str(i),
			Category:            cat,
			Source:              srcCol.Str(i),
			Member:              memberName[txByID[rid].MemberID],
			Cleared:             cleared,
			Selected:            sel[rid],
			Receipts:            nAtt,
			Attachment:          firstAtt,
			Vis:                 colVis,
			BillAccountID:       txByID[rid].BillAccountID,
			SubscriptionName:    txByID[rid].SubscriptionName,
			ExcludedFromReports: txByID[rid].ExcludeFromReports,
			HasNote:             txByID[rid].Note != "",
			HasSplits:           txByID[rid].HasSplits(),
			IsTransfer:          txByID[rid].IsTransfer(),
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
		if g, ok := groupByTxn[r.ID]; ok {
			r.GroupSize = len(g.TxnIDs)
			r.GroupTotal = fmtMoney(txnlinks.GroupSum(txnlinks.GroupMembers(g, txByID)))
		}
		r.OnOpen = openEdit
		r.OnTagClick = onTagFilter
		r.OnToggleSelect = toggleSelect
		r.OnViewReceipt = viewReceipt
		r.OnOpenLink = openLink
		r.OnOpenSplit = openSplit
		r.OnToggleExclude = toggleExclude
		r.OnReceiptSplit = func(id string) { startReceiptSplitFlow(props.App, txByID[id]) }
		r.OnPairRefund = pairRefundRow
		r.OnUnpair = unpairRow
		r.OnUngroup = ungroupRow
		r.OnUnlinkBill = unlinkBillRow
		r.OnOpenFollowUps = openFollowUps
		return ui.CreateElement(txnFrameRow, r)
	}

	// Virtualize the heavy "All" view (only the rows near the viewport are rendered);
	// the bounded pages (25/50/100) are already small, so render the slice directly.
	virtualize := pageSize <= 0 && total > txnVirtualizeThreshold

	// visibleOrder (for shift-range select) spans the rows in the current view: the
	// whole list when virtualized ("All"), otherwise the current page.
	vStart, vEnd := start, end
	if virtualize {
		vStart, vEnd = 0, total
	}
	visibleOrder = make([]string, 0, vEnd-vStart)
	for i := vStart; i < vEnd; i++ {
		visibleOrder = append(visibleOrder, idCol.Str(i))
	}

	var tableBody ui.Node
	switch {
	case len(props.App.Transactions()) == 0:
		tableBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("transactions.empty"), CTALabel: uistate.T("transactions.addFirst"), AddTarget: "transaction", Icon: icon.Transactions, ImportLink: true})
	case total == 0:
		tableBody = P(css.Class("empty"), uistate.T("transactions.noMatch"))
	default:
		// Header columns, built to match the row cells' conditional set exactly (same
		// order): Select + Date + Description are always shown; the rest follow the
		// user's column-visibility choice.
		cols := []uiw.Column{
			{Head: Span(css.Class(tw.SrOnly), "Select")},
			{Label: "Date", SortKey: "date"},
		}
		if colVis.Amount {
			cols = append(cols, uiw.Column{Label: "Amount", SortKey: "amount", Class: "td-amount"})
		}
		// Register mode (TX12): a running-balance column right after Amount. No SortKey —
		// register mode locks the ledger to chronological order, so the figure only reads
		// correctly down the column and the header is not a sort control.
		if showBalance {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colBalance"), Class: "td-amount"})
		}
		cols = append(cols, uiw.Column{Label: "Description", SortKey: "payee"})
		if colVis.Account {
			cols = append(cols, uiw.Column{Label: "Account", SortKey: "account"})
		}
		if colVis.Category {
			cols = append(cols, uiw.Column{Label: "Category", SortKey: "category"})
		}
		if colVis.Source {
			cols = append(cols, uiw.Column{Label: "Source", SortKey: "source"})
		}
		if colVis.User {
			cols = append(cols, uiw.Column{Label: uistate.T("transactions.colUser")})
		}
		cols = append(cols, uiw.Column{Head: Span(css.Class(tw.SrOnly), "Actions"), Class: "td-actions"})
		dtp := uiw.DataTableProps{
			Class:       "txn-table",
			StickyHead:  true,
			Columns:     cols,
			Sort:        f.Sort,
			Dir:         f.Dir,
			OnSort:      sortBy,
			SortSpinner: true,
			Page:        curPage,
			Total:       total,
			PageSize:    pageSize,
			PageSizes:   txnfilter.PageSizes,
			OnPage:      setPage,
			OnPageSize:  setPageSize,
			// On a multi-page ledger, mirror the pager above the table too so rows-per-page
			// (and "All") is reachable without scrolling to the very bottom of a long list.
			TopPager: total > txnfilter.DefaultPageSize,
			// Paging from the bottom pager scrolls the list back to the top, so the user
			// sees the new page's first rows instead of being stranded at the bottom.
			AnchorID: "txn-ledger-anchor",
		}
		if virtualize {
			dtp.Virtual = &uiw.VirtualSpec{
				Count:     total,
				RowHeight: txnRowHeight,
				ColSpan:   len(cols),
				Scroller:  "main.cf-scroll",
				RowAt:     renderRow,
				KeyAt:     func(i int) any { return idCol.Str(i) },
			}
		} else {
			idxs := make([]int, 0, end-start)
			for i := start; i < end; i++ {
				idxs = append(idxs, i)
			}
			dtp.Body = MapKeyed(idxs, func(i int) any { return idCol.Str(i) }, renderRow)
		}
		tableBody = uiw.DataTable(dtp)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-table", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: tableBody,
	})
}

// txnFrameRowProps configures one rendered row of the transactions table tile.
// Display strings are pre-formatted; the callbacks are plain funcs (not hooks) so
// they pass safely through MapKeyed. The row owns its own interaction hooks.
type txnFrameRowProps struct {
	ID      string
	Date    string
	Amount  string
	AmtTone string // color token for the amount figure (e.g. "text-down")
	// AmountMoney is the raw amount (for the merchant-trend delta); TrendMerchant is the
	// resolved merchant name when this row's merchant has enough history to show the
	// spending-trend chip (TX6b), else "".
	AmountMoney   money.Money
	TrendMerchant string
	ShowTrend     bool // defer chip mount until after the table settles (perf)
	Desc          string
	Tags          []string          // appended after the description as small chips (capped, non-stretching)
	OnTagClick    func(tag string)  // click a tag chip → filter the ledger to that tag
	Account       string
	Category      string
	Source        string          // provenance label ("Manual"/"Imported"/…, "—" if unset)
	Member        string          // assigned household member's name ("" if unassigned)
	Vis           uistate.TxnCols // which optional columns to render (must match the header)
	Cleared       bool
	Selected      bool
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
	// OnReceiptSplit (XC11) opens the "Split from receipt…" flow: pick a receipt
	// image, vision reads its line items, and a proposed breakdown pre-fills the
	// split editor for review. Gated like OnOpenSplit (hidden on transfer legs).
	OnReceiptSplit func(txnID string)
	Receipts       int                  // attachment count (drives the paperclip)
	Attachment     domain.AttachmentRef // first attachment, opened by the paperclip
	OnOpen         func(id string)
	OnToggleSelect func(id string, shift bool)
	OnViewReceipt  func(domain.AttachmentRef)
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

// txnFrameRow renders one clickable transaction row. It owns its click/select/
// view hooks (per the GWC rule: On* handlers live inside a per-row component,
// never in a loop). Clicking the row drills into the edit modal; the leading
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

func txnFrameRow(props txnFrameRowProps) ui.Node {
	open := ui.UseEvent(func() { props.OnOpen(props.ID) })
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

	rowClass := "row"
	if props.Selected {
		rowClass += " selected"
	}
	if props.Cleared {
		rowClass += " cleared"
	}
	if props.ExcludedFromReports {
		rowClass += " txn-excluded" // TXC-1: muted, marked out of budgets/reports
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
	// [Account], [Category], [Source], [User]. A muted em dash marks an unassigned
	// member so the User column reads as "nobody" rather than blank.
	member := props.Member
	if strings.TrimSpace(member) == "" {
		member = "—"
	}
	memClass := "td-user"
	if member == "—" {
		memClass += " text-dim"
	}
	rowArgs := []any{ClassStr(rowClass), Attr("data-testid", "txn-row-"+props.ID), OnClick(open)}
	// XC1: a grouped member reads as a physical grouping — a quiet accent tie-line
	// on the left rail shared by every member of the purchase.
	if props.GroupSize > 1 {
		rowArgs = append(rowArgs, Style(map[string]string{"box-shadow": "inset 3px 0 0 0 var(--accent)"}))
	}
	rowArgs = append(rowArgs,
		Td(OnClick(stop),
			Input(Type("checkbox"), Attr("aria-label", uistate.T("transactions.selectRow", props.Desc)), CheckedIf(props.Selected), OnClick(selToggle))),
		Td(props.Date),
		If(props.Vis.Amount, Td(ClassStr("td-amount "+tw.ColorClass(props.AmtTone)), props.Amount)),
		If(props.ShowBalance, Td(ClassStr("td-amount "+tw.ColorClass(props.BalTone)), props.Balance)),
		// The description cell is a flex row: the description text truncates (min-width:0),
		// while the badges and the follow-up pill after it stay at natural size so they're
		// never clipped by a long description.
		Td(ClassStr("row-desc-cell"),
			Div(css.Class("row-desc-inner"),
				If(props.ExcludedFromReports, Span(css.Class("badge txn-excluded-badge"), Attr("data-testid", "txn-excluded-badge"),
					Attr("title", uistate.T("transactions.excludeHint")), uistate.T("transactions.excludedBadge"))),
				If(props.HasNote, Span(css.Class("txn-note-glyph"), Attr("data-testid", "txn-row-note"),
					Attr("title", uistate.T("transactions.hasNote")), uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W35, tw.H35)))),
				Span(css.Class("row-desc-text"), props.Desc),
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
				linkBadge,
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
		If(props.Vis.Account, Td(ClassStr("td-acct"), props.Account)),
		If(props.Vis.Category, Td(ClassStr("td-cat"), props.Category)),
		If(props.Vis.Source, Td(ClassStr(srcClass), props.Source)),
		If(props.Vis.User, Td(ClassStr(memClass), member)),
		Td(ClassStr("td-actions"), OnClick(stop), txnRowMenu(props)),
	)
	return Tr(rowArgs...)
}

// txnRowMenu is the row's ⋯ kebab: entries that open the payment-link flip modal
// (pre-set to Bill or Subscription mode) and the split-into-categories flip modal.
// A ✓ prefixes an entry whose link/breakdown is already set. The picking/clearing
// happens in the modal, so the menu stays short. Built with OverflowMenu
// (loop-safe: it owns each item's click hook).
func txnRowMenu(props txnFrameRowProps) ui.Node {
	var items []uiw.OverflowMenuItem
	if props.OnOpenLink != nil {
		billLabel := uistate.T("transactions.markBill")
		if props.BillAccountID != "" {
			billLabel = "✓ " + billLabel
		}
		subLabel := uistate.T("transactions.markSub")
		if props.SubscriptionName != "" {
			subLabel = "✓ " + subLabel
		}
		items = append(items,
			uiw.OverflowMenuItem{
				Label:    billLabel,
				TestID:   "txn-markbill-open",
				OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeBill) },
			},
			uiw.OverflowMenuItem{
				Label:    subLabel,
				TestID:   "txn-marksub-open",
				OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeSub) },
			})
	}
	// Split-into-categories: not offered on transfer legs (no category to split).
	if props.OnOpenSplit != nil && !props.IsTransfer {
		splitLabel := uistate.T("splitEditor.toggle")
		if props.HasSplits {
			splitLabel = "✓ " + splitLabel
		}
		items = append(items, uiw.OverflowMenuItem{
			Label:    splitLabel,
			TestID:   "txn-split-open",
			OnSelect: func() { props.OnOpenSplit(props.ID) },
		})
	}
	// SM-1: clean up / map this merchant name — a per-transaction entry to the payee
	// mapping that also lives on /rules. Opens the payee-cleanup flip modal.
	if !props.IsTransfer {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("payeeClean.menuAction"),
			TestID:   "txn-cleanname-open",
			OnSelect: func() { uistate.SetPayeeClean(props.ID) },
		})
	}
	// TXC-1: exclude / include this transaction in budgets & reports (still counts
	// toward account balances either way). The label states the action to perform.
	if props.OnToggleExclude != nil {
		excLabel := uistate.T("transactions.kebabExclude")
		if props.ExcludedFromReports {
			excLabel = uistate.T("transactions.kebabInclude")
		}
		items = append(items, uiw.OverflowMenuItem{
			Label:    excLabel,
			TestID:   "txn-toggle-exclude",
			OnSelect: func() { props.OnToggleExclude(props.ID) },
		})
	}
	// Add a follow-up task linked to THIS charge (return it, get reimbursed, dispute it,
	// cancel the subscription…). Seeds the add-task modal with a suggested title + the
	// transaction link pre-selected, then opens it (a due date is optional there).
	{
		merchant := props.TrendMerchant
		if merchant == "" {
			merchant = props.Desc
		}
		txnID := props.ID
		items = append(items, uiw.OverflowMenuItem{
			Label:  uistate.T("transactions.followUpTask"),
			TestID: "txn-followup-task",
			OnSelect: func() {
				uistate.SetTaskAddSeed(uistate.TaskAddSeed{
					Title:    uistate.T("transactions.followUpTaskTitle", merchant),
					LinkType: string(domain.RelatedTransaction),
					LinkID:   txnID,
				})
				uistate.SetAddTarget("task")
			},
		})
	}
	// XC11: propose a split from a receipt image (BYO-key AI). Same transfer-leg
	// gating as the manual split — a transfer leg has no category to split.
	if props.OnReceiptSplit != nil && !props.IsTransfer {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("receiptsplit.menuAction"),
			TestID:   "txn-receipt-split-open",
			OnSelect: func() { props.OnReceiptSplit(props.ID) },
		})
	}
	// XC2: pair a money-in transaction as the refund of an earlier purchase; or
	// remove an existing pairing (offered on either side of the pair).
	if props.OnPairRefund != nil && props.IsIncome && !props.IsRefund && !props.IsTransfer {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("txnlinks.pairAction"),
			TestID:   "txn-pair-refund",
			OnSelect: func() { props.OnPairRefund(props.ID) },
		})
	}
	if props.OnUnpair != nil && (props.IsRefund || props.IsRefunded) {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("txnlinks.unpairAction"),
			TestID:   "txn-unpair",
			OnSelect: func() { props.OnUnpair(props.ID) },
		})
	}
	// TX9: release this row's bill-match link (the occurrence reads unpaid again).
	if props.OnUnlinkBill != nil && props.IsBillMatched {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("billmatch.unlink"),
			TestID:   "txn-unlink-bill",
			OnSelect: func() { props.OnUnlinkBill(props.ID) },
		})
	}
	// XC1: release this row's order group (keeps the transactions).
	if props.OnUngroup != nil && props.GroupSize > 1 {
		items = append(items, uiw.OverflowMenuItem{
			Label:    uistate.T("txnlinks.ungroupAction"),
			TestID:   "txn-ungroup",
			OnSelect: func() { props.OnUngroup(props.ID) },
		})
	}
	if len(items) == 0 {
		return Fragment()
	}
	return uiw.OverflowMenu(uiw.OverflowMenuProps{
		Items:         items,
		TriggerLabel:  uistate.T("transactions.rowActions"),
		TriggerTestID: "txn-kebab-" + props.ID,
	})
}

