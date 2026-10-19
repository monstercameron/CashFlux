// SPDX-License-Identifier: MIT

package importdraft

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
	"github.com/monstercameron/CashFlux/internal/ordermatch"
	"github.com/monstercameron/CashFlux/internal/receiptsplit"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// FindTransaction resolves the first matching ledger identity.
func FindTransaction(txns []domain.Transaction, id string) (domain.Transaction, bool) {
	for _, txn := range txns {
		if txn.ID == id {
			return txn, true
		}
	}
	return domain.Transaction{}, false
}

// OrderCharges selects expense charges, excluding income and transfers.
func OrderCharges(txns []domain.Transaction) []orderimport.Charge {
	var out []orderimport.Charge
	for _, txn := range txns {
		if txn.IsTransfer() || !txn.Amount.IsNegative() {
			continue
		}
		out = append(out, orderimport.Charge{TxnID: txn.ID, Date: txn.Date, AmountMinor: txn.Amount.Amount, Currency: txn.Amount.Currency})
	}
	return out
}

// OrderItem returns the displayed quantity and checked extended price.
func OrderItem(item orderimport.Item) (quantity int, total int64, err error) {
	quantity = max(1, item.Qty)
	total, err = money.Mul(item.UnitMinor, int64(quantity))
	return
}

// OrderItemLabel formats a checked line using the existing item and error copy.
func OrderItemLabel(item orderimport.Item, cur string, translate Translate) string {
	qty, total, err := OrderItem(item)
	if err != nil {
		return translate("documents.linesUnreadable")
	}
	return translate("orderimport.itemLine", qty, item.Name, money.FormatAccounting(total, currency.Decimals(cur), currency.Symbol(cur)))
}

// OrderSplit maps order items to expense categories before proposing a balanced split.
// The returned key names the existing notice when no proposal is possible.
func OrderSplit(txn domain.Transaction, order orderimport.Order, categories []domain.Category, userRules []rules.Rule) (receiptsplit.Proposal, string) {
	if _, err := money.Abs(txn.Amount.Amount); err != nil {
		return receiptsplit.Proposal{}, "receiptsplit.noProposal"
	}
	cur := txn.Amount.Currency
	if cur == "" {
		cur = order.Currency
	}
	lineCategories := map[string]string{}
	var items []receiptsplit.LineItem
	var subtotal int64
	for _, item := range order.Items {
		if item.Name == "" {
			continue
		}
		categoryID := expenseCategory(categories, item.Name)
		if categoryID == "" {
			if rule := rules.FirstMatch(userRules, item.Name); rule != nil {
				categoryID = rule.SetCategoryID
			}
		}
		lineCategories[item.Name] = categoryID
		_, amount, err := OrderItem(item)
		if err != nil {
			return receiptsplit.Proposal{}, "receiptsplit.noProposal"
		}
		magnitude, err := money.Abs(amount)
		if err != nil {
			return receiptsplit.Proposal{}, "receiptsplit.noProposal"
		}
		subtotal, err = money.Add(subtotal, magnitude)
		if err != nil {
			return receiptsplit.Proposal{}, "receiptsplit.noProposal"
		}
		items = append(items, receiptsplit.LineItem{Name: item.Name, Amount: money.New(amount, cur)})
	}
	if len(items) == 0 {
		return receiptsplit.Proposal{}, "orderimport.noItems"
	}
	proposal, ok := receiptsplit.Propose(items, receiptsplit.Target{Amount: txn.Amount, CategoryID: txn.CategoryID}, func(name string) string { return lineCategories[name] })
	if !ok {
		return receiptsplit.Proposal{}, "receiptsplit.noProposal"
	}
	return proposal, ""
}

func expenseCategory(categories []domain.Category, name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, category := range categories {
		if category.Kind == domain.KindExpense && strings.ToLower(category.Name) == name {
			return category.ID
		}
	}
	for _, category := range categories {
		if category.Kind != domain.KindExpense {
			continue
		}
		candidate := strings.ToLower(category.Name)
		if candidate != "" && (strings.Contains(candidate, name) || strings.Contains(name, candidate)) {
			return category.ID
		}
	}
	return ""
}

// OrderText supplies the row's existing verdict, charge heading and apply decision.
func OrderText(review ordermatch.ReviewMatch, translate Translate) (verdict, chargeKey string, canApply bool) {
	m := review.Match
	switch m.Kind {
	case orderimport.MatchSingle:
		verdict = translate("orderimport.matchedSingle")
	case orderimport.MatchMulti:
		verdict = translate("orderimport.matchedMulti", len(m.TxnIDs))
	default:
		verdict = translate("orderimport.unmatched")
	}
	chargeKey = "orderimport.chargeLabel"
	if len(review.Charges) > 1 {
		chargeKey = "orderimport.chargesLabel"
	}
	return verdict, chargeKey, m.Kind != orderimport.MatchNone && len(review.MissingTxnIDs) == 0
}

// ChargePayee selects the displayed payee, description or ledger identity.
func ChargePayee(txn domain.Transaction) string {
	if txn.Payee != "" {
		return txn.Payee
	}
	if txn.Desc != "" {
		return txn.Desc
	}
	return txn.ID
}
