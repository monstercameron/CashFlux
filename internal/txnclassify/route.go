// SPDX-License-Identifier: MIT

package txnclassify

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// # Turning a column of rows into a handful of decisions
//
// Suspects finds the rows that look like unlinked transfers. On a real import
// that is not three rows, it is every savings sweep for a year and every card
// payment — and a review surface that asks "which account?" once per row has
// converted a bad import into an afternoon.
//
// Almost all of those rows have the same answer. This file works out what that
// answer probably is, so the surface can offer one decision per counterparty
// instead of one per row, and the user confirms a grouping rather than repeating
// themselves.
//
// The discipline here is that a SUGGESTION IS NOT AN ANSWER. A wrong counterparty
// applied in bulk is two account balances wrong, discovered later, at the bank.
// So every rule below refuses to guess when the evidence is ambiguous: two
// plausible accounts means no suggestion and the rows land in the pile a person
// has to route by hand. Under-suggesting costs a few clicks; over-suggesting
// costs trust in every figure downstream.
//
// Nothing here writes. It reports what it believes and why, and PlanBulk decides
// what that would actually do.

// SuggestBasis is the evidence a suggestion rests on, so the surface can say why
// it thinks what it thinks. A user asked to confirm a grouping deserves to know
// whether the app found the other side of the move or merely read the label.
type SuggestBasis string

const (
	// BasisNone: no counterparty could be suggested.
	BasisNone SuggestBasis = ""
	// BasisReciprocal: an opposite-direction row of the same size sits in another
	// account within the pair window. This is not an inference — it is the other
	// leg — and it is the only basis that can be called evidence rather than
	// reading.
	BasisReciprocal SuggestBasis = "reciprocal"
	// BasisName: the row's own words name exactly one of the household's accounts
	// ("TRANSFER TO SAVINGS" where an account is called Savings).
	BasisName SuggestBasis = "name"
	// BasisCategory: the row is filed under a card- or loan-payment category and
	// the household has exactly one account of that shape.
	BasisCategory SuggestBasis = "category"
)

// Suggestion is a proposed counterparty for one suspect row.
type Suggestion struct {
	// AccountID is the account the row probably moved to or from.
	AccountID string
	// Basis is the evidence, so the surface can rank and explain it.
	Basis SuggestBasis
	// Exact is meaningful for BasisReciprocal: whether the far side's amount is
	// exactly opposite, or merely close (a wire fee makes the two sides differ).
	Exact bool
}

// minNameMatch is the shortest account name that may be matched inside a
// description. Anything shorter matches by accident: an account called "HSA"
// would claim "PURCHASE" if two characters were allowed, and a household that
// names an account "A" would see every row routed to it.
const minNameMatch = 3

// Suggest proposes the counterparty for one suspect row, or reports that it
// cannot tell.
//
// The rules run strongest first and stop at the first that answers:
//
//  1. A reciprocal row. If exactly one account holds an opposite-direction row
//     within the pair window, that account IS the far side.
//  2. The row's own words naming exactly one account.
//  3. A card- or loan-payment category where the household holds exactly one
//     account of that shape.
//
// "Exactly one" is doing the work in all three. Where two accounts could answer,
// the row gets no suggestion and stays a question for a person.
func Suggest(t domain.Transaction, reason SuspectReason, categoryName string,
	accounts []domain.Account, txns []domain.Transaction) (Suggestion, bool) {
	if t.Amount.Amount == 0 {
		return Suggestion{}, false
	}
	if s, ok := suggestByReciprocal(t, accounts, txns); ok {
		return s, true
	}
	if s, ok := suggestByName(t, accounts); ok {
		return s, true
	}
	if reason == SuspectCategory {
		if s, ok := suggestByCategory(t, categoryName, accounts); ok {
			return s, true
		}
	}
	return Suggestion{}, false
}

