// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The transfer review panel (C676): resolve a statement's worth of mis-filed
// transfers in a handful of decisions rather than one per row.
//
// An import does not produce one unlinked transfer, it produces every savings
// sweep for a year. Asking "which account?" once per row turns a bad import into
// an afternoon, so txnclassify.Batches works out the probable counterparty and
// this panel offers one decision per batch.
//
// Two rules shape it. Nothing is pre-selected: a bulk write across a ledger is
// the widest action in the app, and defaulting it to "yes" would make the
// confirmation a formality. And every batch says WHY it was grouped — whether
// the far side was actually found or the app merely read the label — because a
// user confirming a grouping is entitled to know how much of it is evidence.

// transferReviewSelKey identifies a batch in the selection map: its counterparty,
// its currency, and the rows it actually holds.
//
// The membership is in the key deliberately. Batches are recomputed from live
// data on every render, so if the ledger changes while this panel is open — a
// sync, a recurring charge posting, another tab — the rows grouped under a
// counterparty can change underneath a box the user already ticked. Keying on the
// counterparty alone would carry that tick onto a different set of rows and write
// them without anyone having seen them, which is the exact thing "nothing is
// pre-selected" exists to prevent. With the membership in the key the tick simply
// falls off and has to be given again, which is the safe direction to fail.
func transferReviewSelKey(b txnclassify.Batch) string {
	key := b.CounterpartyID + "|" + b.Currency
	for _, r := range b.Rows {
		key += "|" + r.ID
	}
	return key
}

// transferReviewTestKey is the STABLE half of the key, for test ids and DOM keys.
// Those have to survive a membership change or every row in the panel remounts
// under the user's cursor whenever the ledger moves.
func transferReviewTestKey(b txnclassify.Batch) string {
	if b.CounterpartyID == "" {
		return "unrouted"
	}
	return b.CounterpartyID
}

