// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: registered by agToolsReports (ag_reports.go) via agToolsLedgerEdit

package screens

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// The trace tools end at a list of ids; these two are what makes that list
// worth having. find_transactions is the free-form search behind every report
// row the catalog does not model, and update_transaction is the only tool that
// edits ONE named transaction rather than everything matching a phrase.
//
// That distinction is the point. The existing write tools address rows by a
// payee substring, which is right for "categorize everything from Trader
// Joe's" and wrong for "this one deposit is actually a transfer" — the second
// has always required the user to go and do it by hand. Tracing a figure to its
// rows and then not being able to touch one of them is half a feature.

// agResolveTxn finds the single transaction an id (or an unambiguous id prefix)
// names. It refuses on ambiguity rather than editing the first match: the whole
// reason this tool takes an id is that the caller means one specific row.
func agResolveTxn(txns []domain.Transaction, ref string) (domain.Transaction, error) {
	q := strings.ToLower(strings.TrimSpace(ref))
	if q == "" {
		return domain.Transaction{}, fmt.Errorf("give the transaction id (the first column of a trace or search result)")
	}
	for _, t := range txns {
		if t.ID == strings.TrimSpace(ref) {
			return t, nil
		}
	}
	var hits []domain.Transaction
	for _, t := range txns {
		if strings.HasPrefix(strings.ToLower(t.ID), q) {
			hits = append(hits, t)
		}
	}
	switch len(hits) {
	case 0:
		return domain.Transaction{}, fmt.Errorf("no transaction with id %q — trace or search again to get a current id", ref)
	case 1:
		return hits[0], nil
	}
	return domain.Transaction{}, fmt.Errorf("%q matches %d transactions; use the full id", ref, len(hits))
}

