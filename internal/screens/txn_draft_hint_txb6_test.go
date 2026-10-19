// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestParseTxnSentenceIncomeIntent(t *testing.T) {
	tests := []struct {
		name     string
		sentence string
		income   bool
	}{
		{name: "expense at Refund Center", sentence: "spent 40 at Refund Center", income: false},
		{name: "refund in merchant suffix", sentence: "spent 40 at Northside Refund Center", income: false},
		{name: "refund in merchant middle", sentence: "spent 40 at Refund Center Cafe", income: false},
		{name: "refund in payee after amount", sentence: "40 at Refund Center", income: false},
		{name: "refund embedded in merchant word", sentence: "spent 40 at NoRefund Market", income: false},
		{name: "refund mentioned after merchant", sentence: "spent 40 at Market refund pending", income: false},
		{name: "refund as leading verb", sentence: "refund 40 from Store", income: true},
		{name: "got a refund", sentence: "got a refund of 40 from Store", income: true},
		{name: "got paid", sentence: "got paid 40 from Acme", income: true},
		{name: "received", sentence: "received 40 from Acme", income: true},
		{name: "paid me", sentence: "paid me 40 for the repair", income: true},
		{name: "earned", sentence: "earned 40 at Acme", income: true},
		{name: "income", sentence: "income 40 from Acme", income: true},
		{name: "deposit", sentence: "deposit 40 from Acme", income: true},
		{name: "case insensitive refund", sentence: "REFUND 40 FROM STORE", income: true},
		{name: "refund is not a leading phrase", sentence: "40 refund from Store", income: false},
		{name: "refund prefix is a merchant", sentence: "40 at Refund Center", income: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTxnSentence(tt.sentence)
			if got.Income != tt.income {
				t.Errorf("parseTxnSentence(%q).Income = %t, want %t (draft: %+v)", tt.sentence, got.Income, tt.income, got)
			}
		})
	}
}
