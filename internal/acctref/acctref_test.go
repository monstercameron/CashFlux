// SPDX-License-Identifier: MIT

package acctref

import "testing"

// Every descriptor here is a real line from a credit-union export. They are the
// reason the package exists, so they are the first thing it is held to.
func TestRealStatementDescriptors(t *testing.T) {
	tests := []struct {
		desc   string
		digits string
		dir    Direction
	}{
		{"Transfer to Savings *6500", "6500", To},
		{"Transfer to Account Ending 1677", "1677", To},
		{"Transfer to account ending 1677", "1677", To},
		{"Transfer to account ending in 8945", "8945", To},
		{"Transfer to Account XXXXXXXXX1677", "1677", To},
		{"Transfer to Account *1958", "1958", To},
		{"Transfer to CNS *1958", "1958", To},
		{"Transfer to account ending 1958 — CNS", "1958", To},
		{"Transfer from Savings *6500", "6500", From},
		{"Transfer from Checking *8945", "8945", From},
		{"Transfer from checking ending 8945 to savings", "8945", From},
		{"ACH deposit — Internet transfer from account ending in 8945", "8945", From},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, ok := One(tt.desc)
			if !ok {
				t.Fatalf("named no account — the far side is unrecoverable")
			}
			if got.Digits != tt.digits {
				t.Errorf("digits = %q, want %q", got.Digits, tt.digits)
			}
			if got.Direction != tt.dir {
				t.Errorf("direction = %s, want %s", got.Direction, tt.dir)
			}
		})
	}
}

// A wrong pairing moves money between the wrong two accounts, so a bare number
// must never be read as an account.
func TestBareNumbersAreNotAccountReferences(t *testing.T) {
	for _, desc := range []string{
		"AMZN MKTP US*2A4BC1234",
		"SHELL OIL 57445208",
		"Payment 1234567890123",
		"POS PURCHASE 4523",
		"Invoice 8945 paid",
		"Deposit",
		"",
		"Check 1099",
	} {
		if got, ok := One(desc); ok {
			t.Errorf("One(%q) = %+v — a bare number was read as an account", desc, got)
		}
	}
}

// "transfer from 8945 to 6500" names two accounts. Picking one would be a guess
// dressed as a reading, so it declines.
func TestTwoDifferentAccountsIsNotOne(t *testing.T) {
	const desc = "Transfer from account 8945 to account 6500"
	refs := Refs(desc)
	if len(refs) != 2 {
		t.Fatalf("Refs = %+v, want both accounts", refs)
	}
	if refs[0].Digits != "8945" || refs[0].Direction != From {
		t.Errorf("first ref = %+v, want 8945 from", refs[0])
	}
	if refs[1].Digits != "6500" || refs[1].Direction != To {
		t.Errorf("second ref = %+v, want 6500 to", refs[1])
	}
	if got, ok := One(desc); ok {
		t.Errorf("One = %+v, want a refusal on an ambiguous description", got)
	}
}

// The same account named twice is not ambiguous.
func TestSameAccountNamedTwiceResolves(t *testing.T) {
	got, ok := One("Transfer to account 6500 (savings *6500)")
	if !ok {
		t.Fatalf("declined a description that names one account twice")
	}
	if got.Digits != "6500" {
		t.Errorf("digits = %q, want 6500", got.Digits)
	}
}

// The nearest to/from governs, not the first — "from checking ending 8945 to
// savings" is a movement FROM 8945.
func TestNearestDirectionWordGoverns(t *testing.T) {
	got, ok := One("Transfer from checking ending 8945 to savings")
	if !ok {
		t.Fatal("named no account")
	}
	if got.Direction != From {
		t.Errorf("direction = %s, want from — the trailing 'to savings' is the other end", got.Direction)
	}
}

