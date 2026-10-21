// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
)

type assistantAddTransactionArgs struct {
	AccountID   string          `json:"account_id"`
	Account     string          `json:"account"`
	CategoryID  string          `json:"category_id"`
	Category    string          `json:"category"`
	Amount      json.RawMessage `json:"amount"`
	Currency    string          `json:"currency"`
	Payee       string          `json:"payee"`
	Description string          `json:"description"`
	Date        string          `json:"date"`
}
type assistantCreateCategoryArgs struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}
type assistantCategorizeArgs struct {
	TransactionIDs    []string `json:"transaction_ids"`
	Match             string   `json:"match"`
	CategoryID        string   `json:"category_id"`
	Category          string   `json:"category"`
	OnlyUncategorized *bool    `json:"only_uncategorized"`
}
type assistantCreationPlan struct {
	account          domain.Account
	category         *domain.Category
	draft            domain.Transaction
	flow             money.Money
	duplicate        *domain.Transaction
	categoryDraft    domain.Category
	existingCategory *domain.Category
	rows             []domain.Transaction
	excluded         []string
	categorize       assistantCategorizeArgs
	preview          string
	err              error
	used             bool
	result           string
}

func assistantCreationDecode[T any](raw json.RawMessage) (T, error) {
	var a T
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, fmt.Errorf("couldn't read the details: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return a, fmt.Errorf("provide one argument object")
	}
	return a, nil
}
func assistantCreationAccount(all []domain.Account, accountID, name string) (domain.Account, error) {
	accountID, name = strings.TrimSpace(accountID), strings.TrimSpace(name)
	var matches []domain.Account
	for _, a := range all {
		if accountID != "" && a.ID == accountID || accountID == "" && name != "" && strings.EqualFold(strings.TrimSpace(a.Name), name) {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		return domain.Account{}, fmt.Errorf("provide an account_id or one unambiguous exact account name; %d matches", len(matches))
	}
	if matches[0].Archived {
		return domain.Account{}, fmt.Errorf("account ID %s is archived", matches[0].ID)
	}
	return matches[0], nil
}
func assistantCreationCategory(all []domain.Category, categoryID, name string) (domain.Category, error) {
	categoryID, name = strings.TrimSpace(categoryID), strings.TrimSpace(name)
	var matches []domain.Category
	for _, c := range all {
		if categoryID != "" && c.ID == categoryID || categoryID == "" && name != "" && catname.EqualNames(c.Name, name) {
			matches = append(matches, c)
		}
	}
	if len(matches) != 1 {
		return domain.Category{}, fmt.Errorf("provide a category_id or one unambiguous exact category name; %d matches", len(matches))
	}
	return matches[0], nil
}
func assistantCreationPlain(t domain.Transaction) bool {
	return !t.IsTransfer() && t.TransferGroupID == ""
}
func assistantCreationDuplicate(rows []domain.Transaction, draft domain.Transaction) *domain.Transaction {
	for _, row := range rows {
		if assistantCreationPlain(row) && dedupe.Key(row) == dedupe.Key(draft) {
			copy := row
			return &copy
		}
	}
	return nil
}
func prepareAssistantTransaction(app *appstate.App, raw json.RawMessage) *assistantCreationPlan {
	p := &assistantCreationPlan{}
	a, err := assistantCreationDecode[assistantAddTransactionArgs](raw)
	if err != nil {
		p.err = err
		return p
	}
	accounts, err := app.Store().ListAccounts()
	if err != nil {
		p.err = err
		return p
	}
	p.account, err = assistantCreationAccount(accounts, a.AccountID, a.Account)
	if err != nil {
		p.err = err
		return p
	}
	code := p.account.Currency
	if !currency.Valid(code) {
		p.err = fmt.Errorf("account currency %q is unsupported", code)
		return p
	}
	if a.Currency != "" && !strings.EqualFold(strings.TrimSpace(a.Currency), code) {
		p.err = fmt.Errorf("amount currency must be %s", code)
		return p
	}
	decimal := string(a.Amount)
	if len(a.Amount) > 0 && a.Amount[0] == '"' {
		if err := json.Unmarshal(a.Amount, &decimal); err != nil {
			p.err = err
			return p
		}
	}
	minor, err := money.ParseMinor(decimal, currency.Decimals(code))
	if err != nil {
		p.err = fmt.Errorf("invalid amount in %s: %w", code, err)
		return p
	}
	if minor == 0 {
		p.err = fmt.Errorf("amount must be nonzero")
		return p
	}
	p.flow = money.New(minor, code)
	booked, err := appstate.AssistantTransactionAmount(p.account, p.flow)
	if err != nil {
		p.err = err
		return p
	}
	when := dateutil.CalendarDayUTC(time.Now())
	if strings.TrimSpace(a.Date) != "" {
		when, err = time.Parse("2006-01-02", strings.TrimSpace(a.Date))
		if err != nil {
			p.err = fmt.Errorf("date must be a valid YYYY-MM-DD calendar date: %w", err)
			return p
		}
	}
	desc, payee := strings.TrimSpace(a.Description), strings.TrimSpace(a.Payee)
	if desc == "" {
		desc = payee
	}
	if desc == "" {
		desc = "Income"
		if minor < 0 {
			desc = "Expense"
		}
	}
	p.draft = domain.Transaction{ID: id.New(), AccountID: p.account.ID, Date: when, Payee: payee, Desc: desc, Amount: booked, Source: domain.TxnSourceAssistant}
	categoryLabel := "uncategorized (category_id empty)"
	if strings.TrimSpace(a.CategoryID) != "" || strings.TrimSpace(a.Category) != "" {
		all, err := app.CategoriesChecked()
		if err != nil {
			p.err = err
			return p
		}
		c, err := assistantCreationCategory(all, a.CategoryID, a.Category)
		if err != nil {
			p.err = err
			return p
		}
		p.category = &c
		p.draft.CategoryID = c.ID
		categoryLabel = fmt.Sprintf("%q (ID %s, kind %s)", c.Name, c.ID, c.Kind)
	}
	if issues := validate.ValidateTransaction(p.draft, p.account); !issues.OK() {
		p.err = issues
		return p
	}
	rows, err := app.Store().ListTransactions()
	if err != nil {
		p.err = err
		return p
	}
	p.duplicate = assistantCreationDuplicate(rows, p.draft)
	direction := "income/payment"
	if minor < 0 {
		direction = "expense/charge"
	}
	p.preview = fmt.Sprintf("Record transaction ID %s: public flow %s (%s); booked amount %s. Account %q (ID %s, liability sign %s). Date %s; payee %q; description %q; category %s; source assistant. Plain transaction, cleared=false, reviewed=false, excluded_from_reports=false; no split, transfer or explicit owner assignment.", p.draft.ID, assistantContributionMoney(p.flow), direction, assistantContributionMoney(booked), p.account.Name, p.account.ID, p.account.WithLiabilitySign().LiabilitySign, when.Format("2006-01-02"), payee, desc, categoryLabel)
	if p.duplicate != nil {
		p.preview += fmt.Sprintf(" Canonical duplicate ID %s already exists; this approval will create no transaction.", p.duplicate.ID)
	}
	return p
}
func prepareAssistantCategory(app *appstate.App, raw json.RawMessage) *assistantCreationPlan {
	p := &assistantCreationPlan{}
	a, err := assistantCreationDecode[assistantCreateCategoryArgs](raw)
	if err != nil {
		p.err = err
		return p
	}
	name := strings.TrimSpace(a.Name)
	kind := domain.KindExpense
	if a.Kind != "" {
		kind = domain.CategoryKind(strings.ToLower(strings.TrimSpace(a.Kind)))
	}
	p.categoryDraft = domain.Category{ID: id.New(), Name: name, Kind: kind}
	if issues := validate.ValidateCategory(p.categoryDraft); !issues.OK() {
		p.err = issues
		return p
	}
	all, err := app.CategoriesChecked()
	if err != nil {
		p.err = err
		return p
	}
	for _, c := range all {
		if c.ParentID == "" && catname.EqualNames(c.Name, name) {
			if p.existingCategory != nil {
				p.err = fmt.Errorf("several top-level categories already use this name; use existing category IDs")
				return p
			}
			copy := c
			p.existingCategory = &copy
		}
	}
	if p.existingCategory != nil {
		p.preview = fmt.Sprintf("Existing category %q (ID %s, kind %s, top level) already has that name. Create nothing; preserve its current kind and settings.", p.existingCategory.Name, p.existingCategory.ID, p.existingCategory.Kind)
	} else {
		p.preview = fmt.Sprintf("Create top-level category %q (ID %s), kind %s; parent_id empty, color empty, deductible=false, no custom classification or tax line.", name, p.categoryDraft.ID, kind)
	}
	return p
}
func assistantCategorizationRows(rows []domain.Transaction, a assistantCategorizeArgs, categoryID string) ([]domain.Transaction, []string, error) {
	ids := map[string]bool{}
	q := strings.ToLower(strings.TrimSpace(a.Match))
	if len(a.TransactionIDs) > 100 {
		return nil, nil, fmt.Errorf("select at most100 transaction IDs per approval")
	}
	for _, id := range a.TransactionIDs {
		id = strings.TrimSpace(id)
		if id == "" || ids[id] {
			return nil, nil, fmt.Errorf("transaction_ids must contain distinct nonblank IDs")
		}
		ids[id] = true
	}
	if len(ids) == 0 && q == "" || len(ids) > 0 && q != "" {
		return nil, nil, fmt.Errorf("provide transaction_ids OR a nonblank match phrase")
	}
	only := a.OnlyUncategorized == nil || *a.OnlyUncategorized
	var selected []domain.Transaction
	var excluded []string
	seen := map[string]bool{}
	for _, row := range rows {
		if len(ids) > 0 {
			if !ids[row.ID] {
				continue
			}
			seen[row.ID] = true
		} else if !strings.Contains(strings.ToLower(row.Payee+" "+row.Desc), q) {
			continue
		}
		if !assistantCreationPlain(row) {
			excluded = append(excluded, row.ID)
			continue
		}
		if row.CategoryID == categoryID || only && row.CategoryID != "" {
			continue
		}
		selected = append(selected, row)
	}
	for id := range ids {
		if !seen[id] {
			return nil, nil, fmt.Errorf("transaction ID %s no longer exists", id)
		}
	}
	if len(selected) > 100 || len(excluded) > 100 {
		return nil, nil, fmt.Errorf("more than100 transactions match; narrow the phrase or use explicit transaction_ids so the full preview fits")
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	sort.Strings(excluded)
	return selected, excluded, nil
}
func prepareAssistantCategorization(app *appstate.App, raw json.RawMessage) *assistantCreationPlan {
	p := &assistantCreationPlan{}
	a, err := assistantCreationDecode[assistantCategorizeArgs](raw)
	if err != nil {
		p.err = err
		return p
	}
	p.categorize = a
	all, err := app.CategoriesChecked()
	if err != nil {
		p.err = err
		return p
	}
	c, err := assistantCreationCategory(all, a.CategoryID, a.Category)
	if err != nil {
		p.err = err
		return p
	}
	p.category = &c
	rows, err := app.Store().ListTransactions()
	if err != nil {
		p.err = err
		return p
	}
	p.rows, p.excluded, p.err = assistantCategorizationRows(rows, a, c.ID)
	if p.err != nil {
		return p
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Set exactly %d transaction(s) to category %q (ID %s, kind %s); only_uncategorized=%t. Affected rows:\n", len(p.rows), c.Name, c.ID, c.Kind, a.OnlyUncategorized == nil || *a.OnlyUncategorized)
	for _, row := range p.rows {
		fmt.Fprintf(&b, "- ID %s; account ID %s; %s; booked %s; payee %q; description %q; category ID %q -> %q.\n", row.ID, row.AccountID, row.Date.Format("2006-01-02"), assistantContributionMoney(row.Amount), row.Payee, row.Desc, row.CategoryID, c.ID)
	}
	fmt.Fprintf(&b, "Excluded transfer IDs (%d): %s. Change only CategoryID; preserve amounts, dates, splits, owners, notes and other metadata.", len(p.excluded), strings.Join(p.excluded, ", "))
	p.preview = b.String()
	if len(p.preview) > 24000 {
		p.err = fmt.Errorf("preview exceeds24000 bytes; select fewer transaction IDs")
	}
	return p
}

func executeAssistantCreation(app *appstate.App, name string, p *assistantCreationPlan) string {
	switch name {
	case "create_category":
		if p.existingCategory != nil {
			current, found, err := app.Store().GetCategory(p.existingCategory.ID)
			if err != nil {
				return "Couldn't read the existing category: " + err.Error()
			}
			if !found || !reflect.DeepEqual(current, *p.existingCategory) {
				return "The existing category changed after preview; review and approve again."
			}
			return fmt.Sprintf("Category %q (ID %s) already exists; no category created.", current.Name, current.ID)
		}
		c, created, err := app.CreateCategoryExpected(p.categoryDraft)
		if err != nil {
			return "Couldn't create the category: " + err.Error()
		}
		if !created {
			return fmt.Sprintf("Category %q (ID %s) already exists; no category created.", c.Name, c.ID)
		}
		return fmt.Sprintf("Created category %q (ID %s, kind %s).", c.Name, c.ID, c.Kind)
	case "add_transaction":
		current, found, err := app.Store().GetAccount(p.account.ID)
		if err != nil {
			return "Couldn't read the account: " + err.Error()
		}
		if !found || !reflect.DeepEqual(current, p.account) {
			return "The account changed after preview; review and approve again."
		}
		if p.category != nil {
			current, found, err := app.Store().GetCategory(p.category.ID)
			if err != nil {
				return "Couldn't read the category: " + err.Error()
			}
			if !found || !reflect.DeepEqual(current, *p.category) {
				return "The category changed after preview; review and approve again."
			}
		}
		if p.duplicate != nil {
			current, found, err := app.Store().GetTransaction(p.duplicate.ID)
			if err != nil {
				return "Couldn't read the duplicate: " + err.Error()
			}
			if !found || !reflect.DeepEqual(current, *p.duplicate) {
				return "The duplicate changed after preview; review and approve again."
			}
			return fmt.Sprintf("Canonical duplicate ID %s already exists; no transaction created.", current.ID)
		}
		row, created, err := app.CreateTransactionExpected(p.draft, p.account, p.category)
		if err != nil {
			return "Couldn't record the transaction: " + err.Error()
		}
		if !created {
			return fmt.Sprintf("Canonical duplicate ID %s already exists; no transaction created.", row.ID)
		}
		return fmt.Sprintf("Recorded transaction ID %s: public flow %s, booked %s in account ID %s on %s.", row.ID, assistantContributionMoney(p.flow), assistantContributionMoney(row.Amount), row.AccountID, row.Date.Format("2006-01-02"))
	case "categorize_transactions":
		current, found, err := app.Store().GetCategory(p.category.ID)
		if err != nil {
			return "Couldn't read the category: " + err.Error()
		}
		if !found || !reflect.DeepEqual(current, *p.category) {
			return "The category changed after preview; review and approve again."
		}
		rows, err := app.Store().ListTransactions()
		if err != nil {
			return "Couldn't read the transactions: " + err.Error()
		}
		fresh, excluded, err := assistantCategorizationRows(rows, p.categorize, p.category.ID)
		if err != nil || !reflect.DeepEqual(fresh, p.rows) || !reflect.DeepEqual(excluded, p.excluded) {
			return "Matching transactions changed after preview; review the current IDs and approve again."
		}
		if len(p.rows) == 0 {
			return "No transactions require a category change; nothing recorded."
		}
		if err := app.CategorizeTransactionsExpected(p.rows, *p.category); err != nil {
			return "Couldn't categorize the transactions: " + err.Error()
		}
		return fmt.Sprintf("Categorized exactly %d transaction(s) as %q (category ID %s).", len(p.rows), p.category.Name, p.category.ID)
	}
	return "Unsupported creation tool."
}

// agToolsCreation replaces the three creation/categorization tools with exact
// amounts, full approval previews and checked expected-before appstate writes.
func agToolsCreation(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	plans := map[string]*assistantCreationPlan{}
	makeTool := func(name, description, schema string, prepare func(*appstate.App, json.RawMessage) *assistantCreationPlan) chatTool {
		return chatTool{spec: ai.FunctionTool(name, description, json.RawMessage(schema)), mutates: true, preview: func(raw json.RawMessage) string {
			p := prepare(app, raw)
			plans[name+":"+string(raw)] = p
			if p.err != nil {
				return "Can't prepare this change: " + p.err.Error()
			}
			return p.preview
		}, run: func(raw json.RawMessage) string {
			p, ok := plans[name+":"+string(raw)]
			if !ok {
				return "Review and approve a preview before making this change."
			}
			if p.err != nil {
				return "Can't make this change: " + p.err.Error()
			}
			if p.used {
				return "This approval was already processed. " + p.result
			}
			p.used = true
			p.result = executeAssistantCreation(app, name, p)
			return p.result
		}}
	}
	return []chatTool{
		makeTool("add_transaction", "Record one ordinary transaction after approval. Prefer exact account_id/category_id or unambiguous EXACT names; substring names are refused. amount is an exact decimal string (numeric compatibility accepted without float parsing) in account currency. Negative is public expense/charge, positive income/payment; booking honors the account's fixed liability sign, shown in preview. Optional date is strict YYYY-MM-DD and defaults to the preview's day. Optional category omission stays uncategorized; description derives from payee or Income/Expense. Existing canonical duplicates are no-ops. Use add_transfer for moves between accounts.", `{"type":"object","properties":{"amount":{"type":["string","number"]},"account_id":{"type":"string"},"account":{"type":"string"},"category_id":{"type":"string"},"category":{"type":"string"},"currency":{"type":"string"},"payee":{"type":"string"},"description":{"type":"string"},"date":{"type":"string"}},"required":["amount"],"anyOf":[{"required":["account_id"]},{"required":["account"]}],"additionalProperties":false}`, prepareAssistantTransaction),
		makeTool("create_category", "Create one top-level expense or income category after approval; kind defaults to expense. Existing canonical sibling names are explicit no-ops preserving the existing category's kind/settings. Invalid kinds and blank names are refused.", `{"type":"object","properties":{"name":{"type":"string"},"kind":{"type":"string","enum":["expense","income"]}},"required":["name"],"additionalProperties":false}`, prepareAssistantCategory),
		makeTool("categorize_transactions", "Atomically set category on the exact approved rows. Provide transaction_ids OR a nonblank case-insensitive payee/description match phrase. Prefer category_id or one unambiguous EXACT category name. only_uncategorized defaults true; false also recategorizes. Transfers are excluded and named in preview. At most100 affected/excluded rows per full preview; narrow broader matches. Any changed matching row/set/category refuses the entire stale approval.", `{"type":"object","properties":{"transaction_ids":{"type":"array","items":{"type":"string"},"maxItems":100},"match":{"type":"string"},"category_id":{"type":"string"},"category":{"type":"string"},"only_uncategorized":{"type":"boolean"}},"anyOf":[{"required":["transaction_ids"]},{"required":["match"]}],"additionalProperties":false}`, prepareAssistantCategorization),
	}
}