// TransferReviewBody is the "Review transfers" panel, mounted at the shell root
// by app.TransferReviewHost.
func TransferReviewBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseTransferReviewOpen()

	// Per-batch state: which batches are selected, which counterparty each is
	// pointed at (the suggestion until the user changes it), and whether a
	// liability batch should count as a debt payment.
	selS := ui.UseState(map[string]bool{})
	overrideS := ui.UseState(map[string]string{})
	debtS := ui.UseState(map[string]bool{})
	errS := ui.UseState("")
	close := func() {
		selS.Set(map[string]bool{})
		overrideS.Set(map[string]string{})
		debtS.Set(map[string]bool{})
		errS.Set("")
		openAtom.Set(false)
	}
	onClose := ui.UseEvent(Prevent(func() { close() }))

	if app == nil {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	// Rows indexed by account, built once. AmbiguousLeg's first filter discards
	// everything outside the counterparty account, so handing it the whole ledger
	// per row would rescan thousands of rows to look at a handful — the same shape
	// as the ledger-search regression (C664), and it would run on every checkbox
	// click because the panel re-renders whole.
	byAccount := make(map[string][]domain.Transaction, len(accounts))
	for _, t := range txns {
		byAccount[t.AccountID] = append(byAccount[t.AccountID], t)
	}
	catName := func(id string) string { return budgetCategoryName(app, id) }
	batches := txnclassify.Batches(txnclassify.Suspects(txns, catName), catName, accounts, txns)

	// The counterparty a batch is currently pointed at: the user's override when
	// they have made one, otherwise the suggestion the grouping was built on.
	counterpartyOf := func(b txnclassify.Batch) string {
		if v, ok := overrideS.Get()[transferReviewSelKey(b)]; ok {
			return v
		}
		return b.CounterpartyID
	}

	// Plan every selected batch, so the footer can state the whole consequence
	// before anything is written and the rows that cannot take it can be named.
	var applied []domain.Transaction
	var failures []txnclassify.BulkFailure
	var leavesMinor int64
	debtRows, selectedRows := 0, 0
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	// The selected batches' currencies. A total is only money when there is ONE of
	// them: this package has no rate table, so adding ¥ to $ and stamping the
	// household's symbol on the result would print a figure that means nothing.
	// With several, the panel states the row count and leaves the amount to the
	// per-batch lines, which are each single-currency by construction.
	selCurrencies := map[string]bool{}
	for _, b := range batches {
		if !selS.Get()[transferReviewSelKey(b)] {
			continue
		}
		cp := counterpartyOf(b)
		if cp == "" {
			continue
		}
		plan := txnclassify.PlanBulk(b.Rows, cp, debtS.Get()[transferReviewSelKey(b)], accounts)
		applied = append(applied, plan.Applied...)
		failures = append(failures, plan.Failures...)
		leavesMinor += plan.LeavesTotalsMinor
		debtRows += plan.DebtRows
		selectedRows += len(b.Rows)
		selCurrencies[b.Currency] = true
	}
	canApply := len(applied) > 0

	apply := ui.UseEvent(Prevent(func() {
		if !canApply {
			return
		}
		n, err := app.ClassifyTransfers(applied)
		if err != nil {
			// The write is all-or-nothing, so the panel stays open on the numbers
			// the user was looking at rather than closing over a ledger that did
			// not change.
			errS.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostUndoable(uistate.T("txnreview.appliedToast", plural(n, "transfer")))
		close()
	}))

	// --- nothing to do ---------------------------------------------------------
	if len(batches) == 0 {
		return Div(css.Class("acct-edit-form"),
			Div(css.Class("modal-scroll"),
				P(css.Class("empty"), Attr("data-testid", "txnreview-empty"), uistate.T("txnreview.empty"))),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "txnreview-close"),
					OnClick(onClose), uistate.T("action.done"))),
		)
	}

	// Plain funcs passed to each batch component — never On* hooks inside a
	// variable-length loop (framework rule).
	toggleBatch := func(key string) {
		m := selS.Get()
		next := make(map[string]bool, len(m)+1)
		for k, v := range m {
			next[k] = v
		}
		next[key] = !next[key]
		selS.Set(next)
	}
	setCounterparty := func(key, accountID string) {
		m := overrideS.Get()
		next := make(map[string]string, len(m)+1)
		for k, v := range m {
			next[k] = v
		}
		next[key] = accountID
		overrideS.Set(next)
	}
	toggleDebt := func(key string) {
		m := debtS.Get()
		next := make(map[string]bool, len(m)+1)
		for k, v := range m {
			next[k] = v
		}
		next[key] = !next[key]
		debtS.Set(next)
	}

	failedIDs := map[string]string{}
	for _, f := range failures {
		if f.Err != nil {
			failedIDs[f.TxnID] = f.Err.Error()
		}
	}

	rows := MapKeyed(batches, func(b txnclassify.Batch) any { return transferReviewTestKey(b) },
		func(b txnclassify.Batch) ui.Node {
			key := transferReviewSelKey(b)
			return ui.CreateElement(transferReviewBatch, transferReviewBatchProps{
				Key:            key,
				TestKey:        transferReviewTestKey(b),
				Batch:          b,
				CounterpartyID: counterpartyOf(b),
				Selected:       selS.Get()[key],
				Debt:           debtS.Get()[key],
				Accounts:       accounts,
				Peers:          byAccount[counterpartyOf(b)],
				Base:           base,
				FailedIDs:      failedIDs,
				OnToggle:       toggleBatch,
				OnCounterparty: setCounterparty,
				OnToggleDebt:   toggleDebt,
			})
		})

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "txnreview-form"), OnSubmit(apply),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("txnreview.intro")),
			Div(css.Class("txnreview-batches"), rows),
			errText("txnreview-err", errS.Get()),
		),
		Div(css.Class("modal-foot"),
			// The whole consequence, stated before the button rather than after it.
			If(selectedRows > 0, Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "txnreview-summary"),
				transferReviewSummary(len(applied), leavesMinor, selCurrencies))),
			If(debtRows > 0, Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "txnreview-debt"),
				uistate.T("txnreview.debtNote", plural(debtRows, "row")))),
			If(len(failures) > 0, Span(css.Class("t-caption", tw.TextWarn), Attr("data-testid", "txnreview-failures"),
				uistate.T("txnreview.failures", plural(len(failures), "row")))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txnreview-cancel"),
				OnClick(onClose), uistate.T("action.cancel")),
			buttonWithDisabled(!canApply, []any{css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "txnreview-apply")}, uistate.T("txnreview.apply")),
		),
	)
}

