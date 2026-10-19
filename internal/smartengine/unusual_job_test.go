// SPDX-License-Identifier: MIT
package smartengine

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/debtpay"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"reflect"
	"testing"
	"time"
)

// debtPrincipalTxnIDs returns cash-side debt-payment legs identified by the
// committed debtpay classifier. These payments reduce debt principal rather
// than represent ordinary purchases, so they must not enter unusual-spend
// baselines or findings.
func debtPrincipalTxnIDs(in Input) map[string]bool {
	data := debtpay.Dataset{}
	for _, account := range in.Accounts {
		data.Accounts = append(data.Accounts, debtpay.Account{
			ID: account.ID, Name: account.Name, Lender: account.Institution,
			Currency: account.Currency, Liability: account.Class == domain.ClassLiability,
		})
	}
	for _, txn := range in.Transactions {
		data.Transactions = append(data.Transactions, debtTransaction(txn))
	}
	ids := make(map[string]bool)
	for _, txn := range in.Transactions {
		if !txn.Amount.IsNegative() {
			continue
		}
		candidate := debtpay.Classify(debtTransaction(txn), data)
		if candidate.Confidence != debtpay.ConfidenceNone {
			ids[txn.ID] = true
		}
	}
	return ids
}

func TestUnusualJobMatchesRegisteredAdapter(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{{ID: "cash", Currency: "USD", Class: domain.ClassAsset}, {ID: "loan", Name: "Car loan", Currency: "USD", Class: domain.ClassLiability}}
	in.Transactions = []domain.Transaction{{ID: "purchase", AccountID: "cash", Date: in.Now, Payee: "New Shop", Amount: usd(-40000)}, {ID: "payment", AccountID: "cash", Date: in.Now, Payee: "Car loan", BillAccountID: "loan", Amount: usd(-40000)}}
	principal := debtPrincipalTxnIDs(in)
	if !principal["payment"] || principal["purchase"] {
		t.Fatal("fixture classification invalid")
	}
	expected := u1Unusual(in)
	for _, budget := range []int{1, 8, 64} {
		job := NewUnusualJob(in)
		if job.Step(0) || job.Insights() != nil {
			t.Fatal("empty budget did work")
		}
		for !job.Step(budget) {
		}
		if !reflect.DeepEqual(job.Insights(), expected) {
			t.Fatalf("budget %d changed insights", budget)
		}
	}
}

func TestUnusualJobIndexesDebtCounterparts(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{
		{ID: "cash", Currency: "USD", Class: domain.ClassAsset},
		{ID: "loan", Name: "Car loan", Currency: "USD", Class: domain.ClassLiability},
	}
	in.Transactions = []domain.Transaction{
		{ID: "payment", AccountID: "cash", Date: ref, Payee: "Car loan", Amount: usd(-30000)},
		{ID: "counterpart", AccountID: "loan", Date: ref.AddDate(0, 0, -1), Amount: usd(30000)},
		{ID: "wrong-amount", AccountID: "loan", Date: ref, Amount: usd(30001)},
		{ID: "wrong-currency", AccountID: "loan", Date: ref, Amount: money.New(30000, "EUR")},
		{ID: "outside-window", AccountID: "loan", Date: ref.AddDate(0, 0, -20), Amount: usd(30000)},
	}
	job := NewUnusualJob(in)
	for job.phase < 2 {
		job.Step(1)
	}
	got := job.counterpartCandidates(debtTransaction(in.Transactions[0]))
	if len(got) != 1 || got[0].ID != "counterpart" {
		t.Fatalf("indexed candidates=%+v; want only the in-window exact amount/currency counterpart", got)
	}
}

func TestSOLP2SmartUnusualSlices3300(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{{ID: "cash", Currency: "USD", Class: domain.ClassAsset}, {ID: "loan", Name: "Car loan", Currency: "USD", Class: domain.ClassLiability}}
	for i := 0; i < 3300; i++ {
		in.Transactions = append(in.Transactions, domain.Transaction{ID: fmt.Sprint(i), AccountID: "cash", Date: in.Now.AddDate(0, 0, -i), Payee: fmt.Sprintf("Merchant %d", i%37), Amount: usd(-int64(1000 + i%91))})
	}
	job := NewUnusualJob(in)
	var longest time.Duration
	count := 0
	for {
		start := time.Now()
		done := job.Step(8)
		longest = max(longest, time.Since(start))
		count++
		if done {
			break
		}
	}
	t.Logf("SOLP2 Smart Unusual: rows=3300 steps=%d longest=%s", count, longest)
	if longest > 50*time.Millisecond {
		t.Fatalf("adapter slice exceeded 50 ms: %s", longest)
	}
	// Detector evidence remains present after adapter work; the pure detector's
	// equivalence tests independently compare every field with the old algorithm.
}

func TestUnusualJobPublicationKeepsMuteDismissalAndLocalization(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{{ID: "new-merchant", Date: in.Now, Payee: "Shop", Amount: usd(-40000)}}
	settings := smart.Settings{Enabled: map[string]bool{unusualCode: true}, ExplicitOff: map[string]bool{}}
	for _, feature := range smart.Catalog() {
		if feature.Code != unusualCode {
			settings.ExplicitOff[feature.Code] = true
		}
	}
	job := NewUnusualJob(in)
	for !job.Step(8) {
	}
	if got, want := job.ActiveInsights(settings), RunPage(in, settings, smart.PageTransactions); !reflect.DeepEqual(got, want) {
		t.Fatal("cooperative publication differs from RunPage")
	}
	settings.Muted = map[string]bool{unusualCode: true}
	if len(job.ActiveInsights(settings)) != 0 {
		t.Fatal("muted Unusual insights published")
	}
	settings.Muted = nil
	settings.Dismissed = map[string]bool{job.Insights()[0].Key: true}
	if len(job.ActiveInsights(settings)) != 0 {
		t.Fatal("dismissed Unusual insight published")
	}
}
