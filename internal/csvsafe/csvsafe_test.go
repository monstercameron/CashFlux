// SPDX-License-Identifier: MIT

package csvsafe

import "testing"

func TestGuardAndRestore(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "equals", in: `=HYPERLINK("https://attacker.invalid","open")`, want: `'=HYPERLINK("https://attacker.invalid","open")`},
		{name: "plus", in: "+1+1", want: "'+1+1"},
		{name: "minus", in: "-1+1", want: "'-1+1"},
		{name: "at", in: "@SUM(A1:A2)", want: "'@SUM(A1:A2)"},
		{name: "tab", in: "\t=1+1", want: "'\t=1+1"},
		{name: "carriage_return", in: "\r=1+1", want: "'\r=1+1"},
		{name: "existing_quote_formula", in: "'=ordinary", want: "''=ordinary"},
		{name: "existing_quote_text", in: "'ordinary", want: "''ordinary"},
		{name: "ordinary", in: "Coffee Shop", want: "Coffee Shop"},
		{name: "leading_space", in: " =1+1", want: " =1+1"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Guard(tt.in)
			if got != tt.want {
				t.Fatalf("Guard(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if restored := Restore(got); restored != tt.in {
				t.Errorf("Restore(Guard(%q)) = %q", tt.in, restored)
			}
		})
	}
}

func TestRestoreLeavesUnguardedTextAlone(t *testing.T) {
	for _, value := range []string{"' text", "plain text", ""} {
		if got := Restore(value); got != value {
			t.Errorf("Restore(%q) = %q, want unchanged", value, got)
		}
	}
}
