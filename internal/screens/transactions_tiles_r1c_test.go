// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestTxnActiveFilterChipsExcludePeriodBounds(t *testing.T) {
	active := []txnfilter.ActiveFilter{
		{Field: txnfilter.FieldText, Value: "coffee"},
		{Field: txnfilter.FieldFrom, Value: "2026-10-01"},
		{Field: txnfilter.FieldTo, Value: "2026-10-31"},
		{Field: txnfilter.FieldCategory, Value: "food"},
	}
	got := txnActiveFilterChips(active, func(filter txnfilter.ActiveFilter) string { return filter.Value })
	want := []string{"coffee", "food"}
	labels := make([]string, len(got))
	for i, chip := range got {
		labels[i] = chip.Label
	}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("chip labels = %v, want %v", labels, want)
	}
}

func TestAddTxnTagTrimsAndAvoidsCaseDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		tag  string
		want []string
	}{
		{name: "adds trimmed tag", in: []string{"work"}, tag: "  reimbursable ", want: []string{"work", "reimbursable"}},
		{name: "does not add capitalization duplicate", in: []string{"Travel"}, tag: " travel ", want: []string{"Travel"}},
		{name: "ignores blank tag", in: []string{"work"}, tag: "  ", want: []string{"work"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := addTxnTag(tc.in, tc.tag); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("addTxnTag() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTxnHasTagIgnoresCaseAndWhitespace(t *testing.T) {
	if !txnHasTag([]string{"  Reimbursable "}, "reimbursable") {
		t.Fatal("txnHasTag() did not match a tag after trimming and case folding")
	}
	if txnHasTag([]string{"work"}, "travel") {
		t.Fatal("txnHasTag() matched an absent tag")
	}
}

func TestR1CCopyUsesUSSpellingAndAttentionCallout(t *testing.T) {
	if got := uistate.T("txp3.bulkCategorise"); got != "Categorize" {
		t.Fatalf("bulk category label = %q, want US spelling", got)
	}
	if got := uistate.T("r1c.reviewCallout", 249); got != "Review 249" {
		t.Fatalf("review callout = %q, want concise attention copy", got)
	}
}