// suggestByReciprocal looks for the far side in every account the household
// holds, and answers only when one account offers it.
//
// FindReciprocal answers "is the far side in THIS account"; routing has to ask
// "which account holds it", which is the same search run across the candidates
// with an ambiguity check on top. Two accounts each holding a plausible partner
// is exactly the case where guessing puts money in the wrong place, so it
// declines — unless one of them matches exactly and the other only closely, in
// which case the exact one is not a guess.
func suggestByReciprocal(t domain.Transaction, accounts []domain.Account, txns []domain.Transaction) (Suggestion, bool) {
	var best Suggestion
	found, tied := false, false
	for _, acc := range accounts {
		if acc.ID == t.AccountID || acc.Archived {
			continue
		}
		m, ok := FindReciprocal(t, acc.ID, txns)
		if !ok {
			continue
		}
		// FindReciprocal's NEAR match requires only that the magnitudes are close,
		// not that the currencies agree — a tolerance that exists for wire fees, not
		// for exchange rates. A ¥50,000 row is not the far side of a $500 one, and
		// letting it in would rank a coincidence as this package's strongest
		// evidence tier.
		if m.Txn.Amount.Currency != t.Amount.Currency {
			continue
		}
		switch {
		case !found:
			best, found = Suggestion{AccountID: acc.ID, Basis: BasisReciprocal, Exact: m.Exact}, true
		case m.Exact && !best.Exact:
			// An exact partner beats a near one outright; the near one was never a
			// serious candidate for the same move.
			best, tied = Suggestion{AccountID: acc.ID, Basis: BasisReciprocal, Exact: true}, false
		case m.Exact == best.Exact:
			tied = true
		}
	}
	if !found || tied {
		return Suggestion{}, false
	}
	return best, true
}

// suggestByName matches the household's account names inside the row's own text.
//
// Matched on word boundaries over the normalized text, so "Savings" finds
// "TRANSFER TO SAVINGS" and "ONLINE XFER SAVINGS 1234" but an account called
// "Car" does not claim "CARWASH". Two accounts matching means the words did not
// settle it — "transfer savings to investment" names both sides and says nothing
// about which one this row moved to.
func suggestByName(t domain.Transaction, accounts []domain.Account) (Suggestion, bool) {
	text := normalizeName(t.Desc + " " + t.Payee)
	if text == "" {
		return Suggestion{}, false
	}
	words := strings.Fields(text)
	var hit string
	matches := 0
	for _, acc := range accounts {
		if acc.ID == t.AccountID || acc.Archived {
			continue
		}
		name := normalizeName(acc.Name)
		if len([]rune(name)) < minNameMatch {
			continue
		}
		if !containsPhrase(words, strings.Fields(name)) {
			continue
		}
		matches++
		hit = acc.ID
	}
	if matches != 1 {
		return Suggestion{}, false
	}
	return Suggestion{AccountID: hit, Basis: BasisName}, true
}

