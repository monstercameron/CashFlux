// SPDX-License-Identifier: MIT

package split

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name  string
		lines []Line
		want  Draft
	}{
		{
			name:  "no rows at all",
			lines: nil,
			want:  Draft{},
		},
		{
			// The exact draft the editor seeds for an unsplit transaction: the whole
			// amount on its current category plus a blank row to carve into. This is
			// the C566 case — it must NOT read as a valid two-line split.
			name: "the seeded draft is one complete row and one blank",
			lines: []Line{
				{CategoryID: "cat-groceries", Value: "120.00"},
				{CategoryID: "", Value: ""},
			},
			want: Draft{Complete: 1, Blank: 1},
		},
		{
			name: "a finished two-line split",
			lines: []Line{
				{CategoryID: "cat-groceries", Value: "80.00"},
				{CategoryID: "cat-household", Value: "40.00"},
			},
			want: Draft{Complete: 2},
		},
		{
			name: "a category with no amount is incomplete, not blank",
			lines: []Line{
				{CategoryID: "cat-groceries", Value: "80.00"},
				{CategoryID: "cat-household", Value: ""},
			},
			want: Draft{Complete: 1, Incomplete: 1},
		},
		{
			name: "an amount with no category is incomplete too",
			lines: []Line{
				{CategoryID: "cat-groceries", Value: "80.00"},
				{CategoryID: "", Value: "40.00"},
			},
			want: Draft{Complete: 1, Incomplete: 1},
		},
		{
			// Whitespace is what the user sees as empty, so it counts as empty.
			name: "whitespace-only fields read as empty",
			lines: []Line{
				{CategoryID: "  ", Value: "\t"},
				{CategoryID: "cat-a", Value: "  "},
			},
			want: Draft{Blank: 1, Incomplete: 1},
		},
		{
			name: "a mixed draft counts each kind",
			lines: []Line{
				{CategoryID: "cat-a", Value: "10"},
				{CategoryID: "cat-b", Value: "20"},
				{CategoryID: "cat-c", Value: ""},
				{CategoryID: "", Value: ""},
				{CategoryID: "", Value: "5"},
			},
			want: Draft{Complete: 2, Blank: 1, Incomplete: 2},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.lines); got != tc.want {
				t.Errorf("Classify() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDraftSaveable(t *testing.T) {
	tests := []struct {
		name     string
		draft    Draft
		parseOK  bool
		balanced bool
		want     bool
	}{
		{
			name:  "the seeded draft is not saveable even though the money balances",
			draft: Draft{Complete: 1, Blank: 1}, parseOK: true, balanced: true,
			want: false, // the C566 regression: "Balanced" must not mean "Save"
		},
		{
			name:  "two complete balanced rows save",
			draft: Draft{Complete: 2}, parseOK: true, balanced: true,
			want: true,
		},
		{
			name:  "a half-written row blocks the save",
			draft: Draft{Complete: 2, Incomplete: 1}, parseOK: true, balanced: true,
			want: false,
		},
		{
			name:  "a trailing blank row does not block a valid split",
			draft: Draft{Complete: 2, Blank: 1}, parseOK: true, balanced: true,
			want: true,
		},
		{
			name:  "unbalanced money blocks the save",
			draft: Draft{Complete: 2}, parseOK: true, balanced: false,
			want: false,
		},
		{
			name:  "an unparseable value blocks the save",
			draft: Draft{Complete: 2}, parseOK: false, balanced: true,
			want: false,
		},
		{
			name:  "one complete row is not a split",
			draft: Draft{Complete: 1}, parseOK: true, balanced: true,
			want: false,
		},
		{
			name:  "an empty draft is not saveable",
			draft: Draft{}, parseOK: true, balanced: true,
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.draft.Saveable(tc.parseOK, tc.balanced); got != tc.want {
				t.Errorf("Saveable(parseOK=%v, balanced=%v) = %v, want %v",
					tc.parseOK, tc.balanced, got, tc.want)
			}
		})
	}
}
