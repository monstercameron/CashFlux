// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var routeDay = time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

func routeAcct(id, name string, ty domain.AccountType) domain.Account {
	return domain.Account{ID: id, Name: name, Type: ty, Class: ty.Class(), Currency: "USD"}
}

func routeTxn(id, acct, desc string, minor int64, day time.Time) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Desc: desc, Date: day,
		Amount: money.New(minor, "USD"),
	}
}

// The far side of a move is evidence, not a reading — when exactly one account
// holds it, that account IS the counterparty.
func TestSuggestPrefersTheReciprocalRow(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-save", "Rainy Day", domain.TypeSavings),
	}
	out := routeTxn("t-out", "a-check", "ONLINE XFER", -50000, routeDay)
	in := routeTxn("t-in", "a-save", "ONLINE XFER", 50000, routeDay.AddDate(0, 0, 1))

	got, ok := Suggest(out, SuspectDescription, "", accounts, []domain.Transaction{out, in})
	if !ok {
		t.Fatal("no suggestion where the far side is sitting in another account")
	}
	if got.AccountID != "a-save" {
		t.Errorf("suggested %q, want a-save", got.AccountID)
	}
	if got.Basis != BasisReciprocal {
		t.Errorf("basis = %q, want %q", got.Basis, BasisReciprocal)
	}
	if !got.Exact {
		t.Error("an exactly-opposite amount should report as an exact match")
	}
}

// Two accounts each holding a plausible partner is exactly when guessing puts
// money in the wrong place.
func TestSuggestDeclinesWhenTwoAccountsCouldBeTheFarSide(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-one", "Savings One", domain.TypeSavings),
		routeAcct("a-two", "Savings Two", domain.TypeSavings),
	}
	out := routeTxn("t-out", "a-check", "ONLINE XFER", -50000, routeDay)
	txns := []domain.Transaction{
		out,
		routeTxn("t-a", "a-one", "ONLINE XFER", 50000, routeDay),
		routeTxn("t-b", "a-two", "ONLINE XFER", 50000, routeDay),
	}
	if _, ok := Suggest(out, SuspectDescription, "", accounts, txns); ok {
		t.Error("suggested a counterparty where two accounts held an equally good partner")
	}
}

// An exact partner is not a guess even when a near one exists elsewhere.
func TestSuggestExactPartnerBeatsANearOne(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-exact", "Alpha", domain.TypeSavings),
		routeAcct("a-near", "Beta", domain.TypeSavings),
	}
	out := routeTxn("t-out", "a-check", "ONLINE XFER", -50000, routeDay)
	txns := []domain.Transaction{
		out,
		routeTxn("t-near", "a-near", "ONLINE XFER", 49500, routeDay),
		routeTxn("t-exact", "a-exact", "ONLINE XFER", 50000, routeDay),
	}
	got, ok := Suggest(out, SuspectDescription, "", accounts, txns)
	if !ok {
		t.Fatal("declined where one account held an exact partner")
	}
	if got.AccountID != "a-exact" || !got.Exact {
		t.Errorf("suggested %+v, want the exact partner in a-exact", got)
	}
}

// The row's own words name an account — matched on whole words, so a short name
// cannot be claimed by a longer one it happens to sit inside.
func TestSuggestByName(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-save", "Savings", domain.TypeSavings),
		routeAcct("a-car", "Car", domain.TypeSavings),
		routeAcct("a-rd", "Rainy Day", domain.TypeSavings),
	}
	cases := []struct {
		name string
		desc string
		want string
	}{
		{"names the account", "TRANSFER TO SAVINGS", "a-save"},
		{"with trailing digits", "ONLINE XFER SAVINGS 1234", "a-save"},
		{"two-word name", "TRANSFER TO RAINY DAY", "a-rd"},
		{"not a substring hit", "CARWASH DOWNTOWN", ""},
		{"names two accounts settles nothing", "TRANSFER SAVINGS TO CAR", ""},
		{"names none", "ONLINE TRANSFER", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx := routeTxn("t1", "a-check", c.desc, -50000, routeDay)
			got, ok := Suggest(tx, SuspectDescription, "", accounts, []domain.Transaction{tx})
			if c.want == "" {
				if ok {
					t.Errorf("suggested %q from %q, want no suggestion", got.AccountID, c.desc)
				}
				return
			}
			if !ok {
				t.Fatalf("no suggestion from %q", c.desc)
			}
			if got.AccountID != c.want {
				t.Errorf("suggested %q, want %q", got.AccountID, c.want)
			}
			if got.Basis != BasisName {
				t.Errorf("basis = %q, want %q", got.Basis, BasisName)
			}
		})
	}
}

