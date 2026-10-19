// SPDX-License-Identifier: MIT

package reports

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestReportCSVGuardsDynamicText(t *testing.T) {
	const formula = "=HYPERLINK(\"https://example.invalid\")"
	amount := func(int64) string { return "1.00" }
	name := func(string) string { return formula }
	tests := []struct {
		name string
		data []byte
		row  int
		col  int
	}{
		{name: "category", data: CategoryCSV([]CategorySpend{{CategoryID: "id", Amount: 100}}, name, amount), row: 1, col: 0},
		{name: "member", data: MemberCSV([]MemberSpend{{MemberID: "id", Amount: 100}}, name, amount), row: 1, col: 0},
		{name: "custom field value", data: CustomFieldCSV([]CustomFieldSpend{{Value: formula, Amount: 100}}, "Field", amount), row: 1, col: 0},
		{name: "custom field header", data: CustomFieldCSV(nil, formula, amount), row: 0, col: 0},
		{name: "payee", data: PayeeCSV([]PayeeTotal{{Name: formula, Amount: 100}}, amount), row: 1, col: 0},
		{name: "largest expense description", data: LargestExpensesCSV([]ExpenseItem{{Desc: formula, CategoryID: "id", Amount: 100, Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}, name, amount), row: 1, col: 0},
		{name: "largest expense category", data: LargestExpensesCSV([]ExpenseItem{{Desc: "safe", CategoryID: "id", Amount: 100, Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}, name, amount), row: 1, col: 1},
		{name: "year tax category", data: YearTaxCSV(YearTaxSummary{Year: 2026, Rows: []YearTaxRow{{CategoryID: "id", Income: 100}}}, name, amount), row: 2, col: 0},
		{name: "deductible category", data: DeductibleCSV(DeductibleSummary{Rows: []DeductibleRow{{CategoryID: "id", Amount: 100}}}, name, amount), row: 1, col: 0},
		{name: "investment account", data: InvestmentPerformanceCSV([]AccountPerformance{{Name: formula, Invested: 100}}, amount), row: 1, col: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := csv.NewReader(strings.NewReader(string(tt.data)))
			reader.FieldsPerRecord = -1
			records, err := reader.ReadAll()
			if err != nil {
				t.Fatalf("parse CSV: %v", err)
			}
			if got := records[tt.row][tt.col]; got != "'"+formula {
				t.Errorf("text cell = %q, want guarded %q", got, "'"+formula)
			}
		})
	}
}
