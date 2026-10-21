// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBudgetTemplateReviewLetsUserChooseBucket(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	chosen := budgeting.BucketWants
	f.Render(ui.CreateElement(tmplReviewRow, tmplReviewRowProps{
		ID: "vegas", Label: "Las Vegas trip", AmountStr: "$100.00", Bucket: budgeting.BucketWants,
		OnToggle: func(string) {}, OnBucketChange: func(_ string, value string) {
			if value == "0" {
				chosen = budgeting.BucketNeeds
			}
		},
	}))

	selectNode := byTestID(f, "select", "tmpl-bucket-vegas")
	if selectNode == nil {
		t.Fatal("review row has no bucket selector")
	}
	if got := selectNode.Text(); got != "Needs Wants Savings" {
		t.Fatalf("bucket choices = %q, want all three choices", got)
	}
	selectNode.Change("0")
	if chosen != budgeting.BucketNeeds {
		t.Fatalf("selected bucket = %s, want Needs", chosen)
	}
}