// A payment category names the SHAPE of the account paid. With one account of
// that shape it identifies it; with two it says nothing about which.
func TestSuggestByCategoryNeedsExactlyOneAccountOfTheShape(t *testing.T) {
	one := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-visa", "Visa", domain.TypeCreditCard),
		routeAcct("a-loan", "Car Loan", domain.TypeLoan),
	}
	tx := routeTxn("t1", "a-check", "PAYMENT THANK YOU", -20000, routeDay)

	got, ok := Suggest(tx, SuspectCategory, "Credit card payment", one, []domain.Transaction{tx})
	if !ok || got.AccountID != "a-visa" || got.Basis != BasisCategory {
		t.Errorf("card payment suggested %+v (ok=%v), want a-visa by category", got, ok)
	}
	got, ok = Suggest(tx, SuspectCategory, "Loan payment", one, []domain.Transaction{tx})
	if !ok || got.AccountID != "a-loan" {
		t.Errorf("loan payment suggested %+v (ok=%v), want a-loan", got, ok)
	}

	two := append(one, routeAcct("a-amex", "Amex", domain.TypeCreditCard))
	if _, ok := Suggest(tx, SuspectCategory, "Credit card payment", two, []domain.Transaction{tx}); ok {
		t.Error("suggested a card where the household holds two — a bulk apply would pay the wrong balance")
	}

	// The category rule is only consulted for a category-based suspicion: a row
	// suspected by its description must not be routed by whatever it is filed as.
	if _, ok := Suggest(tx, SuspectDescription, "Credit card payment", one, []domain.Transaction{tx}); ok {
		t.Error("a description-suspected row was routed by its category")
	}
}

// An archived account is not somewhere money is moving now, and must not absorb
// rows by name or shape.
func TestSuggestIgnoresArchivedAccounts(t *testing.T) {
	archived := routeAcct("a-old", "Savings", domain.TypeSavings)
	archived.Archived = true
	accounts := []domain.Account{routeAcct("a-check", "Checking", domain.TypeChecking), archived}
	tx := routeTxn("t1", "a-check", "TRANSFER TO SAVINGS", -50000, routeDay)
	if _, ok := Suggest(tx, SuspectDescription, "", accounts, []domain.Transaction{tx}); ok {
		t.Error("routed a row to an archived account")
	}
}

// A row cannot be a transfer to its own account, so its own name must never
// route it.
func TestSuggestNeverRoutesToTheRowsOwnAccount(t *testing.T) {
	accounts := []domain.Account{routeAcct("a-save", "Savings", domain.TypeSavings)}
	tx := routeTxn("t1", "a-save", "TRANSFER TO SAVINGS", -50000, routeDay)
	if _, ok := Suggest(tx, SuspectDescription, "", accounts, []domain.Transaction{tx}); ok {
		t.Error("routed a row to the account it already sits in")
	}
}

// Batches turn a column of rows into a handful of decisions, strongest evidence
// first, with the rows nothing could route left for a person at the end.
func TestBatchesGroupAndOrder(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-save", "Savings", domain.TypeSavings),
		routeAcct("a-visa", "Visa", domain.TypeCreditCard),
	}
	// Two rows routed by name, one by category, one unroutable.
	rows := []domain.Transaction{
		routeTxn("t1", "a-check", "TRANSFER TO SAVINGS", -50000, routeDay),
		routeTxn("t2", "a-check", "TRANSFER TO SAVINGS", -25000, routeDay.AddDate(0, 0, 30)),
		routeTxn("t3", "a-check", "PAYMENT THANK YOU", -20000, routeDay),
		routeTxn("t4", "a-check", "ONLINE TRANSFER", -10000, routeDay),
	}
	catName := func(id string) string {
		if id == "c-card" {
			return "Credit card payment"
		}
		return ""
	}
	rows[2].CategoryID = "c-card"

	suspects := []Suspect{
		{Txn: rows[0], Reason: SuspectDescription},
		{Txn: rows[1], Reason: SuspectDescription},
		{Txn: rows[2], Reason: SuspectCategory},
		{Txn: rows[3], Reason: SuspectDescription},
	}
	got := Batches(suspects, catName, accounts, rows)
	if len(got) != 3 {
		t.Fatalf("got %d batches, want 3 (savings, visa, unrouted): %+v", len(got), got)
	}
	// Name beats category, and the unrouted pile is last whatever it holds.
	if got[0].CounterpartyID != "a-save" || got[0].Count() != 2 {
		t.Errorf("first batch = %+v, want the two savings rows", got[0])
	}
	if got[0].LeakedMinor != 75000 {
		t.Errorf("savings batch leaked = %d, want 75000", got[0].LeakedMinor)
	}
	if got[1].CounterpartyID != "a-visa" || got[1].Basis != BasisCategory {
		t.Errorf("second batch = %+v, want the card row by category", got[1])
	}
	last := got[len(got)-1]
	if last.Routed() || last.Count() != 1 || last.Rows[0].ID != "t4" {
		t.Errorf("last batch = %+v, want the single unroutable row", last)
	}
}

