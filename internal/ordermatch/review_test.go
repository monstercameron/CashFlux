// SPDX-License-Identifier: MIT

package ordermatch

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
)

func TestForReviewResolvesChargesInMatchOrder(t *testing.T) {
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	transactions := []domain.Transaction{
		{ID: "second", Date: day.AddDate(0, 0, 1), Payee: "Amazon Marketplace", Amount: money.New(-1250, "USD")},
		{ID: "first", Date: day, Payee: "Amazon", Desc: "Order charge", Amount: money.New(-2000, "USD")},
	}
	matches := []orderimport.Match{{
		Order: orderimport.Order{ID: "order-1"}, Kind: orderimport.MatchMulti,
		TxnIDs: []string{"first", "second"},
	}}

	got := ForReview(matches, transactions)
	if len(got) != 1 || len(got[0].Charges) != 2 {
		t.Fatalf("ForReview() = %#v, want one match with two charges", got)
	}
	if got[0].Charges[0].ID != "first" || got[0].Charges[1].ID != "second" {
		t.Fatalf("charges are not in matcher order: %#v", got[0].Charges)
	}
	if got[0].Charges[0].Payee != "Amazon" || got[0].Charges[0].Amount.Amount != -2000 {
		t.Fatalf("charge details were not preserved: %#v", got[0].Charges[0])
	}
}

func TestForReviewReportsMissingMatchedTransactions(t *testing.T) {
	got := ForReview([]orderimport.Match{{
		Order: orderimport.Order{ID: "order-2"}, Kind: orderimport.MatchSingle,
		TxnIDs: []string{"removed"},
	}}, nil)
	if len(got) != 1 || len(got[0].MissingTxnIDs) != 1 || got[0].MissingTxnIDs[0] != "removed" {
		t.Fatalf("ForReview() = %#v, want the missing transaction id", got)
	}
}
