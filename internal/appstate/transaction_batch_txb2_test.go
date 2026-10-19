// SPDX-License-Identifier: MIT

package appstate

import (
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func txb2Transfer(id, account, other, group, code string, amount int64, when time.Time) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: account, TransferAccountID: other, TransferGroupID: group,
		Date: when, Desc: "Transfer", Amount: money.New(amount, code)}
}

func TestDeleteTransactionWithTransferPairMatchesLegacyFXPair(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "bulk"}[bulk], func(t *testing.T) {
			a := newApp(t, false)
			if err := a.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 100.0 / 92.0}}); err != nil {
				t.Fatal(err)
			}
			when := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
			for _, txn := range []domain.Transaction{
				txb2Transfer("usd", "checking", "savings", "", "USD", -10000, when),
				txb2Transfer("eur", "savings", "checking", "", "EUR", 9200, when),
			} {
				if err := a.Store().PutTransaction(txn); err != nil {
					t.Fatal(err)
				}
			}
			if bulk {
				deleted, err := a.DeleteTransactionsBulk([]string{"usd"})
				if err != nil {
					t.Fatal(err)
				}
				if len(deleted) != 2 || deleted[0] != "usd" || deleted[1] != "eur" {
					t.Fatalf("deleted IDs = %v, want [usd eur]", deleted)
				}
			} else if err := a.DeleteTransactionWithTransferPair("usd"); err != nil {
				t.Fatal(err)
			}
			if got := a.Transactions(); len(got) != 0 {
				t.Fatalf("transactions left after deleting legacy FX pair: %+v", got)
			}
		})
	}
}

func TestDeleteTransactionsBulkReturnsOnlyDeletedIDs(t *testing.T) {
	a := newApp(t, false)
	txn := domain.Transaction{ID: "present", AccountID: "a", Date: time.Now(), Desc: "item", Amount: money.New(-100, "USD")}
	if err := a.Store().PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	deleted, err := a.DeleteTransactionsBulk([]string{"present", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != "present" {
		t.Fatalf("deleted IDs = %v, want [present]", deleted)
	}
}

func TestPutTransactionWithTransferPairWritesBothLegsAndNotifiesOnce(t *testing.T) {
	a := newApp(t, false)
	when := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	prior := txb2Transfer("out", "a", "b", "pair", "USD", -10000, when)
	other := txb2Transfer("in", "b", "a", "pair", "USD", 10000, when)
	for _, txn := range []domain.Transaction{prior, other} {
		if err := a.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	mutations := 0
	a.OnTxnMutated(func() { mutations++ })
	next := prior
	next.Amount = money.New(-12500, "USD")
	if err := a.PutTransactionWithTransferPair(prior, next); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]domain.Transaction)
	for _, txn := range a.Transactions() {
		byID[txn.ID] = txn
	}
	if byID["out"].Amount.Amount != -12500 || byID["in"].Amount.Amount != 12500 {
		t.Fatalf("pair amounts after edit = (%d, %d), want (-12500, 12500)", byID["out"].Amount.Amount, byID["in"].Amount.Amount)
	}
	if mutations != 1 {
		t.Fatalf("observer calls = %d, want 1", mutations)
	}
}

func TestExpandTransferPairsUsesOneIndexedSnapshot(t *testing.T) {
	when := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	const pairs = 3300
	all := make([]domain.Transaction, 0, pairs*2)
	ids := make([]string, 0, pairs)
	for i := 0; i < pairs; i++ {
		left, right := "left-"+strconv.Itoa(i), "right-"+strconv.Itoa(i)
		group := "group-" + strconv.Itoa(i)
		all = append(all, txb2Transfer(left, "a", "b", group, "USD", -int64(i+1), when),
			txb2Transfer(right, "b", "a", group, "USD", int64(i+1), when))
		ids = append(ids, left)
	}
	got := expandTransferPairs(all, ids, aTestRates())
	if len(got) != pairs*2 {
		t.Fatalf("expanded IDs = %d, want %d", len(got), pairs*2)
	}
}

func aTestRates() currency.Rates { return currency.Rates{Base: "USD"} }
