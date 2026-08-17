// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"
	"time"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// What a data table says about itself.
//
//	C626 — selecting "All" announced "1–3284 of 3284" while ~40 rows existed in
//	       the DOM, with no aria-rowcount, no aria-rowindex, and nothing telling
//	       assistive tech that the other 3,244 were not there to be tabbed to.
//	C625 — a sort flipped the announced state about a second before the rows
//	       caught up.
//	C628 — the same shape from the search debounce: stale rows, fully clickable,
//	       with the "Searching…" note in a different tile.
//
// These mount the real DataTable, because every one of those defects is about an
// attribute that exists (or does not) on the rendered element.

func ariaCols() []Column {
	return []Column{
		{Label: "Date", SortKey: "date"},
		{Label: "Amount", SortKey: "amount"},
	}
}

func mountTable(t *testing.T, props DataTableProps) *render.Fixture {
	t.Helper()
	f := render.New(t)
	f.Render(uic.CreateElement(dataTable, props))
	return f
}

func firstTag(t *testing.T, f *render.Fixture, tag string) *render.QueryNode {
	t.Helper()
	all := f.AllByTag(tag)
	if len(all) == 0 {
		t.Fatalf("no <%s> rendered", tag)
	}
	return all[0]
}

// TestVirtualBodyDeclaresTheFullRowCount is the core C626 guard: the table has to
// state how many rows it stands for, not how many it drew.
func TestVirtualBodyDeclaresTheFullRowCount(t *testing.T) {
	f := mountTable(t, DataTableProps{
		Columns: ariaCols(),
		Virtual: &VirtualSpec{
			Count: 3284, RowHeight: 35,
			RowAt: func(i int) uic.Node { return Tr(Td("r")) },
			KeyAt: func(i int) any { return i },
		},
	})
	// 3284 data rows + the header row.
	if got := firstTag(t, f, "table").Attr("aria-rowcount"); got != "3285" {
		t.Errorf("aria-rowcount = %q, want %q — a windowed body that does not declare its size "+
			"tells assistive tech the table is 40 rows long (C626)", got, "3285")
	}
	if got := firstTag(t, f, "tr").Attr("aria-rowindex"); got != "1" {
		t.Errorf("header aria-rowindex = %q, want 1 — the data rows' positions count from it (C626)", got)
	}
}

// TestANonVirtualTableClaimsNoRowCount: a fully rendered page's DOM order already
// is the answer, and an aria-rowcount that merely restated it would be noise (and
// would be wrong the moment the page size changed).
func TestANonVirtualTableClaimsNoRowCount(t *testing.T) {
	f := mountTable(t, DataTableProps{Columns: ariaCols(), Body: Tr(Td("r"))})
	if got := firstTag(t, f, "table").Attr("aria-rowcount"); got != "" {
		t.Errorf("aria-rowcount = %q on a fully rendered table, want none", got)
	}
}

// TestTheCaptionDescribesTheVirtualWindow is the other half of C626: the pager's
// "1–3284 of 3284" is true about the MATCHES and false about the DOM, so the
// table has to say which of the two it is being.
func TestTheCaptionDescribesTheVirtualWindow(t *testing.T) {
	f := mountTable(t, DataTableProps{
		Columns: ariaCols(),
		Virtual: &VirtualSpec{
			Count: 3284, RowHeight: 35,
			RowAt: func(i int) uic.Node { return Tr(Td("r")) },
			KeyAt: func(i int) any { return i },
		},
	})
	cap := firstTag(t, f, "caption").Text()
	if !strings.Contains(cap, "3284") {
		t.Errorf("caption %q does not state the match count", cap)
	}
	// It has to say that only PART of the set is loaded, and give the reader a way
	// to reach the rest — the two facts the pager's "1–3284 of 3284" leaves out.
	if !strings.Contains(strings.ToLower(cap), "only the ones") {
		t.Errorf("caption %q does not say only part of the set is loaded (C626)", cap)
	}
	if !strings.Contains(strings.ToLower(cap), "page controls") {
		t.Errorf("caption %q states the limitation without an actionable way past it (C626)", cap)
	}
	if got := firstTag(t, f, "caption").Attr("role"); got != "status" {
		t.Errorf("caption role = %q, want status — it carries the live state too", got)
	}
}

