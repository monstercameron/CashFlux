// SPDX-License-Identifier: MIT

package worksheet

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

var (
	julyStart = d(2026, time.July, 1)
	julyEnd   = d(2026, time.August, 1)
)

func chk(openingMinor int64, recs ...domain.Reconciliation) domain.Account {
	return domain.Account{
		ID: "chk", Name: "SCCU Checkings", Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: money.New(openingMinor, "USD"), Reconciliations: recs,
	}
}

func row(id string, minor int64, day int) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "chk", Desc: id,
		Amount: money.New(minor, "USD"), Date: d(2026, time.July, day)}
}

func statement(minor int64, day int) domain.Reconciliation {
	on := d(2026, time.July, day)
	return domain.Reconciliation{At: on, StatementDate: on, StatementBalance: money.New(minor, "USD")}
}

// C690's acceptance case: July checking, opening $3,210.20, closing $4,493.95.
// The worksheet's job is to show the arithmetic that gets from one to the other
// and land on zero left over.
func TestJulyCheckingAddsUp(t *testing.T) {
	acc := chk(321020, statement(449395, 31))
	txns := []domain.Transaction{
		row("salary", 500000, 3),
		row("rent", -220000, 5),
		row("groceries", -151625, 12),
	}

	r := Build([]domain.Account{acc}, txns, julyStart, julyEnd)
	l := r.Lines[0]

	if l.Opening != 321020 {
		t.Errorf("Opening = %d, want 321020", l.Opening)
	}
	if l.In != 500000 {
		t.Errorf("In = %d, want 500000", l.In)
	}
	if l.Out != 371625 {
		t.Errorf("Out = %d, want 371625", l.Out)
	}
	if l.Computed != 449395 {
		t.Errorf("Computed = %d, want the statement's 449395", l.Computed)
	}
	if !l.HasStatement || l.Residual != 0 {
		t.Errorf("Residual = %d (hasStatement=%v), want a clean zero", l.Residual, l.HasStatement)
	}
	if !l.Explained() {
		t.Errorf("Explained = false on a line that balances exactly")
	}
	if len(r.Unexplained()) != 0 {
		t.Errorf("Unexplained = %+v, want none", r.Unexplained())
	}
}

// The computed closing must equal what the rest of the app would say the balance
// is. A worksheet that disagrees with the account card is a third opinion, which
// is worse than the two that already disagreed.
func TestComputedClosingMatchesTheLedger(t *testing.T) {
	acc := chk(321020)
	txns := []domain.Transaction{
		row("salary", 500000, 3),
		row("rent", -220000, 5),
		row("sweep", -50000, 20),
	}
	txns[2].TransferAccountID = "sav"

	l := Build([]domain.Account{acc}, txns, julyStart, julyEnd).Lines[0]
	bal, err := ledger.Balance(acc, txns)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if l.Computed != bal.Amount {
		t.Errorf("worksheet says %d, the ledger says %d", l.Computed, bal.Amount)
	}
}

// Transfers are their own term. Folding them into income or spending is the
// mistake this whole line of work started from.
func TestTransfersAreTheirOwnTerm(t *testing.T) {
	acc := chk(100000)
	out := row("sweep-out", -50000, 10)
	out.TransferAccountID = "sav"
	in := row("sweep-in", 20000, 12)
	in.TransferAccountID = "sav"

	l := Build([]domain.Account{acc}, []domain.Transaction{out, in}, julyStart, julyEnd).Lines[0]
	if l.In != 0 || l.Out != 0 {
		t.Errorf("In/Out = %d/%d, want transfers kept out of both", l.In, l.Out)
	}
	if l.Transfers != -30000 {
		t.Errorf("Transfers = %d, want -30000 net", l.Transfers)
	}
	if l.Computed != 70000 {
		t.Errorf("Computed = %d, want 70000 — transfers still move the balance", l.Computed)
	}
}

