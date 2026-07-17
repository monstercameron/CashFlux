// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// chatTool pairs an OpenAI tool spec (what the model sees) with its handler (what
// runs locally when the model calls it). Handlers return a short plain-text result
// fed back to the model. Read tools read the user's data; mutating tools change it
// and require user approval first (preview describes the change for the approval card).
type chatTool struct {
	spec    ai.Tool
	run     func(args json.RawMessage) string
	mutates bool
	preview func(args json.RawMessage) string
}

// approvalReq is a pending mutating tool awaiting the user's yes/no in the chat.
type approvalReq struct {
	tool    string
	preview string
	resp    chan bool
}

// agentStep is one model turn delivered to the tool loop over a channel: either a
// reply (msg+usage) or an error.
type agentStep struct {
	msg   ai.Message
	usage ai.Usage
	err   string
}

// defaultChatSystemPrompt is the assistant's persona + tool-use instructions. The
// user can override it from the chat's "Edit prompt" panel; the live data context is
// always appended separately so a custom prompt never loses it.
const defaultChatSystemPrompt = `You are CashFlux, a capable, confident personal-finance agent built into the user's own budgeting app. You ACT — you don't just describe. You have tools to READ the user's real, on-device figures and to CHANGE their data, and every change is previewed for the user's one-tap approval, so proposing an action is safe and expected. Never say you "can't" do something the tools cover, and never tell the user to go do it manually when a tool exists — do it and let the approval confirm.

Use a tool for every specific figure (category totals, balances, net worth, affordability) — never guess or invent the user's numbers. Combine your own general knowledge (tax brackets, rates, formulas) with evaluate_formula and the user's figures to ESTIMATE things the data doesn't hold (e.g. taxes); state your assumptions rather than refusing. Use web_search / fetch_webpage for current or external facts.

You can change data with these tools (each asks the user to approve first): add/complete tasks; record, delete, and merge/de-duplicate transactions; categorize transactions; create accounts (assets and liabilities); transfer between accounts; set account balances; add goal contributions; create categories.

DUPLICATES: when the user reports a possible duplicate (or a flag mentions one), call find_duplicate_transactions to see the exact entries, then merge_duplicate_transactions to keep one and remove the identical extras — for an exact duplicate, "merge" and "remove the extra" are the same thing, so just do it. Use delete_transaction to remove one specific entry. Always pass the amount and date so you target the right entries.

FLAGGED ACTIVITY: to clear/dismiss the flags under Flagged activity (possible duplicates, spikes, missing transactions, balance anomalies) without changing any data, use dismiss_flagged_activity — omit match to clear them all, or pass a phrase to dismiss specific ones.

CATEGORIZING: call list_uncategorized_transactions, group by merchant, create_category for anything not covered, then categorize_transactions(match, category) per merchant. To fix mis-categorized ones, pass only_uncategorized=false.

FORMULAS: the app derives every figure from named engine variables — ATOMS (indivisible reductions like assets, income, liquid_cash) and MOLECULES (formulas over atoms, like net_worth = assets - liabilities). Call list_formula_metrics to see what's available (with live values and molecule formulas), and evaluate_formula to compute any expression over them (e.g. net_worth * 0.04 / 12). Prefer these for derived math over guessing.

Think in double-entry terms so net worth stays correct. Borrowing against an account (e.g. a 401(k) loan) is roughly NET-WORTH-NEUTRAL the moment you borrow — model it as a new liability for the amount owed PLUS the cash received (add_transfer or update_account_balance), not a one-sided loss. When an event spans accounts, plan the steps, do them in order, and tell the user what you changed. If a detail is missing, pick a sensible default and say so rather than refusing.

Before creating anything, the tools check for an existing or near-duplicate item and return it instead of cloning — relay that rather than forcing a duplicate. Whenever a tool result contains a Markdown link (e.g. "[Open it](/todo#id)"), ALWAYS include that exact link in your reply.

Answer in plain English as short Markdown. The user's money is private and never leaves their device except for these requests.`

// countTxnMatches counts non-transfer transactions whose payee/description contains
// match (case-insensitive); when onlyUncat is true, only those without a category.
func countTxnMatches(txns []domain.Transaction, match string, onlyUncat bool) int {
	q := strings.ToLower(strings.TrimSpace(match))
	if q == "" {
		return 0
	}
	n := 0
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if onlyUncat && t.CategoryID != "" {
			continue
		}
		if strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
			n++
		}
	}
	return n
}

// sameYMD reports whether two times fall on the same calendar day.
func sameYMD(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// dupGroupMatches reports whether a canonical duplicate group (dedupe.FindDuplicates)
// is the one the user means: its description or any member's payee/description contains
// the match phrase, and — when given — its absolute amount and calendar date agree. This
// keeps the agent's merge/delete targeting consistent with the "possible duplicate" flags
// and the Duplicates page, which all key off dedupe.Signature.
func dupGroupMatches(g dedupe.Group, byID map[string]domain.Transaction, match string, amount float64, hasAmount, hasDate bool, dateStr string) bool {
	q := strings.ToLower(strings.TrimSpace(match))
	if q != "" {
		hit := strings.Contains(strings.ToLower(g.Description), q)
		for _, id := range g.IDs {
			if t, ok := byID[id]; ok && strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
				hit = true
			}
		}
		if !hit {
			return false
		}
	}
	if hasAmount && absMinor(g.Amount) != absMinor(currency.MinorFromMajor(amount, g.Currency)) {
		return false
	}
	if hasDate && g.Date != strings.TrimSpace(dateStr) {
		return false
	}
	return true
}

// removalMatches returns non-transfer transactions whose payee/description contains
// match, optionally constrained to an absolute amount (major units) and a calendar
// day (YYYY-MM-DD). It backs the delete_transaction / merge_duplicate_transactions
// tools so they target the exact entries the user means.
func removalMatches(txns []domain.Transaction, match string, amount float64, hasAmount bool, dateStr string) []domain.Transaction {
	q := strings.ToLower(strings.TrimSpace(match))
	day, hasDay := time.Time{}, false
	if strings.TrimSpace(dateStr) != "" {
		if d, err := dateutil.ParseDate(dateStr); err == nil {
			day, hasDay = d, true
		}
	}
	var out []domain.Transaction
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
			continue
		}
		if hasAmount && absMinor(t.Amount.Amount) != absMinor(currency.MinorFromMajor(amount, t.Amount.Currency)) {
			continue
		}
		if hasDay && !sameYMD(t.Date, day) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// categoryNames returns a comma-separated list of the user's category names.
func categoryNames(cats []domain.Category) string {
	ns := make([]string, 0, len(cats))
	for _, c := range cats {
		ns = append(ns, c.Name)
	}
	return strings.Join(ns, ", ")
}

