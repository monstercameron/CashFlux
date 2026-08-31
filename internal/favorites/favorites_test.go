// SPDX-License-Identifier: MIT

package favorites

import (
	"strings"
	"testing"
)

func TestDigitRowOrderNotCountingOrder(t *testing.T) {
	// The tenth slot is "0", not "10". This is the whole reason Max is ten, and the
	// one place the mapping could plausibly be written as i+1 and look correct for
	// nine of the ten cases.
	want := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
	for i, w := range want {
		got, ok := DigitFor(i)
		if !ok {
			t.Errorf("DigitFor(%d) reported no slot", i)
			continue
		}
		if got != w {
			t.Errorf("DigitFor(%d) = %q, want %q", i, got, w)
		}
	}
	if _, ok := DigitFor(Max); ok {
		t.Error("DigitFor(Max) reported a slot; there is no eleventh key")
	}
	if _, ok := DigitFor(-1); ok {
		t.Error("DigitFor(-1) reported a slot")
	}
}

func TestSlotForDigitInvertsDigitFor(t *testing.T) {
	for i := 0; i < Max; i++ {
		d, _ := DigitFor(i)
		got, ok := SlotForDigit(d[0])
		if !ok || got != i {
			t.Errorf("SlotForDigit(%q) = %d,%v; want %d,true", d, got, ok, i)
		}
	}
	for _, b := range []byte{'a', '-', ' '} {
		if _, ok := SlotForDigit(b); ok {
			t.Errorf("SlotForDigit(%q) claimed a slot", b)
		}
	}
}

func TestToggleAppendsSoExistingSlotsSurvive(t *testing.T) {
	// The slots are muscle memory. Pinning something new must never change what a
	// key already does — which rules out inserting at the front, the other obvious
	// choice.
	list := []string{"/a", "/b", "/c"}
	out, pinned := Toggle(list, "/d")
	if !pinned {
		t.Fatal("Toggle reported unpinned for a new path")
	}
	if strings.Join(out, ",") != "/a,/b,/c,/d" {
		t.Errorf("new pin did not go last: %v", out)
	}
	for i, p := range []string{"/a", "/b", "/c"} {
		if IndexOf(out, p) != i {
			t.Errorf("%s moved from slot %d to %d", p, i, IndexOf(out, p))
		}
	}
}

func TestToggleRemovesAndClosesTheGap(t *testing.T) {
	list := []string{"/a", "/b", "/c"}
	out, pinned := Toggle(list, "/b")
	if pinned {
		t.Error("Toggle reported pinned when removing")
	}
	if strings.Join(out, ",") != "/a,/c" {
		t.Errorf("removal left %v, want [/a /c]", out)
	}
	// The input must not be mutated — the caller still holds it as prior state.
	if strings.Join(list, ",") != "/a,/b,/c" {
		t.Errorf("Toggle mutated its input: %v", list)
	}
}

func TestAFullListRefusesRatherThanEvicting(t *testing.T) {
	// Dropping someone's oldest pin to make room for a click they may not have
	// understood is a change they did not ask for and cannot see.
	var list []string
	for i := 0; i < Max; i++ {
		list = append(list, string(rune('a'+i)))
	}
	if !Full(list) {
		t.Fatal("a list of Max is not reported Full")
	}
	out, pinned := Toggle(list, "/new")
	if !pinned {
		t.Error("a refused pin should still report the intent to pin")
	}
	if len(out) != Max {
		t.Errorf("list grew past Max: %d", len(out))
	}
	if Contains(out, "/new") {
		t.Error("a full list accepted an eleventh pin")
	}
	if out[0] != "a" {
		t.Errorf("the oldest pin was evicted: %v", out)
	}
}

func TestUnpinningFromAFullListMakesRoom(t *testing.T) {
	var list []string
	for i := 0; i < Max; i++ {
		list = append(list, string(rune('a'+i)))
	}
	list, _ = Toggle(list, "c")
	if Full(list) {
		t.Fatal("still Full after unpinning")
	}
	out, pinned := Toggle(list, "/new")
	if !pinned || !Contains(out, "/new") {
		t.Errorf("could not pin after making room: %v", out)
	}
}

