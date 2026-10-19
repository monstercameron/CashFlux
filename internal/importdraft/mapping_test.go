// SPDX-License-Identifier: MIT

package importdraft

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/pdftext"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestMappingChoicesAndRawRecords(t *testing.T) {
	for _, tc := range []struct {
		choice         string
		decimal, group rune
		format         importmap.NumberFormat
	}{{"dot", '.', ',', importmap.NumberFormatDotDecimal}, {"comma", ',', '.', importmap.NumberFormatCommaDecimal}, {"auto", 0, 0, importmap.NumberFormatAuto}, {"", 0, 0, importmap.NumberFormatAuto}} {
		got := StatementOptions("day-month", tc.choice)
		if got.DateOrder != "day-month" || got.NumberFormat.DecimalSep != tc.decimal || got.NumberFormat.GroupSep != tc.group || NumberFormatChoice(tc.choice) != tc.format {
			t.Errorf("%s: %+v", tc.choice, got)
		}
	}
	for _, tc := range []struct {
		text   string
		delim  rune
		header int
		value  string
	}{{"", 0, 0, ""}, {"\ufeffDate;Description;Amount\n2026-01-01;\"Cafe\nNorth\";1", ';', 3, "Cafe\nNorth"}, {"Date,Description\n2026-01-01,Shop", 0, 2, "Shop"}, {"Date,Desc\n\"bad", 0, 0, ""}, {"Date\r\n2026-01-01", 0, 1, ""}} {
		header, rows := RawHeaderAndRows(tc.text, tc.delim)
		if len(header) != tc.header {
			t.Fatalf("%q: %v", tc.text, header)
		}
		if tc.value != "" && rows[0][0][1] != tc.value {
			t.Fatal(rows)
		}
	}
	for _, tc := range []struct {
		header, keywords []string
		detected         int
		want             string
	}{{nil, nil, 3, "3"}, {[]string{"Memo", "POSTED Date"}, []string{"date", "memo"}, -1, "1"}, {nil, []string{"date"}, -1, "-1"}, {[]string{"Memo"}, []string{"missing"}, -1, "-1"}} {
		if got := GuessField(tc.header, tc.keywords, tc.detected); got != tc.want {
			t.Fatal(got)
		}
	}
	if !strings.Contains(StatementSystemPrompt(nil), "every category") || !strings.Contains(StatementSystemPrompt([]string{"Food", "Travel"}), "Food, Travel.") {
		t.Fatal("prompt")
	}
}

func TestRowMappingsPreserveDateCurrencyPrecisionAndBounds(t *testing.T) {
	date := time.Date(2026, 1, 31, 23, 0, 0, 0, time.FixedZone("west", -8*3600))
	for _, tc := range []struct {
		amount int64
		dec    int
		want   string
	}{{0, 2, "0.00"}, {-1, 0, "-1"}, {math.MaxInt64, 2, "92233720368547758.07"}, {math.MinInt64, 2, "-92233720368547758.08"}} {
		rows := StatementRows([]statement.Row{{Date: date, Description: "Shop", Amount: tc.amount, Category: "Food"}}, tc.dec)
		if rows[0].Date != "2026-01-31" || rows[0].Amount != tc.want || rows[0].Category != "Food" {
			t.Fatal(rows)
		}
		doc := DocumentRows(rows)
		if doc[0].Date != rows[0].Date || doc[0].Amount != tc.want || doc[0].Description != "Shop" || doc[0].Category != "Food" {
			t.Fatal(doc)
		}
	}
	if DocumentRows(nil) != nil || len(StatementRows(nil, 2)) != 0 {
		t.Fatal("nil rows")
	}
}

func TestCSVLocalizedDiagnostics(t *testing.T) {
	for _, tc := range []struct{ reason, key string }{{"amount is required", "tc4n6.amountRequired"}, {"Possible duplicate", "tc4n6.possibleDuplicate"}, {"too many decimal places", "tc4n6.amountPrecision"}, {"invalid amount", "tc4n6.amountInvalid"}, {"invalid cleared", "tc4n6.clearedInvalid"}, {"member_ids missing", "tc4n6.memberIDsInvalid"}, {"bad date", "tc4n6.dateInvalid"}, {"bad currency", "tc4n6.currencyInvalid"}, {"repeated id", "tc4n6.repeatedID"}, {"", "documents.skipReasonGeneric"}} {
		if got := CSVReason(tc.reason, translation); got != tc.key {
			t.Fatal(got)
		}
	}
	if CSVSkipDetail(nil, translation) != "" || MapWarningSummary(nil, translation) != "" || len(MapErrors(nil, translation)) != 0 {
		t.Fatal("empty diagnostics")
	}
	rows := []store.CSVRowError{{Line: 2, Reason: "invalid amount"}, {Line: 3}, {Line: 4}, {Line: 5}, {Line: 6}}
	got := CSVSkipDetail(rows, translation)
	if strings.Count(got, "documents.skipLine") != 3 || !strings.Contains(got, "documents.skipMore2") {
		t.Fatal(got)
	}
	emptyTranslation := func(key string, args ...any) string {
		if key == "tc4n6.amountInvalid" {
			return ""
		}
		return translation(key, args...)
	}
	if !strings.Contains(CSVSkipDetail(rows[:1], emptyTranslation), "skipReasonGeneric") {
		t.Fatal("fallback")
	}
	for _, field := range []string{"date", "amount", "debit", "credit", "balance", "unknown"} {
		got := MapErrors([]importmap.RowError{{Index: 2, Field: field}}, translation)
		if len(got) != 1 || got[0].Line != 3 || !strings.Contains(got[0].Reason, "rowFieldFailure") {
			t.Fatal(got)
		}
	}
	if got := MapWarningSummary([]importmap.RowWarning{{Field: "date"}, {Field: "other"}, {Field: "amount"}}, translation); got != "tc4n6.ambiguousDates2 tc4n6.ambiguousAmounts1" {
		t.Fatal(got)
	}
}

