// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestRowNoteGlyphOpensNoteWithoutOpeningEditor checks that the note glyph toggles its note
// popover and leaves the row's editor closed. The popover is portalled, so the wasm renderer
// cannot read its text; the toggle's expanded state is what it can show.
func TestRowNoteGlyphOpensNoteWithoutOpeningEditor(t *testing.T) {
	p := r4cRow("row-note")
	p.Note, p.HasNote, p.Cleared = "Split the bill with Sam", true, true
	opened := 0
	p.OnOpen = func(string) { opened++ }
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnFrameRow, &p))
	if byTestID(f, "span", "txn-row-cleared") == nil {
		t.Fatal("cleared row has no check glyph")
	}
	toggle := byTestID(f, "button", "txn-row-note-toggle")
	if toggle == nil {
		t.Fatal("row with a note has no note glyph")
	}
	if toggle.Attr("aria-expanded") != "false" {
		t.Fatal("note glyph starts expanded")
	}
	toggle.Click()
	f.Stabilize()
	if got := byTestID(f, "button", "txn-row-note-toggle"); got == nil || got.Attr("aria-expanded") != "true" {
		t.Fatal("clicking the note glyph does not open the note")
	}
	if opened != 0 {
		t.Fatalf("note glyph opened the editor %d times, want none", opened)
	}
}
