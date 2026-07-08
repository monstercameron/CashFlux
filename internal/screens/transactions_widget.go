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
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
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
//   - txn-toolbar   (Native): search, filters, chips, add/export, import & dupes toggles
//   - txn-bulkbar   (Native): bulk recategorize / clear / export / delete (when a selection exists)
//   - txn-undobar   (Native): undo the last bulk op (when one is pending)
//   - txn-table     (Table) : the engine-hydrated ledger frame, paginated, with row drill-edit
//   - txn-import / txn-duplicates (Native): fill the table slot when those sub-views are active
//
// The tiles share their interaction state (filter, selection, sub-view, undo,
// receipt preview) through atoms in uistate, so no tile embeds another — the host
// just decides which specs are present and the engine renders each. The receipt
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
	viewAtom := uistate.UseTxnView()
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
	shown := txnfilter.ApplyWithLabels(app.Transactions(), f, txnfilter.Labels{Account: accName, Category: catName})

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
	// present; the bulk and undo tiles appear with selection / a pending undo; the
	// main slot is the ledger table unless an import/duplicates sub-view is active.
	specs := []domain.WidgetSpec{txnNativeSpec("txn-toolbar")}
	if len(selAtom.Get()) > 0 {
		specs = append(specs, txnNativeSpec("txn-bulkbar"))
	}
	if len(undoAtom.Get().Prior) > 0 {
		specs = append(specs, txnNativeSpec("txn-undobar"))
	}
	switch viewAtom.Get() {
	case uistate.TxnViewImport:
		specs = append(specs, txnNativeSpec("txn-import"))
	case uistate.TxnViewDuplicates:
		specs = append(specs, txnNativeSpec("txn-duplicates"))
	default:
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
	R("txn-import", func(c widgetrender.RenderCtx) ui.Node {
		return uiw.Widget(uiw.WidgetProps{
			ID: "txn-import", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: ui.CreateElement(DocumentsPanel, documentsPanelProps{}),
		})
	})
	R("txn-duplicates", func(c widgetrender.RenderCtx) ui.Node {
		return uiw.Widget(uiw.WidgetProps{
			ID: "txn-duplicates", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: ui.CreateElement(DuplicatesPanel, duplicatesPanelProps{}),
		})
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
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	previewAtom := uistate.UseTxnPreview()
	colVis := uistate.UseTxnCols().Get() // which optional columns are shown

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
	// Member id → name, for the optional "User" column (the frame carries no member).
	memberName := make(map[string]string)
	for _, m := range props.App.Members() {
		memberName[m.ID] = m.Name
	}
	// openLink opens the payment-link flip modal (shell-root host) for a transaction,
	// pre-set to Bill or Subscription mode. The modal owns the actual write, so the row
	// ⋯ menu just sets the shared target atom.
	linkAtom := uistate.UseTxnLinkTarget()
	openLink := func(txnID, linkMode string) {
		linkAtom.Set(uistate.TxnLinkTarget{TxnID: txnID, Mode: linkMode})
	}

	sel := selAtom.Get()

	// rowPropsAt builds one row's display props from the frame on demand. Factored out
	// so the paginated body and the virtualized window build rows identically — and so
	// the window only materializes the slice it actually shows.
	rowPropsAt := func(i int) txnFrameRowProps {
		rid := idCol.Str(i)
		amt := money.New(amtCol.Int64(i), curCol.Str(i))
		desc := descFull.Str(i)
		if strings.TrimSpace(desc) == "" {
			desc = payeeCol.Str(i)
		}
		cat := catCol.Str(i)
		if strings.TrimSpace(cat) == "" {
			cat = uistate.T("transactions.uncategorized")
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
		return txnFrameRowProps{
			ID: rid,
			// .UTC() is load-bearing: txn dates are UTC-midnight calendar dates
			// (dateutil), and time.Unix reconstructs in the LOCAL zone — west of
			// UTC that rendered every ledger date a day early (Jul 1 → "Jun 30")
			// while /reports showed Jul 1 for the same transaction (C339).
			Date:             time.Unix(int64(dateCol.Num(i)), 0).UTC().Format("Jan 2, 2006"),
			Amount:           fmtMoney(amt),
			AmtTone:          figTone(amt),
			Desc:             desc,
			Account:          accCol.Str(i),
			Category:         cat,
			Source:           srcCol.Str(i),
			Member:           memberName[txByID[rid].MemberID],
			Cleared:          cleared,
			Selected:         sel[rid],
			Receipts:         nAtt,
			Attachment:       firstAtt,
			Vis:              colVis,
			BillAccountID:    txByID[rid].BillAccountID,
			SubscriptionName: txByID[rid].SubscriptionName,
		}
	}
	renderRow := func(i int) ui.Node {
		r := rowPropsAt(i)
		r.OnOpen = openEdit
		r.OnToggleSelect = toggleSelect
		r.OnViewReceipt = viewReceipt
		r.OnOpenLink = openLink
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
	ID       string
	Date     string
	Amount   string
	AmtTone  string // color token for the amount figure (e.g. "text-down")
	Desc     string
	Account  string
	Category string
	Source   string          // provenance label ("Manual"/"Imported"/…, "—" if unset)
	Member   string          // assigned household member's name ("" if unassigned)
	Vis      uistate.TxnCols // which optional columns to render (must match the header)
	Cleared  bool
	Selected bool
	// Payment linkage (the ⋯ row menu): the current bill / subscription links (if any),
	// shown as a ✓ on the menu items. OnOpenLink opens the payment-link flip modal for
	// this transaction, pre-set to the chosen mode (uistate.TxnLinkMode*).
	BillAccountID    string
	SubscriptionName string
	OnOpenLink       func(txnID, mode string)
	Receipts         int                  // attachment count (drives the paperclip)
	Attachment       domain.AttachmentRef // first attachment, opened by the paperclip
	OnOpen           func(id string)
	OnToggleSelect   func(id string, shift bool)
	OnViewReceipt    func(domain.AttachmentRef)
}

// txnFrameRow renders one clickable transaction row. It owns its click/select/
// view hooks (per the GWC rule: On* handlers live inside a per-row component,
// never in a loop). Clicking the row drills into the edit modal; the leading
// checkbox toggles bulk selection (its cell stops click propagation so toggling
// does not also open the modal); the paperclip opens the first receipt.
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
	return Tr(ClassStr(rowClass), Attr("data-testid", "txn-row-"+props.ID), OnClick(open),
		Td(OnClick(stop),
			Input(Type("checkbox"), Attr("aria-label", uistate.T("transactions.selectRow", props.Desc)), CheckedIf(props.Selected), OnClick(selToggle))),
		Td(props.Date),
		If(props.Vis.Amount, Td(ClassStr("td-amount "+tw.ColorClass(props.AmtTone)), props.Amount)),
		Td(props.Desc,
			If(props.Receipts > 0, Button(css.Class("btn btn-icon", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("aria-label", receiptCountLabel(props.Receipts)), Title(receiptCountLabel(props.Receipts)),
				Attr("data-testid", "txn-row-receipt"), OnClick(view),
				uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(strconv.Itoa(props.Receipts))))),
		If(props.Vis.Account, Td(props.Account)),
		If(props.Vis.Category, Td(props.Category)),
		If(props.Vis.Source, Td(ClassStr(srcClass), props.Source)),
		If(props.Vis.User, Td(ClassStr(memClass), member)),
		Td(ClassStr("td-actions"), OnClick(stop), txnRowMenu(props)),
	)
}

// txnRowMenu is the row's ⋯ kebab: two entries that open the payment-link flip modal
// for this transaction, pre-set to Bill or Subscription mode. A ✓ prefixes an entry
// whose link is already set. The picking/clearing happens in the modal, so the menu
// stays short. Built with OverflowMenu (loop-safe: it owns each item's click hook).
func txnRowMenu(props txnFrameRowProps) ui.Node {
	if props.OnOpenLink == nil {
		return Fragment()
	}
	billLabel := uistate.T("transactions.markBill")
	if props.BillAccountID != "" {
		billLabel = "✓ " + billLabel
	}
	subLabel := uistate.T("transactions.markSub")
	if props.SubscriptionName != "" {
		subLabel = "✓ " + subLabel
	}
	items := []uiw.OverflowMenuItem{
		{
			Label:    billLabel,
			TestID:   "txn-markbill-open",
			OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeBill) },
		},
		{
			Label:    subLabel,
			TestID:   "txn-marksub-open",
			OnSelect: func() { props.OnOpenLink(props.ID, uistate.TxnLinkModeSub) },
		},
	}
	return uiw.OverflowMenu(uiw.OverflowMenuProps{
		Items:         items,
		TriggerLabel:  uistate.T("transactions.rowActions"),
		TriggerTestID: "txn-kebab-" + props.ID,
	})
}
