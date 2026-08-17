// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func TestParseLimitMajor(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		decimals int
		wantAmt  int64
		wantProb LimitProblem
	}{
		{"positive", "1300.00", 2, 130000, LimitOK},
		{"positive no decimals", "1300", 2, 130000, LimitOK},
		{"positive with padding", "  12.50  ", 2, 1250, LimitOK},
		{"zero-decimal currency", "1300", 0, 1300, LimitOK},
		{"blank", "", 2, 0, LimitBlank},
		{"whitespace only", "   ", 2, 0, LimitBlank},
		{"negative", "-1", 2, 0, LimitNotPositive},
		{"negative fractional", "-0.01", 2, 0, LimitNotPositive},
		{"explicit plus stays positive", "+5.00", 2, 500, LimitOK},
		{"zero", "0", 2, 0, LimitNotPositive},
		{"zero with decimals", "0.00", 2, 0, LimitNotPositive},
		{"letters", "abc", 2, 0, LimitMalformed},
		{"exponent notation", "1e5", 2, 0, LimitMalformed},
		{"infinity", "Infinity", 2, 0, LimitMalformed},
		{"not a number", "NaN", 2, 0, LimitMalformed},
		{"too many decimals", "1.234", 2, 0, LimitMalformed},
		{"overflows int64", "99999999999999999999", 2, 0, LimitMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			amt, prob := ParseLimitMajor(c.in, c.decimals)
			if prob != c.wantProb {
				t.Fatalf("problem = %q, want %q", prob, c.wantProb)
			}
			if amt != c.wantAmt {
				t.Errorf("amount = %d, want %d", amt, c.wantAmt)
			}
			if prob.OK() != (c.wantProb == LimitOK) {
				t.Errorf("OK() = %v for problem %q", prob.OK(), prob)
			}
		})
	}
}
