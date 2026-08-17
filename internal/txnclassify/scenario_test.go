// SPDX-License-Identifier: MIT

package txnclassify

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// This file works against a real imported statement rather than invented rows:
// a credit-union export where every line is a transfer and every line arrived as
// plain income or spending. It exists so the package is tested against the shape
// that motivated it, and so the headline figures are checked arithmetic rather
// than a claim.
//
// Two things about the export matter:
//
//   - The union exports its sub-accounts in one file, so BOTH legs of a
//     checking↔savings move land on checking. Nine such pairs are here. The leg
//     that does not belong is recognisable from its description naming checking
//     itself as the counterparty ("Transfer from Checking *8945").
//   - Everything else is one-sided: money to *1677 and *1958 whose far side was
//     never imported, plus one card row whose near side was not.

const (
	acctChecking = "sccu-checking"
	acctSavings  = "sccu-savings"
	acct1677     = "acct-1677"
	acct1958     = "acct-1958"
	acctCard     = "apple-card"
)

func statementAccounts() []domain.Account {
	return []domain.Account{
		{ID: acctChecking, Name: "SCCU Checkings", Class: domain.ClassAsset, Type: domain.TypeChecking,
			Currency: "USD", OpeningBalance: money.New(500000, "USD")},
		{ID: acctSavings, Name: "SCCU Savings", Class: domain.ClassAsset, Type: domain.TypeSavings,
			Currency: "USD", OpeningBalance: money.New(0, "USD")},
		{ID: acct1677, Name: "Account *1677", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			Currency: "USD", OpeningBalance: money.New(2000000, "USD")},
		{ID: acct1958, Name: "CNS *1958", Class: domain.ClassLiability, Type: domain.TypeLoan,
			Currency: "USD", OpeningBalance: money.New(1500000, "USD")},
		{ID: acctCard, Name: "Apple Credit Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			Currency: "USD", OpeningBalance: money.New(100000, "USD")},
	}
}

// row is one line of the imported statement, exactly as it arrived: no
// TransferAccountID, and a category the importer guessed.
type row struct {
	month, day  int
	amountMinor int64
	desc        string
	account     string
}

// statementRows is the export, in the order it was listed. Amounts are signed as
// the account saw them.
func statementRows() []row {
	return []row{
		{7, 14, -210276, "Transfer to Account Ending 1677", acctChecking},
		{7, 14, -75000, "Transfer to Account *1958", acctChecking},
		{7, 14, 75000, "Transfer from Savings *6500", acctChecking},
		{7, 14, -50000, "Transfer to Savings *6500", acctChecking},
		{7, 14, 50000, "Transfer from Checking *8945", acctChecking},
		{7, 14, -75000, "Transfer to Checking *8945", acctChecking},
		{7, 7, -51848, "ACH deposit — Internet transfer from account ending in 8945", acctCard},
		{7, 7, -100000, "Transfer to Account Ending 1677", acctChecking},
		{6, 30, -100000, "Transfer to account ending 1677", acctChecking},
		{6, 30, 100000, "Transfer from checking ending 8945 to savings", acctChecking},
		{6, 30, -100000, "Transfer to savings ending 6500", acctChecking},
		{6, 12, -50000, "Transfer to savings ending 6500", acctChecking},
		{6, 12, 50000, "Transfer from checking ending 8945 to savings", acctChecking},
		{6, 12, -75000, "Transfer to account ending 1958", acctChecking},
		{6, 12, -21954, "Transfer to account ending 1677", acctChecking},
		{6, 8, 25000, "Transfer from checking ending 8945 to savings", acctChecking},
		{6, 8, -25000, "Transfer to savings ending 6500", acctChecking},
		{6, 5, 25000, "Transfer from checking ending 8945 to savings", acctChecking},
		{6, 5, -238955, "Transfer to account ending 1677", acctChecking},
		{6, 5, -25000, "Transfer to savings ending 6500", acctChecking},
		{4, 17, 15000, "Transfer from Checking *8945", acctChecking},
		{4, 17, -15000, "Transfer to Savings *6500", acctChecking},
		{4, 14, 25000, "Transfer from Checking *8945", acctChecking},
		{4, 14, -25000, "Transfer to Savings *6500", acctChecking},
		{4, 14, -75000, "Transfer to CNS *1958", acctChecking},
		{4, 14, -44737, "Transfer to Account XXXXXXXXX1677", acctChecking},
		{4, 6, -61448, "Transfer to Account XXXXXXXXX1677", acctChecking},
		{4, 1, -80000, "Transfer to Savings *6500", acctChecking},
		{4, 1, 80000, "Transfer from Checking *8945", acctChecking},
		{4, 1, -139566, "Transfer to Account XXXXXXXXX1677", acctChecking},
		{3, 12, -75000, "Transfer to account ending 1958 — CNS", acctChecking},
		{3, 5, -94002, "Transfer to account ending 1677", acctChecking},
		{3, 1, -200000, "Transfer to account ending 1677", acctChecking},
		{2, 16, -50000, "Transfer to account ending 1677", acctChecking},
		{2, 13, -200000, "Transfer to account ending 1677", acctChecking},
		{2, 11, -200000, "Transfer to account ending 1677", acctChecking},
		{2, 10, -75000, "Transfer to account ending 1958", acctChecking},
		{1, 14, -75000, "Transfer to account ending 1958", acctChecking},
		{1, 14, -50000, "Transfer to account ending 1677", acctChecking},
		{1, 5, -150000, "Transfer to account ending 1677", acctChecking},
	}
}

func statementTxns() []domain.Transaction {
	rows := statementRows()
	out := make([]domain.Transaction, 0, len(rows))
	for i, r := range rows {
		out = append(out, domain.Transaction{
			ID:        string(rune('a'+i/26)) + string(rune('a'+i%26)),
			AccountID: r.account,
			Desc:      r.desc,
			Amount:    money.New(r.amountMinor, "USD"),
			Date:      time.Date(2026, time.Month(r.month), r.day, 0, 0, 0, 0, time.UTC),
			// The importer guessed a category. It is inert once the row is a
			// transfer, and it is left alone on purpose — see the package doc.
			CategoryID: "cat-transfers",
		})
	}
	return out
}

// counterpartyFor is the classification a person would make from each
// description: which of their accounts the money moved to or from.
func counterpartyFor(desc, ownAccount string) string {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "1677"):
		return acct1677
	case strings.Contains(d, "1958") || strings.Contains(d, "cns"):
		return acct1958
	case strings.Contains(d, "6500") || strings.Contains(d, "savings"):
		// A savings row on the savings account is its own far side; on checking
		// the counterparty is savings.
		if ownAccount == acctSavings {
			return acctChecking
		}
		return acctSavings
	case strings.Contains(d, "8945"):
		return acctChecking
	}
	return ""
}