// buildChatTools assembles the read-only finance tools the Insights chat exposes,
// bound to the user's live data, so the model can answer specific questions from
// real figures instead of guessing. All computation is local; only the short tool
// results (totals, counts) go back to the model.
func buildChatTools(app *appstate.App, base string, rates currency.Rates) []chatTool {
	txns := app.Transactions()
	accounts := app.Accounts()
	cats := app.Categories()
	catByID := make(map[string]string, len(cats))
	for _, c := range cats {
		catByID[c.ID] = c.Name
	}
	accNameByID := make(map[string]string, len(accounts))
	for _, ac := range accounts {
		accNameByID[ac.ID] = ac.Name
	}
	accLabel := func(id string) string {
		if n := accNameByID[id]; n != "" {
			return n
		}
		return "an account"
	}
	catLabel := func(id string) string {
		if n := catByID[id]; n != "" {
			return n
		}
		return "uncategorized"
	}
	now := time.Now()
	mStart, mEnd := dateutil.MonthRange(now)
	lmStart, lmEnd := dateutil.MonthRange(dateutil.AddMonths(now, -1))
	fmtM := func(minor int64) string { return fmtMoney(money.New(minor, base)) }

	// resolveCategory maps a user/model-supplied name to a category ID: exact
	// (case-insensitive) first, then a substring match. Returns ok=false if none.
	resolveCategory := func(name string) (domain.Category, bool) {
		q := strings.ToLower(strings.TrimSpace(name))
		for _, c := range cats {
			if strings.ToLower(c.Name) == q {
				return c, true
			}
		}
		for _, c := range cats {
			if q != "" && strings.Contains(strings.ToLower(c.Name), q) {
				return c, true
			}
		}
		return domain.Category{}, false
	}
	catNames := func() string {
		ns := make([]string, 0, len(cats))
		for _, c := range cats {
			ns = append(ns, c.Name)
		}
		return strings.Join(ns, ", ")
	}
	resolveAccount := func(name string) (domain.Account, bool) {
		q := strings.ToLower(strings.TrimSpace(name))
		for _, ac := range accounts {
			if strings.ToLower(ac.Name) == q {
				return ac, true
			}
		}
		for _, ac := range accounts {
			if q != "" && strings.Contains(strings.ToLower(ac.Name), q) {
				return ac, true
			}
		}
		return domain.Account{}, false
	}
	periodRange := func(p string) (time.Time, time.Time, string) {
		switch p {
		case "last_month":
			return lmStart, lmEnd, "last month"
		case "all":
			return time.Time{}, dateutil.AddMonths(now, 1200), "all time"
		default:
			return mStart, mEnd, "this month"
		}
	}

	tools := []chatTool{
		{
			spec: ai.FunctionTool("list_categories", "List the user's spending/income category names. Call this first when unsure which category a question refers to.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				if len(cats) == 0 {
					return "No categories defined."
				}
				return "Categories: " + catNames()
			},
		},
		{
			spec: ai.FunctionTool("spending_by_category",
				"Total the user's spending in a category over a period. Use for 'how much did I spend on X'.",
				json.RawMessage(`{"type":"object","properties":{"category":{"type":"string","description":"category name"},"period":{"type":"string","enum":["this_month","last_month","all"]}},"required":["category"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Category string `json:"category"`
					Period   string `json:"period"`
				}
				_ = json.Unmarshal(raw, &a)
				cat, ok := resolveCategory(a.Category)
				if !ok {
					return fmt.Sprintf("No category matching %q. Available: %s", a.Category, catNames())
				}
				start, end, label := periodRange(a.Period)
				var total, count int64
				for _, t := range txns {
					if !t.IsExpense() || t.CategoryID != cat.ID {
						continue
					}
					if !start.IsZero() && !dateutil.InRange(t.Date, start, end) {
						continue
					}
					if conv, err := rates.Convert(t.Amount.Abs(), base); err == nil {
						total += conv.Amount
						count++
					}
				}
				return fmt.Sprintf("Spent %s on %s (%d transactions, %s).", fmtM(total), cat.Name, count, label)
			},
		},
		{
			spec: ai.FunctionTool("financial_summary", "The user's headline figures: net worth, this month's income/spending/net, and savings rate.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				net, assets, liab, _ := ledger.NetWorth(accounts, txns, rates)
				income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
				netFlow := income.Amount - expense.Amount
				rate := 0.0
				if income.Amount > 0 {
					rate = float64(netFlow) / float64(income.Amount) * 100
				}
				return fmt.Sprintf("Net worth %s (assets %s, liabilities %s). This month: income %s, spending %s, net %s, savings rate %.0f%%.",
					fmtM(net.Amount), fmtM(assets.Amount), fmtM(liab.Amount), fmtM(income.Amount), fmtM(expense.Amount), fmtM(netFlow), rate)
			},
		},
		{
			spec: ai.FunctionTool("account_balances", "List the user's active accounts with their current balances.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				var b strings.Builder
				for _, a := range accounts {
					if a.Archived {
						continue
					}
					bal, err := ledger.Balance(a, txns)
					if err != nil {
						continue
					}
					fmt.Fprintf(&b, "%s: %s\n", a.Name, fmtMoney(bal))
				}
				if b.Len() == 0 {
					return "No active accounts."
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("check_affordability",
				"Project whether the user can afford an amount, optionally by a number of months out, from their assets and this month's net cash flow.",
				json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number","description":"amount in the base currency's major units, e.g. 2000"},"months":{"type":"integer","description":"months from now (0 = now)"}},"required":["amount"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Amount float64 `json:"amount"`
					Months int     `json:"months"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Could not read the amount."
				}
				amount := currency.MinorFromMajor(a.Amount, base)
				_, assets, _, _ := ledger.NetWorth(accounts, txns, rates)
				income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
				res := afford.CanAfford(amount, assets.Amount, income.Amount-expense.Amount, a.Months, 0)
				if res.Affordable {
					return fmt.Sprintf("Affordable: projected balance %s covers %s (%s free to spend).", fmtM(res.ProjectedBalance), fmtM(amount), fmtM(res.Available))
				}
				if res.MonthsNeeded > 0 {
					return fmt.Sprintf("Not yet: short %s now; affordable in about %d months at the current pace.", fmtM(res.Shortfall), res.MonthsNeeded)
				}
				return fmt.Sprintf("Not affordable: short %s, and the current cash flow won't close the gap.", fmtM(res.Shortfall))
			},
		},
		{
			spec: ai.FunctionTool("list_transactions",
				"List the user's recent transactions (date, payee, amount, category), optionally filtered by category and period. Capped at 30 rows.",
				json.RawMessage(`{"type":"object","properties":{"category":{"type":"string"},"period":{"type":"string","enum":["this_month","last_month","all"]},"limit":{"type":"integer"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Category string `json:"category"`
					Period   string `json:"period"`
					Limit    int    `json:"limit"`
				}
				_ = json.Unmarshal(raw, &a)
				start, end, _ := periodRange(a.Period)
				catID, matchCat := "", false
				if a.Category != "" {
					if c, ok := resolveCategory(a.Category); ok {
						catID, matchCat = c.ID, true
					}
				}
				limit := a.Limit
				if limit <= 0 || limit > 30 {
					limit = 15
				}
				rows := make([]domain.Transaction, 0, len(txns))
				for _, t := range txns {
					if matchCat && t.CategoryID != catID {
						continue
					}
					if !start.IsZero() && !dateutil.InRange(t.Date, start, end) {
						continue
					}
					rows = append(rows, t)
				}
				sort.Slice(rows, func(i, j int) bool { return rows[i].Date.After(rows[j].Date) })
				if len(rows) > limit {
					rows = rows[:limit]
				}
				if len(rows) == 0 {
					return "No matching transactions."
				}
				catName := make(map[string]string, len(cats))
				for _, c := range cats {
					catName[c.ID] = c.Name
				}
				var b strings.Builder
				for _, t := range rows {
					label := strings.TrimSpace(t.Payee)
					if label == "" {
						label = strings.TrimSpace(t.Desc)
					}
					cn := catName[t.CategoryID]
					if cn == "" {
						cn = "uncategorized"
					}
					fmt.Fprintf(&b, "%s  %s  %s  [%s]\n", t.Date.Format("Jan 2"), label, fmtMoney(t.Amount), cn)
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("list_members", "List the household members.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				ms := app.Members()
				if len(ms) == 0 {
					return "No household members."
				}
				ns := make([]string, 0, len(ms))
				for _, m := range ms {
					n := m.Name
					if m.IsDefault {
						n += " (default)"
					}
					ns = append(ns, n)
				}
				return "Members: " + strings.Join(ns, ", ")
			},
		},
		{
			spec: ai.FunctionTool("web_search",
				"Search the web for current or external facts (tax brackets, rates, prices, definitions). Returns a short summary. The query is sent to a public search engine; do not include the user's private financial details in it.",
				json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Query) == "" {
					return "Empty search query."
				}
				u := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&t=cashflux&q=" + url.QueryEscape(a.Query)
				var headers map[string]any
				if k := strings.TrimSpace(uistate.LoadWebSearchKey()); k != "" {
					headers = map[string]any{"Authorization": "Bearer " + k}
				}
				body, ok := blockingFetchText(u, headers)
				if !ok {
					return "Web search is unavailable (offline or blocked)."
				}
				return summarizeDDG(a.Query, body)
			},
		},
		{
			spec: ai.FunctionTool("fetch_webpage",
				"Fetch a web page by URL and return its readable text content. Use a URL from web_search results to read the full details.",
				json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Could not read the URL."
				}
				target := strings.TrimSpace(a.URL)
				if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
					return "Please provide a full http(s) URL."
				}
				// Jina Reader returns clean, CORS-accessible readable text for any page.
				reader := "https://r.jina.ai/" + target
				var headers map[string]any
				if k := strings.TrimSpace(uistate.LoadWebSearchKey()); k != "" {
					headers = map[string]any{"Authorization": "Bearer " + k}
				}
				body, ok := blockingFetchText(reader, headers)
				if !ok {
					return "Couldn't fetch that page (it may block reading)."
				}
				body = strings.TrimSpace(body)
				if body == "" {
					return "The page returned no readable text."
				}
				if r := []rune(body); len(r) > 2500 {
					body = string(r[:2500]) + "…"
				}
				return body
			},
		},
		{
			spec: ai.FunctionTool("list_budgets", "List the user's budgets with category, period, and limit.", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				bs := app.Budgets()
				if len(bs) == 0 {
					return "No budgets set."
				}
				var b strings.Builder
				for _, bd := range bs {
					fmt.Fprintf(&b, "%s — %s, %s limit %s\n", bd.Name, catLabel(bd.CategoryID), string(bd.Period), fmtMoney(bd.Limit))
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("list_goals", "List the user's savings goals with progress (current/target, percent, target date).", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				gs := app.Goals()
				if len(gs) == 0 {
					return "No savings goals."
				}
				var b strings.Builder
				for _, g := range gs {
					pct := goalsvc.RawPercent(g)
					line := fmt.Sprintf("%s — %s of %s (%d%%)", g.Name, fmtMoney(g.CurrentAmount), fmtMoney(g.TargetAmount), pct)
					if !g.TargetDate.IsZero() {
						line += ", by " + g.TargetDate.Format("Jan 2, 2006")
					}
					b.WriteString(line + "\n")
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("list_tasks", "List the user's to-do tasks (title, status, priority, due date).", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				ts := app.Tasks()
				if len(ts) == 0 {
					return "No tasks."
				}
				var b strings.Builder
				for _, t := range ts {
					line := fmt.Sprintf("[%s] %s (%s priority)", string(t.Status), t.Title, string(t.Priority))
					if !t.Due.IsZero() {
						line += ", due " + t.Due.Format("Jan 2")
					}
					b.WriteString(line + "\n")
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("list_recurring", "List recurring cash flows / upcoming bills (label, amount, cadence, next due date).", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				rs := app.Recurring()
				if len(rs) == 0 {
					return "No recurring items or bills."
				}
				var b strings.Builder
				for _, r := range rs {
					fmt.Fprintf(&b, "%s — %s %s, next %s\n", r.Label, fmtMoney(r.Amount), string(r.Cadence), r.NextDue.Format("Jan 2, 2006"))
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("spending_breakdown", "Top spending categories for a period (where the money went).", json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","enum":["this_month","last_month","all"]},"limit":{"type":"integer"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Period string `json:"period"`
					Limit  int    `json:"limit"`
				}
				_ = json.Unmarshal(raw, &a)
				start, end, label := periodRange(a.Period)
				sums := map[string]int64{}
				for _, t := range txns {
					if !t.IsExpense() {
						continue
					}
					if !start.IsZero() && !dateutil.InRange(t.Date, start, end) {
						continue
					}
					if conv, err := rates.Convert(t.Amount.Abs(), base); err == nil {
						sums[t.CategoryID] += conv.Amount
					}
				}
				if len(sums) == 0 {
					return "No spending in " + label + "."
				}
				type kv struct {
					id  string
					amt int64
				}
				rows := make([]kv, 0, len(sums))
				for id, amt := range sums {
					rows = append(rows, kv{id, amt})
				}
				sort.Slice(rows, func(i, j int) bool { return rows[i].amt > rows[j].amt })
				limit := a.Limit
				if limit <= 0 || limit > 15 {
					limit = 8
				}
				if len(rows) > limit {
					rows = rows[:limit]
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Top spending (%s):\n", label)
				for _, r := range rows {
					fmt.Fprintf(&b, "%s: %s\n", catLabel(r.id), fmtMoney(money.New(r.amt, base)))
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("list_formula_metrics",
				"List the engine's formula variables: ATOMS (indivisible reductions over the user's data — e.g. assets, liabilities, liquid_cash, income, expense) and MOLECULES (compound figures defined as a formula over atoms — e.g. net_worth = assets - liabilities), plus per-entity metrics (budgets, goals, accounts, pools, plans). Each row shows the name, current value, and — for molecules — the formula. Use any name with evaluate_formula. Optionally filter by a substring, or set molecules_only.",
				json.RawMessage(`{"type":"object","properties":{"filter":{"type":"string","description":"optional substring matched on name/label/doc"},"molecules_only":{"type":"boolean"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Filter        string `json:"filter"`
					MoleculesOnly bool   `json:"molecules_only"`
				}
				_ = json.Unmarshal(raw, &a)
				metrics := allFormulaMetrics(app)
				vars := liveEngineVars(app)
				q := strings.ToLower(strings.TrimSpace(a.Filter))
				groupOrder := []string{}
				byGroup := map[string][]string{}
				total := 0
				for _, m := range metrics {
					if a.MoleculesOnly && !m.Molecule {
						continue
					}
					if q != "" && !strings.Contains(strings.ToLower(m.Name+" "+m.Label+" "+m.Doc), q) {
						continue
					}
					val := "—"
					if v, ok := vars[m.Name]; ok {
						val = formatFormulaValue(v)
					}
					line := fmt.Sprintf("%s = %s", m.Name, val)
					if m.Molecule && m.Formula != "" {
						line += "  [" + m.Formula + "]"
					}
					if m.Label != "" {
						line += "  — " + m.Label
					}
					g := string(m.Group)
					if _, ok := byGroup[g]; !ok {
						groupOrder = append(groupOrder, g)
					}
					byGroup[g] = append(byGroup[g], line)
					total++
				}
				if total == 0 {
					return "No matching formula metrics."
				}
				const cap = 140
				var b strings.Builder
				b.WriteString("Formula metrics — atoms + molecules (use any name in evaluate_formula):\n")
				shown := 0
				for _, g := range groupOrder {
					if shown >= cap {
						break
					}
					fmt.Fprintf(&b, "\n[%s]\n", g)
					for _, line := range byGroup[g] {
						if shown >= cap {
							break
						}
						b.WriteString(line + "\n")
						shown++
					}
				}
				if shown < total {
					fmt.Fprintf(&b, "\n… %d more — narrow with the filter argument.", total-shown)
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
		{
			spec: ai.FunctionTool("evaluate_formula",
				evalFormulaToolDesc(),
				json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"}},"required":["expression"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Expression string `json:"expression"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Expression) == "" {
					return "Provide an expression to evaluate."
				}
				expr := strings.TrimSpace(a.Expression)
				vars := liveEngineVars(app)
				val, err := formula.Eval(expr, formula.Env{Vars: vars})
				if err != nil {
					return "Couldn't evaluate “" + expr + "”: " + err.Error()
				}
				out := fmt.Sprintf("%s = %s", expr, formatFormulaValue(val))
				// A bare variable name: show how it's derived (atom source or molecule formula).
				if _, ok := vars[expr]; ok {
					if d, ok := engineenv.Explain(expr, vars, app.Molecules()); ok {
						switch d.Kind {
						case "molecule":
							out += fmt.Sprintf("\nMolecule: %s = %s", d.Name, d.Formula)
							if len(d.Inputs) > 0 {
								names := make([]string, 0, len(d.Inputs))
								for k := range d.Inputs {
									names = append(names, k)
								}
								sort.Strings(names)
								parts := make([]string, 0, len(names))
								for _, k := range names {
									parts = append(parts, fmt.Sprintf("%s=%s", k, formatFormulaValue(d.Inputs[k])))
								}
								out += " (with " + strings.Join(parts, ", ") + ")"
							}
						case "atom":
							if d.Source != "" {
								out += "\nAtom: " + d.Source
							}
						}
					}
				}
				return out
			},
		},

		// --- Write tools (mutates: require the user's approval before running) ---
		{
			spec: ai.FunctionTool("add_task",
				"Add a to-do task for the user.",
				json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"notes":{"type":"string"},"priority":{"type":"string","enum":["low","medium","high"]},"due":{"type":"string","description":"YYYY-MM-DD"}},"required":["title"]}`)),
			mutates: true,
			// QA CF-19: the approval card must show EVERY field that will be
			// written — due date and priority used to be silently applied after a
			// preview that named only the title.
			preview: func(raw json.RawMessage) string {
				var a struct {
					Title    string `json:"title"`
					Notes    string `json:"notes"`
					Priority string `json:"priority"`
					Due      string `json:"due"`
				}
				_ = json.Unmarshal(raw, &a)
				out := "Add a to-do: “" + strings.TrimSpace(a.Title) + "”"
				if p := strings.TrimSpace(a.Priority); p != "" {
					out += " · " + p + " priority"
				}
				if d := strings.TrimSpace(a.Due); d != "" {
					out += " · due " + d
				}
				if n := strings.TrimSpace(a.Notes); n != "" {
					if len(n) > 80 {
						n = n[:80] + "…"
					}
					out += "\nNotes: " + n
				}
				return out
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Title    string `json:"title"`
					Notes    string `json:"notes"`
					Priority string `json:"priority"`
					Due      string `json:"due"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Title) == "" {
					return "A task needs a title."
				}
				title := strings.TrimSpace(a.Title)
				// Dedupe: don't create a near-duplicate open task.
				for _, ex := range app.Tasks() {
					if ex.Status != domain.StatusDone && similarText(ex.Title, title) {
						return fmt.Sprintf("A similar to-do already exists: “%s”.%s", ex.Title, openLink("/todo", ex.ID))
					}
				}
				t := domain.Task{ID: id.New(), Title: title, Notes: a.Notes, Status: domain.StatusOpen, Priority: parseTaskPriority(a.Priority), Source: domain.SourceAI}
				if d, err := dateutil.ParseDate(a.Due); err == nil && a.Due != "" {
					t.Due = d
				}
				if err := app.PutTask(t); err != nil {
					return "Couldn't add the task: " + err.Error()
				}
				return "Added to-do: " + t.Title + "." + openLink("/todo", t.ID)
			},
		},
		{
			spec: ai.FunctionTool("complete_task",
				"Mark a to-do task done, found by (part of) its title.",
				json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Title string `json:"title"`
				}
				_ = json.Unmarshal(raw, &a)
				return "Mark done: the to-do matching “" + strings.TrimSpace(a.Title) + "”"
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Title string `json:"title"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Title) == "" {
					return "Which task? Provide its title."
				}
				q := strings.ToLower(strings.TrimSpace(a.Title))
				for _, t := range app.Tasks() {
					if strings.Contains(strings.ToLower(t.Title), q) {
						t.Status = domain.StatusDone
						if err := app.PutTask(t); err != nil {
							return "Couldn't update the task: " + err.Error()
						}
						return "Marked done: " + t.Title
					}
				}
				return "No to-do matching “" + a.Title + "”."
			},
		},
		{
			spec: ai.FunctionTool("add_transaction",
				"Record a transaction. A negative amount is an expense, positive is income. Resolve account and category by name. (description is optional — a sensible one is used if omitted.)",
				json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number","description":"major units; negative = expense"},"account":{"type":"string"},"category":{"type":"string"},"payee":{"type":"string"},"description":{"type":"string"},"date":{"type":"string","description":"YYYY-MM-DD, defaults to today"}},"required":["amount","account"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Amount  float64 `json:"amount"`
					Account string  `json:"account"`
					Payee   string  `json:"payee"`
				}
				_ = json.Unmarshal(raw, &a)
				return fmt.Sprintf("Record %s in %s%s", fmtMoney(money.New(currency.MinorFromMajor(a.Amount, base), base)), a.Account, ifStr(a.Payee != "", " — "+a.Payee, ""))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Amount      float64 `json:"amount"`
					Account     string  `json:"account"`
					Category    string  `json:"category"`
					Payee       string  `json:"payee"`
					Description string  `json:"description"`
					Date        string  `json:"date"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the transaction details."
				}
				acc, ok := resolveAccount(a.Account)
				if !ok {
					return "No account matching “" + a.Account + "”."
				}
				// Desc is required by validation; derive a sensible one so the model never
				// has to ask the user for it.
				desc := strings.TrimSpace(a.Description)
				if desc == "" {
					desc = strings.TrimSpace(a.Payee)
				}
				if desc == "" {
					if a.Amount >= 0 {
						desc = "Income"
					} else {
						desc = "Expense"
					}
				}
				amt := money.New(currency.MinorFromMajor(a.Amount, acc.Currency), acc.Currency)
				t := domain.Transaction{ID: id.New(), AccountID: acc.ID, Amount: amt, Payee: strings.TrimSpace(a.Payee), Desc: desc, Date: now, Source: domain.TxnSourceAssistant}
				if d, err := dateutil.ParseDate(a.Date); err == nil && a.Date != "" {
					t.Date = d
				}
				if a.Category != "" {
					if c, ok := resolveCategory(a.Category); ok {
						t.CategoryID = c.ID
					}
				}
				// Dedupe: skip an identical entry already recorded the same day.
				for _, ex := range txns {
					if ex.AccountID == t.AccountID && ex.Amount.Amount == t.Amount.Amount &&
						ex.Date.Year() == t.Date.Year() && ex.Date.YearDay() == t.Date.YearDay() &&
						strings.EqualFold(strings.TrimSpace(ex.Payee), t.Payee) {
						return fmt.Sprintf("A matching transaction (%s in %s) is already recorded that day.%s", fmtMoney(amt), acc.Name, openLink("/transactions", ex.ID))
					}
				}
				if err := app.PutTransaction(t); err != nil {
					return "Couldn't record the transaction: " + err.Error()
				}
				return fmt.Sprintf("Recorded %s in %s.%s", fmtMoney(t.Amount), acc.Name, openLink("/transactions", t.ID))
			},
		},
		{
			spec: ai.FunctionTool("list_uncategorized_transactions",
				"List the user's transactions that have no category yet, so you can propose categories for them. Returns up to `limit` (default 30) with payee, description, and amount.",
				json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Limit int `json:"limit"`
				}
				_ = json.Unmarshal(raw, &a)
				if a.Limit <= 0 || a.Limit > 60 {
					a.Limit = 30
				}
				var b strings.Builder
				n := 0
				for _, t := range txns {
					if t.IsTransfer() || t.CategoryID != "" {
						continue
					}
					b.WriteString(fmt.Sprintf("- %s | %s\n", strings.TrimSpace(t.Payee+" — "+t.Desc), fmtM(t.Amount.Amount)))
					if n++; n >= a.Limit {
						break
					}
				}
				if n == 0 {
					return "Every transaction already has a category."
				}
				return fmt.Sprintf("%d uncategorized transaction(s):\n%s", n, b.String())
			},
		},
		{
			spec: ai.FunctionTool("create_category",
				"Create a new spending or income category. Use this when the user's transactions aren't covered by any existing category. Returns the existing one instead if a category with that name already exists.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"kind":{"type":"string","enum":["expense","income"],"description":"defaults to expense"}},"required":["name"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Name string `json:"name"`
					Kind string `json:"kind"`
				}
				_ = json.Unmarshal(raw, &a)
				kind := "expense"
				if strings.EqualFold(a.Kind, "income") {
					kind = "income"
				}
				return fmt.Sprintf("Create the %s category “%s”.", kind, strings.TrimSpace(a.Name))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Name string `json:"name"`
					Kind string `json:"kind"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the category details."
				}
				name := strings.TrimSpace(a.Name)
				if name == "" {
					return "A category needs a name."
				}
				for _, c := range cats {
					if strings.EqualFold(c.Name, name) {
						return fmt.Sprintf("A category “%s” already exists.%s", c.Name, openLink("/categories", c.ID))
					}
				}
				kind := domain.KindExpense
				if strings.EqualFold(a.Kind, "income") {
					kind = domain.KindIncome
				}
				c := domain.Category{ID: id.New(), Name: name, Kind: kind}
				if err := app.PutCategory(c); err != nil {
					return "Couldn't create the category: " + err.Error()
				}
				return fmt.Sprintf("Created the category “%s”.%s", c.Name, openLink("/categories", c.ID))
			},
		},
		{
			spec: ai.FunctionTool("categorize_transactions",
				"Assign a category to the user's transactions whose payee or description contains a phrase. Use to auto-categorize uncategorized transactions or to fix mis-categorized ones. Resolves the category by name; create it first with create_category if it doesn't exist.",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string","description":"case-insensitive phrase found in the payee or description"},"category":{"type":"string"},"only_uncategorized":{"type":"boolean","description":"when true (default), only change transactions that currently have no category; set false to also re-categorize"}},"required":["match","category"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Match             string `json:"match"`
					Category          string `json:"category"`
					OnlyUncategorized *bool  `json:"only_uncategorized"`
				}
				_ = json.Unmarshal(raw, &a)
				onlyUncat := a.OnlyUncategorized == nil || *a.OnlyUncategorized
				n := countTxnMatches(txns, a.Match, onlyUncat)
				return fmt.Sprintf("Set %s matching “%s” to %s.", plural(n, "transaction"), strings.TrimSpace(a.Match), strings.TrimSpace(a.Category))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Match             string `json:"match"`
					Category          string `json:"category"`
					OnlyUncategorized *bool  `json:"only_uncategorized"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the details."
				}
				q := strings.ToLower(strings.TrimSpace(a.Match))
				if q == "" {
					return "Give a phrase to match on."
				}
				c, ok := resolveCategory(a.Category)
				if !ok {
					return "No category matching “" + a.Category + "”. Create it first with create_category. Existing: " + catNames() + "."
				}
				onlyUncat := a.OnlyUncategorized == nil || *a.OnlyUncategorized
				changed := 0
				for _, t := range txns {
					if t.IsTransfer() || t.CategoryID == c.ID {
						continue
					}
					if onlyUncat && t.CategoryID != "" {
						continue
					}
					if !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
						continue
					}
					t.CategoryID = c.ID
					if err := app.PutTransaction(t); err != nil {
						return fmt.Sprintf("Set %d, then hit an error: %s", changed, err.Error())
					}
					changed++
				}
				if changed == 0 {
					return fmt.Sprintf("No transactions matched “%s”.", strings.TrimSpace(a.Match))
				}
				return fmt.Sprintf("Categorized %s as %s.", plural(changed, "transaction"), c.Name)
			},
		},
		{
			spec: ai.FunctionTool("add_goal_contribution",
				"Add money toward a savings goal, found by (part of) its name.",
				json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string"},"amount":{"type":"number","description":"major units"}},"required":["goal","amount"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Goal   string  `json:"goal"`
					Amount float64 `json:"amount"`
				}
				_ = json.Unmarshal(raw, &a)
				return fmt.Sprintf("Add %s toward the “%s” goal", fmtMoney(money.New(currency.MinorFromMajor(a.Amount, base), base)), a.Goal)
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Goal   string  `json:"goal"`
					Amount float64 `json:"amount"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || a.Amount == 0 {
					return "Provide a goal and a non-zero amount."
				}
				q := strings.ToLower(strings.TrimSpace(a.Goal))
				for _, g := range app.Goals() {
					if strings.Contains(strings.ToLower(g.Name), q) {
						g.CurrentAmount = money.New(g.CurrentAmount.Amount+currency.MinorFromMajor(a.Amount, g.CurrentAmount.Currency), g.CurrentAmount.Currency)
						if err := app.PutGoal(g); err != nil {
							return "Couldn't update the goal: " + err.Error()
						}
						return fmt.Sprintf("Added %s to “%s” — now %s of %s.%s", fmtMoney(money.New(currency.MinorFromMajor(a.Amount, base), base)), g.Name, fmtMoney(g.CurrentAmount), fmtMoney(g.TargetAmount), openLink("/goals", g.ID))
					}
				}
				return "No goal matching “" + a.Goal + "”."
			},
		},
		{
			spec: ai.FunctionTool("add_account",
				"Create an account: an ASSET (checking, savings, cash, investment) or a LIABILITY (loan, credit_card, mortgage, line_of_credit). Use a liability for money owed — e.g. a 401(k) loan. For a liability, `balance` is the amount owed (enter it positive). Optional liability fields: apr, credit_limit, min_payment.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"class":{"type":"string","enum":["asset","liability"]},"type":{"type":"string","description":"checking|savings|cash|investment|loan|credit_card|mortgage|line_of_credit"},"balance":{"type":"number","description":"opening balance in major units; for a liability the amount owed"},"apr":{"type":"number"},"credit_limit":{"type":"number"},"min_payment":{"type":"number"}},"required":["name","class"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Name    string  `json:"name"`
					Class   string  `json:"class"`
					Balance float64 `json:"balance"`
				}
				_ = json.Unmarshal(raw, &a)
				return fmt.Sprintf("Create a %s account “%s”%s", a.Class, strings.TrimSpace(a.Name), ifStr(a.Balance != 0, " ("+fmtMoney(money.New(currency.MinorFromMajor(a.Balance, base), base))+")", ""))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Name        string  `json:"name"`
					Class       string  `json:"class"`
					Type        string  `json:"type"`
					Balance     float64 `json:"balance"`
					APR         float64 `json:"apr"`
					CreditLimit float64 `json:"credit_limit"`
					MinPayment  float64 `json:"min_payment"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Name) == "" {
					return "An account needs a name."
				}
				liability := strings.ToLower(strings.TrimSpace(a.Class)) == "liability"
				acType := domain.TypeChecking
				if liability {
					acType = domain.TypeLoan
				}
				if a.Type != "" {
					if cand := domain.AccountType(strings.ToLower(strings.TrimSpace(a.Type))); cand.Valid() {
						acType = cand
					}
				}
				// Dedupe: don't create an account that already exists by name.
				for _, ex := range accounts {
					if !ex.Archived && similarText(ex.Name, a.Name) {
						return fmt.Sprintf("An account named “%s” already exists.%s", ex.Name, openLink("/accounts", ex.ID))
					}
				}
				cls := acType.Class() // keep class consistent with the resolved type
				minor := currency.MinorFromMajor(a.Balance, base)
				if cls == domain.ClassLiability && minor > 0 {
					minor = -minor // an owed amount is stored negative
				}
				acc := domain.Account{
					ID: id.New(), Name: strings.TrimSpace(a.Name), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
					Class: cls, Type: acType, Currency: base, OpeningBalance: money.New(minor, base), BalanceAsOf: now,
				}
				if cls == domain.ClassLiability {
					if a.APR > 0 {
						acc.InterestRateAPR = a.APR
					}
					if a.CreditLimit > 0 {
						acc.CreditLimit = money.New(currency.MinorFromMajor(a.CreditLimit, base), base)
					}
					if a.MinPayment > 0 {
						acc.MinPayment = money.New(currency.MinorFromMajor(a.MinPayment, base), base)
					}
				}
				if err := app.PutAccount(acc); err != nil {
					return "Couldn't create the account: " + err.Error()
				}
				return fmt.Sprintf("Created %s account “%s” with balance %s.%s", cls, acc.Name, fmtMoney(acc.OpeningBalance), openLink("/accounts", acc.ID))
			},
		},
		{
			spec: ai.FunctionTool("add_transfer",
				"Move money between two of the user's accounts (records a matched transfer). Use this for the cash side of events like a loan payout or moving funds. Resolve accounts by name.",
				json.RawMessage(`{"type":"object","properties":{"from_account":{"type":"string"},"to_account":{"type":"string"},"amount":{"type":"number","description":"major units, positive"},"date":{"type":"string","description":"YYYY-MM-DD, defaults to today"}},"required":["from_account","to_account","amount"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					From   string  `json:"from_account"`
					To     string  `json:"to_account"`
					Amount float64 `json:"amount"`
				}
				_ = json.Unmarshal(raw, &a)
				return fmt.Sprintf("Transfer %s from %s to %s", fmtMoney(money.New(currency.MinorFromMajor(a.Amount, base), base)), a.From, a.To)
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					From   string  `json:"from_account"`
					To     string  `json:"to_account"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || a.Amount <= 0 {
					return "A transfer needs both accounts and a positive amount."
				}
				from, ok1 := resolveAccount(a.From)
				to, ok2 := resolveAccount(a.To)
				if !ok1 {
					return "No account matching “" + a.From + "”."
				}
				if !ok2 {
					return "No account matching “" + a.To + "”."
				}
				if from.ID == to.ID {
					return "The two accounts must be different."
				}
				when := now
				if d, err := dateutil.ParseDate(a.Date); err == nil && a.Date != "" {
					when = d
				}
				fromMinor := currency.MinorFromMajor(a.Amount, from.Currency)
				fromMoney := money.New(fromMinor, from.Currency)
				toMoney := money.New(currency.MinorFromMajor(a.Amount, to.Currency), to.Currency)
				if conv, err := rates.Convert(fromMoney.Abs(), to.Currency); err == nil {
					toMoney = conv // honor FX across currencies
				}
				out := domain.Transaction{ID: id.New(), AccountID: from.ID, Amount: money.New(-fromMinor, from.Currency), TransferAccountID: to.ID, Date: when, Payee: to.Name, Desc: "Transfer to " + to.Name, Source: domain.TxnSourceAssistant}
				in := domain.Transaction{ID: id.New(), AccountID: to.ID, Amount: toMoney, TransferAccountID: from.ID, Date: when, Payee: from.Name, Desc: "Transfer from " + from.Name, Source: domain.TxnSourceAssistant}
				if err := app.PutTransaction(out); err != nil {
					return "Couldn't record the transfer: " + err.Error()
				}
				if err := app.PutTransaction(in); err != nil {
					return "Couldn't record the transfer: " + err.Error()
				}
				return fmt.Sprintf("Transferred %s from %s to %s.%s", fmtMoney(fromMoney), from.Name, to.Name, openLink("/transactions", out.ID))
			},
		},
		{
			spec: ai.FunctionTool("update_account_balance",
				"Set an account's current balance (reconcile to a statement), by an adjusting entry. For a liability, `balance` is the amount still owed (enter it positive).",
				json.RawMessage(`{"type":"object","properties":{"account":{"type":"string"},"balance":{"type":"number","description":"the new current balance in major units"}},"required":["account","balance"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Account string  `json:"account"`
					Balance float64 `json:"balance"`
				}
				_ = json.Unmarshal(raw, &a)
				return fmt.Sprintf("Set %s balance to %s", a.Account, fmtMoney(money.New(currency.MinorFromMajor(a.Balance, base), base)))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Account string  `json:"account"`
					Balance float64 `json:"balance"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Provide an account and a balance."
				}
				acc, ok := resolveAccount(a.Account)
				if !ok {
					return "No account matching “" + a.Account + "”."
				}
				target := currency.MinorFromMajor(a.Balance, acc.Currency)
				if acc.Class == domain.ClassLiability && target > 0 {
					target = -target
				}
				cur, err := ledger.Balance(acc, txns)
				if err != nil {
					return "Couldn't read the current balance."
				}
				delta := target - cur.Amount
				acc.OpeningBalance = money.New(acc.OpeningBalance.Amount+delta, acc.Currency)
				acc.BalanceAsOf = now
				if err := app.PutAccount(acc); err != nil {
					return "Couldn't update the balance: " + err.Error()
				}
				return fmt.Sprintf("Set %s balance to %s (adjusted by %s).", acc.Name, fmtMoney(money.New(target, acc.Currency)), fmtMoney(money.New(delta, acc.Currency)))
			},
		},
		{
			spec: ai.FunctionTool("find_duplicate_transactions",
				"Find groups of likely-duplicate transactions — same calendar date, signed amount, and description — the app's CANONICAL duplicate definition (identical to the 'possible duplicate' flags and the Duplicates page). Call this to locate the duplicates behind a flag before merging or deleting them. Optionally filter by a description/payee phrase.",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string","description":"optional description/payee phrase to filter"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Match string `json:"match"`
				}
				_ = json.Unmarshal(raw, &a)
				byID := make(map[string]domain.Transaction, len(txns))
				for _, t := range txns {
					byID[t.ID] = t
				}
				var b strings.Builder
				n := 0
				for _, g := range dedupe.FindDuplicates(txns) {
					if !dupGroupMatches(g, byID, a.Match, 0, false, false, "") {
						continue
					}
					n++
					label := strings.TrimSpace(g.Description)
					acct := ""
					if t, ok := byID[g.IDs[0]]; ok {
						if label == "" {
							label = strings.TrimSpace(t.Payee)
						}
						acct = accLabel(t.AccountID)
					}
					fmt.Fprintf(&b, "- %d× %s  %s  on %s%s\n", len(g.IDs), label, fmtMoney(money.New(absMinor(g.Amount), g.Currency)), g.Date, ifStr(acct != "", "  in "+acct, ""))
				}
				if n == 0 {
					return "No duplicate transactions found."
				}
				return fmt.Sprintf("%d duplicate group(s):\n%sTo fix one, call merge_duplicate_transactions with its description phrase, amount, and date.", n, b.String())
			},
		},
		{
			spec: ai.FunctionTool("merge_duplicate_transactions",
				"Merge a duplicate group: keep one entry (unioning its tags and cleared flag) and delete the identical extras — exactly the merge the Duplicates page performs. This is the fix for a 'possible duplicate' flag. Match by a description/payee phrase; pass the amount (absolute major units, e.g. 38.49) and date (YYYY-MM-DD) to target one group precisely. Only touches true duplicates (same date, amount, description).",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string"},"amount":{"type":"number","description":"absolute amount in major units, e.g. 38.49"},"date":{"type":"string","description":"YYYY-MM-DD"}},"required":["match"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Match  string  `json:"match"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}
				_ = json.Unmarshal(raw, &a)
				byID := make(map[string]domain.Transaction, len(txns))
				for _, t := range txns {
					byID[t.ID] = t
				}
				extras := 0
				for _, g := range dedupe.FindDuplicates(txns) {
					if dupGroupMatches(g, byID, a.Match, a.Amount, a.Amount != 0, strings.TrimSpace(a.Date) != "", a.Date) {
						extras += len(g.IDs) - 1
					}
				}
				if extras == 0 {
					return fmt.Sprintf("Merge duplicates of “%s” — none found to merge.", strings.TrimSpace(a.Match))
				}
				return fmt.Sprintf("Merge duplicates of “%s”: keep one, remove %d extra %s.", strings.TrimSpace(a.Match), extras, entryWord(extras))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Match  string  `json:"match"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the details."
				}
				if strings.TrimSpace(a.Match) == "" {
					return "Give a description or payee phrase to find the duplicates."
				}
				byID := make(map[string]domain.Transaction, len(txns))
				for _, t := range txns {
					byID[t.ID] = t
				}
				removed := 0
				// Mirror the Duplicates page: keep g.IDs[0], union its metadata from the
				// others (dedupe.Merge), then delete the extras.
				for _, g := range dedupe.FindDuplicates(txns) {
					if !dupGroupMatches(g, byID, a.Match, a.Amount, a.Amount != 0, strings.TrimSpace(a.Date) != "", a.Date) {
						continue
					}
					survivor, ok := byID[g.IDs[0]]
					if !ok {
						continue
					}
					others := make([]domain.Transaction, 0, len(g.IDs)-1)
					for _, oid := range g.IDs[1:] {
						if t, ok := byID[oid]; ok {
							others = append(others, t)
						}
					}
					if merged := dedupe.Merge(survivor, others); app.PutTransaction(merged) != nil {
						return fmt.Sprintf("Merged %d, then hit an error saving the kept entry.", removed)
					}
					for _, o := range others {
						if err := app.DeleteTransactionWithTransferPair(o.ID); err != nil {
							return fmt.Sprintf("Merged %d, then hit an error: %s", removed, err.Error())
						}
						removed++
					}
				}
				if removed == 0 {
					return fmt.Sprintf("No duplicates matched “%s” — nothing to merge.", strings.TrimSpace(a.Match))
				}
				return fmt.Sprintf("Merged duplicates: removed %d extra %s, keeping one of each.", removed, entryWord(removed))
			},
		},
		{
			spec: ai.FunctionTool("delete_transaction",
				"Delete a transaction, matched by a payee/description phrase and (strongly recommended) an amount (absolute major units) and date (YYYY-MM-DD). Use to remove one specific entry — e.g. the extra copy of a duplicate. If several NON-identical transactions match, it lists them and asks you to narrow down rather than guessing.",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string"},"amount":{"type":"number","description":"absolute amount in major units, e.g. 38.49"},"date":{"type":"string","description":"YYYY-MM-DD"}},"required":["match"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Match  string  `json:"match"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}
				_ = json.Unmarshal(raw, &a)
				m := removalMatches(txns, a.Match, a.Amount, a.Amount != 0, a.Date)
				if len(m) == 0 {
					return fmt.Sprintf("Delete a transaction matching “%s” — none found.", strings.TrimSpace(a.Match))
				}
				t := m[0]
				label := strings.TrimSpace(t.Payee)
				if label == "" {
					label = strings.TrimSpace(t.Desc)
				}
				if len(m) == 1 || distinctSignatures(m) == 1 {
					return fmt.Sprintf("Delete: %s %s on %s", label, fmtMoney(t.Amount.Abs()), t.Date.Format("2006-01-02"))
				}
				return fmt.Sprintf("Delete one transaction matching “%s” (%d match — will ask to narrow if not identical).", strings.TrimSpace(a.Match), len(m))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Match  string  `json:"match"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the details."
				}
				if strings.TrimSpace(a.Match) == "" {
					return "Give a payee or description phrase for the transaction to delete."
				}
				m := removalMatches(txns, a.Match, a.Amount, a.Amount != 0, a.Date)
				if len(m) == 0 {
					return fmt.Sprintf("No transaction matched “%s”.", strings.TrimSpace(a.Match))
				}
				del := func(t domain.Transaction) string {
					label := strings.TrimSpace(t.Payee)
					if label == "" {
						label = strings.TrimSpace(t.Desc)
					}
					if err := app.DeleteTransactionWithTransferPair(t.ID); err != nil {
						return "Couldn't delete the transaction: " + err.Error()
					}
					return fmt.Sprintf("Deleted %s %s on %s.", label, fmtMoney(t.Amount.Abs()), t.Date.Format("2006-01-02"))
				}
				if len(m) == 1 {
					return del(m[0])
				}
				// Several match. If they are all identical, delete one extra; otherwise ask
				// the user to narrow it down rather than guessing which to remove.
				if distinctSignatures(m) == 1 {
					sort.Slice(m, func(i, j int) bool {
						if !m[i].Date.Equal(m[j].Date) {
							return m[i].Date.After(m[j].Date)
						}
						return m[i].ID > m[j].ID
					})
					out := del(m[0])
					return out + fmt.Sprintf(" (%d identical %s remain).", len(m)-1, entryWord(len(m)-1))
				}
				var b strings.Builder
				for i, t := range m {
					if i >= 6 {
						break
					}
					label := strings.TrimSpace(t.Payee)
					if label == "" {
						label = strings.TrimSpace(t.Desc)
					}
					fmt.Fprintf(&b, "- %s  %s  on %s\n", label, fmtMoney(t.Amount.Abs()), t.Date.Format("2006-01-02"))
				}
				return fmt.Sprintf("%d transactions match “%s” and they differ. Tell me the amount and date of the one to delete:\n%s", len(m), strings.TrimSpace(a.Match), strings.TrimRight(b.String(), "\n"))
			},
		},
		{
			spec: ai.FunctionTool("dismiss_flagged_activity",
				"Dismiss (clear) flagged activities — the 'possible duplicate', spending-spike, missing-transaction, and balance-anomaly flags shown under Flagged activity. Omit match to clear ALL current flags; pass a phrase to dismiss only flags whose title or detail contains it. Dismissing hides a flag until its underlying situation changes; it does NOT delete or alter any transaction (use merge/delete for that).",
				json.RawMessage(`{"type":"object","properties":{"match":{"type":"string","description":"optional phrase matched on a flag's title/detail; omit to clear all flags"}}}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Match string `json:"match"`
				}
				_ = json.Unmarshal(raw, &a)
				n := len(flaggedActivityKeys(app, a.Match))
				if n == 0 {
					return "No flagged activities to dismiss."
				}
				return fmt.Sprintf("Dismiss %d flagged %s.", n, ifStr(n == 1, "activity", "activities"))
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Match string `json:"match"`
				}
				_ = json.Unmarshal(raw, &a)
				keys := flaggedActivityKeys(app, a.Match)
				if len(keys) == 0 {
					return "No flagged activities to dismiss."
				}
				uistate.DismissAllSmartInsights(keys)
				return fmt.Sprintf("Dismissed %d flagged %s — they'll stay hidden until the situation changes.", len(keys), ifStr(len(keys) == 1, "activity", "activities"))
			},
		},
	}
	// AG series — the assistant tool groups built in the ag_*.go / chat_agent_*.go
	// sidecar files (registered here so those files never touched this one):
	// trust (remember facts), the auditor, natural-language rule/workflow authoring,
	// web-grounded benchmarks, rapid capture, tax gather, and document Q&A.
	tools = append(tools, agToolsTrust(app, base, rates)...)
	tools = append(tools, agToolsAuditor(app, base, rates)...)
	tools = append(tools, agToolsAuthoring(app, base, rates)...)
	tools = append(tools, agToolsBenchmark(app, base, rates)...)
	tools = append(tools, agToolsCapture(app, base, rates)...)
	tools = append(tools, agToolsTax(app, base, rates)...)
	tools = append(tools, agToolsDocQA(app, base, rates)...)
	return tools
}