func TestMoveReordersWithoutLosingAnything(t *testing.T) {
	list := []string{"/a", "/b", "/c", "/d"}
	if got := strings.Join(Move(list, 0, 2), ","); got != "/b,/c,/a,/d" {
		t.Errorf("Move(0,2) = %s", got)
	}
	if got := strings.Join(Move(list, 3, 0), ","); got != "/d,/a,/b,/c" {
		t.Errorf("Move(3,0) = %s", got)
	}
	// A drag that ends nowhere must not reorder anything.
	for _, c := range [][2]int{{-1, 1}, {1, -1}, {0, 9}, {9, 0}, {2, 2}} {
		if got := strings.Join(Move(list, c[0], c[1]), ","); got != "/a,/b,/c,/d" {
			t.Errorf("Move%v changed the list: %s", c, got)
		}
	}
}

func TestCleanDropsPinsThatNoLongerResolve(t *testing.T) {
	// Pins outlive what they point at: a deleted custom page, a switched-off
	// module, a renamed route. A dead pin holds a number key that then does
	// nothing, which is the worst outcome for a shortcut — the user cannot see why
	// it stopped working.
	live := map[string]bool{"/a": true, "/c": true}
	got := Clean([]string{"/a", "/gone", "/c"}, func(p string) bool { return live[p] })
	if strings.Join(got, ",") != "/a,/c" {
		t.Errorf("Clean = %v, want [/a /c]", got)
	}
}

func TestCleanRemovesDuplicatesAndBlanks(t *testing.T) {
	got := Clean([]string{"/a", "", "/a", "/b"}, nil)
	if strings.Join(got, ",") != "/a,/b" {
		t.Errorf("Clean = %v, want [/a /b]", got)
	}
}

func TestCleanTrimsToMax(t *testing.T) {
	var list []string
	for i := 0; i < Max+5; i++ {
		list = append(list, string(rune('a'+i)))
	}
	got := Clean(list, nil)
	if len(got) != Max {
		t.Errorf("Clean kept %d, want %d", len(got), Max)
	}
	if got[0] != "a" {
		t.Errorf("Clean dropped from the front: %v", got)
	}
}

func TestIndexOfAndContainsAgree(t *testing.T) {
	list := []string{"/a", "/b"}
	if !Contains(list, "/b") || IndexOf(list, "/b") != 1 {
		t.Error("Contains/IndexOf disagree for a present path")
	}
	if Contains(list, "/z") || IndexOf(list, "/z") != -1 {
		t.Error("Contains/IndexOf disagree for an absent path")
	}
}

func TestTogglingAnEmptyPathIsANoOp(t *testing.T) {
	list := []string{"/a"}
	out, pinned := Toggle(list, "")
	if pinned || strings.Join(out, ",") != "/a" {
		t.Errorf("empty path changed the list: %v pinned=%v", out, pinned)
	}
}

func TestReplaceAtKeepsEveryOtherSlotWhereItWas(t *testing.T) {
	// The eleventh pin. The newcomer takes the given slot; appending it and closing
	// the gap instead would renumber every slot after the one given up, so a swap
	// made to reach one screen would silently move several the user did not touch.
	list := []string{"/a", "/b", "/c", "/d"}
	got := ReplaceAt(list, 1, "/new")
	if strings.Join(got, ",") != "/a,/new,/c,/d" {
		t.Errorf("ReplaceAt(1,/new) = %v, want [/a /new /c /d]", got)
	}
	for i, want := range []string{"/a", "/c", "/d"} {
		_ = i
		if !Contains(got, want) {
			t.Errorf("%s was lost in the swap: %v", want, got)
		}
	}
	if IndexOf(got, "/a") != 0 || IndexOf(got, "/c") != 2 || IndexOf(got, "/d") != 3 {
		t.Errorf("a slot other than the replaced one moved: %v", got)
	}
	if Contains(got, "/b") {
		t.Errorf("the replaced path is still pinned: %v", got)
	}
	// The input is untouched — the caller still holds it as prior state.
	if strings.Join(list, ",") != "/a,/b,/c,/d" {
		t.Errorf("ReplaceAt mutated its input: %v", list)
	}
}

