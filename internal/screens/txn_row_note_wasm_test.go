// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestRowNoteGlyphRevealsNoteWithoutOpeningEditor checks that the note glyph shows the note text
// when clicked and leaves the row's editor closed.
func TestRowNoteGlyphRevealsNoteWithoutOpeningEditor(t *testing.T) {
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
	if byTestID(f, "div", "txn-row-note") != nil {
		t.Fatal("note text shows before the glyph is clicked")
	}
	toggle := byTestID(f, "button", "txn-row-note-toggle")
	if toggle == nil {
		t.Fatal("row with a note has no note glyph")
	}
	toggle.Click()
	f.Stabilize()
	note := byTestID(f, "div", "txn-row-note")
	if note == nil || !strings.Contains(note.Text(), "Split the bill with Sam") {
		t.Fatal("clicking the note glyph does not reveal the note text")
	}
	if opened != 0 {
		t.Fatalf("note glyph opened the editor %d times, want none", opened)
	}
}