// agTxnFilter is the free-form search find_transactions runs.
type agTxnFilter struct {
	Contains  string  `json:"contains"`
	Category  string  `json:"category"`
	Account   string  `json:"account"`
	Member    string  `json:"member"`
	Tag       string  `json:"tag"`
	Kind      string  `json:"kind"` // expense | income | transfer | any
	Period    string  `json:"period"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	MinAmount float64 `json:"min_amount"`
	MaxAmount float64 `json:"max_amount"`
	Uncat     bool    `json:"uncategorized"`
	Sort      string  `json:"sort"` // newest | oldest | largest
	Limit     int     `json:"limit"`
}

// agRunTxnFilter applies the filter and returns the matches in the requested
// order. Unlike the report traces it does NOT apply CountsInReports: a search
// for a row to fix must be able to reach the rows the reports deliberately
// leave out, which are often exactly the ones that are wrong.
func agRunTxnFilter(c agRptCtx, f agTxnFilter) ([]domain.Transaction, string) {
	var (
		catID              string
		catSet, accSet     bool
		accID, memID       string
		memSet             bool
		criteria           []string
		contains           = strings.ToLower(strings.TrimSpace(f.Contains))
		tag                = strings.ToLower(strings.TrimSpace(f.Tag))
		kind               = strings.ToLower(strings.TrimSpace(f.Kind))
		minMinor, maxMinor int64
	)
	if f.Category != "" {
		if id, ok := c.resolveCatID(f.Category); ok {
			catID, catSet = id, true
			criteria = append(criteria, "category "+c.nameOf(id))
		} else {
			criteria = append(criteria, fmt.Sprintf("category %q (no match — ignored)", f.Category))
		}
	}
	if f.Uncat {
		catID, catSet = "", true
		criteria = append(criteria, "uncategorized")
	}
	if q := strings.ToLower(strings.TrimSpace(f.Account)); q != "" {
		for _, a := range c.accounts {
			if strings.ToLower(a.Name) == q || strings.Contains(strings.ToLower(a.Name), q) {
				accID, accSet = a.ID, true
				criteria = append(criteria, "account "+a.Name)
				break
			}
		}
	}
	if q := strings.ToLower(strings.TrimSpace(f.Member)); q != "" {
		for _, m := range c.members {
			if strings.ToLower(m.Name) == q || strings.Contains(strings.ToLower(m.Name), q) {
				memID, memSet = m.ID, true
				criteria = append(criteria, "person "+m.Name)
				break
			}
		}
	}
	if contains != "" {
		criteria = append(criteria, fmt.Sprintf("text %q", strings.TrimSpace(f.Contains)))
	}
	if tag != "" {
		criteria = append(criteria, "tag "+tag)
	}
	if kind != "" && kind != "any" {
		criteria = append(criteria, kind+"s only")
	}
	if f.MinAmount > 0 {
		minMinor = currency.MinorFromMajor(f.MinAmount, c.base)
		criteria = append(criteria, "at least "+c.fmtM(minMinor))
	}
	if f.MaxAmount > 0 {
		maxMinor = currency.MinorFromMajor(f.MaxAmount, c.base)
		criteria = append(criteria, "at most "+c.fmtM(maxMinor))
	}

	var out []domain.Transaction
	for _, t := range c.txns {
		if !c.inWindow(t) {
			continue
		}
		if catSet && t.CategoryID != catID {
			continue
		}
		if accSet && t.AccountID != accID && t.TransferAccountID != accID {
			continue
		}
		if memSet && !t.HasOwner(memID) {
			continue
		}
		if contains != "" && !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), contains) {
			continue
		}
		if tag != "" && !agHasTag(t, tag) {
			continue
		}
		switch kind {
		case "expense":
			if !t.IsExpense() {
				continue
			}
		case "income":
			if !t.IsIncome() {
				continue
			}
		case "transfer":
			if !t.IsTransfer() {
				continue
			}
		}
		mag := t.Amount.Abs()
		if conv, err := c.rates.Convert(mag, c.base); err == nil {
			mag = conv
		}
		if minMinor > 0 && mag.Amount < minMinor {
			continue
		}
		if maxMinor > 0 && mag.Amount > maxMinor {
			continue
		}
		out = append(out, t)
	}

	switch strings.ToLower(strings.TrimSpace(f.Sort)) {
	case "oldest":
		sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	case "largest":
		sort.Slice(out, func(i, j int) bool {
			return agAbs(out[i].Amount.Amount) > agAbs(out[j].Amount.Amount)
		})
	default:
		sort.Slice(out, func(i, j int) bool {
			if !out[i].Date.Equal(out[j].Date) {
				return out[i].Date.After(out[j].Date)
			}
			return out[i].ID < out[j].ID
		})
	}
	what := "your search"
	if len(criteria) > 0 {
		what = strings.Join(criteria, " + ")
	}
	return out, what
}

func agHasTag(t domain.Transaction, tag string) bool {
	for _, got := range t.Tags {
		if strings.ToLower(strings.TrimSpace(got)) == tag {
			return true
		}
	}
	return false
}

// agTxnEdit is the set of changes update_transaction can make. Every field is a
// pointer or a sentinel-free string so "not mentioned" and "set to empty" stay
// distinguishable — an edit that silently blanked the payee because the model
// did not repeat it would be worse than no edit at all.
type agTxnEdit struct {
	ID          string          `json:"id"`
	Category    *string         `json:"category"`
	Payee       *string         `json:"payee"`
	Description *string         `json:"description"`
	Date        *string         `json:"date"`
	Amount      json.RawMessage `json:"amount"`
	CategoryID  *string         `json:"category_id"`
	MemberID    *string         `json:"member_id"`
	Direction   *string         `json:"direction"` // expense | income
	Member      *string         `json:"member"`
	Tags        *[]string       `json:"tags"`
	Cleared     *bool           `json:"cleared"`
}

// agApplyTxnEdit resolves an edit against the current row and returns the
// updated transaction plus a plain-English list of what changed. It resolves
// names to ids here, so both the preview and the write describe the same thing.
func agApplyTxnEdit(c agRptCtx, prior domain.Transaction, e agTxnEdit) (domain.Transaction, []string, error) {
	next := prior
	var changes []string

	if e.Category != nil && e.CategoryID != nil {
		return prior, nil, fmt.Errorf("provide category_id or category, not both")
	}
	if e.Member != nil && e.MemberID != nil {
		return prior, nil, fmt.Errorf("provide member_id or member, not both")
	}
	if e.Category != nil || e.CategoryID != nil {
		if e.CategoryID != nil {
			copy := *e.CategoryID
			e.Category = &copy
		}
		name := strings.TrimSpace(*e.Category)
		if name == "" || e.CategoryID == nil && (strings.EqualFold(name, "uncategorized") || strings.EqualFold(name, "none")) {
			if prior.CategoryID != "" {
				next.CategoryID = ""
				changes = append(changes, "clear the category (was "+c.nameOf(prior.CategoryID)+")")
			}
		} else {
			category, err := assistantCreationCategory(c.cats, "", name)
			if e.CategoryID != nil {
				category, err = assistantCreationCategory(c.cats, name, "")
			}
			if err != nil {
				return prior, nil, fmt.Errorf("no category matching %q — create it first with create_category", name)
			}
			id := category.ID
			if id != prior.CategoryID {
				next.CategoryID = id
				changes = append(changes, fmt.Sprintf("category %s → %s", c.nameOf(prior.CategoryID), c.nameOf(id)))
			}
		}
	}
	if e.Payee != nil && strings.TrimSpace(*e.Payee) != prior.Payee {
		next.Payee = strings.TrimSpace(*e.Payee)
		changes = append(changes, fmt.Sprintf("payee %q → %q", prior.Payee, next.Payee))
	}
	if e.Description != nil && strings.TrimSpace(*e.Description) != prior.Desc {
		next.Desc = strings.TrimSpace(*e.Description)
		changes = append(changes, fmt.Sprintf("description %q → %q", prior.Desc, next.Desc))
	}
	if e.Date != nil {
		d, err := dateutil.ParseDate(strings.TrimSpace(*e.Date))
		if err != nil {
			return prior, nil, fmt.Errorf("couldn't read the date %q — use YYYY-MM-DD", *e.Date)
		}
		if !dateutil.DayStart(d).Equal(dateutil.DayStart(prior.Date)) {
			next.Date = d
			changes = append(changes, fmt.Sprintf("date %s → %s", prior.Date.Format(dateutil.Layout), d.Format(dateutil.Layout)))
		}
	}
	if e.Member != nil || e.MemberID != nil {
		if e.MemberID != nil {
			copy := *e.MemberID
			e.Member = &copy
		}
		name := strings.TrimSpace(*e.Member)
		newID := ""
		if name != "" && (e.MemberID != nil || !strings.EqualFold(name, "unassigned") && !strings.EqualFold(name, "none")) {
			matches := 0
			for _, m := range c.members {
				if e.MemberID != nil && m.ID == name || e.MemberID == nil && strings.EqualFold(strings.TrimSpace(m.Name), name) {
					newID = m.ID
					matches++
				}
			}
			if matches != 1 {
				return prior, nil, fmt.Errorf("provide member_id or an unambiguous exact household member name; %d matches for %q", matches, name)
			}
		}
		if newID != prior.PrimaryOwner() || len(prior.Owners()) > 1 {
			next.SetOwners([]string{newID})
			changes = append(changes, fmt.Sprintf("person %s → %s", c.memberOf(prior.PrimaryOwner()), c.memberOf(newID)))
		}
	}
	if e.Tags != nil {
		clean := make([]string, 0, len(*e.Tags))
		for _, t := range *e.Tags {
			if t = strings.TrimSpace(t); t != "" {
				clean = append(clean, t)
			}
		}
		if !slices.Equal(clean, prior.Tags) {
			next.Tags = clean
			changes = append(changes, fmt.Sprintf("tags [%s] → [%s]", strings.Join(prior.Tags, " "), strings.Join(clean, " ")))
		}
	}
	if e.Cleared != nil && *e.Cleared != prior.Cleared {
		next.Cleared = *e.Cleared
		if *e.Cleared {
			next.ClearedAt = time.Now()
			changes = append(changes, "mark it cleared")
		} else {
			next.ClearedAt = time.Time{}
			changes = append(changes, "mark it not cleared")
		}
	}

	// Amount and direction resolve together: the magnitude is taken as an
	// absolute figure and the sign says whether the money came in or went out,
	// so "make this $40" can never silently flip an expense into income.
	if len(e.Amount) > 0 || e.Direction != nil {
		account, found := domain.AccountByID(c.accounts, prior.AccountID)
		if !found {
			return prior, nil, fmt.Errorf("transaction account is unavailable")
		}
		flow, err := appstate.AssistantTransactionAmount(account, prior.Amount)
		if err != nil {
			return prior, nil, err
		}
		negative := flow.Amount < 0
		var magnitude int64
		if len(e.Amount) > 0 {
			decimal := string(e.Amount)
			if e.Amount[0] == '"' {
				if err := json.Unmarshal(e.Amount, &decimal); err != nil {
					return prior, nil, err
				}
			}
			magnitude, err = money.ParseMinor(decimal, currency.Decimals(prior.Amount.Currency))
			if err != nil {
				return prior, nil, err
			}
			if magnitude <= 0 {
				return prior, nil, fmt.Errorf("amount must be an exact positive magnitude; use direction to change flow")
			}
		} else {
			magnitude, err = money.Abs(flow.Amount)
			if err != nil {
				return prior, nil, err
			}
		}
		if e.Direction != nil {
			switch strings.ToLower(strings.TrimSpace(*e.Direction)) {
			case "expense", "out", "outbound", "spending":
				negative = true
			case "income", "in", "inbound", "deposit":
				negative = false
			default:
				return prior, nil, fmt.Errorf("direction must be \"expense\" or \"income\", not %q", *e.Direction)
			}
		}
		if negative {
			magnitude, err = money.Neg(magnitude)
			if err != nil {
				return prior, nil, err
			}
		}
		next.Amount, err = appstate.AssistantTransactionAmount(account, money.New(magnitude, prior.Amount.Currency))
		if err != nil {
			return prior, nil, err
		}
		if next.Amount != prior.Amount {
			changes = append(changes, fmt.Sprintf("booked amount %s → %s", assistantContributionMoney(prior.Amount), assistantContributionMoney(next.Amount)))
		}
	}

	if len(changes) == 0 {
		return prior, nil, fmt.Errorf("nothing to change — the transaction already looks like that")
	}
	return next, changes, nil
}

// agTxnLabel names one transaction the way an approval card should read.
func agTxnLabel(t domain.Transaction) string {
	label := strings.TrimSpace(t.Payee)
	if label == "" {
		label = strings.TrimSpace(t.Desc)
	}
	if label == "" {
		label = "the transaction"
	}
	return fmt.Sprintf("%s %s on %s", label, fmtMoney(t.Amount), t.Date.Format(dateutil.Layout))
}

type agExpectedEditPlan struct {
	id      string
	native  appstate.TransactionUpdatePreview
	changes []string
	preview string
	err     error
	used    bool
	result  string
}

func agEditContext(app *appstate.App) (agRptCtx, error) {
	c := agRptCtx{app: app, now: agReportNow()}
	var err error
	c.accounts, err = app.Store().ListAccounts()
	if err != nil {
		return c, err
	}
	c.cats, err = app.CategoriesChecked()
	if err != nil {
		return c, err
	}
	c.members, err = app.Store().ListMembers()
	if err != nil {
		return c, err
	}
	c.txns, err = app.Store().ListTransactions()
	if err != nil {
		return c, err
	}
	c.nameOf = func(id string) string {
		for _, category := range c.cats {
			if category.ID == id {
				return category.Name
			}
		}
		if id == "" {
			return "uncategorized"
		}
		return id
	}
	c.memberOf = func(id string) string {
		for _, member := range c.members {
			if member.ID == id {
				return member.Name
			}
		}
		if id == "" {
			return "unassigned"
		}
		return id
	}
	return c, nil
}

func agPrepareExpectedEdit(app *appstate.App, raw json.RawMessage) *agExpectedEditPlan {
	p := &agExpectedEditPlan{}
	e, err := assistantCreationDecode[agTxnEdit](raw)
	if err != nil {
		p.err = err
		return p
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		p.err = err
		return p
	}
	for key, value := range fields {
		if string(value) == "null" {
			p.err = fmt.Errorf("field %s cannot be null; omit it or use its explicit empty value", key)
			return p
		}
	}
	c, err := agEditContext(app)
	if err != nil {
		p.err = err
		return p
	}
	prior, err := agResolveTxn(c.txns, e.ID)
	if err != nil {
		p.err = err
		return p
	}
	p.id = prior.ID
	next, changes, err := agApplyTxnEdit(c, prior, e)
	if err != nil {
		p.err = err
		return p
	}
	p.changes = changes
	p.native, err = app.PreviewTransactionUpdate(prior, next)
	if err != nil {
		p.err = fmt.Errorf("the complete edited transaction is invalid: %w", err)
		return p
	}
	before, err := json.MarshalIndent(p.native.Before, "", "  ")
	if err != nil {
		p.err = err
		return p
	}
	after, err := json.MarshalIndent(p.native.After, "", "  ")
	if err != nil {
		p.err = err
		return p
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Validated update of exact transaction ID %s: %s.\nAll affected group rows are shown; unchanged fees remain unchanged. JSON monetary integers below are exact minor units with their stored currency.\nBefore:\n%s\nAfter:\n%s\nExact booked amounts:\n", prior.ID, strings.Join(changes, "; "), before, after)
	for _, row := range p.native.After {
		account, found := domain.AccountByID(c.accounts, row.AccountID)
		if !found {
			p.err = fmt.Errorf("transaction account is unavailable")
			return p
		}
		flow, err := appstate.AssistantTransactionAmount(account, row.Amount)
		if err != nil {
			p.err = err
			return p
		}
		meaning := "negative public flow is spending; positive public flow is income"
		if account.Class == domain.ClassLiability {
			meaning = "negative public flow is a charge; positive public flow is a payment"
		}
		fmt.Fprintf(&b, "- ID %s: booked %s; public flow %s (account ID %s; %s).\n", row.ID, assistantContributionMoney(row.Amount), assistantContributionMoney(flow), row.AccountID, meaning)
	}
	p.preview = b.String()
	if len(p.preview) > 24000 {
		p.err = fmt.Errorf("complete before/after preview exceeds 24000 bytes; use the transaction editor")
	}
	return p
}

func agExpectedUpdateTool(app *appstate.App) chatTool {
	plans := map[string]*agExpectedEditPlan{}
	return chatTool{spec: ai.FunctionTool("update_transaction", "Edit one exact transaction ID after reviewing its full validated before/after snapshot; a unique legacy ID prefix is resolved only for preview and never retargeted at execution. Only supplied fields change. Prefer exact category_id/member_id or unambiguous exact names; blank IDs clear them. amount is an exact positive decimal magnitude (string preferred; numeric compatibility is parsed without float); existing public flow direction remains unless direction is provided, honoring fixed liability sign. Strict YYYY-MM-DD date. Canonical paired transfer amount/date edits show both legs and unchanged fees; malformed/ambiguous pairs and direction reversal are refused. Stale rows, references, group or FX require a new preview.", json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"category_id":{"type":"string"},"category":{"type":"string"},"member_id":{"type":"string"},"member":{"type":"string"},"payee":{"type":"string"},"description":{"type":"string"},"date":{"type":"string"},"amount":{"type":["string","number"]},"direction":{"type":"string","enum":["expense","income"]},"tags":{"type":"array","items":{"type":"string"}},"cleared":{"type":"boolean"}},"required":["id"],"additionalProperties":false}`)), mutates: true,
		preview: func(raw json.RawMessage) string {
			p := agPrepareExpectedEdit(app, raw)
			plans[string(raw)] = p
			if p.err != nil {
				return "Can't prepare the edit: " + p.err.Error()
			}
			return p.preview
		},
		run: func(raw json.RawMessage) string {
			p, ok := plans[string(raw)]
			if !ok {
				return "Review and approve the transaction edit preview first."
			}
			if p.err != nil {
				return "Can't make the edit: " + p.err.Error()
			}
			if p.used {
				return "This approval was already processed. " + p.result
			}
			p.used = true
			if err := app.UpdateTransactionExpected(p.native); err != nil {
				p.result = "Couldn't save the change; review and approve again: " + err.Error()
				return p.result
			}
			p.result = fmt.Sprintf("Updated exact transaction ID %s — %s.", p.id, strings.Join(p.changes, "; "))
			return p.result
		},
	}
}

