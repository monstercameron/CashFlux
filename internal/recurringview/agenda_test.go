// SPDX-License-Identifier: MIT

package recurringview

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildAgendaSpanMissingFXStaysUnavailable(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	span := BuildAgendaSpan(AgendaSpanInput{
		Recurring: []domain.Recurring{
			{ID: "euro-bill", Label: "Euro bill", Amount: money.New(-12345, "EUR"), Cadence: domain.CadenceMonthly, NextDue: due},
			{ID: "euro-pay", Label: "Euro pay", Amount: money.New(67890, "EUR"), Cadence: domain.CadenceMonthly, NextDue: due},
		},
		Now: now, From: now, Until: due, Base: "USD", Rates: currency.Rates{Base: "USD"},
	})
	if len(span.Occurrences) != 2 {
		t.Fatalf("occurrences = %d, want one bill and one paycheck", len(span.Occurrences))
	}
	for _, occurrence := range span.Occurrences {
		if occurrence.AmountAvailable || occurrence.AmountError == nil {
			t.Errorf("missing FX appeared available: %+v", occurrence)
		}
		if occurrence.Amount.Amount != 0 || occurrence.Amount.Currency != "USD" {
			t.Errorf("unavailable amount carries a plausible base value: %+v", occurrence.Amount)
		}
		if occurrence.Amount.Amount == 12345 || occurrence.Amount.Amount == -12345 || occurrence.Amount.Amount == 67890 {
			t.Errorf("source minor units leaked into base currency: %+v", occurrence.Amount)
		}
	}
}

func TestBuildAgendaSpanMergesObligationsKeepsMonthEdgesAndStableTies(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	recurring := []domain.Recurring{
		{ID: "rent", Label: "Rent", Amount: money.New(-12000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "pay-b", Label: "Pay B", Amount: money.New(40000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "pay-a", Label: "Pay A", Amount: money.New(50000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)},
	}
	accounts := []domain.Account{{
		ID: "home-loan", Name: "Home loan", Class: domain.ClassLiability,
		DueDayOfMonth: 15, MinPayment: money.New(-12000, "USD"),
	}}
	originalDue := recurring[0].NextDue
	originalPayment := accounts[0].MinPayment
	span := BuildAgendaSpan(AgendaSpanInput{
		Accounts: accounts, Recurring: recurring, Now: now, From: from, Until: until,
		Base: "USD", Rates: currency.Rates{Base: "USD"},
	})

	if recurring[0].NextDue != originalDue || accounts[0].MinPayment != originalPayment {
		t.Fatal("BuildAgendaSpan mutated its caller's schedule or account slice")
	}
	if len(span.Occurrences) != 6 {
		t.Fatalf("occurrences = %d, want two merged bills and four paychecks", len(span.Occurrences))
	}
	want := []struct {
		date    string
		name    string
		account string
		amount  int64
	}{
		{"2026-10-15", "Rent", "recurring:rent", -12000},
		{"2026-10-15", "Pay B", "", 40000},
		{"2026-10-15", "Pay A", "", 50000},
		{"2026-11-15", "Rent", "recurring:rent", -12000},
		{"2026-11-15", "Pay B", "", 40000},
		{"2026-11-15", "Pay A", "", 50000},
	}
	for i, expected := range want {
		got := span.Occurrences[i]
		if got.Date.Format("2006-01-02") != expected.date || got.Name != expected.name || got.AccountID != expected.account ||
			!got.AmountAvailable || got.Amount.Amount != expected.amount {
			t.Errorf("occurrence %d = %+v, want %s/%s/%s/%d", i, got, expected.date, expected.name, expected.account, expected.amount)
		}
		if expected.account != "" && got.AnchorAccountID != "home-loan" {
			t.Errorf("merged bill did not retain liability anchor: %+v", got)
		}
		if got.Past != (expected.date == "2026-10-15") {
			t.Errorf("past flag for %s = %v", expected.date, got.Past)
		}
	}
	forward := ForwardAgenda(span, now)
	if len(forward.Occurrences) != 3 || forward.Occurrences[0].Date.Format("2006-01-02") != "2026-11-15" {
		t.Fatalf("forward projection did not omit past calendar rows: %+v", forward.Occurrences)
	}
	if len(span.Occurrences) != 6 {
		t.Fatal("ForwardAgenda mutated the complete calendar span")
	}

	edge := BuildAgendaSpan(AgendaSpanInput{
		Accounts: []domain.Account{{ID: "card", Name: "Card", Class: domain.ClassLiability,
			DueDayOfMonth: 31, MinPayment: money.New(-1000, "USD")}},
		Now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		Base: "USD", Rates: currency.Rates{Base: "USD"},
	})
	if len(edge.Occurrences) != 3 || edge.Occurrences[1].Date.Format("2006-01-02") != "2026-02-28" ||
		edge.Occurrences[2].Date.Format("2006-01-02") != "2026-03-31" {
		t.Fatalf("month-end statement cadence did not clamp and recover: %+v", edge.Occurrences)
	}
}

func TestAgendaSettlementUsesEvidenceAndScheduleProgress(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	recurring := []domain.Recurring{
		{ID: "stalled", Label: "Stalled", Amount: money.New(-100, "USD"), Cadence: domain.CadenceWeekly,
			NextDue: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)},
		{ID: "marked", Label: "Marked", Amount: money.New(-200, "USD"), Cadence: domain.CadenceWeekly,
			NextDue: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)},
		{ID: "matched", Label: "Matched", Amount: money.New(-300, "USD"), Cadence: domain.CadenceWeekly,
			NextDue: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)},
		{ID: "advanced", Label: "Advanced", Amount: money.New(-400, "USD"), Cadence: domain.CadenceWeekly,
			NextDue: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)},
	}
	from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	span := BuildAgendaSpan(AgendaSpanInput{
		Recurring: recurring, Now: now, From: from, Until: now,
		Base: "USD", Rates: currency.Rates{Base: "USD"},
	})
	settled := WithSettlementEvidence(span, []AgendaSettlementEvidence{
		{BillID: "recurring:marked", DueDate: recurring[1].NextDue, Paid: true},
		{BillID: "recurring:matched", RecurringID: "matched", DueDate: recurring[2].NextDue, Matched: true},
	})
	byIDAndDate := make(map[string]AgendaOccurrence, len(settled.Occurrences))
	for _, occurrence := range settled.Occurrences {
		byIDAndDate[occurrence.RecurringID+"|"+occurrence.Date.Format("2006-01-02")] = occurrence
	}
	assert := func(id string, due time.Time, paid, missed bool) {
		t.Helper()
		got, ok := byIDAndDate[id+"|"+due.Format("2006-01-02")]
		if !ok {
			t.Fatalf("occurrence %s on %s missing", id, due.Format("2006-01-02"))
		}
		if got.Paid != paid || got.Missed != missed {
			t.Errorf("%s on %s: paid=%v missed=%v, want paid=%v missed=%v", id, due.Format("2006-01-02"), got.Paid, got.Missed, paid, missed)
		}
	}
	assert("stalled", recurring[0].NextDue, false, true)
	assert("marked", recurring[1].NextDue, true, false)
	assert("matched", recurring[2].NextDue, true, false)
	assert("advanced", recurring[3].NextDue.AddDate(0, 0, -7), true, false)
	assert("advanced", recurring[3].NextDue, false, true)
	for _, occurrence := range span.Occurrences {
		if occurrence.RecurringID == "marked" && occurrence.Date.Equal(recurring[1].NextDue) {
			if occurrence.Paid || !occurrence.Missed {
				t.Fatal("WithSettlementEvidence mutated the source span")
			}
		}
	}
	if !OccurrenceSettled(recurring[3], recurring[3].NextDue.AddDate(0, 0, -7), AgendaSettlementEvidence{}) {
		t.Fatal("schedule advancement did not settle an earlier occurrence")
	}
}

