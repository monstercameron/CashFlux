// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Transactions is the global ledger: add income/expense, list newest first, delete.
func Transactions() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	nav := router.UseNavigate()
	rev := state.UseAtom("rev:transactions", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	accounts := app.Accounts()
	categories := app.Categories()

	accByID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	accName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		accName[a.ID] = a.Name
	}
	selected := ui.UseState(map[string]bool{})
	bulkCat := ui.UseState("")
	errMsg := ui.UseState("")
	// lastBulk holds a one-level undo snapshot for the most recent destructive bulk
	// operation. Count == 0 means no undo is available.
	type bulkSnapshot struct {
		Label string
		Prior []domain.Transaction
	}
	zeroBulk := bulkSnapshot{}
	lastBulk := ui.UseState(zeroBulk)
	noticeAtom := uistate.UseNotice()
	notifyErr := func(text string) { noticeAtom.Set(noticeAtom.Get().With(text, true)) }
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	// L21: honour the top-bar active-member perspective. When a member is
	// selected in the switcher and the per-screen filter has no explicit member
	// constraint of its own, scope the ledger to that member automatically.
	// The persisted filter is never mutated — the switcher scope is layered on
	// top so the user's own manual member filter always takes priority.
	activeMemberAtom := uistate.UseActiveMember()
	if am := activeMemberAtom.Get(); am != "" && f.Member == "" {
		f.Member = am
	}
	setFilter := func(mut func(*uistate.TxFilter)) {
		prev := filterAtom.Get()
		nf := prev
		mut(&nf)
		// A filter or sort change starts a new result set, so jump back to page 1;
		// a pure page/size change keeps your spot.
		nf = nf.ResetPageIfScopeChanged(prev).Normalize()
		filterAtom.Set(nf)
		uistate.PersistTxFilter(nf)
	}
	setPage := func(p int) { setFilter(func(x *uistate.TxFilter) { x.Page = p }) }
	setPageSize := func(s int) { setFilter(func(x *uistate.TxFilter) { x.PageSize, x.Page = s, 1 }) }

	onFilterText := func(v string) { setFilter(func(x *uistate.TxFilter) { x.Text = v }) }
	onFilterAcc := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Account = e.GetValue() }) })
	onFilterCat := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Category = e.GetValue() }) })
	// Click a column header to sort by it; click the active column again to flip
	// direction. (Replaces the old Sort dropdown — C47.)
	sortBy := func(key string) {
		setFilter(func(x *uistate.TxFilter) {
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
	onFilterMember := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Member = e.GetValue() }) })
	onFilterFrom := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.From = v }) })
	onFilterTo := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.To = v }) })
	onFilterCleared := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.Cleared = e.GetValue() }) })
	// Filter by a transaction custom field's value (L18): choosing a field sets the
	// key (and clears the stale value); choosing/typing a value narrows the list.
	onFilterCustomKey := ui.UseEvent(func(e ui.Event) {
		setFilter(func(x *uistate.TxFilter) { x.CustomKey, x.CustomVal = e.GetValue(), "" })
	})
	onFilterCustomVal := ui.UseEvent(func(e ui.Event) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = e.GetValue() }) })
	onFilterCustomValText := ui.UseEvent(func(v string) { setFilter(func(x *uistate.TxFilter) { x.CustomVal = v }) })
	// Remove a single active filter (a chip's ✕). Without is a scope change, so the
	// page resets back to 1 via ResetPageIfScopeChanged.
	removeFilter := func(field txnfilter.FilterField) {
		setFilter(func(x *uistate.TxFilter) { *x = x.Without(field) })
	}
	// clearAllFilters resets every filter at once (the toolbar's "clear all" link).
	clearAllFilters := func() {
		cleared := uistate.TxFilter{}.Normalize()
		filterAtom.Set(cleared)
		uistate.PersistTxFilter(cleared)
	}

	txnDefs := app.CustomFieldDefsFor("transaction")
	clearFilters := ui.UseEvent(Prevent(func() {
		cleared := uistate.TxFilter{}.Normalize()
		filterAtom.Set(cleared)
		uistate.PersistTxFilter(cleared)
	}))
	exportFiltered := ui.UseEvent(Prevent(func() {
		rows := txnfilter.Apply(app.Transactions(), filterAtom.Get())
		if len(rows) == 0 {
			errMsg.Set(uistate.T("transactions.noExport"))
			return
		}
		data, err := app.TransactionsCSV(rows)
		if err != nil {
			errMsg.Set(err.Error())
			return
		}
		downloadBytes("transactions.csv", "text/csv", data)
	}))

	// Receipt attachments (L29): the preview holds the currently-open attachment
	// ("" ArtifactID = closed). attachReceipt uploads an image and links it.
	previewRef := ui.UseState(domain.AttachmentRef{})
	attachReceipt := func(t domain.Transaction) {
		pickFile("image/*", func(name, mime string, data []byte) {
			art := domain.Artifact{ID: id.New(), Name: name, Kind: "image", MIME: mime, Bytes: data, Size: len(data), CreatedAt: time.Now()}
			if err := app.PutArtifact(art); err != nil {
				notifyErr(uistate.T("transactions.attachReceiptTitle") + ": " + err.Error())
				return
			}
			t.Attachments = append(t.Attachments, domain.AttachmentRef{ArtifactID: art.ID, Name: name, Kind: "image", MIME: mime})
			if err := app.PutTransaction(t); err != nil {
				notifyErr(err.Error())
				return
			}
			bump()
		})
	}
	viewReceipt := func(ref domain.AttachmentRef) { previewRef.Set(ref) }
	closePreview := ui.UseEvent(Prevent(func() { previewRef.Set(domain.AttachmentRef{}) }))

	duplicateTxn := func(t domain.Transaction) {
		cp := t
		cp.ID = id.New()
		cp.Date = time.Now()
		cp.TransferAccountID = "" // a duplicate is a standalone entry, not a transfer leg
		if len(t.Tags) > 0 {
			cp.Tags = append([]string(nil), t.Tags...)
		}
		if err := app.PutTransaction(cp); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	// createRuleFromTxn prefills the Rules add-form with this transaction's payee
	// (falling back to its description) and current category, then navigates there
	// so the user can confirm and save the rule in one click.
	createRuleFromTxn := func(t domain.Transaction) {
		phrase := strings.TrimSpace(firstNonEmpty(t.Payee, t.Desc))
		uistate.SetRuleDraft(phrase, t.CategoryID)
		nav.Navigate(uistate.RoutePath("/rules"))
	}

	toggleCleared := func(t domain.Transaction) {
		t.Cleared = !t.Cleared
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	editTxn := func(orig domain.Transaction, newDesc, amountStr, catID, dateStr, memberID string) {
		acc := accByID[orig.AccountID]
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(acc.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("transactions.positiveAmount"))
			return
		}
		if orig.Amount.IsNegative() {
			amt = -amt // preserve the original income/expense sign
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateStr))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		orig.Desc = strings.TrimSpace(newDesc)
		orig.Amount = money.New(amt, orig.Amount.Currency)
		orig.CategoryID = catID
		orig.Date = date
		if memberID != "" {
			orig.MemberID = memberID
		}
		if err := app.PutTransaction(orig); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	deleteTxn := func(txnID string) {
		// Capture where focus is (the row being deleted) before it's removed, so we
		// can land focus on the next row after the re-render instead of dropping it
		// to <body> (§6.7).
		focusIdx := consumeRowDeleteFocus()
		if err := app.DeleteTransactionWithTransferPair(txnID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
		focusRowAfterDelete(".txn-table tbody", "tr.row", focusIdx)
		auditview.CaptureNow()
		uistate.PostUndoable(uistate.T("toast.txnDeleted"))
	}

	toggleSelect := func(txnID string) {
		m := selected.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if nm[txnID] {
			delete(nm, txnID)
		} else {
			nm[txnID] = true
		}
		selected.Set(nm)
	}
	clearSelection := ui.UseEvent(Prevent(func() { selected.Set(map[string]bool{}) }))
	// Select the extra copies in each duplicate group (all but the first), so the
	// existing bulk-delete can clean them up in one go.
	selectDuplicates := ui.UseEvent(Prevent(func() {
		nm := map[string]bool{}
		for _, g := range dedupe.FindDuplicates(app.Transactions()) {
			for _, dupID := range g.IDs[1:] {
				nm[dupID] = true
			}
		}
		selected.Set(nm)
	}))
	bulkDelete := ui.UseEvent(Prevent(func() {
		sel := selected.Get()
		count := len(sel)
		// Gate bulk deletes behind a count-aware confirm dialog (GM3 D1 / L50 safety gap):
		// firing immediately with 50+ rows selected would be irreversible in one click.
		uistate.ConfirmModal(
			uistate.T("transactions.bulkDeleteConfirm", count),
			true,
			func(ok bool) {
				if !ok {
					return
				}
				// Snapshot the transactions about to be deleted before removing them.
				var prior []domain.Transaction
				for _, t := range app.Transactions() {
					if sel[t.ID] {
						prior = append(prior, t)
					}
				}
				for id := range sel {
					deleteTxn(id)
				}
				lastBulk.Set(bulkSnapshot{
					Label: uistate.T("transactions.bulkOpDeleted", len(prior)),
					Prior: prior,
				})
				selected.Set(map[string]bool{})
			},
		)
	}))
	bulkSetCleared := func(val bool) {
		sel := selected.Get()
		// Snapshot the pre-change state of every transaction that will be mutated.
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
				notifyErr(uistate.T("transactions.bulkClearErr", err.Error()))
			}
		}
		opKey := "transactions.bulkOpCleared"
		if !val {
			opKey = "transactions.bulkOpUncleared"
		}
		lastBulk.Set(bulkSnapshot{
			Label: uistate.T(opKey, len(prior)),
			Prior: prior,
		})
		selected.Set(map[string]bool{})
		bump()
	}
	bulkMarkCleared := ui.UseEvent(Prevent(func() { bulkSetCleared(true) }))
	bulkMarkUncleared := ui.UseEvent(Prevent(func() { bulkSetCleared(false) }))
	onBulkCat := ui.UseEvent(func(e ui.Event) { bulkCat.Set(e.GetValue()) })
	bulkRecategorize := ui.UseEvent(Prevent(func() {
		sel := selected.Get()
		cid := bulkCat.Get()
		// Snapshot the pre-change state of every transaction that will be mutated.
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
				notifyErr(uistate.T("transactions.bulkRecatErr", err.Error()))
			}
		}
		lastBulk.Set(bulkSnapshot{
			Label: uistate.T("transactions.bulkOpRecategorized", len(prior)),
			Prior: prior,
		})
		selected.Set(map[string]bool{})
		bulkCat.Set("")
		bump()
	}))

	// undoLastBulk reverts the most recent bulk operation using the captured snapshot.
	undoLastBulk := ui.UseEvent(Prevent(func() {
		snap := lastBulk.Get()
		if len(snap.Prior) == 0 {
			return
		}
		if err := app.RestoreTransactions(snap.Prior); err != nil {
			notifyErr(err.Error())
			return
		}
		lastBulk.Set(zeroBulk)
		bump()
	}))

	// selectAllFiltered selects exactly the transactions visible under the current filter.
	selectAllFiltered := ui.UseEvent(Prevent(func() {
		nm := map[string]bool{}
		for _, t := range txnfilter.Apply(app.Transactions(), filterAtom.Get()) {
			nm[t.ID] = true
		}
		selected.Set(nm)
	}))

	txns := app.Transactions()
	shown := txnfilter.ApplyWithLabels(txns, f, txnfilter.Labels{Account: accName, Category: catName})

	// Heads-up for likely double entries (same date, amount, and description).
	dupCount := dedupe.Count(dedupe.FindDuplicates(txns))

	// Summary of the shown set: count + net total converted to the base currency.
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	var shownNet int64
	var unclearedCount int
	for _, t := range shown {
		if c, err := rates.Convert(t.Amount, base); err == nil {
			shownNet += c.Amount
		}
		if !t.Cleared {
			unclearedCount++
		}
	}

	// Status text for the screen-reader live region: how many transactions match
	// the current filters, announced as the filters change. Mirrors the visible
	// summary, but also covers the zero-results case (the visible summary hides at
	// zero, so without this the "no matches" outcome would never be announced).
	filterStatus := ""
	switch {
	case len(txns) == 0:
		filterStatus = ""
	case len(shown) == 0:
		filterStatus = uistate.T("transactions.noMatch")
	default:
		filterStatus = uistate.T("transactions.summary", plural(len(shown), "transaction"), fmtMoney(money.New(shownNet, base)))
	}

	var listBody ui.Node
	switch {
	case len(txns) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("transactions.empty"), CTALabel: uistate.T("transactions.addFirst"), AddTarget: "transaction", Icon: icon.Transactions})
	case len(shown) == 0:
		listBody = P(css.Class("empty"), uistate.T("transactions.noMatch"))
	default:
		// Paginate the filtered set to the persisted page/size (C47), so a long
		// ledger renders one page at a time.
		total := len(shown)
		pageSize := f.PageSize
		if pageSize == 0 {
			pageSize = txnfilter.DefaultPageSize
		}
		// "All" (negative sentinel) must render every row — slice the whole set on a
		// single page, not the default window (L78-T2).
		curPage := 1
		sliceSize := pageSize
		if pageSize > 0 {
			curPage = pagination.Clamp(f.Page, total, pageSize)
		} else {
			sliceSize = total
			if sliceSize < 1 {
				sliceSize = 1
			}
		}
		page := pagination.Slice(shown, curPage, sliceSize)
		// Hide the Tags column when nothing on this page is tagged (G2 §6): tags are
		// sparse, so an always-on empty column just wastes scan width.
		anyTags := false
		for _, t := range page {
			if len(t.Tags) > 0 {
				anyTags = true
				break
			}
		}
		rows := MapKeyed(page,
			func(t domain.Transaction) any { return t.ID },
			func(t domain.Transaction) ui.Node {
				acc := accByID[t.AccountID]
				return ui.CreateElement(TransactionRow, transactionRowProps{
					Txn: t, Account: acc.Name, Category: catName[t.CategoryID], Categories: categories,
					Members:  app.Members(),
					Selected: selected.Get()[t.ID],
					ShowTags: anyTags,
					OnDelete: deleteTxn, OnDuplicate: duplicateTxn, OnSave: editTxn, OnToggleSelect: toggleSelect, OnToggleCleared: toggleCleared, OnCreateRule: createRuleFromTxn,
					OnAttach: attachReceipt, OnViewReceipt: viewReceipt,
				})
			},
		)
		// Column order (G2 §5): Amount promoted to position 3 (right after Date) so
		// the dollar figure is on Nadia's natural scan path Date → Amount → Desc,
		// instead of buried at column 7 behind Category/Account/Tags.
		cols := []uiw.Column{
			{Head: Span(css.Class(tw.SrOnly), "Select")},
			{Label: "Date", SortKey: "date"},
			{Label: "Amount", SortKey: "amount", Class: "td-amount"},
			{Label: "Description", SortKey: "payee"},
			{Label: "Category", SortKey: "category"},
			{Label: "Account", SortKey: "account"},
		}
		if anyTags {
			cols = append(cols, uiw.Column{Label: "Tags"})
		}
		cols = append(cols,
			uiw.Column{Head: Span(Attr("aria-label", uistate.T("transactions.clearedStatus")), "✓"), Class: "td-cleared"},
			uiw.Column{Label: "Actions", Class: "td-actions"},
		)
		listBody = uiw.DataTable(uiw.DataTableProps{
			Class:      "txn-table",
			Columns:    cols,
			Body:       rows,
			Sort:       f.Sort,
			Dir:        f.Dir,
			OnSort:     sortBy,
			Page:       curPage,
			Total:      total,
			PageSize:   pageSize,
			PageSizes:  txnfilter.PageSizes,
			OnPage:     setPage,
			OnPageSize: setPageSize,
		})
	}

	filterAccOptions := []ui.Node{Option(Value(""), SelectedIf(f.Account == ""), uistate.T("transactions.allAccounts"))}
	for _, a := range accounts {
		filterAccOptions = append(filterAccOptions, Option(Value(a.ID), SelectedIf(f.Account == a.ID), a.Name))
	}
	filterCatOptions := []ui.Node{Option(Value(""), SelectedIf(f.Category == ""), uistate.T("transactions.allCategories"))}
	for _, c := range categories {
		filterCatOptions = append(filterCatOptions, Option(Value(c.ID), SelectedIf(f.Category == c.ID), c.Name))
	}
	filterMemberOptions := []ui.Node{Option(Value(""), SelectedIf(f.Member == ""), uistate.T("transactions.allMembers"))}
	for _, m := range app.Members() {
		filterMemberOptions = append(filterMemberOptions, Option(Value(m.ID), SelectedIf(f.Member == m.ID), m.Name))
	}
	bulkCatOptions := []ui.Node{Option(Value(""), SelectedIf(bulkCat.Get() == ""), uistate.T("transactions.bulkNoCategory"))}
	for _, c := range categories {
		bulkCatOptions = append(bulkCatOptions, Option(Value(c.ID), SelectedIf(bulkCat.Get() == c.ID), c.Name))
	}

	// Resolve an active filter to a human chip label (IDs → names, dates as-is,
	// cleared → its word). Used for the removable chips below the toolbar (C47).
	memberName := make(map[string]string)
	for _, m := range app.Members() {
		memberName[m.ID] = m.Name
	}
	chipLabel := func(af txnfilter.ActiveFilter) string {
		switch af.Field {
		case txnfilter.FieldText:
			return uistate.T("transactions.chipSearch", af.Value)
		case txnfilter.FieldAccount:
			return uistate.T("transactions.chipAccount", accByID[af.Value].Name)
		case txnfilter.FieldCategory:
			return uistate.T("transactions.chipCategory", catName[af.Value])
		case txnfilter.FieldMember:
			return uistate.T("transactions.chipMember", memberName[af.Value])
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

	// The Filters popover body — the controls that used to crowd the inline strip,
	// now grouped inside the toolbar's FlipPanel. Filters apply live (each onChange
	// persists), so the panel is close-only with nothing to "save".
	// Custom-field filter control (L18): a field picker + a value control whose
	// shape follows the field type (select → option dropdown, bool → Yes/No, else
	// a text box). Built inline since the handlers already exist at stable hook
	// positions; the option lists involve no hooks.
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
				valControl = Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), Attr("data-testid", "txn-filter-custom-val"), OnChange(onFilterCustomVal), opts)
			case customfields.TypeBool:
				valControl = Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), Attr("data-testid", "txn-filter-custom-val"), OnChange(onFilterCustomVal),
					Option(Value(""), SelectedIf(f.CustomVal == ""), uistate.T("transactions.filterCustomAny")),
					Option(Value("true"), SelectedIf(f.CustomVal == "true"), uistate.T("common.yes")),
					Option(Value("false"), SelectedIf(f.CustomVal == "false"), uistate.T("common.no")))
			default:
				valControl = Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.filterCustomValue")), Attr("data-testid", "txn-filter-custom-val"), Placeholder(uistate.T("transactions.filterCustomValue")), Value(f.CustomVal), OnInput(onFilterCustomValText))
			}
		}
		customFilterNode = Label(css.Class("field-label"), uistate.T("transactions.filterCustomField"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCustomField")), Attr("data-testid", "txn-filter-custom-key"), OnChange(onFilterCustomKey), keyOpts),
			valControl)
	}

	filtersBody := Div(css.Class("filter-fields"),
		Label(css.Class("field-label"), uistate.T("transactions.filterAccount"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterAccount")), OnChange(onFilterAcc), filterAccOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.filterCategory"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.filterCategory")), OnChange(onFilterCat), filterCatOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.member"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.member")), OnChange(onFilterMember), filterMemberOptions)),
		Label(css.Class("field-label"), uistate.T("transactions.fromDate"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.fromDate")), Value(f.From), OnInput(onFilterFrom))),
		Label(css.Class("field-label"), uistate.T("transactions.toDate"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.toDate")), Value(f.To), OnInput(onFilterTo))),
		Label(css.Class("field-label"), uistate.T("transactions.clearedStatus"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.clearedStatus")), OnChange(onFilterCleared),
				Option(Value(""), SelectedIf(f.Cleared == ""), uistate.T("transactions.clearedAll")),
				Option(Value("no"), SelectedIf(f.Cleared == "no"), uistate.T("transactions.notCleared")),
				Option(Value("yes"), SelectedIf(f.Cleared == "yes"), uistate.T("transactions.cleared")),
			)),
		customFilterNode,
	)

	// Receipt preview overlay (L29): when a row's paperclip is clicked, look up the
	// referenced artifact's bytes and show the image with a close control.
	var previewNode ui.Node = Fragment()
	if ref := previewRef.Get(); ref.ArtifactID != "" {
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
		previewNode = Div(css.Class("receipt-preview-overlay"), Attr("role", "dialog"), Attr("aria-label", uistate.T("transactions.previewReceipt")),
			uiw.Card(uiw.CardProps{
				Header: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
					H2(css.Class("card-title"), uistate.T("transactions.previewReceipt")),
					Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.previewClose")), Attr("data-testid", "receipt-preview-close"), OnClick(closePreview), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
				),
				Body: body,
			}),
		)
	}

	return Div(
		previewNode,
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("transactions.listTitle"),
			Body: Fragment(
				uiw.FilterToolbar(uiw.FilterToolbarProps{
					Search:        f.Text,
					SearchLabel:   uistate.T("transactions.searchPlaceholder"),
					OnSearch:      onFilterText,
					FiltersLabel:  uistate.T("transactions.filters"),
					FiltersTitle:  uistate.T("transactions.filtersTitle"),
					FilterFields:  filtersBody,
					Chips:         chips,
					OnRemoveChip:  func(key string) { removeFilter(txnfilter.FilterField(key)) },
					OnClearAll:    clearAllFilters,
					ClearAllLabel: uistate.T("transactions.clearAllFilters"),
					RemoveLabel:   uistate.T("transactions.removeFilter"),
					Actions: []ui.Node{
						Button(css.Class("btn"), Type("button"), OnClick(clearFilters), uistate.T("transactions.clear")),
						Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.exportTitle")), OnClick(exportFiltered), uistate.T("transactions.exportCsv")),
					},
				}),
				If(len(selected.Get()) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), Style(map[string]string{"margin-bottom": "0.6rem"}),
					Span(css.Class("muted"), uistate.T("transactions.selected", plural(len(selected.Get()), "transaction"))),
					Select(css.Class("field"), Attr("aria-label", uistate.T("transactions.categoryToApply")), Title(uistate.T("transactions.categoryToApply")), OnChange(onBulkCat), bulkCatOptions),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.applyCategoryTitle")), OnClick(bulkRecategorize), uistate.T("transactions.applyCategory")),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.markClearedTitle")), OnClick(bulkMarkCleared), uistate.T("transactions.markCleared")),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.markUnclearedTitle")), OnClick(bulkMarkUncleared), uistate.T("transactions.markUncleared")),
					Button(css.Class("btn-del"), Type("button"), Title(uistate.T("transactions.deleteSelectedTitle")), OnClick(bulkDelete), uistate.T("transactions.deleteSelected")),
					Button(css.Class("btn"), Type("button"), OnClick(clearSelection), uistate.T("transactions.clearSelection")),
				)),
				If(len(lastBulk.Get().Prior) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), Style(map[string]string{"margin-bottom": "0.6rem"}),
					Span(css.Class("muted"), uistate.T("transactions.bulkUndoBanner", lastBulk.Get().Label)),
					Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.undoTitle")), Title(uistate.T("transactions.undoTitle")), OnClick(undoLastBulk), uistate.T("transactions.undoButton")),
				)),
				// Summary + select-all on one line (G2 §7): the select-all button used to
				// sit orphaned above the table, costing ~40px of vertical space; it now
				// rides alongside the count/net summary.
				If(len(shown) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"margin-bottom": "0.4rem"}),
					Span(css.Class("muted"), Attr("aria-hidden", "true"), Text(uistate.T("transactions.summary", plural(len(shown), "transaction"), fmtMoney(money.New(shownNet, base))))),
					If(unclearedCount > 0, Span(css.Class("muted"), Attr("aria-hidden", "true"), Text(uistate.T("transactions.summaryUncleared", unclearedCount)))),
					Button(css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("transactions.selectAllTitle")), Title(uistate.T("transactions.selectAllTitle")), OnClick(selectAllFiltered), uistate.T("transactions.selectAllFiltered")),
				)),
				// Screen-reader live region announcing the match count as filters change
				// (stays mounted across renders, so the zero-results case is announced too).
				P(css.Class(tw.SrOnly), Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-atomic", "true"), Text(filterStatus)),
				If(dupCount > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"margin-bottom": "0.6rem"}),
					Span(css.Class("muted"), uistate.T("transactions.dupNotice", plural(dupCount, "possible duplicate"))),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("transactions.selectDuplicatesTitle")), OnClick(selectDuplicates), uistate.T("transactions.selectDuplicates")),
				)),
				listBody,
			),
		}),
	)
}