// agToolsLedgerEdit is the search-and-edit half of the report tooling: find the
// exact rows behind a figure, then change one of them.
func agToolsLedgerEdit(app *appstate.App, base string, rates currency.Rates) []chatTool {
	return []chatTool{
		{
			spec: ai.FunctionTool("find_transactions",
				"Search the ledger and get matching transactions WITH THEIR IDS, so a specific one can then be edited with update_transaction. Filter by text, category, account, person, tag, kind, amount range and window. Unlike the report sections this reaches rows the reports exclude, which are often the ones that are wrong.",
				json.RawMessage(`{"type":"object","properties":{"contains":{"type":"string","description":"text found in the payee or description"},"category":{"type":"string"},"account":{"type":"string"},"member":{"type":"string","description":"household member name"},"tag":{"type":"string"},"kind":{"type":"string","enum":["expense","income","transfer","any"]},"uncategorized":{"type":"boolean","description":"only transactions with no category"},"min_amount":{"type":"number","description":"absolute amount, major units"},"max_amount":{"type":"number","description":"absolute amount, major units"},`+agPeriodEnum+`,"sort"
 :{"type":"string","enum":["newest","oldest","largest"]},"limit":{"type":"integer","description":"default 25, max 100"}}}`)),
			run: func(raw json.RawMessage) string {
				var f agTxnFilter
				_ = json.Unmarshal(raw, &f)
				c, err := agBuildRptCtx(app, base, rates, f.Period, f.From, f.To, "")
				if err != nil {
					return agReportUnavailable(err)
				}
				c.txns = c.rawTxns // Ledger search also reaches archived accounts, as before.
				rows, what := agRunTxnFilter(c, f)
				limit := f.Limit
				if limit <= 0 {
					limit = 25
				}
				if limit > 100 {
					limit = 100
				}
				return agRenderTxns(c, rows, what, limit)
			},
		},
		agExpectedUpdateTool(app),
	}
}