func TestBuildAgendaSpanCapsIncomeAndUnknownCadenceProgress(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	span := BuildAgendaSpan(AgendaSpanInput{
		Recurring: []domain.Recurring{
			{ID: "daily", Label: "Daily pay", Amount: money.New(100, "USD"), Cadence: domain.CadenceDaily, NextDue: now},
			{ID: "degenerate", Label: "Unknown", Amount: money.New(1, "USD"), Cadence: domain.RecurringCadence("never"), NextDue: now},
			{ID: "overflow", Label: "Too large", Amount: money.New(math.MaxInt64, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
		},
		Now: now, From: now, Until: now.AddDate(0, 2, 0), Base: "USD", Rates: currency.Rates{Base: "USD"},
	})
	counts := map[string]int{}
	for _, occurrence := range span.Occurrences {
		counts[occurrence.RecurringID]++
	}
	if counts["daily"] != agendaIncomeOccurrencesPerFlow || counts["degenerate"] != 3 {
		t.Fatalf("income cap or unknown-cadence monthly progress changed: daily=%d degenerate=%d", counts["daily"], counts["degenerate"])
	}
	if counts["overflow"] != 3 {
		t.Fatalf("large but representable base-currency paycheck missing: %+v", counts)
	}
}

func TestProjectAgendaMonthUsesSignedFactsAndRejectsUnknownFX(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	month := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	span := AgendaSpan{Base: "USD", Occurrences: []AgendaOccurrence{
		{Date: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), Name: "Bill", Amount: money.New(-12000, "USD"), AmountAvailable: true},
		{Date: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), RecurringID: "pay", Name: "Paycheck", Amount: money.New(50000, "USD"), AmountAvailable: true, Income: true},
		{Date: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), Name: "Next month", Amount: money.Zero("USD"), AmountAvailable: false},
	}}
	projection, ok := ProjectAgendaMonth(span, 10000, month, now)
	if !ok {
		t.Fatal("known current-month amounts did not produce a projection")
	}
	closing := make(map[string]int64, len(projection.Days))
	for _, day := range projection.Days {
		closing[day.Date.Format("2006-01-02")] = day.ClosingMinor
	}
	if closing["2026-10-22"] != -2000 || closing["2026-10-23"] != 48000 {
		t.Fatalf("signed bill/paycheck events projected incorrectly: %v", closing)
	}

	span.Occurrences = append(span.Occurrences, AgendaOccurrence{
		Date: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC), Name: "Unknown bill", Amount: money.Zero("USD"),
	})
	if _, ok := ProjectAgendaMonth(span, 10000, month, now); ok {
		t.Fatal("projection treated an unknown in-month amount as zero")
	}
}