func TestDirectionCanBeAbsent(t *testing.T) {
	got, ok := One("Internal movement acct 6500")
	if !ok {
		t.Fatal("named no account")
	}
	if got.Direction != Unknown {
		t.Errorf("direction = %s, want unknown", got.Direction)
	}
}

// A masked full account number is the same reference as its printed last four.
func TestMaskedNumberNormalizesToItsLastFour(t *testing.T) {
	for _, desc := range []string{
		"Transfer to Account XXXXXXXXX1677",
		"Transfer to Account xxxx1677",
		"Transfer to account ending 1677",
		"Transfer to *1677",
	} {
		got, ok := One(desc)
		if !ok || got.Digits != "1677" {
			t.Errorf("One(%q) = %+v ok=%v, want 1677", desc, got, ok)
		}
	}
}

// Names is how a row filed onto the very account its description calls the far
// side gets spotted — the signature of an export that put both legs on one
// account.
func TestNamesSpotsASelfReference(t *testing.T) {
	if !Names("Transfer from Checking *8945", "8945") {
		t.Errorf("did not spot the self-reference")
	}
	if !Names("Transfer from Checking *8945", "008945") {
		t.Errorf("a longer stored number should compare on its last four")
	}
	if Names("Transfer to Savings *6500", "8945") {
		t.Errorf("matched an account the description does not name")
	}
	if Names("Transfer to Savings *6500", "") {
		t.Errorf("empty digits matched something")
	}
}

func TestShortAndLongRunsAreRejected(t *testing.T) {
	if _, ok := One("Transfer to account 65"); ok {
		t.Errorf("a two-digit run was read as an account")
	}
	// A long BARE run is a reference number...
	if _, ok := One("Transfer to account 1234567890123"); ok {
		t.Errorf("a thirteen-digit bare run was read as an account")
	}
	// ...but a masked one is exactly how a full number prints.
	got, ok := One("Transfer to account xxxxxxxxx1677")
	if !ok || got.Digits != "1677" {
		t.Errorf("masked full number = %+v ok=%v, want 1677", got, ok)
	}
}

// The marker window must not reach across a descriptor to bless an unrelated
// number sitting far away from the account word.
func TestMarkerDoesNotReachAcrossTheDescriptor(t *testing.T) {
	const desc = "Account maintenance fee charged on the fifteenth of this month ref 4471"
	if got, ok := One(desc); ok {
		t.Errorf("One = %+v — the marker reached %d characters to an unrelated number", got, len(desc))
	}
}

// Found by an adversarial pass: "acc" is a marker word and was matched as a
// SUBSTRING, so any merchant merely beginning with those letters, followed by a
// reference number, produced a confident wrong account reference — and a wrong
// account reference is the input to a wrong pairing.
func TestMarkerWordsAreNotMatchedInsideOtherWords(t *testing.T) {
	for _, desc := range []string{
		"Acceleration Fee 12345",
		"ACCENTURE CONSULTING 4471",
		"Access Self Storage 8802",
		"Accessorize 3310",
		"ACCLAIM ENERGY 5567",
		"According to invoice 9912",
	} {
		if got, ok := One(desc); ok {
			t.Errorf("One(%q) = %+v — a marker matched inside a longer word", desc, got)
		}
	}
}

// The real markers must still work, including the multi-word one.
func TestMarkerWordsStillMatchWhenTheyAreWords(t *testing.T) {
	for _, tt := range []struct {
		desc   string
		digits string
	}{
		{"Transfer to acc 6500", "6500"},
		{"Transfer to acct 6500", "6500"},
		{"Transfer to account 6500", "6500"},
		{"Transfer to account ending in 6500", "6500"},
		{"Transfer, account ending 6500", "6500"},
		{"Transfer to a/c 6500", "6500"},
	} {
		got, ok := One(tt.desc)
		if !ok || got.Digits != tt.digits {
			t.Errorf("One(%q) = %+v ok=%v, want %s", tt.desc, got, ok, tt.digits)
		}
	}
}