func periodTotals(t *testing.T, txns []domain.Transaction) (income, expense int64) {
	t.Helper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	in, out, err := ledger.PeriodTotals(txns, start, end, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("PeriodTotals: %v", err)
	}
	return in.Amount, out.Amount
}

// The whole point, stated as a test: forty rows of pure account-movement are
// reported as $4,450.00 of income and $28,077.86 of spending until they are
// classified, and as nothing at all afterwards.
func TestImportedStatementReportsAsIncomeAndSpendingUntilClassified(t *testing.T) {
	txns := statementTxns()
	if len(txns) != 40 {
		t.Fatalf("fixture has %d rows, want the 40 from the export", len(txns))
	}

	income, expense := periodTotals(t, txns)
	if income != 445000 {
		t.Errorf("phantom income = %d, want 445000 ($4,450.00)", income)
	}
	if expense != 2807786 {
		t.Errorf("phantom spending = %d, want 2807786 ($28,077.86)", expense)
	}

	accs := statementAccounts()
	classified := make([]domain.Transaction, 0, len(txns))
	for _, x := range txns {
		cp := counterpartyFor(x.Desc, x.AccountID)
		if cp == "" {
			t.Fatalf("no counterparty parsed from %q", x.Desc)
		}
		// Rows naming their OWN account are the far leg, misfiled by the export.
		// Classification alone does not move them; that is a separate repair. What
		// it does do is stop them reading as income.
		if cp == x.AccountID {
			cp = acctSavings
		}
		out, err := Apply(x, cp, false, accs)
		if err != nil {
			t.Fatalf("Apply(%q): %v", x.Desc, err)
		}
		classified = append(classified, out)
	}

	income, expense = periodTotals(t, classified)
	if income != 0 {
		t.Errorf("income after classifying = %d, want 0", income)
	}
	if expense != 0 {
		t.Errorf("spending after classifying = %d, want 0", expense)
	}
}