// transferReviewBatchProps carries one batch plus plain callbacks. Never On*
// options — the row owns its own hooks (framework no-On*-in-loop rule).
type transferReviewBatchProps struct {
	// Key identifies the batch INCLUDING its row membership, so a selection cannot
	// survive the rows underneath it changing.
	Key string
	// TestKey is the stable half — counterparty only — for DOM keys and test ids,
	// which must not churn when the ledger moves.
	TestKey        string
	Batch          txnclassify.Batch
	CounterpartyID string
	Selected       bool
	Debt           bool
	Accounts       []domain.Account
	// Peers are the rows sitting in the counterparty account — the only ones that
	// can be a far side — for the per-row ambiguity read.
	Peers          []domain.Transaction
	Base           string
	FailedIDs      map[string]string
	OnToggle       func(string)
	OnCounterparty func(string, string)
	OnToggleDebt   func(string)
}

// transferReviewBatch renders one grouped decision: what the app thinks these
// rows are, why it thinks so, and the controls to confirm or redirect it.
func transferReviewBatch(props transferReviewBatchProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.Key) })
	toggleDebt := ui.UseEvent(func() { props.OnToggleDebt(props.Key) })
	b := props.Batch

	// The counterparty picker offers every account the rows could move to. Built
	// from the first row because a batch shares a source account in practice; a
	// row that cannot take the chosen account is reported by the plan rather than
	// hidden here, so the list stays the same for every row in the group.
	var options []uiw.SelectOption
	options = append(options, uiw.SelectOption{Value: "", Label: uistate.T("txnreview.pickAccount")})
	for _, acc := range props.Accounts {
		if acc.Archived {
			continue
		}
		if len(b.Rows) > 0 && acc.ID == b.Rows[0].AccountID {
			continue // a row cannot be a transfer to its own account
		}
		options = append(options, uiw.SelectOption{Value: acc.ID, Label: acc.Name})
	}

	isLiability := false
	if acc, ok := domain.AccountByID(props.Accounts, props.CounterpartyID); ok {
		isLiability = acc.Class == domain.ClassLiability
	}

	rowNodes := MapKeyed(b.Rows, func(t domain.Transaction) any { return t.ID },
		func(t domain.Transaction) ui.Node {
			why := props.FailedIDs[t.ID]
			// An ambiguous leg does not make the classification wrong — the row DID
			// move to that account — so it is said, not refused. Refusing would
			// reject a correct answer because a fact this app never records (which
			// row on the far side is the partner) is unknown.
			ambiguous := why == "" && props.CounterpartyID != "" &&
				txnclassify.AmbiguousLeg(t, props.CounterpartyID, props.Peers)
			return Div(ClassStr("txnreview-row"+failedClass(why != "")), Attr("data-testid", "txnreview-row-"+t.ID),
				Span(css.Class("txnreview-row-desc"), txnReviewRowLabel(t)),
				Span(css.Class("txnreview-row-amt", tw.TextDim), fmtMoney(t.Amount)),
				If(why != "", Span(css.Class("txnreview-row-err", tw.TextWarn),
					Attr("data-testid", "txnreview-rowerr-"+t.ID), why)),
				If(ambiguous, Span(css.Class("txnreview-row-note", tw.TextFaint),
					Attr("data-testid", "txnreview-rowambig-"+t.ID), uistate.T("txnreview.ambiguousLeg"))),
			)
		})

	return Div(css.Class("txnreview-batch"), Attr("data-testid", "txnreview-batch-"+props.TestKey),
		Label(css.Class("txnreview-batch-head"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"),
				Attr("data-testid", "txnreview-select-"+props.TestKey), OnChange(toggle)},
				checkedAttr(props.Selected)...)...),
			Span(css.Class("txnreview-batch-title"), transferBatchTitle(b, props.Accounts)),
			Span(css.Class("txnreview-batch-meta", tw.TextDim),
				uistate.T("txnreview.batchMeta", plural(b.Count(), "row"),
					fmtMoney(money.New(b.LeakedMinor, batchCurrency(b, props.Base))))),
		),
		// WHY these rows were grouped. A user confirming a grouping is entitled to
		// know whether the app found the far side or only read the label.
		P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "txnreview-basis-"+props.TestKey),
			Style(map[string]string{"margin": "0"}), uistate.T(transferBasisKey(b.Basis))),
		labeledField(uistate.T("txnreview.counterparty"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   options,
				Selected:  props.CounterpartyID,
				OnChange:  func(v string) { props.OnCounterparty(props.Key, v) },
				AriaLabel: uistate.T("txnreview.counterparty"),
				TestID:    "txnreview-account-" + props.TestKey,
			})),
		// The debt claim is offered only where it means something. Showing it for a
		// savings sweep would invite a claim the classifier would then ignore.
		If(isLiability, Label(css.Class("txnreview-debt-toggle"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"),
				Attr("data-testid", "txnreview-debt-"+props.TestKey), OnChange(toggleDebt)},
				checkedAttr(props.Debt)...)...),
			Span(uistate.T("txnreview.countTowardDebt")))),
		Div(css.Class("txnreview-rows"), rowNodes),
	)
}

