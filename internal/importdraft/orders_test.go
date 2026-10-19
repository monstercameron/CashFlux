// SPDX-License-Identifier: MIT

package importdraft

import (
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
	"github.com/monstercameron/CashFlux/internal/ordermatch"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func TestOrderLedgerAdapters(t *testing.T) {
	txns := []domain.Transaction{{ID: "expense", Amount: money.New(-1, "USD")}, {ID: "income", Amount: money.New(1, "EUR")}, {ID: "zero"}, {ID: "transfer", Amount: money.New(-1, "USD"), TransferAccountID: "other"}}
	if tx, ok := FindTransaction(txns, "income"); !ok || tx.ID != "income" {
		t.Fatal(tx, ok)
	}
	if _, ok := FindTransaction(nil, "missing"); ok {
		t.Fatal("missing")
	}
	if got := OrderCharges(txns); len(got) != 1 || got[0].TxnID != "expense" || got[0].Currency != "USD" || got[0].AmountMinor != -1 {
		t.Fatal(got)
	}
	if OrderCharges(nil) != nil {
		t.Fatal("nil")
	}
	for _, tc := range []struct {
		txn  domain.Transaction
		want string
	}{{domain.Transaction{Payee: "payee", Desc: "desc", ID: "id"}, "payee"}, {domain.Transaction{Desc: "desc", ID: "id"}, "desc"}, {domain.Transaction{ID: "id"}, "id"}} {
		if got := ChargePayee(tc.txn); got != tc.want {
			t.Fatal(got)
		}
	}
}

func TestOrderItemAndSplitArithmetic(t *testing.T) {
	for _, tc := range []struct {
		item     orderimport.Item
		qty      int
		total    int64
		overflow bool
	}{{orderimport.Item{UnitMinor: 100, Qty: 2}, 2, 200, false}, {orderimport.Item{UnitMinor: -100, Qty: 0}, 1, -100, false}, {orderimport.Item{Qty: -1}, 1, 0, false}, {orderimport.Item{UnitMinor: math.MaxInt64, Qty: 2}, 2, 0, true}} {
		qty, total, err := OrderItem(tc.item)
		if qty != tc.qty || total != tc.total || (err != nil) != tc.overflow {
			t.Fatal(qty, total, err)
		}
	}
	categories := []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "income", Name: "Salary", Kind: domain.KindIncome}, {ID: "empty", Kind: domain.KindExpense}}
	for _, tc := range []struct {
		name   string
		items  []orderimport.Item
		amount money.Money
		want   string
	}{
		{"empty", nil, money.New(-200, "USD"), "orderimport.noItems"},
		{"unnamed", []orderimport.Item{{UnitMinor: 100}}, money.New(-200, "USD"), "orderimport.noItems"},
		{"exact", []orderimport.Item{{Name: "Food", UnitMinor: 100}}, money.New(-200, "USD"), ""},
		{"partial", []orderimport.Item{{Name: "Food item", UnitMinor: 100}}, money.New(-200, "USD"), ""},
		{"rule", []orderimport.Item{{Name: "Special", UnitMinor: 100}}, money.New(-200, "USD"), ""},
		{"unknown", []orderimport.Item{{Name: "Unknown", UnitMinor: 100}}, money.New(-200, "USD"), "receiptsplit.noProposal"},
		{"space", []orderimport.Item{{Name: " ", UnitMinor: 100}}, money.New(-200, "USD"), "receiptsplit.noProposal"},
		{"currency", []orderimport.Item{{Name: "Food", UnitMinor: 100}}, money.New(-200, ""), "receiptsplit.noProposal"},
		{"multiply overflow", []orderimport.Item{{Name: "Food", UnitMinor: math.MaxInt64, Qty: 2}}, money.New(-200, "USD"), "receiptsplit.noProposal"},
		{"sum overflow", []orderimport.Item{{Name: "Food", UnitMinor: math.MaxInt64}, {Name: "Food", UnitMinor: 1}}, money.New(-200, "USD"), "receiptsplit.noProposal"},
		{"absolute overflow", []orderimport.Item{{Name: "Food", UnitMinor: math.MinInt64}}, money.New(-200, "USD"), "receiptsplit.noProposal"},
		{"target overflow", nil, money.New(math.MinInt64, "USD"), "receiptsplit.noProposal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proposal, key := OrderSplit(domain.Transaction{Amount: tc.amount, CategoryID: "other"}, orderimport.Order{Items: tc.items, Currency: "USD"}, categories, []rules.Rule{{Match: "Special", SetCategoryID: "food"}})
			if key != tc.want {
				t.Fatal(key)
			}
			if key == "" {
				var sum int64
				for _, split := range proposal.Splits {
					var err error
					sum, err = money.Add(sum, split.Amount.Amount)
					if err != nil {
						t.Fatal(err)
					}
				}
				if sum != tc.amount.Amount {
					t.Fatal(proposal)
				}
			}
		})
	}
}