// Checkpoints are their own term too: absent from every report, present in the
// balance, and therefore the usual answer to "why does my balance not match my
// spending".
func TestCheckpointsAreTheirOwnTerm(t *testing.T) {
	acc := chk(100000)
	cp, ok := provisional.New("cp", "chk", "Balance checkpoint", 0, 25000, "USD",
		d(2026, time.July, 20), d(2026, time.July, 20))
	if !ok {
		t.Fatal("fixture produced no checkpoint")
	}
	spend := row("groceries", -4000, 3)

	l := Build([]domain.Account{acc}, []domain.Transaction{cp, spend}, julyStart, julyEnd).Lines[0]
	if l.Checkpoints != 25000 {
		t.Errorf("Checkpoints = %d, want 25000", l.Checkpoints)
	}
	if l.In != 0 {
		t.Errorf("In = %d — a checkpoint was counted as income", l.In)
	}
	if l.Computed != 121000 {
		t.Errorf("Computed = %d, want 121000", l.Computed)
	}
}

// Everything before the period is opening balance, whatever it was; everything
// after is somebody else's period.
func TestOnlyThePeriodCountsAsMovement(t *testing.T) {
	acc := chk(0)
	txns := []domain.Transaction{
		{ID: "june", AccountID: "chk", Desc: "june", Amount: money.New(10000, "USD"), Date: d(2026, time.June, 20)},
		row("july", 5000, 10),
		{ID: "aug", AccountID: "chk", Desc: "aug", Amount: money.New(99999, "USD"), Date: d(2026, time.August, 2)},
	}
	l := Build([]domain.Account{acc}, txns, julyStart, julyEnd).Lines[0]
	if l.Opening != 10000 {
		t.Errorf("Opening = %d, want June folded into it", l.Opening)
	}
	if l.In != 5000 {
		t.Errorf("In = %d, want only July's movement", l.In)
	}
	if l.Computed != 15000 {
		t.Errorf("Computed = %d, want August left out", l.Computed)
	}
}

// A gap the ledger cannot explain is stated once, as a number to go looking for,
// rather than implied by two figures that disagree in different places.
func TestAResidualIsStatedOnce(t *testing.T) {
	acc := chk(100000, statement(150000, 31))
	l := Build([]domain.Account{acc}, []domain.Transaction{row("pay", 40000, 4)}, julyStart, julyEnd).Lines[0]

	if l.Computed != 140000 {
		t.Errorf("Computed = %d, want 140000", l.Computed)
	}
	if l.Residual != 10000 {
		t.Errorf("Residual = %d, want the missing 10000", l.Residual)
	}
	if l.Explained() {
		t.Errorf("Explained = true with $100 unaccounted for")
	}
}

// No statement covering the period means there is nothing to disagree with, and
// a residual of zero would be a claim rather than an absence.
func TestNoStatementMeansNoResidualClaim(t *testing.T) {
	acc := chk(100000, statement(999999, 31))
	// The statement is July's; the worksheet is August's.
	l := Build([]domain.Account{acc}, nil, d(2026, time.August, 1), d(2026, time.September, 1)).Lines[0]

	if l.HasStatement {
		t.Errorf("HasStatement = true for a statement outside the period")
	}
	if l.Residual != 0 {
		t.Errorf("Residual = %d, want 0 when there is nothing to compare against", l.Residual)
	}
	if !l.Explained() {
		t.Errorf("Explained = false where no statement disagrees")
	}
}

// A period can hold two statements — a card closing mid-month and again at month
// end. The one that closes the period is the one to check the closing figure
// against.
func TestTheLastStatementInThePeriodWins(t *testing.T) {
	acc := chk(0, statement(11111, 10), statement(22222, 28))
	l := Build([]domain.Account{acc}, nil, julyStart, julyEnd).Lines[0]
	if l.Statement != 22222 {
		t.Errorf("Statement = %d, want the later one", l.Statement)
	}
}

