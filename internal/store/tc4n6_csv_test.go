// SPDX-License-Identifier: MIT

package store

import "testing"

func TestAccountAwareCSVParserUsesResolvedAccountAndRowCurrency(t *testing.T) {
	cases := []struct {
		name      string
		csv       string
		fallback  string
		wantID    string
		wantCur   string
		wantAmt   int64
		wantError bool
	}{
		{
			name: "fallback account currency before amount parse",
			csv:  "date,payee,amount\n2026-01-02,Hotel,-12.34\n", fallback: "usd-account",
			wantID: "usd-account", wantCur: "USD", wantAmt: -1234,
		},
		{
			name:   "account name is resolved before amount parse",
			csv:    "date,payee,amount,account\n2026-01-02,Hotel,-12.34,Travel Card\n",
			wantID: "usd-account", wantCur: "USD", wantAmt: -1234,
		},
		{
			name:   "explicit row currency controls precision",
			csv:    "date,payee,amount,currency,account\n2026-01-02,Hotel,-12.34,USD,Travel Card\n",
			wantID: "usd-account", wantCur: "USD", wantAmt: -1234,
		},
		{
			name:      "explicit currency mismatch is rejected",
			wantError: true,
			csv:       "date,payee,amount,currency,account\n2026-01-02,Hotel,-12.34,EUR,Travel Card\n",
			wantID:    "usd-account", wantCur: "EUR", wantAmt: -1234,
		},
	}
	accounts := []CSVAccount{{ID: "usd-account", Name: "Travel Card", Currency: "USD"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, skipped, err := TransactionsFromCSVResilientForAccountList([]byte(tc.csv), "JPY", tc.fallback, accounts)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantError {
				if len(rows) != 0 || len(skipped) != 1 {
					t.Fatalf("rows=%+v skipped=%+v; want a row failure", rows, skipped)
				}
				return
			}
			if len(skipped) != 0 || len(rows) != 1 {
				t.Fatalf("rows=%+v skipped=%+v", rows, skipped)
			}
			got := rows[0]
			if got.AccountID != tc.wantID || got.Amount.Currency != tc.wantCur || got.Amount.Amount != tc.wantAmt {
				t.Fatalf("transaction = %+v; want account=%q currency=%q amount=%d", got, tc.wantID, tc.wantCur, tc.wantAmt)
			}
		})
	}
}

func TestAccountAwareCSVParserKeepsRowFailures(t *testing.T) {
	data := []byte("date,payee,amount,account\n2026-01-02,Hotel,-12.345,Travel Card\n")
	rows, skipped, err := TransactionsFromCSVResilientForAccountList(data, "JPY", "", []CSVAccount{{ID: "usd-account", Name: "Travel Card", Currency: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 || len(skipped) != 1 || skipped[0].Line != 2 {
		t.Fatalf("rows=%+v skipped=%+v, want one reported line 2 failure", rows, skipped)
	}
}