// A batch carries the STRONGEST evidence among its rows, so a group whose far
// side was found does not present as a guess because one row joined it by name.
func TestBatchBasisTakesTheStrongestEvidence(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-save", "Savings", domain.TypeSavings),
	}
	byName := routeTxn("t-name", "a-check", "TRANSFER TO SAVINGS", -25000, routeDay)
	byPair := routeTxn("t-pair", "a-check", "ONLINE XFER", -50000, routeDay)
	partner := routeTxn("t-far", "a-save", "ONLINE XFER", 50000, routeDay)
	txns := []domain.Transaction{byName, byPair, partner}

	got := Batches([]Suspect{
		{Txn: byName, Reason: SuspectDescription},
		{Txn: byPair, Reason: SuspectDescription},
	}, nil, accounts, txns)

	if len(got) != 1 {
		t.Fatalf("got %d batches, want 1 — both rows route to Savings: %+v", len(got), got)
	}
	if got[0].Basis != BasisReciprocal {
		t.Errorf("batch basis = %q, want %q — the strongest evidence in the group", got[0].Basis, BasisReciprocal)
	}
	if got[0].Count() != 2 {
		t.Errorf("batch holds %d rows, want 2", got[0].Count())
	}
}

// Batching is stable: the same suspects in the same order produce the same
// batches in the same order, so a re-render cannot reshuffle what is on screen
// under a user's cursor.
func TestBatchesAreStable(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-one", "Alpha", domain.TypeSavings),
		routeAcct("a-two", "Beta", domain.TypeSavings),
	}
	rows := []domain.Transaction{
		routeTxn("t1", "a-check", "TRANSFER TO ALPHA", -10000, routeDay),
		routeTxn("t2", "a-check", "TRANSFER TO BETA", -10000, routeDay),
	}
	suspects := []Suspect{{Txn: rows[0], Reason: SuspectDescription}, {Txn: rows[1], Reason: SuspectDescription}}

	first := Batches(suspects, nil, accounts, rows)
	for i := 0; i < 20; i++ {
		again := Batches(suspects, nil, accounts, rows)
		if len(again) != len(first) {
			t.Fatalf("batch count wobbled: %d then %d", len(first), len(again))
		}
		for j := range first {
			if again[j].CounterpartyID != first[j].CounterpartyID {
				t.Fatalf("batch order wobbled at %d: %q then %q", j, first[j].CounterpartyID, again[j].CounterpartyID)
			}
		}
	}
}

// An empty input is an empty result, not a nil-deref or a phantom unrouted batch.
func TestBatchesEmpty(t *testing.T) {
	if got := Batches(nil, nil, nil, nil); len(got) != 0 {
		t.Errorf("got %d batches from no suspects, want 0", len(got))
	}
}

// Review finding 1: the category rule starts from the SHAPE of an account rather
// than from the row, so it has to be told that a row's own account is never its
// counterparty. A card's own on-statement payment credit is exactly that row.
func TestSuggestByCategoryNeverRoutesToTheRowsOwnAccount(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-check", "Checking", domain.TypeChecking),
		routeAcct("a-visa", "Visa", domain.TypeCreditCard),
	}
	// The row is posted TO the card itself.
	tx := routeTxn("t1", "a-visa", "PAYMENT THANK YOU", 20000, routeDay)
	if got, ok := Suggest(tx, SuspectCategory, "Credit card payment", accounts, []domain.Transaction{tx}); ok {
		t.Errorf("routed a card's own row to %q — a row cannot be a transfer to its own account", got.AccountID)
	}
	// And with a second card in the household it still declines, rather than
	// routing the Visa's own payment credit to the Amex: a row already sitting on
	// an account of that shape is the receiving leg, and its counterparty is the
	// asset account that paid, which this rule cannot see.
	two := append(accounts, routeAcct("a-amex", "Amex", domain.TypeCreditCard))
	if got, ok := Suggest(tx, SuspectCategory, "Credit card payment", two, []domain.Transaction{tx}); ok {
		t.Errorf("routed a card's own payment credit to %q", got.AccountID)
	}
	// But a row from checking, with one card, still routes.
	fromCheck := routeTxn("t2", "a-check", "AUTOPAY", -20000, routeDay)
	if got, ok := Suggest(fromCheck, SuspectCategory, "Credit card payment", accounts, []domain.Transaction{fromCheck}); !ok || got.AccountID != "a-visa" {
		t.Errorf("the ordinary card payment stopped routing: %+v ok=%v", got, ok)
	}
}