func TestOrderTextAndMagnitude(t *testing.T) {
	for _, tc := range []struct {
		review      ordermatch.ReviewMatch
		key, charge string
		apply       bool
	}{
		{ordermatch.ReviewMatch{}, "orderimport.unmatched", "orderimport.chargeLabel", false},
		{ordermatch.ReviewMatch{Match: orderimport.Match{Kind: orderimport.MatchSingle}}, "orderimport.matchedSingle", "orderimport.chargeLabel", true},
		{ordermatch.ReviewMatch{Match: orderimport.Match{Kind: orderimport.MatchMulti, TxnIDs: []string{"a", "b"}}, Charges: make([]domain.Transaction, 2), MissingTxnIDs: []string{"a"}}, "orderimport.matchedMulti2", "orderimport.chargesLabel", false},
	} {
		verdict, charge, apply := OrderText(tc.review, translation)
		if verdict != tc.key || charge != tc.charge || apply != tc.apply {
			t.Fatal(verdict, charge, apply)
		}
	}
	for _, tc := range []struct {
		amount int64
		want   string
	}{{100, "$1.00"}, {-100, "$1.00"}, {0, "$0.00"}, {math.MinInt64, "$92,233,720,368,547,758.08"}} {
		if got := MagnitudeText(tc.amount, "USD"); got != tc.want {
			t.Fatal(got)
		}
	}
}

func TestChoicesAndBrowserInputPolicies(t *testing.T) {
	for _, current := range []string{"", "Food", "Unknown"} {
		choices := CategoryChoices([]domain.Category{{Name: "Food"}}, current, translation)
		if len(choices) != 2 && current != "Unknown" {
			t.Fatal(choices)
		}
		selected := 0
		for _, choice := range choices {
			if choice.Selected {
				selected++
				if choice.Value != current {
					t.Fatal(choice)
				}
			}
		}
		if selected != 1 {
			t.Fatal(choices)
		}
	}
	if len(CategoryChoices(nil, "", translation)) != 1 {
		t.Fatal("nil")
	}
	for _, selected := range []string{"", "-1", "0"} {
		choices := ColumnChoices([]string{"", "Amount"}, selected, translation)
		if len(choices) != 3 || choices[0].Selected != (selected != "0") || choices[1].Selected != (selected == "0") || choices[1].Label != "imb.columnSuffiximb.column00" {
			t.Fatal(choices)
		}
	}
	if len(ColumnChoices(nil, "", translation)) != 1 {
		t.Fatal("nil")
	}
	for _, tc := range []struct{ raw, want string }{{"", ""}, {"bad", ""}, {`{"activeId":"missing"}`, "missing"}, {`{"activeId":"b","workspaces":[{"id":"a"},{"id":"b"}]}`, "b"}, {`{"workspaces":[{"id":"a"}]}`, "a"}} {
		if got := ActiveWorkspace(tc.raw); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		mime string
		size int
		want string
	}{{"", 0, "documents.imageTypeInvalid"}, {"text/plain", 11, "documents.imageTypeInvalid"}, {"image/png", 10, ""}, {"image/jpeg", 11, "documents.imageTooLarge"}} {
		if got := ImageError(tc.mime, tc.size, 10); got != tc.want {
			t.Fatal(got)
		}
	}
	if !reflect.DeepEqual(LearnedCategories(nil), []string(nil)) {
		t.Fatal("nil learning")
	}
}

func TestOrderItemLabel(t *testing.T) {
	if got := OrderItemLabel(orderimport.Item{Name: "Food", UnitMinor: 100, Qty: 2}, "USD", translation); got != "orderimport.itemLine2Food$2.00" {
		t.Fatal(got)
	}
	if got := OrderItemLabel(orderimport.Item{UnitMinor: math.MaxInt64, Qty: 2}, "USD", translation); got != "documents.linesUnreadable" {
		t.Fatal(got)
	}
}