// containsPhrase reports whether phrase appears in words as a consecutive run of
// whole words. Whole-word matching is what keeps an account named "Car" out of
// "carwash" while still finding a two-word name like "Rainy Day".
func containsPhrase(words, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(words) {
		return false
	}
	for i := 0; i+len(phrase) <= len(words); i++ {
		match := true
		for j, p := range phrase {
			if words[i+j] != p {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// suggestByCategory reads a card- or loan-payment category as naming the shape of
// the account paid, and answers when the household holds exactly one of that
// shape.
//
// This is the weakest rule and the most tempting to over-apply. "Credit card
// payment" identifies an account only for a household with ONE card; with two it
// says which kind of thing happened and nothing about which card, and a bulk
// apply on that basis would pay down the wrong balance.
func suggestByCategory(t domain.Transaction, categoryName string, accounts []domain.Account) (Suggestion, bool) {
	var want func(domain.AccountType) bool
	switch normalizeName(categoryName) {
	case "credit card payment", "card payment":
		want = func(ty domain.AccountType) bool { return ty.IsRevolving() }
	case "loan payment":
		want = func(ty domain.AccountType) bool { return ty.IsInstallment() }
	default:
		return Suggestion{}, false
	}
	// A row already sitting ON an account of that shape is the RECEIVING leg — the
	// card's own record of being paid — and its counterparty is whatever asset
	// account sent the money, which this rule cannot see. Reading the category as
	// naming a destination there would route a Visa's payment credit to whichever
	// other card the household happens to hold. The rule only speaks about rows
	// paying INTO a shape, never about rows already in one.
	if own, ok := domain.AccountByID(accounts, t.AccountID); ok && want(own.Type) {
		return Suggestion{}, false
	}
	var hit string
	matches := 0
	for _, acc := range accounts {
		if acc.ID == t.AccountID || acc.Archived || !want(acc.Type) {
			continue
		}
		matches++
		hit = acc.ID
	}
	if matches != 1 {
		return Suggestion{}, false
	}
	return Suggestion{AccountID: hit, Basis: BasisCategory}, true
}

// Batch is a set of suspect rows that share a suggested counterparty — one
// decision instead of one per row.
type Batch struct {
	// CounterpartyID is the suggested account, or "" for the rows nothing could
	// route. The unrouted batch is not a failure state: it is the honest residue
	// of refusing to guess, and it is where a person's attention belongs.
	CounterpartyID string
	// Currency is the currency every row in the batch is denominated in.
	//
	// Batches split on it as well as on the counterparty, so LeakedMinor is a
	// number that can be shown as money. This package has no rate table by design;
	// a batch that mixed currencies would hand the caller a sum of unlike units
	// and invite exactly the presentation error the totals exist to prevent — a
	// figure with a currency symbol on it that means nothing.
	Currency string
	// Basis is the strongest evidence behind the grouping, so a surface can lead
	// with the batches it actually found the far side for.
	Basis SuggestBasis
	// Rows are the suspects, in input order.
	Rows []domain.Transaction
	// LeakedMinor is what these rows are currently adding to income and spending,
	// as a magnitude in minor units. Not FX-converted, per the package rule.
	LeakedMinor int64
}

// Count is how many rows the batch holds.
func (b Batch) Count() int { return len(b.Rows) }

// Routed reports whether the batch has a counterparty to apply.
func (b Batch) Routed() bool { return b.CounterpartyID != "" }

// basisRank orders the evidence, strongest first, so batches the app actually
// found the far side for are offered before ones it inferred from a label.
func basisRank(b SuggestBasis) int {
	switch b {
	case BasisReciprocal:
		return 0
	case BasisName:
		return 1
	case BasisCategory:
		return 2
	}
	return 3
}

// Batches groups suspects by the counterparty each was routed to, strongest
// evidence first, with the unroutable rows last.
//
// Ordering is deliberate rather than incidental: the batches the app is most
// confident about come first because they are the ones a user can confirm at a
// glance, and the pile it could not route comes last because it is the only part
// that needs real work. A surface that led with the hard pile would make the
// whole feature feel like it had failed.
//
// A batch's Basis is the strongest basis among its rows — a group whose far side
// was found for most rows is a "we found it" group, even if one row joined it by
// name.
func Batches(suspects []Suspect, categoryName func(id string) string,
	accounts []domain.Account, txns []domain.Transaction) []Batch {
	index := map[string]int{}
	var out []Batch
	for _, s := range suspects {
		name := ""
		if categoryName != nil {
			name = categoryName(s.Txn.CategoryID)
		}
		sug, _ := Suggest(s.Txn, s.Reason, name, accounts, txns)
		cur := s.Txn.Amount.Currency
		key := sug.AccountID + "|" + cur
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, Batch{CounterpartyID: sug.AccountID, Currency: cur, Basis: sug.Basis})
		}
		amt := s.Txn.Amount.Amount
		if amt < 0 {
			amt = -amt
		}
		out[i].Rows = append(out[i].Rows, s.Txn)
		out[i].LeakedMinor += amt
		if basisRank(sug.Basis) < basisRank(out[i].Basis) {
			out[i].Basis = sug.Basis
		}
	}
	// Strongest evidence first; unrouted last however many rows it holds. Ties
	// break on row count so the biggest saving is offered first, then on account
	// id so the order cannot wobble between renders of the same data.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Routed() != b.Routed() {
			return a.Routed()
		}
		if ra, rb := basisRank(a.Basis), basisRank(b.Basis); ra != rb {
			return ra < rb
		}
		if len(a.Rows) != len(b.Rows) {
			return len(a.Rows) > len(b.Rows)
		}
		if a.CounterpartyID != b.CounterpartyID {
			return a.CounterpartyID < b.CounterpartyID
		}
		return a.Currency < b.Currency
	})
	return out
}
