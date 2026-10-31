// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestExpectedUpdateValidatesFullRowAndProtectsPreview(t *testing.T) {
	a, _, category := creationFixture(t)
	row := creationTxn("edit")
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	row = a.Transactions()[0]
	next := row
	next.Desc = "Updated"
	next.CategoryID = category.ID
	p, err := a.PreviewTransactionUpdate(row, next)
	if err != nil {
		t.Fatal(err)
	}
	forged := p
	forged.After = cloneTransactions(p.After)
	forged.After[0].Amount = money.New(999, "USD")
	if err := a.UpdateTransactionExpected(forged); err == nil {
		t.Fatal("modified public preview accepted")
	}
	if err := a.UpdateTransactionExpected(p); err != nil {
		t.Fatal(err)
	}
	if got := a.Transactions()[0]; got.Desc != "Updated" || got.CategoryID != category.ID || got.Amount != row.Amount {
		t.Fatalf("wrong edit: %+v", got)
	}
	bad := gotTransaction(t, a, row.ID)
	bad.Desc = ""
	if _, err := a.PreviewTransactionUpdate(gotTransaction(t, a, row.ID), bad); err == nil {
		t.Fatal("blank required description previewed")
	}
}
func gotTransaction(t *testing.T, a *App, id string) domain.Transaction {
	t.Helper()
	row, found, err := a.store.GetTransaction(id)
	if err != nil || !found {
		t.Fatalf("missing row %s: %v", id, err)
	}
	return row
}

func TestExpectedUpdateRejectsEveryChangedReference(t *testing.T) {
	for _, kind := range []string{"transaction", "account", "category", "member"} {
		t.Run(kind, func(t *testing.T) {
			a, account, category := creationFixture(t)
			member := domain.Member{ID: "owner", Name: "Owner"}
			if err := a.PutMember(member); err != nil {
				t.Fatal(err)
			}
			row := creationTxn("edit")
			row.CategoryID = category.ID
			row.SetOwners([]string{member.ID})
			if err := a.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			row = gotTransaction(t, a, row.ID)
			next := row
			next.Payee = "New payee"
			p, err := a.PreviewTransactionUpdate(row, next)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "transaction":
				row.Desc = "Changed"
				err = a.PutTransaction(row)
			case "account":
				account.Name = "Changed"
				err = a.PutAccount(account)
			case "category":
				category.Name = "Changed"
				err = a.PutCategory(category)
			case "member":
				member.Name = "Changed"
				err = a.PutMember(member)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, revision := a.Transactions(), a.Rev()
			if err := a.UpdateTransactionExpected(p); err == nil || a.Rev() != revision || !reflect.DeepEqual(a.Transactions(), before) {
				t.Fatalf("stale %s overwrote live data: %v", kind, err)
			}
		})
	}
}

