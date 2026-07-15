// SPDX-License-Identifier: MIT

package taxgather

import (
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func exp(id, cat string, minor int64, day int, tags []string, attach bool) domain.Transaction {
	t := domain.Transaction{
		ID:         id,
		CategoryID: cat,
		Amount:     money.New(-minor, "USD"),
		Date:       time.Date(2025, 3, day, 0, 0, 0, 0, time.UTC),
		Tags:       tags,
		Payee:      id,
	}
	if attach {
		t.Attachments = []domain.AttachmentRef{{ArtifactID: "a1"}}
	}
	return t
}

func TestGather(t *testing.T) {
	cats := []domain.Category{
		{ID: "med", Name: "Medical", Deductible: true},
		{ID: "give", Name: "Charitable Giving"},
		{ID: "food", Name: "Groceries"},
	}
	txns := []domain.Transaction{
		exp("doctor", "med", 20000, 5, nil, false),              // deductible, no receipt -> gap
		exp("dentist", "med", 10000, 6, nil, true),              // deductible, has receipt
		exp("redcross", "give", 5000, 7, nil, false),            // donation via category -> gap
		exp("church", "food", 3000, 8, []string{"tithe"}, true), // donation via tag, has receipt
		exp("mortgage", "food", 90000, 9, []string{"interest"}, false),
		exp("bread", "food", 400, 10, nil, false), // ordinary, ignored
	}
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s, err := Gather(txns, cats, 2025, start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	if s.Deductible.Total != 30000 {
		t.Errorf("deductible total = %d want 30000", s.Deductible.Total)
	}
	if s.Charitable.Count != 2 || s.Charitable.Total != 8000 {
		t.Errorf("charitable = %+v want count 2 total 8000", s.Charitable)
	}
	if s.InterestPaid.Count != 1 || s.InterestPaid.Total != 90000 {
		t.Errorf("interest = %+v want count 1 total 90000", s.InterestPaid)
	}
	// Gaps: doctor (deductible, no receipt) + redcross (donation, no receipt).
	if len(s.Gaps) != 2 {
		t.Fatalf("gaps = %d want 2: %+v", len(s.Gaps), s.Gaps)
	}
}

func TestGatherCSV(t *testing.T) {
	cats := []domain.Category{{ID: "med", Name: "Medical", Deductible: true}}
	txns := []domain.Transaction{exp("doctor", "med", 20000, 5, nil, false)}
	rates := currency.Rates{Base: "USD"}
	s, _ := Gather(txns, cats, 2025,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), rates)
	name := func(id string) string { return "Medical" }
	amt := func(v int64) string { return strconv.FormatInt(v, 10) }
	csv := string(GatherCSV(s, name, amt))
	if !contains(csv, "Charitable donations") || !contains(csv, "Gaps to resolve") {
		t.Errorf("csv missing sections:\n%s", csv)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
