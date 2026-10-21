// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTxnDraftHintEnterAppliesParsedSentenceWithoutDismissing(t *testing.T) {
	var applied []TxnDraftFill
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TxnDraftHint, TxnDraftHintProps{
		OnApply: func(fill TxnDraftFill) { applied = append(applied, fill) },
	}))

	input := f.ByID("sm15-input")
	if input == nil {
		t.Fatal("sentence input did not render")
	}
	input.Input("spent 40 at whole foods yesterday")
	input.Dispatch("onkeydown", render.Event{Key: "Enter"})

	if len(applied) != 1 {
		t.Fatalf("Enter applied %d fills, want 1", len(applied))
	}
	fill := applied[0]
	if fill.AmountMajor != "40" || fill.Payee != "Whole Foods" || fill.Date.IsZero() {
		t.Fatalf("Enter applied incomplete parse: %+v", fill)
	}
	if f.ByID("sm15-input") == nil {
		t.Fatal("sentence input disappeared after Enter; the panel must remain open")
	}
	input = f.ByID("sm15-input")
	input.Input("something unrecognized")
	input.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if len(applied) != 1 {
		t.Fatalf("Enter without a parse applied %d fills, want no additional fill", len(applied))
	}
	if f.ByID("sm15-input") == nil {
		t.Fatal("Enter without a parse dismissed the sentence input")
	}
}
