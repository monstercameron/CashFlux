// SPDX-License-Identifier: MIT

// Package txnlink prepares payment-link choices and save decisions without UI or
// storage dependencies. Callers own persistence and translate the returned keys.
package txnlink

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

// Input is the current dataset and the subscription detection preferences.
// MinOccurrences is the already resolved preference (the detector enforces two).
type Input struct {
	Transactions        []domain.Transaction
	Accounts            []domain.Account
	Categories          []domain.Category
	BaseCurrency        string
	FXRates             map[string]float64
	IgnoredCategoryIDs  []string
	IgnoredAccountTypes []string
	MinOccurrences      int
	Confirmed           map[string]bool
	Ignored             []domain.SubscriptionIgnore
}

// Option is a picker value and its display name, in dataset/detector order.
type Option struct {
	Value string
	Label string
}

// Model contains the resolved transaction, eligible choices and initial draft.
type Model struct {
	Transaction   domain.Transaction
	Found         bool
	AccountNames  map[string]string
	Bills         []Option
	Subscriptions []Option
	Initial       Draft
}

// Prepare resolves a payment and its choices. Existing subscription links remain
// editable even if no longer detected. Detection failures yield no suggestions,
// matching the dialog's existing handling of an incomplete exchange-rate table.
func Prepare(in Input, txnID string) Model {
	m := Model{AccountNames: make(map[string]string, len(in.Accounts))}
	for _, txn := range in.Transactions {
		if txn.ID == txnID {
			m.Transaction, m.Found = txn, true
			break
		}
	}
	for _, account := range in.Accounts {
		m.AccountNames[account.ID] = account.Name
		if !account.Archived && account.Class == domain.ClassLiability {
			m.Bills = append(m.Bills, Option{Value: account.ID, Label: account.Name})
		}
	}
	base := in.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: in.FXRates}
	filtered := detectionTransactions(in)
	detected := detect(filtered, rates, in.MinOccurrences)
	categoryNames := make(map[string]string, len(in.Categories))
	for _, category := range in.Categories {
		categoryNames[category.ID] = category.Name
	}
	detected = FilterSubscriptionKinds(detected, filtered, in.Transactions, in.Accounts, categoryNames, in.Confirmed, in.Ignored)
	names := SubscriptionNames(detected)
	existing := m.Transaction.SubscriptionName
	if existing != "" && !contains(names, existing) {
		names = append([]string{existing}, names...)
	}
	for _, name := range names {
		m.Subscriptions = append(m.Subscriptions, Option{Value: name, Label: name})
	}
	m.Initial = Draft{BillAccountID: m.Transaction.BillAccountID, SubscriptionName: existing}
	if existing == "" {
		m.Initial.SubscriptionName = BestSubscription(m.Transaction, detected, rates)
	}
	return m
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func detectionTransactions(in Input) []domain.Transaction {
	accountTypes := make(map[string]string, len(in.Accounts))
	for _, account := range in.Accounts {
		accountTypes[account.ID] = string(account.Type)
	}
	ignoredCategories := make(map[string]bool, len(in.IgnoredCategoryIDs))
	for _, id := range in.IgnoredCategoryIDs {
		ignoredCategories[id] = true
	}
	ignoredTypes := make(map[string]bool, len(in.IgnoredAccountTypes))
	for _, accountType := range in.IgnoredAccountTypes {
		ignoredTypes[accountType] = true
	}
	filtered := make([]domain.Transaction, 0, len(in.Transactions))
	for _, txn := range in.Transactions {
		if !ignoredCategories[txn.CategoryID] && !ignoredTypes[accountTypes[txn.AccountID]] {
			filtered = append(filtered, txn)
		}
	}
	return filtered
}

func detect(txns []domain.Transaction, rates currency.Rates, minCount int) []subscriptions.Subscription {
	// The shared detector still uses the panicking Money.Abs API. Check its
	// inputs before delegation so unrepresentable magnitudes cannot crash a form.
	for _, txn := range txns {
		if !txn.IsExpense() {
			continue
		}
		converted, err := rates.Convert(txn.Amount, rates.Base)
		if err != nil {
			return nil
		}
		if _, err := converted.AbsChecked(); err != nil {
			return nil
		}
	}
	detected, _ := subscriptions.Detect(txns, rates, minCount)
	return detected
}

// Draft holds both pending links; changing the visible tab never clears either.
type Draft struct {
	BillAccountID    string
	SubscriptionName string
	AutoLink         bool
}

// WithBill changes the bill choice while retaining the subscription and rule choice.
func (d Draft) WithBill(accountID string) Draft {
	d.BillAccountID = accountID
	return d
}

