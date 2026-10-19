// SPDX-License-Identifier: MIT

package importdraft

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
)

func translation(key string, args ...any) string { return key + fmt.Sprint(args...) }

func TestReviewCurrencyAndAccountName(t *testing.T) {
	accounts := []domain.Account{{ID: "a", Name: "Bank", Currency: "JPY"}, {ID: "b"}}
	for _, tc := range []struct{ id, base, want string }{{"a", "EUR", "JPY"}, {"b", "EUR", "EUR"}, {"missing", "", "USD"}} {
		if got := ReviewCurrency(accounts, tc.id, tc.base); got != tc.want {
			t.Errorf("%+v: %s", tc, got)
		}
	}
	if ReviewCurrency(nil, "", "") != "USD" || AccountName(accounts, "a") != "Bank" || AccountName(nil, "a") != "" {
		t.Fatal("account defaults")
	}
}

func TestAmountPresentation(t *testing.T) {
	format := func(m money.Money) string { return fmt.Sprintf("%s:%d", m.Currency, m.Amount) }
	for _, tc := range []struct {
		raw, cur, text, class string
		color                 bool
		kind                  domain.CategoryKind
	}{
		{" 1.00 ", "USD", "USD:100", "amount fig amount-income", true, domain.KindIncome},
		{"-1", "JPY", "JPY:-1", "amount fig amount-expense", true, domain.KindExpense},
		{"0", "USD", "USD:0", "amount fig", true, domain.KindExpense},
		{"-2", "USD", "USD:-200", "amount fig", false, domain.KindExpense},
		{" 2 ", "", " 2 ", "amount fig", false, domain.KindIncome},
		{"bad", "USD", "bad", "amount fig", true, domain.KindExpense},
		{"92233720368547758.08", "USD", "92233720368547758.08", "amount fig", true, domain.KindExpense},
	} {
		text, class, kind := AmountPresentation(tc.raw, tc.cur, tc.color, format)
		if text != tc.text || class != tc.class || kind != tc.kind {
			t.Errorf("%+v: %q %q %v", tc, text, class, kind)
		}
	}
}

func TestReconcileAndEnablement(t *testing.T) {
	for _, tc := range []struct {
		lines                                    []extract.Row
		total, cur                               string
		errors                                   int
		saving, reconciled, unreadable, disabled bool
	}{
		{nil, "0", "", 0, false, true, false, false},
		{[]extract.Row{{Amount: "10"}, {Amount: "-2"}}, "8", "USD", 0, false, true, false, false},
		{[]extract.Row{{Amount: "2"}}, "3", "USD", 0, false, false, false, true},
		{nil, "bad", "USD", 0, false, false, true, true},
		{[]extract.Row{{Amount: "bad"}}, "3", "USD", 0, false, false, true, true},
		{nil, "0", "USD", 1, false, true, false, true},
		{nil, "0", "USD", 0, true, true, false, true},
		{nil, "-92233720368547758.08", "USD", 0, false, false, true, true},
		{[]extract.Row{{Amount: "92233720368547758.07"}, {Amount: "1"}}, "0", "USD", 0, false, false, true, true},
	} {
		r := ReconcileReceipt(tc.lines, tc.total, tc.cur, tc.errors, tc.saving)
		if r.Reconciled != tc.reconciled || r.Unreadable != tc.unreadable || r.Disabled != tc.disabled || r.Currency == "" {
			t.Errorf("%+v: %+v", tc, r)
		}
	}
	for _, tc := range []struct {
		s                Summary
		errors           int
		saving, disabled bool
	}{
		{Summary{Importable: 1}, 0, false, false}, {Summary{}, 0, false, true}, {Summary{Importable: 1, Invalid: 1}, 0, false, true}, {Summary{Importable: 1, Overflow: true}, 0, false, true}, {Summary{Importable: 1}, 1, false, true}, {Summary{Importable: 1}, 0, true, true},
	} {
		if got := StatementDisabled(tc.s, tc.errors, tc.saving); got != tc.disabled {
			t.Errorf("%+v: %v", tc, got)
		}
	}
}

func TestRecentDocumentsStableAndImmutable(t *testing.T) {
	if len(RecentDocuments(nil)) != 0 {
		t.Fatal("nil")
	}
	docs := make([]domain.Document, 10000)
	for i := range docs {
		docs[i] = domain.Document{ID: fmt.Sprint(i), UploadedAt: time.Unix(int64(i/2), 0)}
	}
	before := append([]domain.Document(nil), docs...)
	out := RecentDocuments(docs)
	if len(out) != 5 || out[0].ID != "9998" || out[1].ID != "9999" || !reflect.DeepEqual(docs, before) {
		t.Fatal("unstable or mutating sort")
	}
	out[0].ID = "changed"
	if !reflect.DeepEqual(docs, before) {
		t.Fatal("aliased result")
	}
}

