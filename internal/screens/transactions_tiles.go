// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// --- toolbar field helpers ------------------------------------------------------
// These compose the reusable uiw primitives (SelectInput / OptionsFrom) so the
// toolbar stays declarative — "label + options + a plain handler" — instead of
// repeating the hand-rolled Select/aria/Option-loop pattern per field.

// withAllOption prepends the empty-value "all / none" choice to an option list.
func withAllOption(allLabel string, opts []uiw.SelectOption) []uiw.SelectOption {
	return append([]uiw.SelectOption{{Value: "", Label: allLabel}}, opts...)
}

// withFieldLabel wraps a control in the toolbar's `field-label` shell.
func withFieldLabel(label string, control ui.Node) ui.Node {
	return Label(css.Class("field-label"), label, control)
}

// filterSelect is one labeled <select> filter: the reusable SelectInput (which owns
// its own change hook) inside a field-label. onPick gets the chosen value.
func filterSelect(label, selected string, opts []uiw.SelectOption, onPick func(string)) ui.Node {
	return withFieldLabel(label, uiw.SelectInput(uiw.SelectInputProps{
		Options: opts, Selected: selected, AriaLabel: label, OnChange: onPick,
	}))
}

// dateField / amountField are labeled date / amount inputs. Their change hook is
// created by the caller and passed in, since a text input fires on every keystroke
// (so the handler must live at a stable position in the owning component).
func dateField(label, value string, onInput ui.Handler) ui.Node {
	return withFieldLabel(label, Input(css.Class("field"), Type("date"),
		Attr("aria-label", label), Value(value), OnInput(onInput)))
}

func amountField(label, placeholder, value string, onInput ui.Handler) ui.Node {
	return withFieldLabel(label, Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
		Attr("aria-label", label), Placeholder(placeholder), Value(value), OnInput(onInput)))
}

// actionBtn is one labelled action button (used by the bulk-action bar) — collapses
// the repeated Button(class, type, title, [testid], onClick, label) boilerplate.
// toolbarIconBtn renders one sleek transactions-toolbar action: a fixed-size glyph
// button whose text label reveals on hover/focus as a styled tooltip (.tbar-tip). The
// label doubles as the aria-label so the icon-only control stays accessible. variant is
// "" (neutral), "primary" (accent — the Add action), or "danger" (delete).
func toolbarIconBtn(testID string, ic icon.Name, label string, onClick ui.Handler, variant string) ui.Node {
	classes := []any{"tbar-btn"}
	if variant != "" {
		classes = append(classes, variant)
	}
	args := []any{
		css.Class(classes...), Type("button"),
		Attr("aria-label", label), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.W4, tw.H4)),
		Span(css.Class("tbar-tip"), Attr("aria-hidden", "true"), label),
	}
	if testID != "" {
		args = append(args, Attr("data-testid", testID))
	}
	return Button(args...)
}

// txnToolbarProps carries the data the toolbar tile reads to build its filter
// option lists, chips, duplicate notice, and screen-reader summary.
type txnToolbarProps struct {
	App   *appstate.App
	Base  string
	Rates currency.Rates
	Shown []domain.Transaction
}

