// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCheckpointBusyDisablesReviewedImportENC34(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(checkpointBusyDraftFixture, struct{}{}))
	button := findByTestID(f, "button", "draft-import-all")
	if button == nil {
		t.Fatal("reviewed import button was not rendered")
	}
	if button.Attr("disabled") == "" {
		t.Fatal("reviewed import button remained enabled while checkpoint was saving")
	}
}

func checkpointBusyDraftFixture(_ struct{}) ui.Node {
	row := extract.Row{Date: "2026-10-01", Description: "Groceries", Amount: "-12.00"}
	return paginatedDraftReview(draftReviewProps{
		Rows: []extract.Row{row}, VisibleRows: []extract.Row{row},
		ReviewCur: "USD", SavingCheckpoint: true,
	})
}
