// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// C627: "Jump to page" accepted 132 and left the ledger on page 1, with the input
// still reading 132 — a control reporting a page that was not the rendered page.
//
// The cause was a controlled input committing from a single OnChange that read the
// event's value, which this framework resolves one render late (the same trap the
// rows-per-page control avoids by using buttons). These mount the real Pager and
// drive it the way a user does, so the fix is asserted through behaviour rather
// than through the shape of the handler.

// pagerJump renders a Pager over a 3284-row ledger at 25/page and returns the
// fixture plus a pointer to the page the component asked for (-1 = never asked).
func pagerJump(t *testing.T, page int) (*render.Fixture, *int) {
	t.Helper()
	got := -1
	f := render.New(t)
	f.Render(uic.CreateElement(pager, PagerProps{
		Page: page, Total: 3284, PageSize: 25,
		PageSizes:  []int{25, 50, 100},
		OnPage:     func(p int) { got = p },
		OnPageSize: func(int) {},
		IDPrefix:   "txn",
	}))
	return f, &got
}

func jumpBox(t *testing.T, f *render.Fixture) *render.QueryNode {
	t.Helper()
	for _, n := range f.AllByTag("input") {
		if strings.Contains(n.Attr("class"), "std-pager-jump-input") {
			return n
		}
	}
	t.Fatal("jump-to-page input not found")
	return nil
}

// TestJumpCommitsTheTypedPage is the core guard: type a page, commit, and the
// component must ask for THAT page.
func TestJumpCommitsTheTypedPage(t *testing.T) {
	f, got := pagerJump(t, 1)
	box := jumpBox(t, f)
	box.Input("132")  // keystrokes land in local state
	box.Change("132") // blur/commit
	if *got != 132 {
		t.Errorf("OnPage = %d, want 132 — the typed page was not what got committed (C627)", *got)
	}
}

// TestJumpClampsAboveTheLastPage keeps the clamp behaviour the old handler had:
// an out-of-range entry goes to the last page rather than being dropped.
func TestJumpClampsAboveTheLastPage(t *testing.T) {
	f, got := pagerJump(t, 1)
	box := jumpBox(t, f)
	box.Input("99999")
	box.Change("99999")
	// 3284 rows at 25/page = 132 pages.
	if *got != 132 {
		t.Errorf("OnPage = %d, want the last page (132)", *got)
	}
}

func TestJumpClampsBelowOne(t *testing.T) {
	f, got := pagerJump(t, 5)
	box := jumpBox(t, f)
	box.Input("0")
	box.Change("0")
	if *got != 1 {
		t.Errorf("OnPage = %d, want 1", *got)
	}
}

// TestJumpIgnoresJunkAndDoesNotNavigate: an unparseable entry must not move the
// ledger. The old handler returned early too, but left the junk sitting in the
// box; the field now falls back to the real page.
func TestJumpIgnoresJunkAndDoesNotNavigate(t *testing.T) {
	f, got := pagerJump(t, 7)
	box := jumpBox(t, f)
	box.Input("abc")
	box.Change("abc")
	if *got != -1 {
		t.Errorf("OnPage was called with %d for junk input — it must not navigate", *got)
	}
}

// TestJumpToTheCurrentPageIsANoOp avoids a pointless re-render (and a scroll jump)
// when the typed page is the one already shown.
func TestJumpToTheCurrentPageIsANoOp(t *testing.T) {
	f, got := pagerJump(t, 42)
	box := jumpBox(t, f)
	box.Input("42")
	box.Change("42")
	if *got != -1 {
		t.Errorf("OnPage = %d for the page already shown — expected no navigation", *got)
	}
}

// TestJumpBoxShowsTheRealPageWhenIdle is the other half of the report: the input
// must never sit there displaying a page the ledger is not on.
func TestJumpBoxShowsTheRealPageWhenIdle(t *testing.T) {
	f, _ := pagerJump(t, 7)
	// Read the ATTRIBUTE, not the property. A text field's value stopped being a
	// value prop when it moved to uiw.FieldValue — it is declared as data-cf-value
	// and carried into the DOM property at runtime by the delegated input listener
	// and MutationObserver that FieldValue installs. Those are browser machinery,
	// and this fixture is a mock DOM that does not run them, so asking for the
	// property here measures the testkit rather than the app: it reads "" no matter
	// what the pager declares.
	//
	// The user-facing contract is unchanged and still checked — the attribute IS
	// what the component says the page is. Confirmed in a real browser against this
	// commit: the idle box reads 1, and after two Next clicks reads 3 while the
	// range label says "51–75 of 3230".
	got := jumpBox(t, f).Attr(fieldValueAttr)
	if got != "7" {
		t.Errorf("jump box value = %q, want %q — an idle box must read the page actually shown", got, "7")
	}
}
