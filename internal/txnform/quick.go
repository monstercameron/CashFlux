// SPDX-License-Identifier: MIT

package txnform

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/amountmath"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payees"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	"github.com/monstercameron/CashFlux/internal/smarttext"
	"github.com/monstercameron/CashFlux/internal/textutil"
)

// QuickDraft holds Quick Add's fields; Direction uses its existing Income/Expense values.
type QuickDraft struct {
	Draft
	NeedsReview, OwnersTouched bool
}

// QuickSource supplies the current household and explicit local date to a preview.
type QuickSource struct {
	TemplateCount                int
	Accounts                     []domain.Account
	Transactions                 []domain.Transaction
	Categories                   []domain.Category
	Members                      []domain.Member
	Rules                        []rules.Rule
	MemberLens                   []string
	ActiveMember, DefaultAccount string
	Now                          time.Time
	LearnTally                   learntally.Tally
	LearnThreshold               int
}

// QuickView contains all non-rendering decisions needed by the form's children.
type QuickView struct {
	AccountErrorKey, AmountErrorKey, PayeeErrorKey string
	TemplatePayee, TemplateNote                    string
	ShowMembers, ShowTemplates                     bool
	AccountID, Date, Currency                      string
	Accounts, Categories, Members                  []Option
	CategoryKind                                   domain.CategoryKind
	Owners                                         []string
	Colors                                         map[string]string
	Payees                                         []string
	AccountError, AmountError, PayeeError          bool
	ImpactMinor, TemplateMinor                     int64
	TemplateDirection                              domain.TxnDirection
	CleanDescription, DescriptionSuggestion        string
	RuleCategoryID, CategorySuggestion             string
	LearnedCategoryID, LearnedCategoryName         string
	AmountNote                                     string
}

// NewQuickDraft resets the form, retaining only the active-member default.
func NewQuickDraft(activeMember string) QuickDraft {
	return QuickDraft{Draft: Draft{Direction: "Expense", Owners: QuickOwnerDefaults(activeMember)}}
}

// QuickCategoryValid checks a category against the selected direction.
func QuickCategoryValid(categories []domain.Category, categoryID, direction string) bool {
	want := domain.KindExpense
	if direction == "Income" {
		want = domain.KindIncome
	}
	for _, c := range categories {
		if c.ID == categoryID {
			return c.Kind == want
		}
	}
	return false
}

// QuickHasAccount reports whether the account still exists, including archived accounts.
func QuickHasAccount(accounts []domain.Account, id string) bool {
	_, ok := domain.AccountByID(accounts, id)
	return ok
}

// QuickAccountID resolves the effective account using the shared account-selection policy.
func QuickAccountID(input accountselect.ResolveInput) string {
	id, _ := accountselect.Resolve(input)
	return id
}

// QuickAmountValid validates a nonzero, representable signed amount in the selected currency.
func QuickAmountValid(accounts []domain.Account, accountID, raw, kind string) bool {
	a, ok := domain.AccountByID(accounts, accountID)
	if !ok {
		return false
	}
	_, _, err := amountmath.ParseSigned(raw, currency.Decimals(a.Currency), quickDirection(kind))
	return err == nil
}

func quickDirection(kind string) amountmath.Direction {
	if kind == "Income" {
		return amountmath.In
	}
	return amountmath.Out
}

// QuickInvalidField preserves the form's account, amount, then payee validation order.
func QuickInvalidField(accounts []domain.Account, accountID, raw, kind, payee, desc string) (string, string) {
	if !QuickHasAccount(accounts, accountID) {
		return "txn-add-account", "txu3.needAccount"
	}
	if !QuickAmountValid(accounts, accountID, raw, kind) {
		return "txn-add-amount", "txu3.needAmount"
	}
	if strings.TrimSpace(payee) == "" && strings.TrimSpace(desc) == "" {
		return "txn-add-payee", "r4f.addPayeeOrDescription"
	}
	return "", ""
}

// QuickDescription falls back to the trimmed payee when the bank description is blank.
func QuickDescription(payee, description string) string {
	if s := strings.TrimSpace(description); s != "" {
		return s
	}
	return strings.TrimSpace(payee)
}

// QuickNextFocus selects the first unfilled field after applying a sentence.
func QuickNextFocus(accounts []domain.Account, accountID, amount, payee, desc string, fill smartcatview.DraftFill) string {
	if strings.TrimSpace(amount) == "" && fill.AmountMajor == "" {
		return "txn-add-amount"
	}
	if !QuickHasAccount(accounts, accountID) {
		return "txn-add-account"
	}
	if strings.TrimSpace(payee) == "" && strings.TrimSpace(desc) == "" && fill.Payee == "" {
		return "txn-add-payee"
	}
	return "txn-add-save"
}

// QuickMemberValue preserves explicit unassignment as well as explicit member selection.
func QuickMemberValue(activeMember, selected string, explicit bool) string {
	if explicit {
		return selected
	}
	return activeMember
}

