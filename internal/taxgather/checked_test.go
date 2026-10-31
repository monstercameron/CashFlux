// SPDX-License-Identifier: MIT

package taxgather

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func TestGatherCheckedMatchesReportLinesRefundsExclusionsAndGapUnion(t *testing.T) {
	start, end, _ := YearBounds(2025)
	cats := []domain.Category{{ID: "ded", Name: "Work supplies", Deductible: true}, {ID: "donate", Name: "Charitable giving", Deductible: true}, {ID: "interest", Name: "Interest"}, {ID: "other", Name: "Other"}}
	txns := []domain.Transaction{
		{ID: "original", Date: start, CategoryID: "ded", Amount: money.New(-10000, "USD")},
		{ID: "refund", Date: end, CategoryID: "ded", Amount: money.New(4000, "USD")},
		{ID: "mixed", Date: start, CategoryID: "other", Amount: money.New(-7000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "donate", Amount: money.New(-5000, "USD")}, {CategoryID: "interest", Amount: money.New(-3000, "USD")}, {CategoryID: "ded", Amount: money.New(1000, "USD")}}},
		{ID: "foreign", Date: start, CategoryID: "ded", Amount: money.New(-500, "EUR"), Attachments: []domain.AttachmentRef{{ArtifactID: "receipt"}}},
		{ID: "excluded", Date: start, CategoryID: "donate", ExcludeFromReports: true, Amount: money.New(math.MinInt64, "NOFX")},
		{ID: "transfer", Date: start, CategoryID: "donate", TransferAccountID: "other", Amount: money.New(-9999, "NOFX")},
		{ID: "end", Date: end, CategoryID: "donate", Amount: money.New(-9999, "NOFX")},
	}
	links := []domain.TxnLink{{ID: "refund-link", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original", "refund"}}}
	before, _ := json.Marshal(txns)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	got, err := GatherWithLinks(txns, cats, links, 2025, start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := reports.SpendingByCategoryChecked(budgeting.NewNettedLedger(txns, links), start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	var reportDeductible int64
	for _, r := range checked {
		if r.CategoryID == "ded" || r.CategoryID == "donate" {
			reportDeductible += r.Amount
		}
	}
	if got.Deductible.Total != 12000 || got.Deductible.Total != reportDeductible || got.Charitable.Total != 5000 || got.Charitable.Count != 1 || got.InterestPaid.Total != 3000 || got.InterestPaid.Count != 1 {
		t.Fatal("canonical split/refund/report parity failed", got)
	}
	if len(got.Gaps) != 2 || got.Gaps[0].TxnID != "original" || got.Gaps[0].Amount != 6000 || got.Gaps[1].TxnID != "mixed" || got.Gaps[1].Amount != 5000 {
		t.Fatal("receipt gaps must select the union once, not whole mixed receipt", got.Gaps)
	}
	after, _ := json.Marshal(txns)
	if string(before) != string(after) {
		t.Fatal("gather mutated ledger or nested split lines")
	}
}

func TestGatherCheckedRejectsMagnitudeFXAndAccumulationFailures(t *testing.T) {
	start, end, _ := YearBounds(2025)
	for _, tc := range []struct {
		name  string
		cats  []domain.Category
		txns  []domain.Transaction
		rates currency.Rates
		want  error
	}{
		{"magnitude", []domain.Category{{ID: "ded", Deductible: true}}, []domain.Transaction{{ID: "min", Date: start, CategoryID: "ded", Amount: money.New(math.MinInt64, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"FX", []domain.Category{{ID: "ded", Deductible: true}}, []domain.Transaction{{ID: "fx", Date: start, CategoryID: "ded", Amount: money.New(-1, "EUR")}}, currency.Rates{Base: "USD"}, currency.ErrUnknownRate},
		{"category", []domain.Category{{ID: "ded", Deductible: true}}, []domain.Transaction{{ID: "one", Date: start, CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "two", Date: start, CategoryID: "ded", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"grand", []domain.Category{{ID: "a", Deductible: true}, {ID: "b", Deductible: true}}, []domain.Transaction{{ID: "one", Date: start, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "two", Date: start, CategoryID: "b", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"donations", []domain.Category{{ID: "a", Name: "Donation"}}, []domain.Transaction{{ID: "one", Date: start, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "two", Date: start, CategoryID: "a", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"interest", []domain.Category{{ID: "a", Name: "Interest"}}, []domain.Transaction{{ID: "one", Date: start, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "two", Date: start, CategoryID: "a", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GatherWithLinks(tc.txns, tc.cats, nil, 2025, start, end, tc.rates)
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(got, Summary{}) {
				t.Fatal("failed inputs became valid partial/zero gathering", got, err)
			}
		})
	}
	// A valid near-limit value remains exact, without float formatting loss.
	got, err := Gather([]domain.Transaction{{Date: start, CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}}, []domain.Category{{ID: "ded", Deductible: true}}, 2025, start, end, currency.Rates{Base: "USD"})
	if err != nil || got.Deductible.Total != math.MaxInt64 {
		t.Fatal(got, err)
	}
}

func TestGatherCheckedMakesAmbiguousOrOverallocatedRefundsUnavailable(t *testing.T) {
	start, end, _ := YearBounds(2025)
	original := domain.Transaction{ID: "split-purchase", Date: start, Amount: money.New(-100, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-60, "USD")}, {CategoryID: "other", Amount: money.New(-40, "USD")}}}
	refund := domain.Transaction{ID: "refund", Date: end, Amount: money.New(40, "USD")}
	link := domain.TxnLink{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{original.ID, refund.ID}}
	if _, err := GatherWithLinks([]domain.Transaction{original, refund}, nil, []domain.TxnLink{link}, 2025, start, end, currency.Rates{Base: "USD"}); err == nil || !strings.Contains(err.Error(), "split-purchase") || !strings.Contains(err.Error(), "category allocation") {
		t.Fatal("partial split refund guessed its tax categories", err)
	}
	refund.Amount.Amount = 100
	if got, err := GatherWithLinks([]domain.Transaction{original, refund}, nil, []domain.TxnLink{link}, 2025, start, end, currency.Rates{Base: "USD"}); err != nil || len(got.Gaps) != 0 || got.Deductible.Total != 0 {
		t.Fatal("fully refunded split should have no spend", got, err)
	}
	original.Splits = nil
	for _, amount := range []int64{100, math.MaxInt64} {
		original.Amount.Amount = -amount
		refund.Amount.Amount = amount
		if _, err := GatherWithLinks([]domain.Transaction{original, refund}, nil, []domain.TxnLink{link, link}, 2025, start, end, currency.Rates{Base: "USD"}); err == nil {
			t.Fatal("duplicate refund allocations became a plausible result", amount)
		}
	}
}

func TestGatherCSVExactPreservesCurrencyDecimalsAndUntrustedLabels(t *testing.T) {
	s := Summary{Year: 2025, Deductible: reports.DeductibleSummary{Total: math.MaxInt64, Rows: []reports.DeductibleRow{{CategoryID: "c", Amount: math.MaxInt64}}}, Gaps: []Gap{{Label: "=SUM(A1),\"quoted\"\r\nnext", Reason: "no receipt", Amount: math.MaxInt64, Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	for _, tc := range []struct{ code, want string }{{"JPY", "9223372036854775807"}, {"USD", "92233720368547758.07"}, {"ZZZ", "92233720368547758.07"}} {
		b, err := GatherCSVExact(s, func(string) string { return "=untrusted category" }, tc.code)
		if err != nil || !strings.Contains(string(b), tc.want) || !strings.HasPrefix(string(b), "Currency,"+tc.code+"\n") {
			t.Fatal(tc.code, string(b), err)
		}
		r := csv.NewReader(strings.NewReader(string(b)))
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil {
			t.Fatal("malformed CSV", string(b), err)
		}
		if records[2][0] != "'=untrusted category" || records[len(records)-1][0][0] != '\'' {
			t.Fatal("text label was executable/lost on CSV round trip", records)
		}
	}
	if _, _, err := YearBounds(-1); err == nil {
		t.Fatal("negative year accepted")
	}
	if _, _, err := YearBounds(10000); err == nil {
		t.Fatal("out-of-range year accepted")
	}
}