// WithSubscription changes the subscription without affecting the other tab.
func (d Draft) WithSubscription(name string) Draft {
	d.SubscriptionName = name
	return d
}

// ToggleAutoLink flips the pending rule choice without altering either link.
func (d Draft) ToggleAutoLink() Draft {
	d.AutoLink = !d.AutoLink
	return d
}

// Decision describes a pending write and the unchanged save/preview wording.
type Decision struct {
	Transaction     domain.Transaction
	Changed         bool
	Linked          bool
	HasBill         bool
	HasSubscription bool
	Clearing        bool
	Remember        bool
	SaveKey         string
	HintKey         string
	HintTestID      string
}

// Evaluate builds a copy of the transaction and explains whether Save changes
// anything. Remembering a chosen bill is meaningful even if its link is unchanged.
func (d Draft) Evaluate(txn domain.Transaction) Decision {
	out := Decision{Transaction: txn, SaveKey: "txnlink.save"}
	out.Transaction.BillAccountID = d.BillAccountID
	out.Transaction.SubscriptionName = d.SubscriptionName
	out.HasBill = d.BillAccountID != ""
	out.HasSubscription = d.SubscriptionName != ""
	out.Linked = out.HasBill || out.HasSubscription
	out.Remember = d.AutoLink && out.HasBill
	out.Changed = d.BillAccountID != txn.BillAccountID || d.SubscriptionName != txn.SubscriptionName || out.Remember
	out.Clearing = !out.Linked && (txn.BillAccountID != "" || txn.SubscriptionName != "")
	if out.Clearing {
		out.SaveKey = "txnlink.saveRemove"
	}
	switch {
	case !out.Changed:
		out.HintKey, out.HintTestID = "txnlink.nothingToSave", "txnlink-nothing"
	case out.Clearing:
		out.HintKey, out.HintTestID = "txnlink.clearHint", "txnlink-clearhint"
	}
	return out
}

// NoticeKey selects the success wording after the write has succeeded.
func (d Decision) NoticeKey(ruleMade bool) string {
	if ruleMade {
		return "txnlink.savedWithRule"
	}
	if !d.Linked {
		return "txnlink.cleared"
	}
	return "txnlink.saved"
}

// BillRule builds a merchant/account rule unless no valid bill is selected or an
// equivalent rule already exists. ID and order are supplied by the storage shell.
func BillRule(existing []rules.Rule, txn domain.Transaction, draft Draft, id string, order int) (rules.Rule, bool) {
	phrase := strings.TrimSpace(merchantPhrase(txn))
	if !draft.Evaluate(txn).Remember || phrase == "" {
		return rules.Rule{}, false
	}
	for _, rule := range existing {
		if rule.SetBillAccountID == draft.BillAccountID && strings.EqualFold(strings.TrimSpace(rule.Match), phrase) {
			return rules.Rule{}, false
		}
	}
	return rules.Rule{ID: id, Match: phrase, SetBillAccountID: draft.BillAccountID, Order: order}, true
}

// Description prefers the untrimmed description, then payee, then localized fallback.
func Description(txn domain.Transaction, fallback string) string {
	if txn.Desc != "" {
		return txn.Desc
	}
	if txn.Payee != "" {
		return txn.Payee
	}
	return fallback
}

func merchantPhrase(txn domain.Transaction) string {
	if txn.Payee != "" {
		return txn.Payee
	}
	return txn.Desc
}

// Merchant trims the first nonempty payee/description, then uses the localized
// fallback. A whitespace-only payee intentionally does not fall through to Desc.
func Merchant(txn domain.Transaction, fallback string) string {
	if phrase := strings.TrimSpace(merchantPhrase(txn)); phrase != "" {
		return phrase
	}
	return fallback
}

// Summary is the payment's formatted description, date/account and signed amount.
type Summary struct {
	Description string
	AccountLine string
	Amount      string
	Tone        string
}

// Summarize formats a payment using the existing preference and money formatters.
// It preserves the date's calendar location; it performs no date arithmetic.
func Summarize(txn domain.Transaction, names map[string]string, display prefs.Prefs, fallback string) Summary {
	s := Summary{Description: Description(txn, fallback), AccountLine: display.FormatDate(txn.Date),
		Amount: money.FormatAccounting(txn.Amount.Amount, currency.Decimals(txn.Amount.Currency), currency.Symbol(txn.Amount.Currency))}
	if name := names[txn.AccountID]; name != "" {
		s.AccountLine += " · " + name
	}
	switch {
	case txn.Amount.IsNegative():
		s.Tone = "text-down"
	case txn.Amount.IsPositive():
		s.Tone = "text-up"
	}
	return s
}
