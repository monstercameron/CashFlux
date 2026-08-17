// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// C676: the bulk transfer classification workflow. The planner was already
// tested; these mount the surface that drives it.

var txrDay = time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)

// txrApp seeds a household with two savings sweeps that name their destination
// and one row nothing can route — the shape a statement import actually produces.
func txrApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	for _, acc := range []domain.Account{
		{ID: "a-check", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", BalanceAsOf: time.Now()},
		{ID: "a-save", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", BalanceAsOf: time.Now()},
	} {
		if err := app.PutAccount(acc); err != nil {
			t.Fatalf("PutAccount %s: %v", acc.ID, err)
		}
	}
	rows := []domain.Transaction{
		{ID: "t1", AccountID: "a-check", Desc: "TRANSFER TO SAVINGS", Date: txrDay, Amount: money.New(-50000, "USD")},
		{ID: "t2", AccountID: "a-check", Desc: "TRANSFER TO SAVINGS", Date: txrDay.AddDate(0, 1, 0), Amount: money.New(-25000, "USD")},
		{ID: "t3", AccountID: "a-check", Desc: "ONLINE TRANSFER", Date: txrDay, Amount: money.New(-10000, "USD")},
	}
	for _, tx := range rows {
		if err := app.PutTransaction(tx); err != nil {
			t.Fatalf("PutTransaction %s: %v", tx.ID, err)
		}
	}
	prev := appstate.Default
	appstate.Default = app
	uistate.BumpDataRevision()
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

// The panel groups a column of rows into decisions, and the pile it could not
// route comes last rather than leading.
func TestTransferReviewGroupsRowsIntoDecisions(t *testing.T) {
	txrApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	if byTestID(f, "div", "txnreview-batch-a-save") == nil {
		t.Fatal("the two savings sweeps did not group into one decision")
	}
	if byTestID(f, "div", "txnreview-batch-unrouted") == nil {
		t.Fatal("the unroutable row is missing — refusing to guess must still show the row")
	}
	// The grouping says WHY, so a user confirming it knows how much is evidence.
	basis := byTestID(f, "p", "txnreview-basis-a-save")
	if basis == nil || basis.Text() == "" {
		t.Fatal("the savings batch does not say why it was grouped")
	}
	if want := uistate.T(transferBasisKey(txnclassify.BasisName)); basis.Text() != want {
		t.Errorf("basis line = %q, want %q", basis.Text(), want)
	}
}

// Nothing is pre-selected, and nothing can be applied until something is: a bulk
// write defaulted to "yes" would make the confirmation a formality.
func TestTransferReviewSelectsNothingByDefault(t *testing.T) {
	txrApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	apply := byTestID(f, "button", "txnreview-apply")
	if apply == nil {
		t.Fatal("no apply control rendered")
	}
	if !isDisabled(apply) {
		t.Error("apply is live before anything is selected")
	}
	if sel := byTestID(f, "input", "txnreview-select-a-save"); sel == nil {
		t.Fatal("no selection control for the savings batch")
	} else if v, ok := sel.Property("checked").(bool); ok && v {
		t.Error("a batch arrived pre-selected")
	}
	if byTestID(f, "span", "txnreview-summary") != nil {
		t.Error("a consequence is claimed with nothing selected")
	}
}

// Selecting a batch states the whole consequence before the button, and applying
// it writes exactly those rows.
func TestTransferReviewStatesConsequenceThenApplies(t *testing.T) {
	app := txrApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	sel := byTestID(f, "input", "txnreview-select-a-save")
	if sel == nil {
		t.Fatal("no selection control for the savings batch")
	}
	sel.Change("on")
	f.Stabilize()

	summary := byTestID(f, "span", "txnreview-summary")
	if summary == nil || summary.Text() == "" {
		t.Fatal("selecting a batch stated no consequence")
	}
	// $500 + $250 leaves the totals; the figure has to be there before the click.
	if !strings.Contains(summary.Text(), fmtMoney(money.New(75000, "USD"))) {
		t.Errorf("summary %q does not name what stops counting", summary.Text())
	}
	apply := byTestID(f, "button", "txnreview-apply")
	if isDisabled(apply) {
		t.Fatal("apply stayed disabled with a batch selected")
	}

	var form *render.QueryNode
	for _, n := range f.AllByTag("form") {
		if n.Attr("data-testid") == "txnreview-form" {
			form = n
		}
	}
	if form == nil {
		t.Fatal("the panel is not a form, so Enter cannot commit it")
	}
	form.Submit()
	f.Stabilize()

	linked, untouched := 0, 0
	for _, got := range app.Transactions() {
		switch got.ID {
		case "t1", "t2":
			if got.TransferAccountID != "a-save" {
				t.Errorf("row %s points at %q, want a-save", got.ID, got.TransferAccountID)
			}
			linked++
		case "t3":
			if got.IsTransfer() {
				t.Error("the unselected, unrouted row was written anyway")
			}
			untouched++
		}
	}
	if linked != 2 || untouched != 1 {
		t.Errorf("linked %d rows and left %d, want 2 and 1", linked, untouched)
	}
}

// The unrouted pile cannot be applied until a person places it — that is the
// whole point of refusing to guess.
func TestTransferReviewCannotApplyTheUnroutedPileUntilPlaced(t *testing.T) {
	txrApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	sel := byTestID(f, "input", "txnreview-select-unrouted")
	if sel == nil {
		t.Fatal("the unrouted pile offers no selection control")
	}
	sel.Change("on")
	f.Stabilize()

	if !isDisabled(byTestID(f, "button", "txnreview-apply")) {
		t.Error("apply went live for a batch with no counterparty chosen")
	}

	// Choosing an account for it makes it appliable.
	picker := byTestID(f, "select", "txnreview-account-unrouted")
	if picker == nil {
		t.Fatal("the unrouted pile offers no account picker")
	}
	picker.Change("a-save")
	f.Stabilize()
	if isDisabled(byTestID(f, "button", "txnreview-apply")) {
		t.Error("apply stayed disabled after the pile was placed by hand")
	}
}

// The debt claim is offered only where it means something. A savings sweep
// cannot be a payment toward a balance, and showing the option there would
// invite a claim the classifier then ignores.
func TestTransferReviewOffersTheDebtClaimOnlyForLiabilities(t *testing.T) {
	app := txrApp(t)
	if err := app.PutAccount(domain.Account{
		ID: "a-visa", Name: "Visa", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", BalanceAsOf: time.Now(),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "c-card", Name: "Credit card payment", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "t-card", AccountID: "a-check", Desc: "AUTOPAY", CategoryID: "c-card", Date: txrDay,
		Amount: money.New(-20000, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	uistate.BumpDataRevision()

	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	if byTestID(f, "input", "txnreview-debt-a-save") != nil {
		t.Error("the debt claim is offered for a savings batch, where it means nothing")
	}
	// The card row routes to the one liability account, which CAN take the claim.
	if byTestID(f, "input", "txnreview-debt-a-visa") == nil {
		t.Error("the debt claim is missing on the liability batch, where it is the point")
	}
	// And it follows the CURRENT counterparty, not just the suggested one: placing
	// the unrouted pile on a liability by hand has to offer the same claim.
	if byTestID(f, "input", "txnreview-debt-unrouted") != nil {
		t.Fatal("the unrouted pile offered a debt claim before it was placed anywhere")
	}
	picker := byTestID(f, "select", "txnreview-account-unrouted")
	if picker == nil {
		t.Fatal("the unrouted pile offers no account picker")
	}
	picker.Change("a-visa")
	f.Stabilize()
	if byTestID(f, "input", "txnreview-debt-unrouted") == nil {
		t.Error("placing a batch on a liability by hand did not offer the debt claim")
	}
}

// An empty state is a statement about the data, not a blank panel.
func TestTransferReviewEmptyState(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	prev := appstate.Default
	appstate.Default = app
	uistate.BumpDataRevision()
	t.Cleanup(func() { appstate.Default = prev })

	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))
	if node := byTestID(f, "p", "txnreview-empty"); node == nil || node.Text() == "" {
		t.Fatal("an empty queue says nothing")
	}
	if byTestID(f, "button", "txnreview-apply") != nil {
		t.Error("an apply control is offered with nothing to apply")
	}
}

// The toolbar badge counts the whole ledger, not the filtered view: a count that
// shrank when you typed in the search box would be describing the search.
func TestTransferReviewBadgeCountsTheWholeLedger(t *testing.T) {
	app := txrApp(t)
	if got := transferSuspectCount(app); got != 3 {
		t.Errorf("suspect count = %d, want 3", got)
	}
	if label := transferReviewBtnLabel(3); !strings.Contains(label, "3") {
		t.Errorf("badge label = %q, want it to carry the count", label)
	}
	if label := transferReviewBtnLabel(0); strings.Contains(label, "0") {
		t.Errorf("badge label with nothing to review = %q, want no count", label)
	}
	if transferSuspectCount(nil) != 0 {
		t.Error("a nil app must count zero rather than panic")
	}
}

// Review finding 4: batches are recomputed from live data on every render, so a
// tick keyed on the counterparty alone would carry onto a different set of rows
// if the ledger moved — writing rows nobody had seen. The key carries the
// membership, so the tick falls off instead, which is the safe direction to fail.
func TestTransferReviewSelectionDoesNotSurviveAMembershipChange(t *testing.T) {
	app := txrApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	sel := byTestID(f, "input", "txnreview-select-a-save")
	if sel == nil {
		t.Fatal("no selection control for the savings batch")
	}
	sel.Change("on")
	f.Stabilize()
	if isDisabled(byTestID(f, "button", "txnreview-apply")) {
		t.Fatal("apply did not go live after selecting")
	}

	// A third savings sweep arrives while the panel is open.
	if err := app.PutTransaction(domain.Transaction{
		ID: "t-new", AccountID: "a-check", Desc: "TRANSFER TO SAVINGS",
		Date: txrDay.AddDate(0, 2, 0), Amount: money.New(-9900, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()

	if !isDisabled(byTestID(f, "button", "txnreview-apply")) {
		t.Error("a row joined a ticked batch and stayed appliable — it would be written unseen")
	}
	if byTestID(f, "span", "txnreview-summary") != nil {
		t.Error("a consequence is still claimed for a batch whose rows changed")
	}
	// The control is still there to be ticked again, under its stable test id.
	if byTestID(f, "input", "txnreview-select-a-save") == nil {
		t.Error("the batch's control churned its id when its membership changed")
	}
}

// Review finding 2: this app supports multi-currency households, and there is no
// rate table at this layer. Batches split by currency so each figure is money,
// and the footer refuses to sum across them.
func TestTransferReviewKeepsCurrenciesApart(t *testing.T) {
	app := txrApp(t)
	if err := app.PutAccount(domain.Account{
		ID: "a-eur", Name: "EU Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR", BalanceAsOf: time.Now(),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "t-eur", AccountID: "a-eur", Desc: "TRANSFER TO SAVINGS", Date: txrDay,
		Amount: money.New(-40000, "EUR"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	uistate.BumpDataRevision()

	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	// Two savings batches now exist, one per currency — and each states its own
	// money rather than a sum of unlike units.
	var metas []string
	for _, n := range f.AllByTag("span") {
		if strings.Contains(n.Attr("class"), "txnreview-batch-meta") {
			metas = append(metas, n.Text())
		}
	}
	joined := strings.Join(metas, " | ")
	if !strings.Contains(joined, fmtMoney(money.New(75000, "USD"))) {
		t.Errorf("no batch states the USD total; got %q", joined)
	}
	if !strings.Contains(joined, fmtMoney(money.New(40000, "EUR"))) {
		t.Errorf("no batch states the EUR total; got %q", joined)
	}
}

// And the footer, with two currencies selected, states the row count rather than
// a summed figure wearing one symbol.
func TestTransferReviewSummaryRefusesToSumAcrossCurrencies(t *testing.T) {
	one := map[string]bool{"USD": true}
	two := map[string]bool{"USD": true, "EUR": true}

	single := transferReviewSummary(3, 75000, one)
	if !strings.Contains(single, fmtMoney(money.New(75000, "USD"))) {
		t.Errorf("a single-currency selection should name the amount; got %q", single)
	}
	mixed := transferReviewSummary(4, 115000, two)
	if strings.Contains(mixed, fmtMoney(money.New(115000, "USD"))) {
		t.Errorf("a mixed-currency selection summed unlike units into one figure: %q", mixed)
	}
	if !strings.Contains(mixed, "2") {
		t.Errorf("the mixed-currency line should say how many currencies are in play; got %q", mixed)
	}
}

// Review finding 6: an uncertain pairing is said, not refused — the row did move
// to that account, and refusing would reject a correct answer over a fact this
// app never records.
func TestTransferReviewNotesAnAmbiguousLegWithoutRefusingIt(t *testing.T) {
	app := txrApp(t)
	// Two equally-good far sides for t1's $500 move.
	for _, id := range []string{"f1", "f2"} {
		if err := app.PutTransaction(domain.Transaction{
			ID: id, AccountID: "a-save", Desc: "DEPOSIT", Date: txrDay,
			Amount: money.New(50000, "USD"),
		}); err != nil {
			t.Fatalf("PutTransaction %s: %v", id, err)
		}
	}
	uistate.BumpDataRevision()

	f := render.New(t)
	f.Render(ui.CreateElement(TransferReviewBody, struct{}{}))

	note := byTestID(f, "span", "txnreview-rowambig-t1")
	if note == nil || note.Text() == "" {
		t.Fatal("two equally-good partners produced no note on the row")
	}
	// It is a note, not a refusal: the row is still appliable.
	if byTestID(f, "span", "txnreview-rowerr-t1") != nil {
		t.Error("an uncertain pairing was reported as a refusal — the classification is still correct")
	}
}