func TestHistoryText(t *testing.T) {
	for _, tc := range []struct {
		kind domain.DocumentKind
		key  string
	}{{domain.DocCSV, "documents.historyKindCSV"}, {"pdf", "imb.historyPDF"}, {"statement", "imb.historyStatement"}, {"mixed", "imb.historyMixed"}, {domain.DocImage, "documents.historyKindDoc"}, {"unknown", "documents.historyKindCSV"}} {
		label, _, _ := HistoryText(domain.Document{Kind: tc.kind}, "Bank", nil, translation)
		if label != tc.key+" · Bank" {
			t.Errorf("%s: %s", tc.kind, label)
		}
	}
	for _, tc := range []struct {
		doc      domain.Document
		txns     []domain.Transaction
		contains []string
	}{
		{domain.Document{ID: "run", RowCount: 2, RowsLinked: true, DuplicateCount: 3, FailedCount: 1}, []domain.Transaction{{SourceDocID: "run"}}, []string{"historyDuplicates3", "historyFailed1", "historyRemoved1 1"}},
		{domain.Document{RowCount: 1, SkippedCount: 2}, nil, []string{"historySkipped2", "historyUntraceable"}},
		{domain.Document{Extracted: []domain.DocumentRow{{}}}, nil, []string{"imb.transactionOne1", "historyUntraceable"}},
	} {
		_, result, _ := HistoryText(tc.doc, "", tc.txns, translation)
		for _, want := range tc.contains {
			if !strings.Contains(result, want) {
				t.Errorf("%s lacks %s", result, want)
			}
		}
	}
	when := time.Date(2026, 1, 31, 23, 0, 0, 0, time.FixedZone("local", -5*3600))
	_, _, got := HistoryText(domain.Document{UploadedAt: when}, "", nil, translation)
	if got != "Jan 31, 2026 11:00 PM" {
		t.Fatal(got)
	}
}

func TestMatchReasonStatusAndLearning(t *testing.T) {
	for _, tc := range []struct {
		candidate receiptmatch.Candidate
		want      string
	}{{receiptmatch.Candidate{}, "documents.receiptMatchSameDay"}, {receiptmatch.Candidate{DaysApart: 2, MerchantHit: true}, "documents.receiptMatchDaysApart2 · documents.receiptMatchMerchant"}} {
		if got := MatchReason(tc.candidate, translation); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		msg, failure             string
		loading, saving, pending bool
		want                     string
	}{
		{"", "", false, false, false, "imb.ready"}, {"message", "", false, false, false, "message"}, {"message", "failure", false, false, false, "failure"}, {"message", "failure", true, false, false, "documents.reading"}, {"message", "failure", true, true, false, "enc34.saving"}, {"message", "failure", false, true, true, "failure"},
	} {
		if got := StatusText(tc.msg, tc.failure, tc.loading, tc.saving, tc.pending, translation); got != tc.want {
			t.Fatal(got)
		}
	}
	if LearnedCategories(nil) != nil {
		t.Fatal("nil")
	}
	if got := LearnedCategories([]domain.CategorySplit{{CategoryID: "a"}, {}, {CategoryID: "a"}, {CategoryID: "b"}}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
}

func FuzzReceiptResidualReconciles(f *testing.F) {
	f.Add(int64(1234), int64(-234))
	f.Add(int64(0), int64(0))
	f.Fuzz(func(t *testing.T, a, b int64) {
		total, err := money.Add(a, b)
		if err != nil {
			return
		}
		rows := []extract.Row{{Amount: money.FormatMinor(a, 2)}, {Amount: money.FormatMinor(b, 2)}}
		got := ReconcileReceipt(rows, money.FormatMinor(total, 2), "USD", 0, false)
		if !got.Reconciled || got.Disabled {
			t.Fatalf("%d + %d: %+v", a, b, got)
		}
	})
}

func FuzzRecentDocumentsOrder(f *testing.F) {
	f.Add([]byte{4, 2, 2, 9, 0, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 10000 {
			return
		}
		docs := make([]domain.Document, len(data))
		for i, b := range data {
			docs[i] = domain.Document{ID: fmt.Sprint(i), UploadedAt: time.Unix(int64(b), 0)}
		}
		out := RecentDocuments(docs)
		for i := 1; i < len(out); i++ {
			if out[i].UploadedAt.After(out[i-1].UploadedAt) {
				t.Fatal("out of order")
			}
		}
	})
}

func TestSignedMinor(t *testing.T) {
	for _, tc := range []struct {
		amount int64
		want   string
	}{{1, "+0.01"}, {0, "0.00"}, {-1, "-0.01"}, {math.MinInt64, "-92233720368547758.08"}} {
		if got := SignedMinor(tc.amount, 2); got != tc.want {
			t.Fatal(got)
		}
	}
	if dateutil.FormatDate(Reminder("id", "title", time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)).Due) != "2026-03-03" {
		t.Fatal("monthly normalization changed")
	}
}