// QuickOwnerDefaults supplies the active member as the initial selection.
func QuickOwnerDefaults(activeMember string) []string {
	if activeMember == "" {
		return nil
	}
	return []string{activeMember}
}

// QuickOwners copies the selection and falls back to individual account ownership.
func QuickOwners(owners []string, active string, account domain.Account, touched bool) []string {
	if touched || len(owners) > 0 {
		return append([]string(nil), owners...)
	}
	if active != "" {
		return []string{active}
	}
	if account.Scope == domain.ScopeIndividual && account.OwnerID != "" {
		return []string{account.OwnerID}
	}
	return nil
}

// QuickReviewed converts the Needs review checkbox into the stored reviewed flag.
func QuickReviewed(needsReview bool) bool { return !needsReview }

// QuickSetDirection clears a category only when it is incompatible with the new direction.
func QuickSetDirection(d QuickDraft, next string, categories []domain.Category) QuickDraft {
	if d.CategoryID != "" && !QuickCategoryValid(categories, d.CategoryID, next) {
		d.CategoryID = ""
	}
	d.Direction = next
	return d
}

// QuickChooseDirection separates the transfer handoff from ordinary direction changes.
func QuickChooseDirection(d QuickDraft, next string, categories []domain.Category) (QuickDraft, bool) {
	if next == "Transfer" {
		return d, true
	}
	return QuickSetDirection(d, next, categories), false
}

// QuickToggleOwner marks even an empty resulting selection as explicitly chosen.
func QuickToggleOwner(d QuickDraft, owner string) QuickDraft {
	d.Owners = ToggleOwner(d.Owners, owner)
	d.OwnersTouched = true
	return d
}

// QuickInputAmount makes an explicit minus immediately select Expense.
func QuickInputAmount(d QuickDraft, raw string, categories []domain.Category) QuickDraft {
	d.Amount = raw
	if strings.HasPrefix(strings.TrimSpace(raw), "-") && d.Direction != "Expense" {
		d = QuickSetDirection(d, "Expense", categories)
	}
	return d
}

// QuickBlurAmount evaluates the existing arithmetic syntax and removes a leading minus.
// Evaluation is delegated to amountmath; monetary parsing remains integer-only.
func QuickBlurAmount(d QuickDraft, raw string) QuickDraft {
	if v, ok := amountmath.EvalAmount(raw); ok {
		raw = strconv.FormatFloat(v, 'f', 2, 64)
		d.Amount = raw
	}
	if s := strings.TrimSpace(raw); strings.HasPrefix(s, "-") {
		d.Amount = strings.TrimSpace(strings.TrimPrefix(s, "-"))
	}
	return d
}

// QuickApplyTemplate overlays only the fields historically supplied by a quick template.
func QuickApplyTemplate(d QuickDraft, t domain.TxnTemplate, categories []domain.Category) QuickDraft {
	if t.AccountID != "" {
		d.AccountID = t.AccountID
	}
	direction := "Income"
	if t.IsExpense() {
		direction = "Expense"
	}
	d = QuickSetDirection(d, direction, categories)
	if t.AmountMinor != 0 {
		d.Amount = money.FormatMinor(editMagnitude(t.AmountMinor), currency.Decimals(t.Currency))
	}
	d.Payee = t.Payee
	if t.Note != "" {
		d.Desc = t.Note
	}
	d.CategoryID = ""
	if QuickCategoryValid(categories, t.CategoryID, d.Direction) {
		d.CategoryID = t.CategoryID
	}
	return d
}

// QuickApplyFill overlays only fields found in the sentence, preserving partially typed input.
func QuickApplyFill(d QuickDraft, f smartcatview.DraftFill, categories []domain.Category) QuickDraft {
	if f.AmountMajor != "" {
		d.Amount = f.AmountMajor
	}
	if f.Payee != "" {
		d.Payee = f.Payee
		if strings.TrimSpace(d.Desc) == "" {
			d.Desc = f.Payee
		}
	}
	if !f.Date.IsZero() {
		d.Date = dateutil.FormatDate(f.Date)
	}
	if f.Income {
		d = QuickSetDirection(d, "Income", categories)
	} else if f.AmountMajor != "" {
		d = QuickSetDirection(d, "Expense", categories)
	}
	if f.CategoryID != "" && QuickCategoryValid(categories, f.CategoryID, d.Direction) {
		d.CategoryID = f.CategoryID
	}
	return d
}

