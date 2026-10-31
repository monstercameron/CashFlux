// SPDX-License-Identifier: MIT

package services

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var noRates = currency.Rates{Base: "USD"}

func TestOfKeepsOnlyServicesInStoredOrder(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "rent", Label: "Rent"},
		{ID: "xfinity", Label: "Comcast Xfinity", Service: true},
		{ID: "power", Label: "Power", Service: true},
	}
	got := Of(recs)
	if len(got) != 2 || got[0].ID != "xfinity" || got[1].ID != "power" {
		t.Fatalf("Of = %+v, want xfinity then power", got)
	}
}

func TestMonthlyTotalSumsActiveServicesAndSkipsPaused(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "xfinity", Label: "Xfinity", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-8000, "USD")},
		{ID: "power", Label: "Power", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-5000, "USD")},
		{ID: "gym", Label: "Gym", Service: true, Paused: true, Cadence: domain.CadenceMonthly, Amount: money.New(-3000, "USD")},
		{ID: "rent", Label: "Rent", Cadence: domain.CadenceMonthly, Amount: money.New(-150000, "USD")},
	}
	got, err := MonthlyTotal(recs, "USD", noRates)
	if err != nil {
		t.Fatal(err)
	}
	if got != 13000 {
		t.Fatalf("MonthlyTotal = %d, want 13000 (rent and the paused gym are left out)", got)
	}
}

func TestMonthlyTotalNormalisesCadence(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "yearly", Label: "Domain", Service: true, Cadence: domain.CadenceYearly, Amount: money.New(-12000, "USD")},
	}
	got, err := MonthlyTotal(recs, "USD", noRates)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000 {
		t.Fatalf("yearly 120.00 as a monthly total = %d, want 1000", got)
	}
}

func TestNewMonthlyStoresASpendingServiceMarkedAsAService(t *testing.T) {
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	r := NewMonthly(Draft{ID: "svc-1", Name: "Comcast Xfinity", Provider: "Comcast",
		Currency: "USD", MonthlyMinor: 8000, NextDue: due})
	if !r.Service || r.Provider != "Comcast" || r.Label != "Comcast Xfinity" {
		t.Fatalf("NewMonthly fields = %+v", r)
	}
	if r.Amount.Amount != -8000 || r.Cadence != domain.CadenceMonthly || !r.NextDue.Equal(due) {
		t.Fatalf("NewMonthly amount/cadence/due = %d %q %v", r.Amount.Amount, r.Cadence, r.NextDue)
	}
}