func TestExpectedUpdateTransferUsesSharedPlanAndGuardsFee(t *testing.T) {
	for _, staleFee := range []bool{false, true} {
		t.Run(map[bool]string{false: "pair commit", true: "fee changed"}[staleFee], func(t *testing.T) {
			a, account, _ := creationFixture(t)
			other := account
			other.ID = "other"
			other.Name = "Other"
			if err := a.PutAccount(other); err != nil {
				t.Fatal(err)
			}
			out, in, err := a.CreateTransferPair(TransferParams{FromAccountID: account.ID, ToAccountID: other.ID, AmountMinor: 1000, FeeMinor: 10, Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			row := gotTransaction(t, a, out)
			next := row
			next.Amount = money.New(-2000, "USD")
			next.Date = next.Date.AddDate(0, 0, 1)
			p, err := a.PreviewTransactionUpdate(row, next)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Before) != 3 || len(p.After) != 3 {
				t.Fatalf("fee missing from full preview: before=%d after=%d", len(p.Before), len(p.After))
			}
			feeID := ""
			for _, tx := range p.Before {
				if tx.ID != out && tx.ID != in {
					feeID = tx.ID
				}
			}
			if staleFee {
				fee := gotTransaction(t, a, feeID)
				fee.Desc = "Changed fee"
				if err := a.PutTransaction(fee); err != nil {
					t.Fatal(err)
				}
			}
			before, revision := a.Transactions(), a.Rev()
			err = a.UpdateTransactionExpected(p)
			if staleFee {
				if err == nil || a.Rev() != revision || !reflect.DeepEqual(a.Transactions(), before) {
					t.Fatalf("fee edit permitted partial pair write: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotTransaction(t, a, out).Amount.Amount != -2000 || gotTransaction(t, a, in).Amount.Amount != 2000 || gotTransaction(t, a, feeID).Amount.Amount != -10 || !gotTransaction(t, a, in).Date.Equal(next.Date) {
				t.Fatal("shared transfer plan broke pair/fee")
			}
		})
	}
}

func TestExpectedUpdateSplitAndUnknownFXAreUnavailableBeforeCommit(t *testing.T) {
	a, account, category := creationFixture(t)
	row := creationTxn("split")
	row.Splits = []domain.CategorySplit{{CategoryID: category.ID, Amount: row.Amount}}
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	row = gotTransaction(t, a, row.ID)
	next := row
	next.Amount = money.New(-200, "USD")
	if _, err := a.PreviewTransactionUpdate(row, next); err == nil {
		t.Fatal("nonreconciling split amount previewed")
	}
	eur := account
	eur.ID = "eur"
	eur.Name = "Euro"
	eur.Currency = "EUR"
	eur.OpeningBalance = money.Zero("EUR")
	if err := a.PutAccount(eur); err != nil {
		t.Fatal(err)
	}
	out, _, err := a.CreateTransferPair(TransferParams{FromAccountID: eur.ID, ToAccountID: account.ID, AmountMinor: 100, ReceivedMinor: 100, Date: row.Date})
	if err != nil {
		t.Fatal(err)
	}
	row = gotTransaction(t, a, out)
	next = row
	next.Amount = money.New(-200, "EUR")
	if _, err := a.PreviewTransactionUpdate(row, next); err == nil || !strings.Contains(err.Error(), "rate") {
		t.Fatalf("unknown FX previewed: %v", err)
	}
}

func TestExpectedUpdateRejectsLedgerChangesAndCheckedOverflow(t *testing.T) {
	for _, kind := range []string{"changed ledger", "overflow"} {
		t.Run(kind, func(t *testing.T) {
			a, account, _ := creationFixture(t)
			row := creationTxn("edit")
			if err := a.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			row = gotTransaction(t, a, row.ID)
			next := row
			next.Amount = money.New(-200, "USD")
			if kind == "overflow" {
				account.OpeningBalance = money.New(-9223372036854775700, "USD")
				if err := a.PutAccount(account); err != nil {
					t.Fatal(err)
				}
				if _, err := a.PreviewTransactionUpdate(row, next); err == nil {
					t.Fatal("overflowing prospective balance previewed")
				}
				return
			}
			p, err := a.PreviewTransactionUpdate(row, next)
			if err != nil {
				t.Fatal(err)
			}
			extra := creationTxn("new-ledger-row")
			if err := a.PutTransaction(extra); err != nil {
				t.Fatal(err)
			}
			before, rev := a.Transactions(), a.Rev()
			if err := a.UpdateTransactionExpected(p); err == nil || a.Rev() != rev || !reflect.DeepEqual(before, a.Transactions()) {
				t.Fatalf("changed ledger committed: %v", err)
			}
		})
	}
}

func TestExpectedUpdateGuardsCanonicalFeeAbsenceAndAddedGroupRows(t *testing.T) {
	for _, kind := range []string{"new fee", "extra leg"} {
		t.Run(kind, func(t *testing.T) {
			a, account, _ := creationFixture(t)
			other := account
			other.ID = "other"
			other.Name = "Other"
			if err := a.PutAccount(other); err != nil {
				t.Fatal(err)
			}
			out, _, err := a.CreateTransferPair(TransferParams{OperationID: "no-fee", FromAccountID: account.ID, ToAccountID: other.ID, AmountMinor: 100, Date: time.Now()})
			if err != nil {
				t.Fatal(err)
			}
			row := gotTransaction(t, a, out)
			next := row
			next.Desc = "Updated"
			p, err := a.PreviewTransactionUpdate(row, next)
			if err != nil {
				t.Fatal(err)
			}
			extra := creationTxn(transferID("no-fee", "fee"))
			if kind == "extra leg" {
				extra.ID = "extra"
				extra.TransferGroupID = row.TransferGroupID
				extra.TransferAccountID = other.ID
			}
			if err := a.PutTransaction(extra); err != nil {
				t.Fatal(err)
			}
			before, rev := a.Transactions(), a.Rev()
			if err := a.UpdateTransactionExpected(p); err == nil || a.Rev() != rev || !reflect.DeepEqual(before, a.Transactions()) {
				t.Fatalf("new related row accepted: %v", err)
			}
		})
	}
}

func TestExpectedUpdateGuardsSplitCategoryReference(t *testing.T) {
	a, _, category := creationFixture(t)
	row := creationTxn("split-refs")
	row.Splits = []domain.CategorySplit{{CategoryID: category.ID, Amount: row.Amount}}
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	row = gotTransaction(t, a, row.ID)
	next := row
	next.Desc = "Reviewed"
	p, err := a.PreviewTransactionUpdate(row, next)
	if err != nil {
		t.Fatal(err)
	}
	category.Name = "Changed split category"
	if err := a.PutCategory(category); err != nil {
		t.Fatal(err)
	}
	before, rev := a.Transactions(), a.Rev()
	if err := a.UpdateTransactionExpected(p); err == nil || a.Rev() != rev || !reflect.DeepEqual(before, a.Transactions()) {
		t.Fatalf("split ref changed: %v", err)
	}
}
