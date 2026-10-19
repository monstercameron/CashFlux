// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestParseTxnSentenceTitleCasesMerchantPayee(t *testing.T) {
	got := parseTxnSentence("spent 40 at whole foods")
	if got.Payee != "Whole Foods" {
		t.Fatalf("parseTxnSentence().Payee = %q, want %q", got.Payee, "Whole Foods")
	}
	if got.AmountMajor != "40" {
		t.Fatalf("parseTxnSentence().AmountMajor = %q, want 40", got.AmountMajor)
	}
	if got.Income {
		t.Fatal("expense sentence was parsed as income")
	}
}