// txnToolbarWidget is the txn-toolbar tile: the search box, the collapsible filter
// fields, the active-filter chips, and the primary actions (add, clear, export CSV,
// and the import / review-duplicates sub-view toggles), plus a select-all control, a
// duplicate notice, and a screen-reader live summary of the filtered set. It writes
// the shared filter / view / selection atoms so the table and bulk tiles react in step.
func txnToolbarWidget(props txnToolbarProps) ui.Node {
	app := props.App
	filterAtom := uistate.UseTxFilter()
	selAtom := uistate.UseTxnSelection()
	colsModalAtom := uistate.UseTxnColsModalOpen()
	openCols := ui.UseEvent(Prevent(func() { colsModalAtom.Set(true) }))
	smartCatAtom := uistate.UseTxnSmartCatOpen()
	openSmartCat := ui.UseEvent(Prevent(func() { smartCatAtom.Set(true) }))
	importPanelAtom := uistate.UseImportPanelOpen()
	openImportPanel := ui.UseEvent(Prevent(func() { importPanelAtom.Set(true) }))
	dupModalAtom := uistate.UseDuplicatesModalOpen()
	openDuplicates := ui.UseEvent(Prevent(func() { dupModalAtom.Set(true) }))

	f := filterAtom.Get()
	if am := uistate.UseActiveMember().Get(); am != "" && f.Member == "" {
		f.Member = am
	}

	accounts := app.Accounts()
	categories := app.Categories()
	members := app.Members()
	txns := app.Transactions()

	accName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		accName[a.ID] = a.Name
	}
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	memberName := make(map[string]string, len(members))
	for _, m := range members {
		memberName[m.ID] = m.Name
	}

	setFilter := func(mut func(*uistate.TxFilter)) { setTxFilterOn(filterAtom, mut) }
	removeFilter := func(field txnfilter.FilterField) {
		setFilter(func(x *uistate.TxFilter) { *x = x.Without(field) })
	}
	clearAllFilters := func() {
		cleared := uistate.TxFilter{}.Normalize()
		filterAtom.Set(cleared)
		uistate.PersistTxFilter(cleared)
	}

	// Text/amount inputs fire on every keystroke, and each setFilter re-filters the whole
	// ledger + persists the filter — so those are DEBOUNCED (~250ms) to filter/persist
	// once you pause typing rather than on every character; the native input holds what
	// you type between renders, so search-as-you-type stays smooth on a large ledger. The
	// date fields commit a single value from the picker, so they set immediately.
	debFilter := func(key string, mut func(*uistate.TxFilter)) {
		debounce.Call("txn-filter:"+key, 250*time.Millisecond, func() { setFilter(mut) })
	}
	onFilterText := func(v string) { debFilter("text", func(x *uistate.TxFilter) { x.Text = v }) }
	onFilterAmountMin := ui.UseEvent(func(v string) { debFilter("amtmin", func(x *uistate.TxFilter) { x.AmountMin = v }) })
	onFilterAmountMax := ui.UseEvent(func(v string) { debFilter("amtmax", func(x *uistate.TxFilter) { x.AmountMax = v }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })
	onFilterCustomValText := ui.UseEvent(func(v string) { debFilter("customval", func(x *uistate.TxFilter) { x.CustomVal = v }) })
	clearFilters := ui.UseEvent(Prevent(clearAllFilters))
	onAdd := ui.UseEvent(Prevent(func() { uistate.SetQuickAdd(true) }))

	exportFiltered := ui.UseEvent(Prevent(func() {
		rows := txnfilter.Apply(app.Transactions(), filterAtom.Get())
		if len(rows) == 0 {
			uistate.PostNotice(uistate.T("transactions.noExport"), true)
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		downloadBytes("transactions.csv", "text/csv", data)
	}))

	selectAllFiltered := ui.UseEvent(Prevent(func() {
		nm := map[string]bool{}
		for _, t := range txnfilter.Apply(app.Transactions(), filterAtom.Get()) {
			nm[t.ID] = true
		}
		selAtom.Set(nm)
	}))
	selectDuplicates := ui.UseEvent(Prevent(func() {
		nm := map[string]bool{}
		for _, g := range dedupe.FindDuplicates(props.Shown) {
			for _, dupID := range g.IDs[1:] {
				nm[dupID] = true
			}
		}
		selAtom.Set(nm)
		if n := len(nm); n > 0 {
			uistate.PostNotice(uistate.T("transactions.dupSelected", plural(n, "duplicate")), false)
		} else {
			uistate.PostNotice(uistate.T("transactions.dupNoneSelected"), false)
		}
	}))

	// Filter option lists, built once from the entity slices via the reusable
	// OptionsFrom helper (value extractor + label extractor) rather than per-field loops.
	accOpts := withAllOption(uistate.T("transactions.allAccounts"),
		uiw.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, ""))
	catOpts := withAllOption(uistate.T("transactions.allCategories"),
		uiw.OptionsFrom(categories, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name }, ""))
	memberOpts := withAllOption(uistate.T("transactions.allMembers"),
		uiw.OptionsFrom(members, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, ""))
	sourceOpts := withAllOption(uistate.T("transactions.allSources"),
		uiw.OptionsFrom(domain.AllTxnSources, func(s domain.TxnSource) string { return string(s) }, func(s domain.TxnSource) string { return s.Label() }, ""))

	tagSet := map[string]struct{}{}
	for _, t := range txns {
		for _, tg := range t.Tags {
			if s := strings.TrimSpace(tg); s != "" {
				tagSet[s] = struct{}{}
			}
		}
	}
	tagList := make([]string, 0, len(tagSet))
	for tg := range tagSet {
		tagList = append(tagList, tg)
	}
	sort.Strings(tagList)
	tagOpts := withAllOption(uistate.T("transactions.allTags"),
		uiw.OptionsFrom(tagList, func(s string) string { return s }, func(s string) string { return s }, ""))

	clearedOpts := []uiw.SelectOption{
		{Value: "", Label: uistate.T("transactions.clearedAll")},
		{Value: "no", Label: uistate.T("transactions.notCleared")},
		{Value: "yes", Label: uistate.T("transactions.cleared")},
	}

	chipLabel := func(af txnfilter.ActiveFilter) string {
		switch af.Field {
		case txnfilter.FieldText:
			return uistate.T("transactions.chipSearch", af.Value)
		case txnfilter.FieldAccount:
			return uistate.T("transactions.chipAccount", accName[af.Value])
		case txnfilter.FieldCategory:
			return uistate.T("transactions.chipCategory", catName[af.Value])
		case txnfilter.FieldMember:
			return uistate.T("transactions.chipMember", memberName[af.Value])
		case txnfilter.FieldSource:
			return uistate.T("transactions.chipSource", domain.TxnSource(af.Value).Label())
		case txnfilter.FieldTag:
			return uistate.T("transactions.chipTag", af.Value)
		case txnfilter.FieldAmountMin:
			return uistate.T("transactions.chipAmountMin", af.Value)
		case txnfilter.FieldAmountMax:
			return uistate.T("transactions.chipAmountMax", af.Value)
		case txnfilter.FieldFrom:
			return uistate.T("transactions.chipFrom", af.Value)
		case txnfilter.FieldTo:
			return uistate.T("transactions.chipTo", af.Value)
		case txnfilter.FieldCleared:
			if af.Value == "yes" {
				return uistate.T("transactions.cleared")
			}
			return uistate.T("transactions.notCleared")
		}
		return af.Value
	}
	active := f.ActiveFilters()
	chips := make([]uiw.Chip, 0, len(active))
	for _, af := range active {
		chips = append(chips, uiw.Chip{Key: string(af.Field), Label: chipLabel(af)})
	}

	// Custom-field filter (L18): a field picker + a value control shaped by the field's
	// type — both built from the same reusable SelectInput.
	txnDefs := app.CustomFieldDefsFor("transaction")
	var customFilterNode ui.Node = Fragment()
	if len(txnDefs) > 0 {
		var selDef *customfields.Def
		for i := range txnDefs {
			if txnDefs[i].Key == f.CustomKey {
				selDef = &txnDefs[i]
			}
		}
		keyOpts := withAllOption(uistate.T("transactions.filterCustomNone"),
			uiw.OptionsFrom(txnDefs, func(d customfields.Def) string { return d.Key }, func(d customfields.Def) string { return d.Label }, ""))
		keySelect := uiw.SelectInput(uiw.SelectInputProps{
			Options: keyOpts, Selected: f.CustomKey, AriaLabel: uistate.T("transactions.filterCustomField"),
			OnChange: func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomKey, x.CustomVal = v, "" }) },
		})
		var valControl ui.Node = Fragment()
		if selDef != nil {
			setVal := func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = v }) }
			valAria := uistate.T("transactions.filterCustomValue")
			switch selDef.Type {
			case customfields.TypeSelect:
				opts := withAllOption(uistate.T("transactions.filterCustomAny"),
					uiw.OptionsFrom(selDef.Options, func(o string) string { return o }, func(o string) string { return o }, ""))
				valControl = uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: f.CustomVal, AriaLabel: valAria, OnChange: setVal})
			case customfields.TypeBool:
				opts := []uiw.SelectOption{
					{Value: "", Label: uistate.T("transactions.filterCustomAny")},
					{Value: "true", Label: uistate.T("common.yes")},
					{Value: "false", Label: uistate.T("common.no")},
				}
				valControl = uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: f.CustomVal, AriaLabel: valAria, OnChange: setVal})
			default:
				valControl = Input(css.Class("field"), Type("text"), Attr("aria-label", valAria), Placeholder(valAria), Value(f.CustomVal), OnInput(onFilterCustomValText))
			}
		}
		customFilterNode = withFieldLabel(uistate.T("transactions.filterCustomField"), Fragment(keySelect, valControl))
	}

	filtersBody := Div(css.Class("filter-fields"),
		filterSelect(uistate.T("transactions.filterAccount"), f.Account, accOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Account = v }) }),
		filterSelect(uistate.T("transactions.filterCategory"), f.Category, catOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Category = v; x.Categories = "" }) }),
		filterSelect(uistate.T("transactions.member"), f.Member, memberOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Member = v }) }),
		filterSelect(uistate.T("transactions.filterSource"), f.Source, sourceOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Source = v }) }),
		If(len(tagList) > 0, filterSelect(uistate.T("transactions.filterTag"), f.Tag, tagOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Tag = v }) })),
		dateField(uistate.T("transactions.fromDate"), f.From, onFilterFrom),
		dateField(uistate.T("transactions.toDate"), f.To, onFilterTo),
		amountField(uistate.T("transactions.filterAmountMin"), uistate.T("transactions.filterAmountMinPh"), f.AmountMin, onFilterAmountMin),
		amountField(uistate.T("transactions.filterAmountMax"), uistate.T("transactions.filterAmountMaxPh"), f.AmountMax, onFilterAmountMax),
		filterSelect(uistate.T("transactions.clearedStatus"), f.Cleared, clearedOpts, func(v string) { setFilter(func(x *uistate.TxFilter) { x.Cleared = v }) }),
		customFilterNode,
	)

	// The Import and duplicates-review buttons both open flip modals now, so they're
	// plain actions — no open/close view-toggle labels. The dupes button badges its
	// count when there are possible duplicates to review.
	dupCount := dedupe.Count(dedupe.FindDuplicates(props.Shown))
	importBtnLabel := uistate.T("transactions.importBtn")
	dupBtnLabel := uistate.T("transactions.dupReviewBtn")
	if dupCount > 0 {
		dupBtnLabel = uistate.T("transactions.dupReviewBadge", plural(dupCount, "duplicate"))
	}

	toolbar := uiw.FilterToolbar(uiw.FilterToolbarProps{
		Search:       f.Text,
		SearchLabel:  uistate.T("transactions.searchPlaceholder"),
		OnSearch:     onFilterText,
		FiltersLabel: uistate.T("transactions.filters"),
		FiltersTitle: uistate.T("transactions.filtersTitle"),
		ActiveAriaLabel: func(n int) string {
			if n == 0 {
				return uistate.T("transactions.filters")
			}
			return uistate.T("transactions.filtersActiveAria", plural(n, "filter"))
		},
		FilterFields:  filtersBody,
		Chips:         chips,
		OnRemoveChip:  func(key string) { removeFilter(txnfilter.FilterField(key)) },
		OnClearAll:    clearAllFilters,
		ClearAllLabel: uistate.T("transactions.clearAllFilters"),
		RemoveLabel:   uistate.T("transactions.removeFilter"),
		// Sleek icon toolbar: each action is a fixed-size glyph button whose label
		// reveals on hover/focus (a styled tooltip), so the row reads evenly instead of
		// as a run of uneven-width text buttons. aria-label keeps them accessible.
		Actions: []ui.Node{
			toolbarIconBtn("txn-add-btn", icon.Plus, uistate.T("transactions.addTitle"), onAdd, "primary"),
			If(len(active) > 0, toolbarIconBtn("", icon.Close, uistate.T("transactions.clear"), clearFilters, "")),
			toolbarIconBtn("txn-export-btn", icon.ArrowDown, uistate.T("transactions.exportCsv"), exportFiltered, ""),
			toolbarIconBtn("txn-import-btn", icon.Upload, importBtnLabel, openImportPanel, ""),
			toolbarIconBtn("txn-dupes-btn", icon.Copy, dupBtnLabel, openDuplicates, ""),
			toolbarIconBtn("txn-columns-btn", icon.List, uistate.T("transactions.columns"), openCols, ""),
			toolbarIconBtn("txn-smartcat-btn", icon.Sparkles, uistate.T("smartcat.button"), openSmartCat, ""),
		},
	})

	// Select-all + duplicate notice controls row (below the toolbar proper) — glyph
	// buttons with hover labels, matching the toolbar.
	selectAllNode := If(len(props.Shown) > 0,
		toolbarIconBtn("txn-selectall-btn", icon.CheckCircle, uistate.T("transactions.selectAllTitle"), selectAllFiltered, ""))
	dupNotice := If(dupCount > 0, Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		Span(css.Class("muted", tw.Text13), uistate.T("transactions.dupNotice", plural(dupCount, "possible duplicate"))),
		toolbarIconBtn("txn-selectdupes-btn", icon.Copy, uistate.T("transactions.selectDuplicatesTitle"), selectDuplicates, "")))
	controlsRow := If(len(props.Shown) > 0 || dupCount > 0,
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2, tw.Mt2), selectAllNode, dupNotice))

	// Screen-reader live region announcing the match count + net as filters change.
	var shownNet int64
	for _, t := range props.Shown {
		if c, err := props.Rates.Convert(t.Amount, props.Base); err == nil {
			shownNet += c.Amount
		}
	}
	filterStatus := ""
	switch {
	case len(txns) == 0:
		filterStatus = ""
	case len(props.Shown) == 0:
		filterStatus = uistate.T("transactions.noMatch")
	default:
		filterStatus = uistate.T("transactions.summary", plural(len(props.Shown), "transaction"), fmtMoney(money.New(shownNet, props.Base)))
	}
	statusLine := P(css.Class(tw.SrOnly), Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-atomic", "true"), Text(filterStatus))

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(toolbar, controlsRow, statusLine),
	})
}