// A debt's line reads the way the account card and the reconcile dialog read it,
// whichever sign the account happens to store — otherwise the worksheet is a
// fourth convention in an app that already had two.
func TestALiabilityLineReadsInTheStatedConvention(t *testing.T) {
	for _, openingStored := range []int64{50000, -50000} {
		card := domain.Account{
			ID: "chk", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			Currency: "USD", OpeningBalance: money.New(openingStored, "USD"),
		}
		// A $200 purchase, in whichever sign this account stores debt.
		purchase := row("purchase", 20000, 5)
		if openingStored < 0 {
			purchase.Amount = money.New(-20000, "USD")
		}

		l := Build([]domain.Account{card}, []domain.Transaction{purchase}, julyStart, julyEnd).Lines[0]
		if l.Opening != -50000 {
			t.Errorf("opening stored %d: Opening = %d, want -50000 owed", openingStored, l.Opening)
		}
		if l.Computed != -70000 {
			t.Errorf("opening stored %d: Computed = %d, want -70000 owed", openingStored, l.Computed)
		}
	}
}

func TestAccountsWithNoActivityStillGetALine(t *testing.T) {
	acc := chk(321020)
	r := Build([]domain.Account{acc}, nil, julyStart, julyEnd)
	if len(r.Lines) != 1 {
		t.Fatalf("got %d lines, want one per account", len(r.Lines))
	}
	if r.Lines[0].Computed != 321020 {
		t.Errorf("Computed = %d, want the opening balance unchanged", r.Lines[0].Computed)
	}
}

// One account's transactions must never leak into another's line.
func TestAccountsAreKeptApart(t *testing.T) {
	a := chk(0)
	b := domain.Account{ID: "sav", Name: "Savings", Class: domain.ClassAsset,
		Type: domain.TypeSavings, Currency: "USD", OpeningBalance: money.New(0, "USD")}
	txns := []domain.Transaction{
		row("chk-pay", 10000, 3),
		{ID: "sav-int", AccountID: "sav", Desc: "interest", Amount: money.New(500, "USD"), Date: d(2026, time.July, 9)},
	}
	r := Build([]domain.Account{a, b}, txns, julyStart, julyEnd)
	if r.Lines[0].In != 10000 || r.Lines[1].In != 500 {
		t.Errorf("in = %d / %d, want 10000 and 500", r.Lines[0].In, r.Lines[1].In)
	}
}

// Found by an adversarial pass: Build only finds a statement already recorded in
// the account's history, but the statement a person is TYPING into the reconcile
// dialog is not recorded until they press Record. So the residual — the whole
// reason to show a worksheet during a reconciliation — could not appear until
// after the reconciliation was over.
func TestTheResidualWorksAgainstAStatementBeingTyped(t *testing.T) {
	// Last reconciled through June 30; the user is now typing July's statement.
	acc := chk(321020, domain.Reconciliation{
		At: d(2026, time.June, 30), StatementDate: d(2026, time.June, 30),
		StatementBalance: money.New(321020, "USD"),
	})
	txns := []domain.Transaction{row("pay", 40000, 4)}

	l := Build([]domain.Account{acc}, txns, julyStart, julyEnd).Lines[0]
	if l.HasStatement {
		t.Fatalf("fixture should have no July statement recorded yet")
	}
	if l.Computed != 361020 {
		t.Fatalf("Computed = %d, want 361020", l.Computed)
	}

	// The dialog hands over what is being typed: $3,640.20, a $30 gap.
	got := l.AgainstStatement(364020)
	if !got.HasStatement {
		t.Errorf("HasStatement = false after being handed a statement")
	}
	if got.Residual != 3000 {
		t.Errorf("Residual = %d, want the $30 gap", got.Residual)
	}
	if got.Explained() {
		t.Errorf("Explained = true with $30 unaccounted for")
	}
	// And an exact match reconciles.
	if exact := l.AgainstStatement(361020); !exact.Explained() || exact.Residual != 0 {
		t.Errorf("an exact statement did not reconcile: %+v", exact)
	}
}
