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
	// The picker itself. Its first option is the way OUT of the classification,
	// so it has to read as a real answer rather than an empty slot.
	"transactions.classifyLabel": "Other account this money moved to or from",
	"transactions.classifyNone":  "None — this is income or spending",
	"transactions.classifyHint":  "Money you move between your own accounts was not earned or spent, so it drops out of income, spending, budgets and reports. Balances do not change either way.",
	// Shown under the picker once an account is chosen, so the consequence is
	// visible before saving rather than discovered afterwards.
	"transactions.classifyEffectNeutral": "Kept out of income and spending — it moved between your own accounts.",
	"transactions.classifyEffectDebt":    "Kept out of spending — it pays down what you owe on %s.",
	// The debt claim, offered only when the chosen account is a card or loan.
	"transactions.classifyDebtLabel": "Count this as a payment toward %s",
	"transactions.classifyDebtHint":  "The debt pages read the most recent payment you have marked this way as what you actually paid.",
}

func init() {
	for k, v := range txnClassifyKeys {
		english[k] = v
	}
}
