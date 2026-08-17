// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: registered by agToolsReports (ag_reports.go) via agToolsLedgerEdit

package screens

import (
	"encoding/json"
	"fmt"
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
	var hits []domain.Transaction
	for _, t := range txns {
		if strings.ToLower(t.ID) == q {
			return t, nil
		}
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
		if memSet && t.MemberID != memID {
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
	ID          string    `json:"id"`
	Category    *string   `json:"category"`
	Payee       *string   `json:"payee"`
	Description *string   `json:"description"`
	Date        *string   `json:"date"`
	Amount      *float64  `json:"amount"`
	Direction   *string   `json:"direction"` // expense | income
	Member      *string   `json:"member"`
	Tags        *[]string `json:"tags"`
	Cleared     *bool     `json:"cleared"`
}

// agApplyTxnEdit resolves an edit against the current row and returns the
// updated transaction plus a plain-English list of what changed. It resolves
// names to ids here, so both the preview and the write describe the same thing.
func agApplyTxnEdit(c agRptCtx, prior domain.Transaction, e agTxnEdit) (domain.Transaction, []string, error) {
	next := prior
	var changes []string

	if e.Category != nil {
		name := strings.TrimSpace(*e.Category)
		if name == "" || strings.EqualFold(name, "uncategorized") || strings.EqualFold(name, "none") {
			if prior.CategoryID != "" {
				next.CategoryID = ""
				changes = append(changes, "clear the category (was "+c.nameOf(prior.CategoryID)+")")
			}
		} else {
			id, ok := c.resolveCatID(name)
			if !ok {
				return prior, nil, fmt.Errorf("no category matching %q — create it first with create_category", name)
			}
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
	if e.Member != nil {
		name := strings.TrimSpace(*e.Member)
		newID := ""
		if name != "" && !strings.EqualFold(name, "unassigned") && !strings.EqualFold(name, "none") {
			for _, m := range c.members {
				if strings.EqualFold(m.Name, name) || strings.Contains(strings.ToLower(m.Name), strings.ToLower(name)) {
					newID = m.ID
					break
				}
			}
			if newID == "" {
				return prior, nil, fmt.Errorf("no household member matching %q", name)
			}
		}
		if newID != prior.MemberID {
			next.MemberID = newID
			changes = append(changes, fmt.Sprintf("person %s → %s", c.memberOf(prior.MemberID), c.memberOf(newID)))
		}
	}
	if e.Tags != nil {
		clean := make([]string, 0, len(*e.Tags))
		for _, t := range *e.Tags {
			if t = strings.TrimSpace(t); t != "" {
				clean = append(clean, t)
			}
		}
		if strings.Join(clean, ",") != strings.Join(prior.Tags, ",") {
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
	magnitude := agAbs(next.Amount.Amount)
	sign := int64(1)
	if next.Amount.Amount < 0 {
		sign = -1
	}
	amountChanged := false
	if e.Amount != nil {
		newMag := agAbs(currency.MinorFromMajor(*e.Amount, next.Amount.Currency))
		if newMag != magnitude {
			magnitude, amountChanged = newMag, true
		}
	}
	if e.Direction != nil {
		switch strings.ToLower(strings.TrimSpace(*e.Direction)) {
		case "expense", "out", "outbound", "spending":
			if sign != -1 {
				sign, amountChanged = -1, true
				changes = append(changes, "reclassify it as money going out")
			}
		case "income", "in", "inbound", "deposit":
			if sign != 1 {
				sign, amountChanged = 1, true
				changes = append(changes, "reclassify it as money coming in")
			}
		case "":
		default:
			return prior, nil, fmt.Errorf("direction must be \"expense\" or \"income\", not %q", *e.Direction)
		}
	}
	if amountChanged {
		next.Amount = money.New(sign*magnitude, next.Amount.Currency)
		if agAbs(next.Amount.Amount) != agAbs(prior.Amount.Amount) {
			changes = append(changes, fmt.Sprintf("amount %s → %s", fmtMoney(prior.Amount), fmtMoney(next.Amount)))
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

// agToolsLedgerEdit is the search-and-edit half of the report tooling: find the
// exact rows behind a figure, then change one of them.
func agToolsLedgerEdit(app *appstate.App, base string, rates currency.Rates) []chatTool {
	return []chatTool{
		{
			spec: ai.FunctionTool("find_transactions",
				"Search the ledger and get matching transactions WITH THEIR IDS, so a specific one can then be edited with update_transaction. Filter by text, category, account, person, tag, kind, amount range and window. Unlike the report sections this reaches rows the reports exclude, which are often the ones that are wrong.",
				json.RawMessage(`{"type":"object","properties":{"contains":{"type":"string","description":"text found in the payee or description"},"category":{"type":"string"},"account":{"type":"string"},"member":{"type":"string","description":"household member name"},"tag":{"type":"string"},"kind":{"type":"string","enum":["expense","income","transfer","any"]},"uncategorized":{"type":"boolean","description":"only transactions with no category"},"min_amount":{"type":"number","description":"absolute amount, major units"},"max_amount":{"type":"number","description":"absolute amount, major units"},`+agPeriodEnum+`,"sort":{"type":"string","enum":["newest","oldest","largest"]},"limit":{"type":"integer","description":"default 25, max 100"}}}`)),
			run: func(raw json.RawMessage) string {
				var f agTxnFilter
				_ = json.Unmarshal(raw, &f)
				c := agBuildRptCtx(app, base, rates, f.Period, f.From, f.To, "")
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
		{
			spec: ai.FunctionTool("update_transaction",
				"Edit ONE transaction, named by the id from a trace or search result. Use it to fix a single row behind a report figure — recategorize it, rename the payee, correct the date or amount, reassign it to a person, retag it, or flip it between money in and money out. Only the fields you pass are changed. To change many rows at once by payee phrase, use categorize_transactions instead.",
				json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"the transaction id from trace_report_row or find_transactions"},"category":{"type":"string","description":"category name, or \"uncategorized\" to clear it"},"payee":{"type":"string"},"description":{"type":"string"},"date":{"type":"string","description":"YYYY-MM-DD"},"amount":{"type":"number","description":"absolute amount in major units; the direction is kept unless you also pass direction"},"direction":{"type":"string","enum":["expense","income"],"description":"whether this is money going out or coming in"},"member":{"type":"string","description":"household member name, or \"unassigned\""},"tags":{"type":"array","items":{"type":"string"},"description":"replaces the whole tag list"},"cleared":{"type":"boolean"}},"required":["id"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var e agTxnEdit
				if err := json.Unmarshal(raw, &e); err != nil {
					return "Edit one transaction."
				}
				c := agBuildRptCtx(app, base, rates, "all", "", "", "")
				prior, err := agResolveTxn(c.txns, e.ID)
				if err != nil {
					return "Edit one transaction — " + err.Error()
				}
				_, changes, err := agApplyTxnEdit(c, prior, e)
				if err != nil {
					return "Edit " + agTxnLabel(prior) + " — " + err.Error()
				}
				return "On " + agTxnLabel(prior) + ": " + strings.Join(changes, "; ") + "."
			},
			run: func(raw json.RawMessage) string {
				var e agTxnEdit
				if err := json.Unmarshal(raw, &e); err != nil {
					return "Couldn't read the edit."
				}
				c := agBuildRptCtx(app, base, rates, "all", "", "", "")
				prior, err := agResolveTxn(c.txns, e.ID)
				if err != nil {
					return err.Error()
				}
				next, changes, err := agApplyTxnEdit(c, prior, e)
				if err != nil {
					return err.Error()
				}
				// A transfer's two legs have to stay reciprocal, so the edit goes
				// through the pair-aware path rather than writing one side.
				if err := app.PutTransactionWithTransferPair(prior, next); err != nil {
					return "Couldn't save the change: " + err.Error()
				}
				return fmt.Sprintf("Updated %s — %s.%s", agTxnLabel(next), strings.Join(changes, "; "), openLink("/transactions", next.ID))
			},
		},
	}
}
