// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

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
// duplicate notice, and a screen-reader live summary of the filtered set. It owns
// every filter-field hook and writes the shared filter / view / selection atoms so
// the table and bulk tiles react in step.
func txnToolbarWidget(props txnToolbarProps) ui.Node {
	app := props.App
	filterAtom := uistate.UseTxFilter()
	viewAtom := uistate.UseTxnView()
	selAtom := uistate.UseTxnSelection()

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

	// Filter handlers — UseEvent hooks at stable top-level positions inside this tile.
	onFilterText := func(v string) { setFilter(func(x *uistate.TxFilter) { x.Text = v }) }
	onFilterAcc := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Account = e.GetValue() }) })
	onFilterCat := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Category = e.GetValue() }) })
	onFilterMember := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Member = e.GetValue() }) })
	onFilterSource := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Source = e.GetValue() }) })
	onFilterTag := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Tag = e.GetValue() }) })
	onFilterAmountMin := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.AmountMin = v }) })
	onFilterAmountMax := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.AmountMax = v }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })
	onFilterCleared := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Cleared = e.GetValue() }) })
	onFilterCustomKey := ui.UseEvent(func(e ui.Event) {
		setFilter(func(x *uistate.TxFilter) { x.CustomKey, x.CustomVal = e.GetValue(), "" })
	})
	onFilterCustomVal := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = e.GetValue() }) })
	onFilterCustomValText := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = v }) })
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

	onShowImport := ui.UseEvent(Prevent(func() {
		if viewAtom.Get() == uistate.TxnViewImport {
			viewAtom.Set(uistate.TxnViewLedger)
		} else {
			viewAtom.Set(uistate.TxnViewImport)
		}
	}))
	onShowDuplicates := ui.UseEvent(Prevent(func() {
		if viewAtom.Get() == uistate.TxnViewDuplicates {
			viewAtom.Set(uistate.TxnViewLedger)
		} else {
			viewAtom.Set(uistate.TxnViewDuplicates)
		}
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

	// Filter option lists + chips.
	filterAccOptions := []ui.Node{Option(Value(""), SelectedIf(f.Account == ""), uistate.T("transactions.allAccounts"))}
	for _, a := range accounts {
		filterAccOptions = append(filterAccOptions, Option(Value(a.ID), SelectedIf(f.Account == a.ID), a.Name))
	}
	filterCatOptions := []ui.Node{Option(Value(""), SelectedIf(f.Category == ""), uistate.T("transactions.allCategories"))}
	for _, c := range categories {
		filterCatOptions = append(filterCatOptions, Option(Value(c.ID), SelectedIf(f.Category == c.ID), c.Name))
	}
	filterSourceOptions := []ui.Node{Option(Value(""), SelectedIf(f.Source == ""), uistate.T("transactions.allSources"))}
	for _, s := range domain.AllTxnSources {
		filterSourceOptions = append(filterSourceOptions, Option(Value(string(s)), SelectedIf(f.Source == string(s)), s.Label()))
	}
	filterMemberOptions := []ui.Node{Option(Value(""), SelectedIf(f.Member == ""), uistate.T("transactions.allMembers"))}
	for _, m := range members {
		filterMemberOptions = append(filterMemberOptions, Option(Value(m.ID), SelectedIf(f.Member == m.ID), m.Name))
	}
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
	filterTagOptions := []ui.Node{Option(Value(""), SelectedIf(f.Tag == ""), uistate.T("transactions.allTags"))}
	for _, tg := range tagList {
		filterTagOptions = append(filterTagOptions, Option(Value(tg), SelectedIf(f.Tag == tg), tg))
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

	// Custom-field filter control (L18): field picker + a value control shaped by type.
	txnDefs := app.CustomFieldDefsFor("transaction")
	var customFilterNode ui.Node = Fragment()
	if len(txnDefs) > 0 {
		keyOpts := []ui.Node{Option(Value(""), SelectedIf(f.CustomKey == ""), uistate.T("transactions.filterCustomNone"))}
		var selDef *customfields.Def
		for i := range txnDefs {
			d := txnDefs[i]
			keyOpts = append(keyOpts, Option(Value(d.Key), SelectedIf(f.CustomKey == d.Key), d.Label))
			if d.Key == f.CustomKey {
				selDef = &txnDefs[i]
			}
		}
		var valControl ui.Node = Fragment()
		if selDef != nil {
			switch selDef.Type {
			case customfields.TypeSelect:
				opts := []ui.Node{Option(Value(""), SelectedIf(f.CustomVal == ""), uistate.T("transactions.filterCustomAny"))}
				for _, o := range selDef.Options {
					opts = append(opts, Option(Value(o), SelectedIf(f.CustomVal == o), o))
				}
				valControl = Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), OnChange(onFilterCustomVal), opts)
			case customfields.TypeBool:
				valControl = Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), OnChange(onFilterCustomVal),
					Option(Value(""), SelectedIf(f.CustomVal == ""), uistate.T("transactions.filterCustomAny")),
					Option(Value("true"), SelectedIf(f.CustomVal == "true"), uistate.T("common.yes")),
					Option(Value("false"), SelectedIf(f.CustomVal == "false"), uistate.T("common.no")))
			default:
				valControl = Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), Placeholder(uistate.T("transactions.filterCustomValue")), Value(f.CustomVal), OnInput(onFilterCustomValText))
			}
		}
		customFilterNode = Label(css.Class("field-label"), uistate.T("transactions.filterCustomField"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomField")), OnChange(onFilterCustomKey), keyOpts),
			valControl)
	}

	filtersBody := Div(css.Class("filter-fields"),
		Label(css.Class("field-label"), uistate.T("transactions.filterAccount"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterAccount")), OnChange(onFilterAcc), filterAccOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.filterCategory"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCategory")), OnChange(onFilterCat), filterCatOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.member"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.member")), OnChange(onFilterMember), filterMemberOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.filterSource"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterSource")), OnChange(onFilterSource), filterSourceOptions)),
		If(len(tagList) > 0, Label(css.Class("field-label"), uistate.T("transactions.filterTag"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterTag")), OnChange(onFilterTag), filterTagOptions))),
		Label(css.Class("field-label"), uistate.T("transactions.fromDate"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.fromDate")), Value(f.From), OnInput(onFilterFrom))),
		Label(css.Class("field-label"), uistate.T("transactions.toDate"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.toDate")), Value(f.To), OnInput(onFilterTo))),
		Label(css.Class("field-label"), uistate.T("transactions.filterAmountMin"),
			Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"), Attr("aria-label", uistate.T("transactions.filterAmountMin")), Placeholder(uistate.T("transactions.filterAmountMinPh")), Value(f.AmountMin), OnInput(onFilterAmountMin))),
		Label(css.Class("field-label"), uistate.T("transactions.filterAmountMax"),
			Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"), Attr("aria-label", uistate.T("transactions.filterAmountMax")), Placeholder(uistate.T("transactions.filterAmountMaxPh")), Value(f.AmountMax), OnInput(onFilterAmountMax))),
		Label(css.Class("field-label"), uistate.T("transactions.clearedStatus"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.clearedStatus")), OnChange(onFilterCleared),
				Option(Value(""), SelectedIf(f.Cleared == ""), uistate.T("transactions.clearedAll")),
				Option(Value("no"), SelectedIf(f.Cleared == "no"), uistate.T("transactions.notCleared")),
				Option(Value("yes"), SelectedIf(f.Cleared == "yes"), uistate.T("transactions.cleared")),
			)),
		customFilterNode,
	)

	// Import / duplicates sub-view toggle labels (badge the dupes button with a count).
	dupCount := dedupe.Count(dedupe.FindDuplicates(props.Shown))
	importBtnLabel := uistate.T("transactions.importBtn")
	if viewAtom.Get() == uistate.TxnViewImport {
		importBtnLabel = uistate.T("transactions.importBtnClose")
	}
	dupBtnLabel := uistate.T("transactions.dupReviewBtn")
	if viewAtom.Get() == uistate.TxnViewDuplicates {
		dupBtnLabel = uistate.T("transactions.dupReviewClose")
	} else if dupCount > 0 {
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
		Actions: []ui.Node{
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txn-add-btn"), OnClick(onAdd), uistate.T("transactions.addTitle")),
			If(len(active) > 0, Button(css.Class("btn"), Type("button"), OnClick(clearFilters), uistate.T("transactions.clear"))),
			Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.exportTitle")), OnClick(exportFiltered), uistate.T("transactions.exportCsv")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-import-btn"), Attr("aria-label", importBtnLabel), OnClick(onShowImport), Text(importBtnLabel)),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-dupes-btn"), Attr("aria-label", dupBtnLabel), OnClick(onShowDuplicates), Text(dupBtnLabel)),
		},
	})

	// Select-all + duplicate notice controls row (below the toolbar proper).
	selectAllNode := If(len(props.Shown) > 0, Button(css.Class("btn"), Type("button"),
		Attr("aria-label", uistate.T("transactions.selectAllTitle")), Title(uistate.T("transactions.selectAllTitle")),
		OnClick(selectAllFiltered), uistate.T("transactions.selectAllFiltered")))
	dupNotice := If(dupCount > 0, Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		Span(css.Class("muted"), uistate.T("transactions.dupNotice", plural(dupCount, "possible duplicate"))),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.selectDuplicatesTitle")), OnClick(selectDuplicates), uistate.T("transactions.selectDuplicates"))))
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
	undoAtom := uistate.UseTxnUndo()

	clearSel := func() {
		selAtom.Set(map[string]bool{})
		anchorAtom.Set("")
	}

	onBulkCat := ui.UseEvent(func(e ui.Event) { bulkCatAtom.Set(e.GetValue()) })

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

	bulkCatOptions := []ui.Node{Option(Value(""), SelectedIf(bulkCatAtom.Get() == ""), uistate.T("transactions.bulkNoCategory"))}
	for _, c := range app.Categories() {
		bulkCatOptions = append(bulkCatOptions, Option(Value(c.ID), SelectedIf(bulkCatAtom.Get() == c.ID), c.Name))
	}

	n := len(selAtom.Get())
	body := Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Span(css.Class("muted"), uistate.T("transactions.selected", plural(n, "transaction"))),
		Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.categoryToApply")), Title(uistate.T("transactions.categoryToApply")), OnChange(onBulkCat), bulkCatOptions),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.applyCategoryTitle")), OnClick(bulkRecategorize), uistate.T("transactions.applyCategory")),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.markClearedTitle")), OnClick(bulkMarkCleared), uistate.T("transactions.markCleared")),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.markUnclearedTitle")), OnClick(bulkMarkUncleared), uistate.T("transactions.markUncleared")),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.exportSelectedTitle")), Attr("data-testid", "bulk-export-selected"), OnClick(exportSelected), uistate.T("transactions.exportSelected")),
		Button(css.Class("btn-del"), Type("button"), Title(uistate.T("transactions.deleteSelectedTitle")), OnClick(bulkDelete), uistate.T("transactions.deleteSelected")),
		Button(css.Class("btn"), Type("button"), OnClick(clearSelection), uistate.T("transactions.clearSelection")),
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
