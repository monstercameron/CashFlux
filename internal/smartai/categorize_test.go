// SPDX-License-Identifier: MIT

package smartai

import "testing"

func TestParseCategorySuggestions(t *testing.T) {
	existing := map[string]bool{"groceries": true, "rent": true}
	answer := "" +
		"Dining Out | expense\n" +
		"- Salary | income\n" +
		"Groceries | expense\n" + // already exists → dropped
		"AB | expense\n" + // too short → dropped
		"Dining Out | expense\n" + // duplicate → dropped
		"Coffee\n" // no kind → defaults expense
	got := ParseCategorySuggestions(answer, existing)
	if len(got) != 3 {
		t.Fatalf("got %d suggestions, want 3: %+v", len(got), got)
	}
	if got[0].Name != "Dining Out" || got[0].Kind != "expense" {
		t.Errorf("first = %+v, want Dining Out/expense", got[0])
	}
	if got[1].Name != "Salary" || got[1].Kind != "income" {
		t.Errorf("second = %+v, want Salary/income", got[1])
	}
	if got[2].Name != "Coffee" || got[2].Kind != "expense" {
		t.Errorf("third = %+v, want Coffee/expense (default)", got[2])
	}
}

func TestParseCategorySuggestionsCap(t *testing.T) {
	answer := ""
	for i := 0; i < 20; i++ {
		answer += "Cat" + string(rune('A'+i)) + " | expense\n"
	}
	if got := ParseCategorySuggestions(answer, nil); len(got) != 8 {
		t.Errorf("cap = %d, want 8", len(got))
	}
}

func TestParseCategoryAssignments(t *testing.T) {
	cats := map[string]string{"Groceries": "c1", "Dining Out": "c2"}
	answer := "" +
		"1 => Groceries\n" +
		"2. => Dining Out\n" + // tolerant of "2."
		"3 => Nonexistent\n" + // unknown category → dropped
		"9 => Groceries\n" + // out of range → dropped
		"1 => Dining Out\n" // duplicate ref → dropped
	got := ParseCategoryAssignments(answer, 4, cats)
	if len(got) != 2 {
		t.Fatalf("got %d assignments, want 2: %+v", len(got), got)
	}
	if got[0].Ref != 1 || got[0].CategoryID != "c1" {
		t.Errorf("first = %+v, want ref 1 / c1", got[0])
	}
	if got[1].Ref != 2 || got[1].CategoryID != "c2" || got[1].CategoryName != "Dining Out" {
		t.Errorf("second = %+v, want ref 2 / c2 / Dining Out", got[1])
	}
}

func TestAtoiSafe(t *testing.T) {
	cases := map[string]int{"3": 3, "  12 ": 12, "#3": 3, "3.": 3, "abc": 0, "": 0, "7x": 7}
	for in, want := range cases {
		if got := atoiSafe(in); got != want {
			t.Errorf("atoiSafe(%q) = %d, want %d", in, got, want)
		}
	}
}