func TestReplaceAtOnAFullListStaysFull(t *testing.T) {
	var list []string
	for i := 0; i < Max; i++ {
		list = append(list, string(rune('a'+i)))
	}
	got := ReplaceAt(list, 3, "/new")
	if len(got) != Max {
		t.Errorf("length changed: %d, want %d", len(got), Max)
	}
	if got[3] != "/new" {
		t.Errorf("slot 3 = %q, want /new", got[3])
	}
	if IndexOf(got, "a") != 0 || IndexOf(got, string(rune('a'+Max-1))) != Max-1 {
		t.Errorf("the ends moved: %v", got)
	}
}

func TestReplaceAtWithAnAlreadyPinnedPathReordersWithoutAGap(t *testing.T) {
	// Dropping a path onto another slot when it is already pinned is a reorder.
	// Writing it into the target and leaving its old slot holding a duplicate — or
	// a hole — would give one number key nothing to open.
	list := []string{"/a", "/b", "/c", "/d"}
	got := ReplaceAt(list, 0, "/c")
	if strings.Join(got, ",") != "/c,/a,/b,/d" {
		t.Errorf("ReplaceAt(0,/c) = %v, want [/c /a /b /d]", got)
	}
	if len(got) != len(list) {
		t.Errorf("length changed on a reorder: %d", len(got))
	}
	seen := map[string]int{}
	for _, p := range got {
		seen[p]++
		if seen[p] > 1 {
			t.Errorf("%s appears twice: %v", p, got)
		}
	}
}

func TestReplaceAtRejectsSlotsThatDoNotExist(t *testing.T) {
	list := []string{"/a", "/b"}
	for _, at := range []int{-1, 2, 99} {
		if got := ReplaceAt(list, at, "/new"); strings.Join(got, ",") != "/a,/b" {
			t.Errorf("ReplaceAt(%d) changed the list: %v", at, got)
		}
	}
	if got := ReplaceAt(list, 0, ""); strings.Join(got, ",") != "/a,/b" {
		t.Errorf("an empty path changed the list: %v", got)
	}
}

// The rail shows a CLEANED list and stores the raw one. These two functions are
// what keeps an edit made against the display from corrupting the storage, so
// the case that matters is a raw list holding something the display does not.
func TestMoveBeforeAddressesTheRawListNotTheVisibleOne(t *testing.T) {
	raw := []string{"/a", "/gone", "/b", "/c"}
	// The rail is showing /a, /b, /c — "/gone" is a page that has not loaded.
	// Dragging /c onto /b must move it in the RAW list, keeping /gone where it is.
	got := MoveBefore(raw, "/c", "/b")
	if strings.Join(got, ",") != "/a,/gone,/c,/b" {
		t.Errorf("MoveBefore = %v, want [/a /gone /c /b]", got)
	}
	if !Contains(got, "/gone") {
		t.Error("an unreachable pin was dropped by a reorder — it would be lost on the next save")
	}
	if len(got) != len(raw) {
		t.Errorf("length changed: %d -> %d", len(raw), len(got))
	}
}

func TestReplacePathKeepsUnreachablePins(t *testing.T) {
	raw := []string{"/a", "/gone", "/b"}
	got := ReplacePath(raw, "/b", "/new")
	if strings.Join(got, ",") != "/a,/gone,/new" {
		t.Errorf("ReplacePath = %v, want [/a /gone /new]", got)
	}
}

func TestPathEditsIgnoreUnknownPaths(t *testing.T) {
	raw := []string{"/a", "/b"}
	if got := MoveBefore(raw, "/nope", "/a"); strings.Join(got, ",") != "/a,/b" {
		t.Errorf("MoveBefore with an unknown mover changed the list: %v", got)
	}
	if got := MoveBefore(raw, "/a", "/nope"); strings.Join(got, ",") != "/a,/b" {
		t.Errorf("MoveBefore with an unknown target changed the list: %v", got)
	}
	if got := ReplacePath(raw, "/nope", "/new"); strings.Join(got, ",") != "/a,/b" {
		t.Errorf("ReplacePath with an unknown victim changed the list: %v", got)
	}
}