func TestWizardAndSourcePolicies(t *testing.T) {
	s := WizardSelection{Date: "0", Description: "1", Amount: "2", Debit: "-1", Credit: "bad", NumberFormat: "dot", DateOrder: string(statement.MonthDay)}
	p := s.Profile("manual", 2)
	if p.DateCol != 0 || p.DescCol != 1 || p.AmountCol != 2 || p.DebitCol != -1 || p.CreditCol != 0 || p.BalanceCol != -1 || p.CurrencyCol != -1 || p.Name != "manual" || p.Decimals != 2 {
		t.Fatal(p)
	}
	groups := [][][]string{{{"2026-01-31", "Shop", "-2.00"}}, {}, {{"bad", "Shop", "1"}}}
	if len(FlattenRecords(nil)) != 0 || len(FlattenRecords(groups)) != 2 {
		t.Fatal("flatten")
	}
	rows, failures, _ := ApplyProfile(p, groups)
	if len(rows) != 1 || rows[0].Amount != "-2.00" || rows[0].Category != "" || len(failures) != 1 {
		t.Fatal(rows, failures)
	}
	for _, tc := range []struct {
		data       string
		structured bool
	}{{"%PDF-1", true}, {"PK\x03\x04", true}, {"ofxheader:", true}, {"<?xml", true}, {"<?ofx", true}, {"<ofx", true}, {"Date,Desc", false}, {"", false}} {
		if StructuredSource(tc.data) != tc.structured {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		err  error
		want string
	}{{nil, ""}, {pdftext.ErrNoText, "scanned"}, {pdftext.ErrEncrypted, "scanned"}, {errors.New("PDF is encrypted"), "scanned"}, {errors.New("no text found"), "scanned"}, {errors.New("could not find date"), "mapping"}, {errors.New("invalid"), "error"}} {
		if got := StatementFailure(tc.err); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, now := range []time.Time{time.Date(2024, 1, 31, 23, 0, 0, 0, time.FixedZone("local", -5*3600)), time.Time{}} {
		task := Reminder("id", "title", now)
		if !task.Due.Equal(now.AddDate(0, 1, 0)) || task.ID != "id" || task.Title != "title" || task.Recurrence != domain.CadenceMonthly || task.Status != domain.StatusOpen || task.Priority != domain.PriorityMedium || task.Source != domain.SourceNudge {
			t.Fatal(task)
		}
	}
}

func TestCategoryAndExtractionAdapters(t *testing.T) {
	cats := []domain.Category{{ID: "a", Name: "Food"}, {ID: "b", Name: ""}}
	if got := CategoryNames(cats); !reflect.DeepEqual(got, []string{"Food", ""}) {
		t.Fatal(got)
	}
	if CategoryNames(nil) != nil {
		t.Fatal("nil")
	}
	if name, ok := CategoryName(cats, "a"); !ok || name != "Food" {
		t.Fatal(name, ok)
	}
	if _, ok := CategoryName(nil, "a"); ok {
		t.Fatal("missing")
	}
	rows := []extract.Row{{ID: "a", Description: "Shop", Date: "2026-01-01", Amount: "-1.00"}, {ID: "b", Category: "Food"}, {ID: "c", Currency: "EUR", Amount: "bad"}}
	inputs, ids := CategoryInputs(rows)
	if len(inputs) != 2 || inputs[1].Index != 1 || ids[1] != "c" || inputs[0].Description != "Shop" {
		t.Fatal(inputs, ids)
	}
	validated := ValidateExtraction(rows, []domain.Account{{ID: "bank"}}, "USD", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	if len(validated) != 3 || validated[0].Row.Currency != "USD" || validated[2].Row.Currency != "EUR" || validated[2].Valid || rows[0].Currency != "" {
		t.Fatal(validated)
	}
	if len(ValidateExtraction(nil, nil, "", time.Time{})) != 0 {
		t.Fatal("nil")
	}
}
