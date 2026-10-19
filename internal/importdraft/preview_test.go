// SPDX-License-Identifier: MIT

package importdraft

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/staging"
)

func TestBatchPreviewChecksAccountCurrencyAndArithmetic(t *testing.T) {
	account := domain.Account{ID: "a", Currency: "USD", OpeningBalance: money.New(100, "USD")}
	for _, tc := range []struct {
		name       string
		account    domain.Account
		txns       []domain.Transaction
		incoming   []domain.Transaction
		valid      bool
		net, after int64
	}{
		{"zero", account, nil, nil, true, 0, 100},
		{"signed", account, nil, []domain.Transaction{{AccountID: "a", Amount: money.New(-50, "USD")}, {AccountID: "other", Amount: money.New(800, "EUR")}}, true, -50, 50},
		{"currency without FX", account, nil, []domain.Transaction{{AccountID: "a", Amount: money.New(1, "EUR")}}, false, 0, 0},
		{"ledger currency mismatch", account, []domain.Transaction{{AccountID: "a", Amount: money.New(1, "EUR")}}, nil, false, 0, 0},
		{"opening mismatch", domain.Account{ID: "a", Currency: "USD", OpeningBalance: money.New(1, "EUR")}, nil, nil, false, 0, 0},
		{"overflow total", account, nil, []domain.Transaction{{Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(1, "USD")}}, false, 0, 0},
		{"overflow balance", account, nil, []domain.Transaction{{Amount: money.New(math.MaxInt64, "USD")}}, false, 0, 0},
		{"jump", account, nil, []domain.Transaction{{Amount: money.New(2000000, "USD")}}, true, 2000000, 2000100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := staging.Batch{Rows: []staging.Row{{Verdict: staging.Duplicate}, {Verdict: staging.Unusable, Line: 7, Reason: "bad"}}}
			for _, tx := range tc.incoming {
				batch.Rows = append(batch.Rows, staging.Row{Verdict: staging.New, Transaction: tx})
			}
			p := BatchPreview(batch, []domain.Account{tc.account}, tc.txns, "a")
			if p.HasBalance != tc.valid || p.Net != tc.net || p.After != tc.after || len(p.Failures) != 1 || p.Failures[0].Line != 7 || p.Failures[0].Reason != "bad" {
				t.Fatalf("%+v", p)
			}
			if tc.name == "jump" && !p.Jump {
				t.Fatal("jump missing")
			}
		})
	}
	if BatchPreview(staging.Batch{}, nil, nil, "a").HasBalance || ApprovedTransactions(staging.Batch{}) != nil {
		t.Fatal("nil")
	}
}

func TestRawPreviewSkipsInvalidAndChecksSums(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opening    int64
		rows       []extract.Row
		valid      bool
		net, after int64
	}{
		{"empty", 100, nil, true, 0, 100},
		{"invalid", 100, []extract.Row{{Amount: "bad"}, {Amount: " -2.00 "}, {Amount: "0"}}, true, -200, -100},
		{"sum overflow", 0, []extract.Row{{Amount: "92233720368547758.07"}, {Amount: "1"}}, false, 0, 0},
		{"balance overflow", 100, []extract.Row{{Amount: "92233720368547758.07"}}, false, math.MaxInt64, 0},
		{"jump", 100, []extract.Row{{Amount: "20000"}}, true, 2000000, 2000100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := domain.Account{ID: "a", Currency: "USD", OpeningBalance: money.New(tc.opening, "USD")}
			p := RawPreview([]domain.Account{account}, nil, "a", tc.rows)
			if p.HasBalance != tc.valid || p.Net != tc.net || p.After != tc.after {
				t.Fatal(p)
			}
		})
	}
	account := domain.Account{ID: "a", Currency: "USD"}
	if RawPreview(nil, nil, "a", nil).HasBalance || RawPreview([]domain.Account{account}, []domain.Transaction{{AccountID: "a", Amount: money.New(1, "EUR")}}, "a", nil).HasBalance {
		t.Fatal("invalid balance exposed")
	}
}

func TestReceiptCandidates(t *testing.T) {
	date := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{{ID: "a", Date: date, Desc: "Shop", Amount: money.New(-100, "USD")}}
	for _, total := range []string{"bad", "0", "-1"} {
		if got := ReceiptCandidates(nil, "Shop", total, 2, txns, date); got != nil {
			t.Fatal(got)
		}
	}
	if got := ReceiptCandidates([]extract.Row{{Date: "2026-01-31"}}, "Shop", "1.00", 2, txns, time.Time{}); len(got) != 1 || got[0].Txn.ID != "a" {
		t.Fatal(got)
	}
	if got := ReceiptCandidates(nil, "Shop", "1.00", 2, nil, date); len(got) != 0 {
		t.Fatal(got)
	}
}

func FuzzRawPreviewReconciles(f *testing.F) {
	f.Add(int64(500), int64(-200), int64(300))
	f.Fuzz(func(t *testing.T, opening, a, b int64) {
		net, err := money.Add(a, b)
		if err != nil {
			return
		}
		after, err := money.Add(opening, net)
		if err != nil {
			return
		}
		account := domain.Account{ID: "a", Currency: "USD", OpeningBalance: money.New(opening, "USD")}
		p := RawPreview([]domain.Account{account}, nil, "a", []extract.Row{{Amount: money.FormatMinor(a, 2)}, {Amount: money.FormatMinor(b, 2)}})
		if !p.HasBalance || p.Net != net || p.After != after {
			t.Fatalf("%d + (%d + %d): %+v", opening, a, b, p)
		}
	})
}