// Classification is a reporting decision, not a money one: every account's booked
// balance must be identical before and after.
func TestClassifyingNeverMovesMoney(t *testing.T) {
	accs := statementAccounts()
	txns := statementTxns()

	before := map[string]int64{}
	for _, a := range accs {
		b, err := ledger.Balance(a, txns)
		if err != nil {
			t.Fatalf("Balance(%s): %v", a.Name, err)
		}
		before[a.ID] = b.Amount
	}

	classified := make([]domain.Transaction, 0, len(txns))
	for _, x := range txns {
		cp := counterpartyFor(x.Desc, x.AccountID)
		if cp == x.AccountID {
			cp = acctSavings
		}
		out, err := Apply(x, cp, false, accs)
		if err != nil {
			t.Fatalf("Apply(%q): %v", x.Desc, err)
		}
		classified = append(classified, out)
	}

	for _, a := range accs {
		b, err := ledger.Balance(a, classified)
		if err != nil {
			t.Fatalf("Balance(%s): %v", a.Name, err)
		}
		if b.Amount != before[a.ID] {
			t.Errorf("%s balance moved: %d -> %d", a.Name, before[a.ID], b.Amount)
		}
	}
}

// The two one-sided destinations, priced. These are the figures the review queue
// would show beside "which account is this?", so they are worth pinning.
func TestOneSidedDestinationTotals(t *testing.T) {
	var to1677, to1958 int64
	for _, x := range statementTxns() {
		switch counterpartyFor(x.Desc, x.AccountID) {
		case acct1677:
			to1677 += -x.Amount.Amount
		case acct1958:
			to1958 += -x.Amount.Amount
		}
	}
	if to1677 != 1860938 {
		t.Errorf("to *1677 = %d, want 1860938 ($18,609.38)", to1677)
	}
	if to1958 != 450000 {
		t.Errorf("to *1958 = %d, want 450000 ($4,500.00) — six months at $750", to1958)
	}
}

// Nine same-date mirror pairs sit on the checking account, both legs. They are
// what makes the misfiling visible, and the reason classification alone is not
// the whole repair.
func TestMisfiledLegsAreDetectableFromTheirDescription(t *testing.T) {
	own := map[string]string{acctChecking: "8945", acctSavings: "6500"}
	misfiled := 0
	var netOnChecking int64
	for _, x := range statementTxns() {
		mine, ok := own[x.AccountID]
		if !ok {
			continue
		}
		if strings.Contains(x.Desc, mine) {
			misfiled++
			netOnChecking += x.Amount.Amount
		}
	}
	if misfiled != 9 {
		t.Errorf("found %d rows naming their own account, want 9", misfiled)
	}
	// Eight incoming legs of $3,700 and one outgoing leg of $750 do not belong on
	// checking, so checking reads $2,950 richer than it is.
	if netOnChecking != 295000 {
		t.Errorf("misfiled net = %d, want 295000 ($2,950.00 overstated)", netOnChecking)
	}
}

// A debt payment classified toward a card must be claimable as THE payment, so
// the debt surfaces can read it; a savings sweep must not be.
func TestStatementDebtPaymentsCanBeClaimedAsPayments(t *testing.T) {
	accs := statementAccounts()
	var debtRows, neutralRows, selfNamed int
	for _, x := range statementTxns() {
		cp := counterpartyFor(x.Desc, x.AccountID)
		if cp == "" || cp == x.AccountID {
			// A row naming its own account is the misfiled far leg; it needs
			// reassigning before it can be classified against anything.
			selfNamed++
			continue
		}
		acc, ok := domain.AccountByID(accs, cp)
		if !ok {
			t.Fatalf("counterparty %q not in the account list", cp)
		}
		wantDebt := acc.Class == domain.ClassLiability
		out, err := Apply(x, cp, wantDebt, accs)
		if err != nil {
			t.Fatalf("Apply(%q): %v", x.Desc, err)
		}
		if wantDebt {
			debtRows++
			if out.BillAccountID != cp {
				t.Errorf("%q: BillAccountID = %q, want %q", x.Desc, out.BillAccountID, cp)
			}
			if KindOf(out, accs) != KindDebt {
				t.Errorf("%q: KindOf = %q, want debt", x.Desc, KindOf(out, accs))
			}
		} else {
			neutralRows++
			if out.BillAccountID != "" {
				t.Errorf("%q: a savings sweep claimed a debt payment", x.Desc)
			}
			if KindOf(out, accs) != KindNeutral {
				t.Errorf("%q: KindOf = %q, want neutral", x.Desc, KindOf(out, accs))
			}
		}
	}
	if debtRows != 21 {
		t.Errorf("classified %d debt rows, want 21 (15 to *1677, 6 to *1958)", debtRows)
	}
	if neutralRows != 14 {
		t.Errorf("classified %d neutral rows, want 14", neutralRows)
	}
	if selfNamed != 5 {
		t.Errorf("%d rows named their own account, want 5", selfNamed)
	}
	if debtRows+neutralRows+selfNamed != 40 {
		t.Errorf("rows accounted for = %d, want all 40", debtRows+neutralRows+selfNamed)
	}
}