// QuickPreview builds options, validation, suggestions and integer monetary previews.
func QuickPreview(d QuickDraft, s QuickSource, attempted bool) QuickView {
	member := ""
	if len(s.MemberLens) == 1 {
		member = s.MemberLens[0]
	}
	v := QuickView{AccountID: QuickAccountID(accountselect.ResolveInput{Accounts: s.Accounts, Transactions: s.Transactions,
		ExplicitID: d.AccountID, ActiveMemberID: member, Members: s.Members, WorkspaceDefaultID: s.DefaultAccount})}
	v.Date = d.Date
	if v.Date == "" {
		v.Date = dateutil.FormatDate(s.Now)
	}
	v.Accounts = accountOptions(acctsort.ForPicker(s.Accounts), v.AccountID)
	direction := "out"
	if d.Direction == "Income" {
		direction = "in"
	}
	v.Categories, v.CategoryKind = Categories(s.Categories, direction)
	v.Members, v.Colors = Members(s.Members)
	a, _ := domain.AccountByID(s.Accounts, v.AccountID)
	v.Owners = QuickOwners(d.Owners, s.ActiveMember, a, d.OwnersTouched)
	v.Currency = a.Currency
	v.ImpactMinor, _ = money.ParseMinor(strings.TrimSpace(d.Amount), currency.Decimals(a.Currency))
	if !QuickHasAccount(s.Accounts, v.AccountID) {
		v.ImpactMinor = 0
	}
	v.TemplateMinor = v.ImpactMinor
	v.TemplatePayee = strings.TrimSpace(d.Payee)
	v.TemplateNote = strings.TrimSpace(d.Desc)
	v.TemplateDirection = domain.DirectionExpense
	if d.Direction == "Income" {
		v.TemplateDirection = domain.DirectionIncome
	}
	v.AccountError = attempted && !QuickHasAccount(s.Accounts, v.AccountID)
	v.AmountError = attempted && !QuickAmountValid(s.Accounts, v.AccountID, d.Amount, d.Direction)
	v.PayeeError = attempted && strings.TrimSpace(d.Payee) == "" && strings.TrimSpace(d.Desc) == ""
	if v.AccountError {
		v.AccountErrorKey = "txu3.needAccount"
	}
	if v.AmountError {
		v.AmountErrorKey = "txu3.needAmount"
	}
	if v.PayeeError {
		v.PayeeErrorKey = "r4f.addPayeeOrDescription"
	}
	v.Payees = payees.RecentPayees(s.Transactions, 50)
	v.ShowMembers = len(s.Members) > 0
	v.ShowTemplates = len(s.Transactions) > 0 || s.TemplateCount > 0
	v.CleanDescription = smarttext.CleanMerchant(d.Desc)
	if v.CleanDescription != strings.TrimSpace(d.Desc) && v.CleanDescription != "" {
		v.DescriptionSuggestion = v.CleanDescription
	}
	quickSuggestions(&v, d, s)
	if strings.HasPrefix(strings.TrimSpace(d.Amount), "-") && v.ImpactMinor != 0 {
		v.AmountNote = money.FormatMinor(editMagnitude(v.ImpactMinor), currency.Decimals(a.Currency))
	}
	return v
}

func quickSuggestions(v *QuickView, d QuickDraft, s QuickSource) {
	v.RuleCategoryID = rules.Category(s.Rules, "", strings.TrimSpace(d.Desc))
	if v.RuleCategoryID != "" && d.CategoryID == "" && QuickCategoryValid(s.Categories, v.RuleCategoryID, d.Direction) {
		v.CategorySuggestion, _ = SuggestionName(s.Categories, v.RuleCategoryID, "", true)
	}
	if v.RuleCategoryID == "" && d.CategoryID == "" {
		key := RulePhrase(domain.Transaction{Payee: d.Payee, Desc: d.Desc})
		if id, ok := s.LearnTally.ShouldSuggest(key, s.LearnThreshold); ok && QuickCategoryValid(s.Categories, id, d.Direction) {
			v.LearnedCategoryID = id
			v.LearnedCategoryName, _ = SuggestionName(s.Categories, id, "", true)
		}
	}
}

// BuildQuick constructs a manual transaction without persistence or automatic categorization.
// Invalid dates retain the historical fallback to the caller's explicit local now.
func BuildQuick(d QuickDraft, accounts []domain.Account, accountID, activeMember, id string, now time.Time) (domain.Transaction, error) {
	if field, key := QuickInvalidField(accounts, accountID, d.Amount, d.Direction, d.Payee, d.Desc); field != "" {
		return domain.Transaction{}, fmt.Errorf("quick add %s: %s", field, key)
	}
	a, _ := domain.AccountByID(accounts, accountID)
	// QuickInvalidField has already parsed this immutable draft against this account.
	amount, _, _ := amountmath.ParseSigned(d.Amount, currency.Decimals(a.Currency), quickDirection(d.Direction))
	date, err := dateutil.ParseDate(strings.TrimSpace(d.Date))
	if err != nil {
		date = now
	}
	return domain.Transaction{ID: id, AccountID: a.ID, Date: date, Payee: strings.TrimSpace(d.Payee),
		Desc: QuickDescription(d.Payee, d.Desc), CategoryID: d.CategoryID, Amount: money.New(amount, a.Currency),
		MemberIDs: QuickOwners(d.Owners, activeMember, a, d.OwnersTouched), Reviewed: QuickReviewed(d.NeedsReview),
		Source: domain.TxnSourceManual, Tags: textutil.CommaFields(d.Tags), Note: strings.TrimSpace(d.Note),
		Cleared: d.Cleared, ExcludeFromReports: d.Exclude}, nil
}
