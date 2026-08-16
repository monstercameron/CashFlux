// SPDX-License-Identifier: MIT

package imageroute

import (
	"strings"
	"testing"
)

func TestAReceiptBecomesAProposedTransaction(t *testing.T) {
	got := Decide(Reading{Merchant: "Trader Joe's", TotalMinor: 4210, LineItems: 7})
	if got.Destination != ToTransaction || !got.Sure {
		t.Fatalf("route = %+v", got)
	}
	if got.Why == "" {
		t.Fatal("a routing decision with no stated reason cannot be corrected")
	}
}

func TestManyDatedRowsMeanAStatementNotALongReceipt(t *testing.T) {
	// A supermarket receipt has many LINE ITEMS and one date; a statement has many
	// dates. Confusing the two sends a whole month's rows into a single-transaction
	// preview.
	long := Decide(Reading{Merchant: "Costco", TotalMinor: 18000, LineItems: 40})
	if long.Destination != ToTransaction {
		t.Fatalf("a 40-item receipt routed to %v", long.Destination)
	}
	statement := Decide(Reading{Merchant: "Chase", Rows: 22})
	if statement.Destination != ToStatement || !statement.Sure {
		t.Fatalf("route = %+v", statement)
	}
}

func TestAnExplicitSplitCueRoutesToASharedExpense(t *testing.T) {
	got := Decide(Reading{Merchant: "Sushi Place", TotalMinor: 8800, Text: "dinner, split with Priya"})
	if got.Destination != ToSplit || !got.Sure {
		t.Fatalf("route = %+v", got)
	}
}

func TestASplitCueIsOnlyATieBreakerNotAKeyword(t *testing.T) {
	// A shop called "Split Coffee" must not become a shared expense. The cue is
	// a phrase about sharing, not the word on its own.
	got := Decide(Reading{Merchant: "Split Coffee", TotalMinor: 450, Text: "Split Coffee Co · 1 flat white"})
	if got.Destination != ToTransaction {
		t.Fatalf("a merchant name containing 'split' routed to %v", got.Destination)
	}
}

func TestASplitCueDoesNotOverrideAStatement(t *testing.T) {
	// Rows come first: a statement mentioning a split is still a statement, and
	// routing it to the split flow would drop every other row on the page.
	got := Decide(Reading{Merchant: "Chase", Rows: 12, Text: "we split with the joint account"})
	if got.Destination != ToStatement {
		t.Fatalf("route = %v", got.Destination)
	}
}

func TestAnUnreadableImageIsFiledRatherThanGuessedAt(t *testing.T) {
	got := Decide(Reading{Unreadable: true, Text: "blurry"})
	if got.Destination != ToDocument || !got.Sure {
		t.Fatalf("route = %+v", got)
	}
	if !strings.Contains(got.Why, "couldn't read") {
		t.Fatalf("reason = %q", got.Why)
	}
}

func TestTextWithNoMerchantAndNoTotalIsFiledNotProposed(t *testing.T) {
	// Proposing a transaction with neither a payee nor an amount creates a record
	// somebody then has to go and fix — worse than filing the picture.
	got := Decide(Reading{Text: "TERMS AND CONDITIONS OF SERVICE"})
	if got.Destination != ToDocument {
		t.Fatalf("route = %v", got.Destination)
	}
}

func TestHalfAReceiptStillRoutesButAdmitsItIsGuessing(t *testing.T) {
	// A total with no merchant is worth proposing; being wrong should cost a click,
	// not a wrong record — so it routes, flagged.
	got := Decide(Reading{TotalMinor: 4210})
	if got.Destination != ToTransaction {
		t.Fatalf("route = %v", got.Destination)
	}
	if got.Sure {
		t.Fatal("a reading with no merchant reported itself as certain")
	}
	if !strings.Contains(got.Why, "check the details") {
		t.Fatalf("reason = %q, want it to say what is uncertain", got.Why)
	}
}

func TestAMerchantWithNoTotalIsAGuessToo(t *testing.T) {
	got := Decide(Reading{Merchant: "Trader Joe's"})
	if got.Destination != ToTransaction || got.Sure {
		t.Fatalf("route = %+v, want an unsure transaction", got)
	}
}

func TestEveryDestinationHasAButtonLabel(t *testing.T) {
	for _, d := range []Destination{ToTransaction, ToSplit, ToStatement, ToDocument} {
		if strings.TrimSpace(d.Label()) == "" {
			t.Errorf("destination %v has no label", d)
		}
	}
}

func TestAlternativesOfferEveryOtherRouteSoAWrongGuessCostsAClick(t *testing.T) {
	// Making somebody re-photograph a receipt to correct the app's classification
	// is the least forgivable version of this feature.
	for _, d := range []Destination{ToTransaction, ToSplit, ToStatement, ToDocument} {
		alts := d.Alternatives()
		if len(alts) != 3 {
			t.Fatalf("%v offered %d alternatives, want 3", d, len(alts))
		}
		for _, a := range alts {
			if a == d {
				t.Fatalf("%v offered itself as an alternative", d)
			}
		}
	}
}