// flaggedActivityKeys returns the dismissal keys of the current flagged activities
// (the SMART anomaly flags) whose title/detail contains match — or all of them when
// match is empty. Backs the dismiss_flagged_activity tool.
func flaggedActivityKeys(app *appstate.App, match string) []string {
	q := strings.ToLower(strings.TrimSpace(match))
	var keys []string
	for _, f := range runAnomalyDetectors(app, uistate.LoadPrefs().WeekStartWeekday()) {
		if q == "" || strings.Contains(strings.ToLower(f.Title+" "+f.Detail), q) {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

// distinctSignatures counts how many distinct canonical duplicate signatures
// (dedupe.Signature) appear across txns — 1 means every entry is a duplicate of the
// others. Used by delete_transaction to tell "the extra copy of one charge" (safe to
// remove one) from "several different charges that happen to match" (ask to narrow).
func distinctSignatures(txns []domain.Transaction) int {
	seen := map[string]bool{}
	for _, t := range txns {
		seen[dedupe.Signature(t)] = true
	}
	return len(seen)
}

// entryWord returns "entry" or "entries" for a count (plural() would produce "entrys").
func entryWord(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}

// openLink returns a Markdown deep link to an entity's screen, anchored to its id so
// the chat can offer a "jump to it" link (the screens render a matching element id;
// the Insights link handler navigates in-app and scrolls to it).
func openLink(route, id string) string {
	return " [Open it](" + uistate.RoutePath(route) + "#" + id + ")"
}

// normText lowercases, trims, and collapses a string to its word tokens for fuzzy
// comparison.
func normText(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

// similarText reports whether two names are the same or near-duplicates: equal token
// sets, one a subset of the other, or a high word overlap (Jaccard >= 0.6). Used to
// avoid creating duplicate/semi-cloned entities.
func similarText(a, b string) bool {
	wa, wb := normText(a), normText(b)
	if len(wa) == 0 || len(wb) == 0 {
		return false
	}
	set := make(map[string]bool, len(wa))
	for _, w := range wa {
		set[w] = true
	}
	inter := 0
	bset := make(map[string]bool, len(wb))
	for _, w := range wb {
		bset[w] = true
		if set[w] {
			inter++
		}
	}
	union := len(set) + len(bset) - inter
	if union == 0 {
		return false
	}
	// Subset (every word of the shorter appears in the longer) or high Jaccard.
	if inter == len(set) || inter == len(bset) {
		return true
	}
	return float64(inter)/float64(union) >= 0.6
}

// parseTaskPriority maps a string to a TaskPriority (default medium).
func parseTaskPriority(s string) domain.TaskPriority {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return domain.PriorityLow
	case "high":
		return domain.PriorityHigh
	default:
		return domain.PriorityMedium
	}
}

// ifStr returns a when cond, else b.
// evalFormulaToolDesc builds the evaluate_formula tool description from the
// engine's real function list (formula.Functions()), so the description the
// model sees can never drift from what the evaluator actually supports.
func evalFormulaToolDesc() string {
	var b strings.Builder
	b.WriteString("Evaluate an arithmetic/logic expression over the engine's formula variables (the atoms + molecules from list_formula_metrics), computed from the user's live data. Supports + - * / % parentheses, comparisons, and these functions: ")
	for i, f := range formula.Functions() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.Signature)
	}
	b.WriteString(". e.g. 'net_worth * 0.04 / 12' or 'safediv(expense, income, 0) * 100', or a bare variable name like 'liquid_cash' (a single variable also reports how it's derived). Money variables are in the base currency's major units.")
	return b.String()
}

func ifStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// blockingFetchText performs a GET and returns the response body, blocking the
// calling goroutine until it resolves. Safe inside the tool loop's goroutine: Go
// wasm schedules cooperatively, so the JS fetch callback resumes this goroutine.
func blockingFetchText(u string, headers map[string]any) (string, bool) {
	type res struct {
		body string
		ok   bool
	}
	rc := make(chan res, 1)
	var onResp, onText, onErr js.Func
	rel := func() { onResp.Release(); onText.Release(); onErr.Release() }
	onResp = js.FuncOf(func(_ js.Value, a []js.Value) any { return a[0].Call("text") })
	onText = js.FuncOf(func(_ js.Value, a []js.Value) any { rc <- res{a[0].String(), true}; return nil })
	onErr = js.FuncOf(func(_ js.Value, a []js.Value) any { rc <- res{"", false}; return nil })
	opts := map[string]any{}
	if len(headers) > 0 {
		opts["headers"] = headers
	}
	js.Global().Call("fetch", u, opts).Call("then", onResp).Call("then", onText).Call("catch", onErr)
	r := <-rc
	rel()
	return r.body, r.ok
}

// summarizeDDG turns a DuckDuckGo Instant-Answer JSON body into a short text
// summary (answer/abstract/definition + a few related topics), or a clear
// "nothing found" note keyed to the query.
func summarizeDDG(query, body string) string {
	var d struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		Answer        string `json:"Answer"`
		Definition    string `json:"Definition"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		return "Web search returned no usable summary for: " + query
	}
	parts := make([]string, 0, 4)
	urls := make([]string, 0, 4)
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	addURL := func(u string) {
		if u = strings.TrimSpace(u); u != "" && len(urls) < 4 {
			urls = append(urls, u)
		}
	}
	add(d.Answer)
	add(d.AbstractText)
	add(d.Definition)
	addURL(d.AbstractURL)
	for _, rt := range d.RelatedTopics {
		if len(parts) < 4 {
			add(rt.Text)
		}
		addURL(rt.FirstURL)
	}
	if len(parts) == 0 && len(urls) == 0 {
		return "No direct answer found for: " + query + ". Use your own knowledge to estimate and state assumptions."
	}
	out := strings.Join(parts, " ")
	if r := []rune(out); len(r) > 700 {
		out = string(r[:700]) + "…"
	}
	res := "Web search — " + out
	if len(urls) > 0 {
		res += "\nSources (use fetch_webpage to read): " + strings.Join(urls, ", ")
	}
	return res
}
