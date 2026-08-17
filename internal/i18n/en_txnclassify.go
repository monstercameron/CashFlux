// SPDX-License-Identifier: MIT

package i18n

// txnClassifyKeys holds the strings for the edit modal's movement classifier:
// naming the other account a row moved to or from, and claiming a card or loan
// payment as a payment. Merged via init so this file never touches en.go.
//
// The wording avoids the word "transfer" as a NOUN the user has to define. A
// statement import already labels half these rows "Transfer" while filing them as
// spending, so the word alone has proved it does not tell anyone what the app
// will do with the row. The control says what changes instead.
var txnClassifyKeys = Catalog{
	// C673: the group heading. "Other account this money moved to or from" named
	// a FIELD and left the reader to work out what filling it in would do; it read
	// as optional metadata beside the category, which is exactly how a row ends up
	// filed under a category called "Transfer" while still counting as spending.
	// The heading now names the ACTION, and the picker underneath is the question
	// it needs answered.
	"transactions.classifyGroup":     "Mark as transfer",
	"transactions.classifyGroupHint": "A transfer is money moving between accounts you own. Choose the account on the other side and this row stops counting as income or spending.",
	// The picker itself. Its first option is the way OUT of the classification,
	// so it has to read as a real answer rather than an empty slot.
	"transactions.classifyLabel": "Account on the other side",
	"transactions.classifyNone":  "None — this is income or spending",
	"transactions.classifyHint":  "Money you move between your own accounts was not earned or spent, so it drops out of income, spending, budgets and reports. Balances do not change either way.",
	// C673: the sign is a property of the LEG, not a classification. The same move
	// is negative where it left and positive where it arrived, so a reader looking
	// at one leg has to be told the minus does not mean "spending" here.
	"transactions.classifySignHint": "The amount stays as it is: it reads negative on the account the money left and positive on the account it reached. That is the same move seen from two sides, not a spend and an earn.",
	// C673: said next to the CATEGORY field, because choosing a category named
	// "Transfer" is the mistake this whole control exists to catch. A category
	// never removes a row from a total; only the account on the other side does.
	"transactions.classifyCategoryNote": "Choosing a category called “Transfer” does not make this a transfer — pick the account on the other side above.",
	// Shown under the picker once an account is chosen, so the consequence is
	// visible before saving rather than discovered afterwards.
	"transactions.classifyEffectNeutral": "Kept out of income and spending — it moved between your own accounts.",
	"transactions.classifyEffectDebt":    "Kept out of spending — it pays down what you owe on %s.",
	// The debt claim, offered only when the chosen account is a card or loan.
	// C674: whether the far side was found. Classifying a leg never creates its
	// counterpart — that would invent money — so the other side is usually another
	// imported row, and whether one exists changes what to expect. Saying nothing
	// is how a one-sided transfer is discovered later, from a balance that
	// disagrees with the bank.
	"transactions.classifyPairFound": "The matching row in %s will pair with this one.",
	"transactions.classifyPairNear":  "A row in %s looks like the other side, for %s — close, but not the same amount. Check it before relying on the pair.",
	"transactions.classifyPairNone":  "No matching row was found in %s. Only this side will change; the other account will not balance until its row is imported or added.",

	"transactions.classifyDebtLabel": "Count this as a payment toward %s",
	"transactions.classifyDebtHint":  "The debt pages read the most recent payment you have marked this way as what you actually paid.",

	// C675: what the row IS, said in the ledger. A transfer's amount is signed
	// account-relative, so income-green / spending-red argues the opposite of the
	// truth on one of the two legs — these badges carry the meaning instead.
	"transactions.badgeTransfer":          "Transfer",
	"transactions.badgeTransferTitle":     "Moved between your own accounts — not income or spending.",
	"transactions.badgeTransferWithTitle": "Moved between your own accounts, with %s — not income or spending.",
	// Distinct from Transfer, not a stronger version of it (C677): naming a card
	// says the money stayed in the household; counting it as a payment is a claim
	// about which debt it settles.
	"transactions.badgeDebtPayment":      "Debt payment",
	"transactions.badgeDebtPaymentTitle": "Counted as a payment toward %s — it reduces what you owe rather than being spending.",
	// C672: not structurally a transfer, but it looks like one, and it is counting
	// as income or spending right now. A question, because the app does not know.
	"transactions.badgeSuspectTransfer":      "Transfer?",
	"transactions.badgeSuspectTransferTitle": "This looks like a move between your own accounts, but no account is set on the other side — so it is still counting as income or spending. Open it to say which account it moved to or from.",
}

func init() {
	for k, v := range txnClassifyKeys {
		english[k] = v
	}
}