// TxnColumnsBody is the body of the "show / hide columns" flip modal: a checkbox
// per optional ledger column, writing the persisted visibility atom so the table
// re-renders in step. Date and Description are the row's identity and always show,
// so they aren't offered. It is mounted at the SHELL ROOT (by the app package's
// TxnColumnsHost) rather than inside the toolbar tile, because a tile's CSS
// transform would otherwise mis-position and clip the fixed modal (C-modals).
func TxnColumnsBody(_ struct{}) ui.Node {
	colsAtom := uistate.UseTxnCols()
	cols := colsAtom.Get()
	apply := func(c uistate.TxnCols) { colsAtom.Set(c); uistate.PersistTxnCols(c); uistate.BumpDataRevision() }

	amount := ui.UseEvent(func() { c := colsAtom.Get(); c.Amount = !c.Amount; apply(c) })
	account := ui.UseEvent(func() { c := colsAtom.Get(); c.Account = !c.Account; apply(c) })
	category := ui.UseEvent(func() { c := colsAtom.Get(); c.Category = !c.Category; apply(c) })
	source := ui.UseEvent(func() { c := colsAtom.Get(); c.Source = !c.Source; apply(c) })
	user := ui.UseEvent(func() { c := colsAtom.Get(); c.User = !c.User; apply(c) })

	row := func(label, testID string, on bool, onClick ui.Handler) ui.Node {
		return Label(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"padding": "0.35rem 0", "cursor": "pointer"}),
			Input(Type("checkbox"), Attr("data-testid", testID), CheckedIf(on), OnClick(onClick)),
			Span(label),
		)
	}
	return Div(css.Class(tw.FlexCol),
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("transactions.columnsHint")),
		row(uistate.T("transactions.colAmount"), "col-toggle-amount", cols.Amount, amount),
		row(uistate.T("transactions.colAccount"), "col-toggle-account", cols.Account, account),
		row(uistate.T("transactions.colCategory"), "col-toggle-category", cols.Category, category),
		row(uistate.T("transactions.colSource"), "col-toggle-source", cols.Source, source),
		row(uistate.T("transactions.colUser"), "col-toggle-user", cols.User, user),
	)
}

