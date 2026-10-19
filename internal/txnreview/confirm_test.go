// SPDX-License-Identifier: MIT

package txnreview

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

func TestConfirm(t *testing.T) {
	for _, reviewed := range []bool{false, true} {
		for _, category := range []string{"food", ""} {
			t.Run(category+map[bool]string{true: "/reviewed", false: "/unreviewed"}[reviewed], func(t *testing.T) {
				original := domain.Transaction{ID: "one", Desc: "Market", Reviewed: reviewed, CategoryID: "old", Tags: []string{"keep", reviewqueue.ReviewTag, " " + reviewqueue.ReviewTag + " "}, Splits: []domain.CategorySplit{{CategoryID: "old", Amount: money.New(-100, "USD")}}}
				got := Confirm(original, category)
				if !got.Reviewed || got.CategoryID != category || len(got.Splits) != 0 || !reflect.DeepEqual(got.Tags, []string{"keep"}) {
					t.Fatalf("confirmation = %+v", got)
				}
				got.Tags[0] = "changed"
				if original.Tags[0] != "keep" || len(original.Splits) != 1 {
					t.Fatal("mutated original")
				}
				if !reflect.DeepEqual(Confirm(Confirm(original, category), category), Confirm(original, category)) {
					t.Fatal("confirmation is not idempotent")
				}
			})
		}
	}
}