// TestBusyMarksTheTableAndSaysWhy is C628: while a debounced search is in flight
// the rows answer the previous query, and the table has to say so where the rows
// are — not only in the toolbar tile that owns the search box.
func TestBusyMarksTheTableAndSaysWhy(t *testing.T) {
	f := mountTable(t, DataTableProps{
		Columns: ariaCols(), Body: Tr(Td("r")),
		Busy: true, BusyLabel: "Searching…",
	})
	if got := firstTag(t, f, "table").Attr("aria-busy"); got != "true" {
		t.Errorf("aria-busy = %q, want true while the shown rows answer a superseded query (C628)", got)
	}
	if got := firstTag(t, f, "caption").Text(); got != "Searching…" {
		t.Errorf("caption = %q, want the busy label", got)
	}
}

// TestASettledTableAnnouncesTheOrderItIsIn is C625's other transition. A busy
// state that goes quiet without naming what settled leaves the reader knowing
// only that something changed.
func TestASettledTableAnnouncesTheOrderItIsIn(t *testing.T) {
	f := mountTable(t, DataTableProps{
		Columns: ariaCols(), Body: Tr(Td("r")),
		Sort: "amount", Dir: "asc",
	})
	cap := firstTag(t, f, "caption").Text()
	if !strings.Contains(cap, "Amount") {
		t.Errorf("caption %q does not name the sorted column", cap)
	}
	if !strings.Contains(cap, "ascending") {
		t.Errorf("caption %q does not name the direction in words", cap)
	}
	if got := firstTag(t, f, "table").Attr("aria-busy"); got == "true" {
		t.Error("a settled table is marked busy — the state that outlasts its cause is the C625 defect from the other side")
	}
}

// TestClickingASortHeaderMarksTheTableBusy is the C625 guard proper: the busy
// window has to OPEN on the click, before the re-sort runs, because that window
// is the whole point — it is when the rows on screen are the old order.
func TestClickingASortHeaderMarksTheTableBusy(t *testing.T) {
	sorted := 0
	f := mountTable(t, DataTableProps{
		Columns: ariaCols(), Body: Tr(Td("r")),
		Sort: "date", Dir: "desc", SortSpinner: true,
		OnSort: func(string) { sorted++ },
	})
	var header *render.QueryNode
	for _, n := range f.AllByTag("button") {
		if strings.Contains(n.Attr("class"), "th-sort") {
			header = n
			break
		}
	}
	if header == nil {
		t.Fatal("no sortable header button rendered")
	}
	header.Click()
	if got := firstTag(t, f, "table").Attr("aria-busy"); got != "true" {
		t.Errorf("aria-busy = %q immediately after a sort click, want true — the rows on screen are "+
			"still the OLD order at this point (C625)", got)
	}
	if sorted != 0 {
		t.Error("the re-sort ran synchronously with the click, so the busy state never got a frame to paint in")
	}
	// Let the deferred macrotask run. It is a raw setTimeout (the framework's own
	// timer queue is not involved), so a real yield to the JS event loop is what
	// advances it — Sleep on js/wasm does exactly that.
	time.Sleep(30 * time.Millisecond)
	f.Stabilize()
	if sorted != 1 {
		t.Fatalf("OnSort ran %d times after one click, want 1", sorted)
	}
	// The props still carry the OLD Sort/Dir — the caller has not re-rendered with
	// the new order — so the table is still legitimately busy. A second click in
	// that window must be swallowed, not queued: two re-sorts racing is how the
	// header ends up describing an order the rows never settled into.
	header.Click()
	time.Sleep(30 * time.Millisecond)
	f.Stabilize()
	if sorted != 1 {
		t.Errorf("OnSort ran %d times for two clicks inside one busy window, want 1 (C625)", sorted)
	}
}