func TestMonthlyTotalTable(t *testing.T) {
	eurRates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	tests := []struct {
		name    string
		recs    []domain.Recurring
		rates   currency.Rates
		want    int64
		wantErr bool
	}{
		{"empty list", nil, noRates, 0, false},
		{"no services in the list", []domain.Recurring{
			{ID: "rent", Label: "Rent", Cadence: domain.CadenceMonthly, Amount: money.New(-150000, "USD")},
		}, noRates, 0, false},
		{"mixed currencies convert to base", []domain.Recurring{
			{ID: "us", Label: "US plan", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-5000, "USD")},
			{ID: "eu", Label: "EU plan", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-1000, "EUR")},
		}, eurRates, 6100, false},
		{"missing FX rate is an error", []domain.Recurring{
			{ID: "us", Label: "US plan", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-5000, "USD")},
			{ID: "uk", Label: "UK plan", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-1000, "GBP")},
		}, eurRates, 0, true},
		{"a paused foreign service needs no rate", []domain.Recurring{
			{ID: "uk", Label: "UK plan", Service: true, Paused: true, Cadence: domain.CadenceMonthly, Amount: money.New(-1000, "GBP")},
		}, eurRates, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MonthlyTotal(tt.recs, "USD", tt.rates)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MonthlyTotal error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("MonthlyTotal = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseDraft(t *testing.T) {
	now := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		in      [4]string
		want    Draft
		wantErr error
	}{
		{"valid", [4]string{"  Xfinity ", " Comcast ", "80.50", "USD"},
			Draft{Name: "Xfinity", Provider: "Comcast", Currency: "USD", MonthlyMinor: 8050,
				NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, nil},
		{"provider is optional", [4]string{"Power", "", "50", "USD"},
			Draft{Name: "Power", Currency: "USD", MonthlyMinor: 5000, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, nil},
		{"zero decimal currency", [4]string{"Phone", "", "3000", "JPY"},
			Draft{Name: "Phone", Currency: "JPY", MonthlyMinor: 3000, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, nil},
		{"blank name", [4]string{"   ", "", "10", "USD"}, Draft{}, ErrNameRequired},
		{"zero amount", [4]string{"Gym", "", "0", "USD"}, Draft{}, ErrAmountInvalid},
		{"negative amount", [4]string{"Gym", "", "-5", "USD"}, Draft{}, ErrAmountInvalid},
		{"blank amount", [4]string{"Gym", "", "  ", "USD"}, Draft{}, ErrAmountInvalid},
		{"not a number", [4]string{"Gym", "", "ten", "USD"}, Draft{}, ErrAmountInvalid},
		{"blank currency", [4]string{"Gym", "", "10", "  "}, Draft{}, ErrCurrencyInvalid},
		{"two-letter currency", [4]string{"Gym", "", "10", "US"}, Draft{}, ErrCurrencyInvalid},
		{"four-letter currency", [4]string{"Gym", "", "10", "USDX"}, Draft{}, ErrCurrencyInvalid},
		{"digits in currency", [4]string{"Gym", "", "10", "U5D"}, Draft{}, ErrCurrencyInvalid},
		{"non-ASCII currency", [4]string{"Gym", "", "10", "\u00dcSD"}, Draft{}, ErrCurrencyInvalid},
		{"currency is checked before the amount", [4]string{"Gym", "", "ten", "xx"}, Draft{}, ErrCurrencyInvalid},
		{"name is checked before the currency", [4]string{"", "", "10", "xx"}, Draft{}, ErrNameRequired},
		{"name is checked before the amount", [4]string{"", "", "ten", "USD"}, Draft{}, ErrNameRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDraft(tt.in[0], tt.in[1], tt.in[2], tt.in[3], now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseDraft error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ParseDraft = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewMonthlyNeverStoresAServiceAsIncome(t *testing.T) {
	tests := []struct {
		name    string
		monthly int64
		want    int64
	}{
		{"positive cost is negated", 8000, -8000},
		{"negative draft is still spending", -8000, -8000},
		{"zero stays zero", 0, 0},
		{"largest positive cost", math.MaxInt64, -math.MaxInt64},
		{"MinInt64 does not overflow into income", math.MinInt64, -math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMonthly(Draft{ID: "x", Name: "Svc", Currency: "USD", MonthlyMinor: tt.monthly}).Amount.Amount; got != tt.want {
				t.Fatalf("Amount = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewMonthlyCheckedRejectsNonPositiveCosts(t *testing.T) {
	for _, minor := range []int64{0, -1, math.MinInt64} {
		if _, err := NewMonthlyChecked(Draft{Name: "Svc", Currency: "USD", MonthlyMinor: minor}); !errors.Is(err, ErrAmountInvalid) {
			t.Fatalf("MonthlyMinor %d: err = %v, want ErrAmountInvalid", minor, err)
		}
	}
	got, err := NewMonthlyChecked(Draft{ID: "x", Name: "Svc", Currency: "USD", MonthlyMinor: 500})
	if err != nil || got.Amount.Amount != -500 || !got.Service {
		t.Fatalf("NewMonthlyChecked = %+v, %v", got, err)
	}
}

func TestParseDraftNormalisesCurrencyAndWrapsErrors(t *testing.T) {
	now := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	got, err := ParseDraft("Phone", "", "3000", " jpy ", now)
	if err != nil || got.Currency != "JPY" || got.MonthlyMinor != 3000 {
		t.Fatalf("lower-case currency: %+v, %v", got, err)
	}
	if got.ID != "" {
		t.Fatalf("ParseDraft set ID %q; the caller assigns it", got.ID)
	}

	_, err = ParseDraft(" ", "", "10", "USD", now)
	if !errors.Is(err, ErrNameRequired) || err == ErrNameRequired || !strings.Contains(err.Error(), "parse draft") {
		t.Fatalf("name error = %v, want ErrNameRequired wrapped with context", err)
	}

	_, err = ParseDraft("Gym", "", "ten", "USD", now)
	if !errors.Is(err, ErrAmountInvalid) || !errors.Is(err, money.ErrInvalidAmount) || !strings.Contains(err.Error(), `"ten"`) {
		t.Fatalf("amount error = %v, want ErrAmountInvalid wrapping the parse failure", err)
	}
}