// txnBulkBarProps carries the app the bulk tile mutates.
type txnBulkBarProps struct {
	App *appstate.App
}

// txnBulkBarWidget is the txn-bulkbar tile, shown by the host when a selection
// exists. It recategorizes, marks cleared/uncleared, exports, or deletes the
// selected transactions, captures a before-snapshot into the shared undo atom, and
// clears the selection. Each op bumps the data revision so the surface re-renders.
func txnBulkBarWidget(props txnBulkBarProps) ui.Node {
	app := props.App
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	bulkCatAtom := uistate.UseTxnBulkCat()
	bulkMemAtom := uistate.UseTxnBulkMember()
	undoAtom := uistate.UseTxnUndo()

	clearSel := func() {
		selAtom.Set(map[string]bool{})
		anchorAtom.Set("")
	}

	bulkSetCleared := func(val bool) {
		sel := selAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.Cleared != val {
				prior = append(prior, t)
			}
		}
		for _, t := range app.Transactions() {
			if !sel[t.ID] || t.Cleared == val {
				continue
			}
			t.Cleared = val
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(uistate.T("transactions.bulkClearErr", err.Error()), true)
			}
		}
		opKey := "transactions.bulkOpCleared"
		if !val {
			opKey = "transactions.bulkOpUncleared"
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T(opKey, len(prior)), Prior: prior})
		clearSel()
		uistate.BumpDataRevision()
	}
	bulkMarkCleared := ui.UseEvent(Prevent(func() { bulkSetCleared(true) }))
	bulkMarkUncleared := ui.UseEvent(Prevent(func() { bulkSetCleared(false) }))

	bulkRecategorize := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		cid := bulkCatAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && !t.IsTransfer() {
				prior = append(prior, t)
			}
		}
		for _, t := range app.Transactions() {
			if !sel[t.ID] || t.IsTransfer() {
				continue
			}
			t.CategoryID = cid
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(uistate.T("transactions.bulkRecatErr", err.Error()), true)
			}
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("transactions.bulkOpRecategorized", len(prior)), Prior: prior})
		clearSel()
		bulkCatAtom.Set("")
		uistate.BumpDataRevision()
	}))

	bulkAssignMember := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		mid := bulkMemAtom.Get()
		var prior []domain.Transaction
		for _, t := range app.Transactions() {
			if sel[t.ID] && t.MemberID != mid {
				prior = append(prior, t)
			}
		}
		for _, t := range app.Transactions() {
			if !sel[t.ID] || t.MemberID == mid {
				continue
			}
			t.MemberID = mid
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(uistate.T("transactions.bulkAssignErr", err.Error()), true)
			}
		}
		undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("transactions.bulkOpAssigned", len(prior)), Prior: prior})
		clearSel()
		bulkMemAtom.Set("")
		uistate.BumpDataRevision()
	}))

	exportSelected := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		if len(sel) == 0 {
			return
		}
		rows := make([]domain.Transaction, 0, len(sel))
		for _, t := range app.Transactions() {
			if sel[t.ID] {
				rows = append(rows, t)
			}
		}
		if len(rows) == 0 {
			uistate.PostNotice(uistate.T("transactions.noExport"), true)
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		downloadBytes("transactions-selected.csv", "text/csv", data)
	}))

	bulkDelete := ui.UseEvent(Prevent(func() {
		sel := selAtom.Get()
		count := len(sel)
		uistate.ConfirmModal(uistate.T("transactions.bulkDeleteConfirm", count), true, func(ok bool) {
			if !ok {
				return
			}
			var prior []domain.Transaction
			for _, t := range app.Transactions() {
				if sel[t.ID] {
					prior = append(prior, t)
				}
			}
			for id := range sel {
				if err := app.DeleteTransactionWithTransferPair(id); err != nil {
					uistate.PostNotice(err.Error(), true)
				}
			}
			undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("transactions.bulkOpDeleted", len(prior)), Prior: prior})
			clearSel()
			uistate.BumpDataRevision()
			uistate.PostUndoable(uistate.T("toast.txnDeleted"))
		})
	}))

	clearSelection := ui.UseEvent(Prevent(clearSel))

	bulkCatOpts := withAllOption(uistate.T("transactions.bulkNoCategory"),
		uiw.OptionsFrom(app.Categories(), func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name }, ""))
	bulkMemOpts := withAllOption(uistate.T("transactions.bulkNoMember"),
		uiw.OptionsFrom(app.Members(), func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, ""))

	n := len(selAtom.Get())
	// Bulk-action bar: keep the category/member pickers as selects, but the actions are
	// sleek glyph buttons with hover labels (matching the toolbar). "Clear selection" is
	// the escape action, kept as a plain text link.
	body := Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Span(css.Class("muted", tw.Text13), uistate.T("transactions.selected", plural(n, "transaction"))),
		uiw.SelectInput(uiw.SelectInputProps{
			Options: bulkCatOpts, Selected: bulkCatAtom.Get(), AriaLabel: uistate.T("transactions.categoryToApply"),
			OnChange: func(v string) { bulkCatAtom.Set(v) },
		}),
		toolbarIconBtn("", icon.Check, uistate.T("transactions.applyCategoryTitle"), bulkRecategorize, ""),
		If(len(app.Members()) > 0, uiw.SelectInput(uiw.SelectInputProps{
			Options: bulkMemOpts, Selected: bulkMemAtom.Get(), AriaLabel: uistate.T("transactions.memberToAssign"), TestID: "bulk-member-select",
			OnChange: func(v string) { bulkMemAtom.Set(v) },
		})),
		If(len(app.Members()) > 0, toolbarIconBtn("bulk-assign-member", icon.Users, uistate.T("transactions.assignMemberTitle"), bulkAssignMember, "")),
		toolbarIconBtn("", icon.CheckCircle, uistate.T("transactions.markClearedTitle"), bulkMarkCleared, ""),
		toolbarIconBtn("", icon.Ban, uistate.T("transactions.markUnclearedTitle"), bulkMarkUncleared, ""),
		toolbarIconBtn("bulk-export-selected", icon.ArrowDown, uistate.T("transactions.exportSelectedTitle"), exportSelected, ""),
		toolbarIconBtn("", icon.Close, uistate.T("transactions.deleteSelectedTitle"), bulkDelete, "danger"),
		Button(css.Class("btn-link"), Type("button"), OnClick(clearSelection), uistate.T("transactions.clearSelection")),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-bulkbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// txnUndoBarProps carries the app the undo tile restores into.
type txnUndoBarProps struct {
	App *appstate.App
}

// txnUndoBarWidget is the txn-undobar tile, shown by the host while a bulk op can be
// undone. It restores the snapshot the last op captured and clears the pending undo.
func txnUndoBarWidget(props txnUndoBarProps) ui.Node {
	undoAtom := uistate.UseTxnUndo()
	snap := undoAtom.Get()

	undoLastBulk := ui.UseEvent(Prevent(func() {
		s := undoAtom.Get()
		if len(s.Prior) == 0 {
			return
		}
		if err := props.App.RestoreTransactions(s.Prior); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		undoAtom.Set(uistate.BulkSnapshot{})
		uistate.BumpDataRevision()
	}))

	body := Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Span(css.Class("muted"), uistate.T("transactions.bulkUndoBanner", snap.Label)),
		Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.undoTitle")), Title(uistate.T("transactions.undoTitle")), OnClick(undoLastBulk), uistate.T("transactions.undoButton")),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-undobar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
