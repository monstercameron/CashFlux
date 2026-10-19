// SPDX-License-Identifier: MIT

package subscriptions

import (
	"encoding/csv"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCSV(t *testing.T) {
	subs := []Subscription{
		{
			Name: "Netflix", Cadence: CadenceMonthly, Amount: 1599, Currency: "USD",
			NextRenewal: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Name: "Domain", Cadence: CadenceYearly, Amount: 1200, Currency: "USD",
			NextRenewal: time.Date(2027, time.June, 11, 0, 0, 0, 0, time.UTC),
		},
	}
	amount := func(v int64) string { return strconv.FormatInt(v, 10) } // raw minor units for assertion

	out := string(CSV(subs, amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "Name,Cadence,Charge,Monthly,Annual,Next renewal" {
		t.Errorf("header = %q", lines[0])
	}
	// Netflix: monthly charge 1599 → monthly 1599, annual 1599*12 = 19188.
	if strings.TrimRight(lines[1], "\r") != "Netflix,monthly,1599,1599,19188,2026-07-01" {
		t.Errorf("netflix row = %q", lines[1])
	}
	// Domain: yearly charge 1200 → monthly 100, annual 1200.
	if strings.TrimRight(lines[2], "\r") != "Domain,yearly,1200,100,1200,2027-06-11" {
		t.Errorf("domain row = %q", lines[2])
	}
}

func TestCSVGuardsNameAndCadence(t *testing.T) {
	const formula = "=HYPERLINK(\"https://example.invalid\")"
	subs := []Subscription{{Name: formula, Cadence: Cadence(formula), Amount: 100, NextRenewal: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)}}
	data := CSV(subs, func(int64) string { return "1.00" })
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	for _, col := range []int{0, 1} {
		if got := records[1][col]; got != "'"+formula {
			t.Errorf("text column %d = %q, want guarded value", col, got)
		}
	}
	if got := records[1][2]; got != "1.00" {
		t.Errorf("amount = %q, want numeric 1.00", got)
	}
}
