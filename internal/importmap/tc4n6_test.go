// SPDX-License-Identifier: MIT

package importmap

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/statement"
)

func TestDateOrderAppliesAndPersists(t *testing.T) {
	profile := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, BalanceCol: -1, Decimals: 2, DateOrder: statement.DayMonth}
	rows, failures, warnings := ApplyWithDiagnostics(profile, [][]string{
		{"13/03/2024", "Rent", "1.00"},
		{"05/03/2024", "Cafe", "2.00"},
		{"03/13/2024", "US date", "3.00"},
	})
	if len(failures) != 0 || len(rows) != 3 || len(warnings) != 1 {
		t.Fatalf("rows=%+v failures=%+v warnings=%+v", rows, failures, warnings)
	}
	if got := rows[0].Date.Format("2006-01-02"); got != "2024-03-13" {
		t.Fatalf("unambiguous day-first date = %s, want 2024-03-13", got)
	}
	if got := rows[1].Date.Format("2006-01-02"); got != "2024-03-05" {
		t.Fatalf("ambiguous day-first date = %s, want 2024-03-05", got)
	}
	if got := rows[2].Date.Format("2006-01-02"); got != "2024-03-13" {
		t.Fatalf("unambiguous month-first date = %s, want 2024-03-13", got)
	}
	if warnings[0].Warning == "" {
		t.Fatal("ambiguous day-first date should be reported")
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var restored Profile
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.DateOrder != statement.DayMonth {
		t.Fatalf("restored date order = %q, want %q", restored.DateOrder, statement.DayMonth)
	}
}

func TestAmbiguousNumericDateWarningSupportsShortForms(t *testing.T) {
	profile := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, BalanceCol: -1, Decimals: 2}
	for _, value := range []string{"5/3/2024", "05/03/24"} {
		t.Run(value, func(t *testing.T) {
			rows, failures, warnings := ApplyWithDiagnostics(profile, [][]string{{value, "Cafe", "1.00"}})
			if len(rows) != 1 || len(failures) != 0 || len(warnings) != 1 {
				t.Fatalf("rows=%+v failures=%+v warnings=%+v", rows, failures, warnings)
			}
		})
	}
}

func TestThreeDecimalCommaSeparators(t *testing.T) {
	profile := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, BalanceCol: -1, Decimals: 3}
	rows, failures, warnings := ApplyWithDiagnostics(profile, [][]string{
		{"2024-01-01", "fractional KWD", "0,500"},
		{"2024-01-02", "ambiguous KWD", "12,500"},
	})
	if len(failures) != 0 || len(rows) != 2 {
		t.Fatalf("rows=%+v failures=%+v", rows, failures)
	}
	if rows[0].Amount != 500 {
		t.Errorf("0,500 = %d minor units, want 500", rows[0].Amount)
	}
	if rows[1].Amount != 12_500_000 {
		t.Errorf("12,500 = %d minor units, want grouped 12,500 KWD", rows[1].Amount)
	}
	if len(warnings) != 1 || warnings[0].Index != 1 {
		t.Fatalf("warnings=%+v, want warning for ambiguous second row", warnings)
	}
}

func TestDateOrderMonthDayRemainsDefault(t *testing.T) {
	rows, failures, warnings := ApplyWithDiagnostics(Profile{DateCol: 0, DescCol: 1, AmountCol: 2, BalanceCol: -1, Decimals: 2},
		[][]string{{"05/03/2024", "Cafe", "1.00"}})
	if len(failures) != 0 || len(rows) != 1 || len(warnings) != 1 || rows[0].Date.Month() != time.May {
		t.Fatalf("legacy default parse changed: rows=%+v failures=%+v warnings=%+v", rows, failures, warnings)
	}
}