// Review finding 2a: FindReciprocal's near-match tolerance exists for wire fees,
// not exchange rates. A same-magnitude row in another currency is a coincidence,
// and must not rank as this package's strongest evidence.
func TestSuggestIgnoresAReciprocalInAnotherCurrency(t *testing.T) {
	yen := routeAcct("a-yen", "Tokyo", domain.TypeSavings)
	yen.Currency = "JPY"
	accounts := []domain.Account{routeAcct("a-check", "Checking", domain.TypeChecking), yen}

	out := routeTxn("t-out", "a-check", "ONLINE XFER", -50000, routeDay)
	far := domain.Transaction{
		ID: "t-far", AccountID: "a-yen", Desc: "ONLINE XFER", Date: routeDay,
		Amount: money.New(50000, "JPY"),
	}
	if _, ok := Suggest(out, SuspectDescription, "", accounts, []domain.Transaction{out, far}); ok {
		t.Error("a ¥50,000 row was taken as the far side of a $500 one")
	}
}

// Review finding 2b: a batch is one counterparty AND one currency, so its total
// is a number that can be shown as money.
func TestBatchesSplitByCurrency(t *testing.T) {
	accounts := []domain.Account{
		routeAcct("a-usd", "US Checking", domain.TypeChecking),
		routeAcct("a-eur", "EU Checking", domain.TypeChecking),
		routeAcct("a-save", "Savings", domain.TypeSavings),
	}
	usd := routeTxn("t-usd", "a-usd", "TRANSFER TO SAVINGS", -50000, routeDay)
	eur := domain.Transaction{
		ID: "t-eur", AccountID: "a-eur", Desc: "TRANSFER TO SAVINGS", Date: routeDay,
		Amount: money.New(-40000, "EUR"),
	}
	rows := []domain.Transaction{usd, eur}
	got := Batches([]Suspect{
		{Txn: usd, Reason: SuspectDescription},
		{Txn: eur, Reason: SuspectDescription},
	}, nil, accounts, rows)

	if len(got) != 2 {
		t.Fatalf("got %d batches, want 2 — one per currency: %+v", len(got), got)
	}
	seen := map[string]int64{}
	for _, b := range got {
		if b.CounterpartyID != "a-save" {
			t.Errorf("batch routed to %q, want a-save", b.CounterpartyID)
		}
		if b.Currency == "" {
			t.Error("a batch carries no currency, so its total cannot be shown as money")
		}
		seen[b.Currency] = b.LeakedMinor
	}
	if seen["USD"] != 50000 || seen["EUR"] != 40000 {
		t.Errorf("per-currency totals = %+v, want USD 50000 and EUR 40000", seen)
	}
}

// Review finding 6: an uncertain pairing is reported, not guessed at. Two equally
// good candidates on the far side is ambiguous; an exact one beside a near one is
// not, because the exact one is the partner.
func TestAmbiguousLeg(t *testing.T) {
	base := routeTxn("t-out", "a-check", "ONLINE XFER", -50000, routeDay)
	mk := func(id string, minor int64, dayOffset int) domain.Transaction {
		return routeTxn(id, "a-save", "ONLINE XFER", minor, routeDay.AddDate(0, 0, dayOffset))
	}
	cases := []struct {
		name string
		far  []domain.Transaction
		want bool
	}{
		{"no candidates", nil, false},
		{"one exact", []domain.Transaction{mk("f1", 50000, 0)}, false},
		{"two exact", []domain.Transaction{mk("f1", 50000, 0), mk("f2", 50000, 1)}, true},
		{"exact beats near", []domain.Transaction{mk("f1", 50000, 0), mk("f2", 49000, 0)}, false},
		{"two near, same gap", []domain.Transaction{mk("f1", 49000, 1), mk("f2", 48000, 1)}, true},
		{"two near, different gaps", []domain.Transaction{mk("f1", 49000, 1), mk("f2", 48000, 2)}, false},
		{"outside the window", []domain.Transaction{mk("f1", 50000, 0), mk("f2", 50000, 9)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			txns := append([]domain.Transaction{base}, c.far...)
			if got := AmbiguousLeg(base, "a-save", txns); got != c.want {
				t.Errorf("AmbiguousLeg = %v, want %v", got, c.want)
			}
		})
	}
	if AmbiguousLeg(base, "", []domain.Transaction{base}) {
		t.Error("no counterparty cannot be ambiguous")
	}
}
