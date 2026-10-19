// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/nlfilter"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestNaturalLanguagePreviewIncludesQueryDateBounds(t *testing.T) {
	parsed, ok := nlfilter.Parse("coffee last week", nlfilter.Context{
		Now:       time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC),
		WeekStart: time.Sunday,
	})
	if !ok {
		t.Fatal("Parse() did not recognize the date clause")
	}
	if parsed.Text != "coffee" || parsed.From != "2026-10-25" || parsed.To != "2026-10-31" {
		t.Fatalf("parsed query = %+v, want coffee from 2026-10-25 through 2026-10-31", parsed)
	}

	var preview []string
	for _, criterion := range parsed.AllCriteria() {
		switch criterion.Field {
		case txnfilter.FieldFrom:
			preview = append(preview, uistate.T("transactions.chipFrom", formatTxnChipDateWith(criterion.Value, func(date time.Time) string { return date.Format("Jan 2") })))
		case txnfilter.FieldTo:
			preview = append(preview, uistate.T("transactions.chipTo", formatTxnChipDateWith(criterion.Value, func(date time.Time) string { return date.Format("Jan 2") })))
		case txnfilter.FieldText:
			preview = append(preview, txnSearchChipLabel(criterion.Value))
		}
	}
	got := strings.Join(preview, " · ")
	for _, want := range []string{"From Oct 25", "Through Oct 31", "Search: coffee"} {
		if !strings.Contains(got, want) {
			t.Errorf("natural-language preview %q does not mention %q", got, want)
		}
	}
}
