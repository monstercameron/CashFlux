// SPDX-License-Identifier: MIT

package bills

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCSV(t *testing.T) {
	bs := []Bill{
		{AccountID: "visa", Name: "Visa", Amount: money.New(5000, "USD"), DueDate: d(2026, time.June, 15), DaysUntil: 5},
		{AccountID: "loan", Name: "Car loan", Amount: money.New(30000, "USD"), DueDate: d(2026, time.July, 1), DaysUntil: 21},
	}
	amount := func(m money.Money) string { return strconv.FormatInt(m.Amount, 10) }

	out := string(CSV(bs, amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "Name,Due date,Days until,Amount" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.TrimRight(lines[1], "\r") != "Visa,2026-06-15,5,5000" {
		t.Errorf("visa row = %q", lines[1])
	}
	// Comma in the name must be quoted by encoding/csv.
	if strings.TrimRight(lines[2], "\r") != "Car loan,2026-07-01,21,30000" {
		t.Errorf("loan row = %q", lines[2])
	}
}
