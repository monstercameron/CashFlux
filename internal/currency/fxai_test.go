// SPDX-License-Identifier: MIT

package currency

import (
	"strings"
	"testing"
)

func TestBuildFXPrompt(t *testing.T) {
	prompt := BuildFXPrompt("USD", []string{"EUR", "GBP", "JPY"})
	if !strings.Contains(prompt, "USD") {
		t.Error("prompt should mention the base currency USD")
	}
	if !strings.Contains(prompt, "EUR") || !strings.Contains(prompt, "GBP") || !strings.Contains(prompt, "JPY") {
		t.Error("prompt should list all requested currency codes")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should instruct the model to return JSON")
	}
	// Orientation check: the prompt must explain the rate direction
	if !strings.Contains(prompt, "how many") || !strings.Contains(prompt, "equal ONE unit") {
		t.Error("prompt should explain the rate orientation (base units per 1 unit of target)")
	}
}

func TestParseFXReply(t *testing.T) {
	cases := []struct {
		name       string
		reply      string
		base       string
		wantCodes  []string
		wantAsOf   string
		wantErrSub string // non-empty means we expect an error containing this substring
	}{
		{
			name:      "clean JSON",
			reply:     `{"base":"USD","asOf":"2026-06-24","rates":{"EUR":1.08,"GBP":1.27,"JPY":0.0067}}`,
			base:      "USD",
			wantCodes: []string{"EUR", "GBP", "JPY"},
			wantAsOf:  "2026-06-24",
		},
		{
			name: "JSON in markdown fences",
			reply: "Here are today's rates:\n```json\n" +
				`{"base":"USD","asOf":"2026-06-24","rates":{"EUR":1.08,"GBP":1.27}}` +
				"\n```\nThese are mid-market rates.",
			base:      "USD",
			wantCodes: []string{"EUR", "GBP"},
			wantAsOf:  "2026-06-24",
		},
		{
			name: "JSON surrounded by prose",
			reply: "I searched the web and found: " +
				`{"base":"USD","asOf":"2026-06-24","rates":{"EUR":1.08,"CAD":0.74}}` +
				" These rates are as of today.",
			base:      "USD",
			wantCodes: []string{"EUR", "CAD"},
			wantAsOf:  "2026-06-24",
		},
		{
			name:      "base code dropped from rates",
			reply:     `{"base":"USD","asOf":"2026-06-24","rates":{"USD":1.0,"EUR":1.08}}`,
			base:      "USD",
			wantCodes: []string{"EUR"},
			wantAsOf:  "2026-06-24",
		},
		{
			name:      "unknown codes filtered out",
			reply:     `{"base":"USD","asOf":"2026-06-24","rates":{"EUR":1.08,"XYZ":99.9,"ABC":0.5}}`,
			base:      "USD",
			wantCodes: []string{"EUR"},
			wantAsOf:  "2026-06-24",
		},
		{
			name:      "non-positive rates filtered out",
			reply:     `{"base":"USD","asOf":"2026-06-24","rates":{"EUR":1.08,"GBP":-1.0,"JPY":0}}`,
			base:      "USD",
			wantCodes: []string{"EUR"},
			wantAsOf:  "2026-06-24",
		},
		{
			name:       "empty reply → error",
			reply:      "I could not find any rates.",
			base:       "USD",
			wantErrSub: "JSON",
		},
		{
			name:       "all rates filtered → error",
			reply:      `{"base":"USD","asOf":"2026-06-24","rates":{"FAKE":1.0}}`,
			base:       "USD",
			wantErrSub: "no valid exchange rates",
		},
		{
			name:       "malformed JSON → error",
			reply:      `{this is not json at all}`,
			base:       "USD",
			wantErrSub: "parse",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rates, asOf, err := ParseFXReply(tc.reply, tc.base)

			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if asOf != tc.wantAsOf {
				t.Errorf("asOf = %q, want %q", asOf, tc.wantAsOf)
			}
			for _, code := range tc.wantCodes {
				if _, ok := rates[code]; !ok {
					t.Errorf("expected rate for %s, not found in %v", code, rates)
				}
			}
			// Base currency must never appear in the clean map
			if _, ok := rates[tc.base]; ok {
				t.Errorf("base currency %s should not appear in rates map", tc.base)
			}
		})
	}
}