// failedClass tones a row the plan refused.
func failedClass(failed bool) string {
	if failed {
		return " is-failed"
	}
	return ""
}

// txnReviewRowLabel is the row's own words, falling back to its date when the
// statement gave it none — an unlabelled row still has to be identifiable.
func txnReviewRowLabel(t domain.Transaction) string {
	if t.Desc != "" {
		return t.Desc
	}
	if t.Payee != "" {
		return t.Payee
	}
	return uistate.LoadPrefs().FormatDate(t.Date)
}

// transferBatchTitle names the account a batch would move to, or says plainly
// that the app could not work it out. "Unrouted" is not an error state — it is
// the residue of refusing to guess — so it reads as a question, not a failure.
func transferBatchTitle(b txnclassify.Batch, accounts []domain.Account) string {
	if acc, ok := domain.AccountByID(accounts, b.CounterpartyID); ok {
		return acc.Name
	}
	return uistate.T("txnreview.unrouted")
}

// transferBasisKey names the evidence behind a grouping.
func transferBasisKey(basis txnclassify.SuggestBasis) string {
	switch basis {
	case txnclassify.BasisReciprocal:
		return "txnreview.basisReciprocal"
	case txnclassify.BasisName:
		return "txnreview.basisName"
	case txnclassify.BasisCategory:
		return "txnreview.basisCategory"
	}
	return "txnreview.basisNone"
}

// transferReviewSummary states what the selected batches would do. It names an
// amount only when every selected batch shares one currency — this app supports
// multi-currency households, and there is no rate table at this layer, so a sum
// across currencies with a symbol on it would be a number that looks precise and
// is not. With several currencies it reports the row count and lets the per-batch
// lines, each single-currency by construction, carry the amounts.
func transferReviewSummary(rows int, leavesMinor int64, currencies map[string]bool) string {
	switch len(currencies) {
	case 0:
		// Not reachable from the panel (a selected batch always contributes its
		// currency), but a summary that said "across 0 currencies" would be worse
		// than one that simply counts rows.
		return uistate.T("txnreview.summaryRows", plural(rows, "row"))
	case 1:
		for cur := range currencies {
			return uistate.T("txnreview.summary", plural(rows, "row"), fmtMoney(money.New(leavesMinor, cur)))
		}
	}
	return uistate.T("txnreview.summaryMixed", plural(rows, "row"), strconv.Itoa(len(currencies)))
}

// batchCurrency is the currency a batch's figures are in — its own, always. The
// household base is only a fallback for an empty batch, which cannot render one.
func batchCurrency(b txnclassify.Batch, base string) string {
	if b.Currency != "" {
		return b.Currency
	}
	return base
}

// transferSuspectCount is how many rows currently look like unlinked transfers,
// for the toolbar badge. Computed over the WHOLE ledger, not the filtered view:
// the badge is a claim about the household's data, and one that shrank when you
// typed in the search box would be describing the search.
func transferSuspectCount(app *appstate.App) int {
	if app == nil {
		return 0
	}
	return len(txnclassify.Suspects(app.Transactions(), func(id string) string {
		return budgetCategoryName(app, id)
	}))
}

// transferReviewBtnLabel is the toolbar item's label, badged with the count when
// there is anything to review.
func transferReviewBtnLabel(count int) string {
	if count > 0 {
		return uistate.T("txnreview.reviewBadge", strconv.Itoa(count))
	}
	return uistate.T("txnreview.reviewBtn")
}
