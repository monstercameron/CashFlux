package reports

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// cfExpense builds a non-transfer expense in USD with a single custom field set.
func cfExpense(key string, val any, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{
		Amount: money.New(-major*100, "USD"),
		Date:   on,
		Custom: map[string]any{key: val},
	}
}

// cfExpenseNoCustom builds an expense with no custom fields at all.
func cfExpenseNoCustom(major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Amount: money.New(-major*100, "USD"), Date: on}
}

func TestByCustomFieldSelect(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		cfExpense("project", "Freelance", 200, dt(2026, time.June, 5)),
		cfExpense("project", "Freelance", 50, dt(2026, time.June, 15)),
		cfExpense("project", "Personal", 100, dt(2026, time.June, 10)),
	}
	got, err := ByCustomField(txns, "project", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(got), got)
	}
	// Freelance (250) before Personal (100).
	if got[0].Value != "Freelance" || got[0].Amount != 25000 {
		t.Errorf("row 0 = %+v, want Freelance 25000", got[0])
	}
	if got[1].Value != "Personal" || got[1].Amount != 10000 {
		t.Errorf("row 1 = %+v, want Personal 10000", got[1])
	}
}

func TestByCustomFieldBool(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		cfExpense("reimbursable", true, 300, dt(2026, time.June, 1)),
		cfExpense("reimbursable", false, 150, dt(2026, time.June, 2)),
		cfExpense("reimbursable", true, 50, dt(2026, time.June, 3)),
	}
	got, err := ByCustomField(txns, "reimbursable", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(got), got)
	}
	// Yes (350) > No (150)
	if got[0].Value != "Yes" || got[0].Amount != 35000 {
		t.Errorf("row 0 = %+v, want Yes 35000", got[0])
	}
	if got[1].Value != "No" || got[1].Amount != 15000 {
		t.Errorf("row 1 = %+v, want No 15000", got[1])
	}
}

func TestByCustomFieldNumber(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		cfExpense("priority", float64(1), 100, dt(2026, time.June, 1)),
		cfExpense("priority", float64(2), 200, dt(2026, time.June, 2)),
		cfExpense("priority", float64(1), 50, dt(2026, time.June, 3)),
		// json.Number variant
		cfExpense("priority", json.Number("2"), 80, dt(2026, time.June, 4)),
	}
	got, err := ByCustomField(txns, "priority", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(got), got)
	}
	// "2" = 200+80 = 280, "1" = 100+50 = 150
	if got[0].Value != "2" || got[0].Amount != 28000 {
		t.Errorf("row 0 = %+v, want 2/28000", got[0])
	}
	if got[1].Value != "1" || got[1].Amount != 15000 {
		t.Errorf("row 1 = %+v, want 1/15000", got[1])
	}
}

func TestByCustomFieldMissingValueBucket(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		cfExpenseNoCustom(120, dt(2026, time.June, 1)),        // no field at all
		cfExpense("project", nil, 80, dt(2026, time.June, 2)), // explicit nil
		cfExpense("project", "Freelance", 200, dt(2026, time.June, 3)),
	}
	got, err := ByCustomField(txns, "project", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(got), got)
	}
	// Freelance (200) > "" (200 too, but deterministic tiebreak Value="" < "Freelance")
	// Actually "" < "F" so "" sorts first on tiebreak... but both are 200*100=20000.
	// Alphabetic tiebreak: "" < "Freelance", so "" first.
	if got[0].Value != "" || got[0].Amount != 20000 {
		t.Errorf("row 0 = %+v, want empty-value 20000", got[0])
	}
	if got[1].Value != "Freelance" || got[1].Amount != 20000 {
		t.Errorf("row 1 = %+v, want Freelance 20000", got[1])
	}
}

func TestByCustomFieldExcludesTransfersIncomeOutOfRange(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		cfExpense("project", "Work", 100, dt(2026, time.June, 5)),
		// income — excluded
		{Amount: money.New(5000, "USD"), Date: dt(2026, time.June, 5), Custom: map[string]any{"project": "Work"}},
		// transfer — excluded
		{Amount: money.New(-3000, "USD"), TransferAccountID: "acc2", Date: dt(2026, time.June, 5), Custom: map[string]any{"project": "Work"}},
		// out of range — excluded
		cfExpense("project", "Work", 50, dt(2026, time.May, 31)),
	}
	got, err := ByCustomField(txns, "project", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d groups, want 1 (only the in-range expense): %+v", len(got), got)
	}
	if got[0].Value != "Work" || got[0].Amount != 10000 {
		t.Errorf("row 0 = %+v, want Work 10000", got[0])
	}
}

func TestByCustomFieldDeterministicOrder(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	// Two groups with the same total: "beta" and "alpha" — "alpha" should come
	// first on tiebreak because "alpha" < "beta".
	txns := []domain.Transaction{
		cfExpense("tag", "beta", 100, dt(2026, time.June, 1)),
		cfExpense("tag", "alpha", 100, dt(2026, time.June, 2)),
	}
	got, err := ByCustomField(txns, "tag", start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2", len(got))
	}
	if got[0].Value != "alpha" {
		t.Errorf("tiebreak: want alpha first, got %q", got[0].Value)
	}
}

func TestByCustomFieldEmpty(t *testing.T) {
	got, err := ByCustomField(nil, "project", dt(2026, time.June, 1), dt(2026, time.July, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input should yield no rows, got %+v", got)
	}
}

func TestNormaliseCustomValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{true, "Yes"},
		{false, "No"},
		{float64(3), "3"},
		{float64(3.14), "3.14"},
		{float64(1000), "1000"},
		{json.Number("2.50"), "2.5"},
		{json.Number("7"), "7"},
		{"hello", "hello"},
		{"", ""},
	}
	for _, c := range cases {
		got := normaliseCustomValue(c.in)
		if got != c.want {
			t.Errorf("normaliseCustomValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCustomFieldCSV(t *testing.T) {
	rows := []CustomFieldSpend{
		{Value: "Freelance", Amount: 25000},
		{Value: "Personal", Amount: 10000},
		{Value: "", Amount: 5000},
	}
	amount := func(v int64) string { return strconv.FormatInt(v/100, 10) }

	out := string(CustomFieldCSV(rows, "Project", amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + 3): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "Project,Amount" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.TrimRight(lines[1], "\r") != "Freelance,250" {
		t.Errorf("freelance row = %q", lines[1])
	}
	if strings.TrimRight(lines[2], "\r") != "Personal,100" {
		t.Errorf("personal row = %q", lines[2])
	}
	if strings.TrimRight(lines[3], "\r") != ",50" {
		t.Errorf("empty-value row = %q", lines[3])
	}
}
