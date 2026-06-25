// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
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
	"github.com/monstercameron/CashFlux/internal/domain"
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
const defaultChatSystemPrompt = `You are CashFlux, a friendly, concise personal-finance assistant built into the user's own budgeting app. You can call tools to read the user's real, on-device figures — ALWAYS use a tool for any specific figure (a category total, an account balance, net worth, affordability) instead of guessing. If unsure which category a question means, call list_categories first. You may also COMBINE your own general knowledge (tax brackets, rates, formulas) with the calculator tool and the user's figures to ESTIMATE things the data doesn't directly contain (e.g. taxes) — never refuse; compute a clear estimate and state your assumptions. Use web_search for current or external facts, and the calculator for arithmetic. Never invent the user's own numbers.

You can also CHANGE the user's data with tools (every change asks the user to approve first): add/complete tasks, record transactions, create accounts (assets and liabilities), transfer between accounts, set account balances, add goal contributions. Think in double-entry terms so net worth stays correct. Key rule: borrowing against an account (e.g. a 401(k) loan) is roughly NET-WORTH-NEUTRAL at the moment you borrow — model it as a new liability for the amount owed PLUS the cash you received (add_transfer or update_account_balance), not as a one-sided loss. When an event spans multiple accounts, plan the steps, do them in order, and tell the user what you changed. If a detail is missing, pick a sensible default and say so rather than refusing.

Before creating anything, the tools check for an existing or near-duplicate item; if one is found they return it instead of making a clone — relay that to the user rather than forcing a duplicate. Whenever a creation tool's result contains a Markdown link (e.g. "[Open it](/todo#id)"), ALWAYS include that exact link in your reply so the user can jump straight to the new (or existing) item.

Answer in plain English as short Markdown. The user's money is private and never leaves their device except for these requests.`

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

	return []chatTool{
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
				amount := int64(math.Round(a.Amount * 100))
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
			spec: ai.FunctionTool("calculator",
				"Evaluate a finance/math expression. Variables in dollars: net_worth, assets, liabilities, income, spending, net_cashflow (this month). Supports + - * / and parentheses, e.g. 'net_worth * 0.04 / 12'.",
				json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"}},"required":["expression"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Expression string `json:"expression"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Expression) == "" {
					return "Could not read the expression."
				}
				net, assets, liab, _ := ledger.NetWorth(accounts, txns, rates)
				income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
				env := formula.Env{Vars: map[string]float64{
					"net_worth":    float64(net.Amount) / 100,
					"assets":       float64(assets.Amount) / 100,
					"liabilities":  float64(liab.Amount) / 100,
					"income":       float64(income.Amount) / 100,
					"spending":     float64(expense.Amount) / 100,
					"net_cashflow": float64(income.Amount-expense.Amount) / 100,
				}}
				v, err := formula.Eval(a.Expression, env)
				if err != nil {
					return "Calculation error: " + err.Error()
				}
				if n, ok := v.(float64); ok {
					return fmt.Sprintf("%s = %.2f", a.Expression, n)
				}
				return fmt.Sprintf("%s = %v", a.Expression, v)
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

		// --- Write tools (mutates: require the user's approval before running) ---
		{
			spec: ai.FunctionTool("add_task",
				"Add a to-do task for the user.",
				json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"notes":{"type":"string"},"priority":{"type":"string","enum":["low","medium","high"]},"due":{"type":"string","description":"YYYY-MM-DD"}},"required":["title"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Title string `json:"title"`
				}
				_ = json.Unmarshal(raw, &a)
				return "Add a to-do: “" + strings.TrimSpace(a.Title) + "”"
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
				return fmt.Sprintf("Record %s in %s%s", fmtMoney(money.New(int64(math.Round(a.Amount*100)), base)), a.Account, ifStr(a.Payee != "", " — "+a.Payee, ""))
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
				amt := money.New(majorToMinor(a.Amount, acc.Currency), acc.Currency)
				t := domain.Transaction{ID: id.New(), AccountID: acc.ID, Amount: amt, Payee: strings.TrimSpace(a.Payee), Desc: desc, Date: now}
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
				return fmt.Sprintf("Add %s toward the “%s” goal", fmtMoney(money.New(int64(math.Round(a.Amount*100)), base)), a.Goal)
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
						g.CurrentAmount = money.New(g.CurrentAmount.Amount+int64(math.Round(a.Amount*100)), g.CurrentAmount.Currency)
						if err := app.PutGoal(g); err != nil {
							return "Couldn't update the goal: " + err.Error()
						}
						return fmt.Sprintf("Added %s to “%s” — now %s of %s.%s", fmtMoney(money.New(int64(math.Round(a.Amount*100)), base)), g.Name, fmtMoney(g.CurrentAmount), fmtMoney(g.TargetAmount), openLink("/goals", g.ID))
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
				return fmt.Sprintf("Create a %s account “%s”%s", a.Class, strings.TrimSpace(a.Name), ifStr(a.Balance != 0, " ("+fmtMoney(money.New(majorToMinor(a.Balance, base), base))+")", ""))
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
				minor := majorToMinor(a.Balance, base)
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
						acc.CreditLimit = money.New(majorToMinor(a.CreditLimit, base), base)
					}
					if a.MinPayment > 0 {
						acc.MinPayment = money.New(majorToMinor(a.MinPayment, base), base)
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
				return fmt.Sprintf("Transfer %s from %s to %s", fmtMoney(money.New(majorToMinor(a.Amount, base), base)), a.From, a.To)
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
				fromMinor := majorToMinor(a.Amount, from.Currency)
				fromMoney := money.New(fromMinor, from.Currency)
				toMoney := money.New(majorToMinor(a.Amount, to.Currency), to.Currency)
				if conv, err := rates.Convert(fromMoney.Abs(), to.Currency); err == nil {
					toMoney = conv // honor FX across currencies
				}
				out := domain.Transaction{ID: id.New(), AccountID: from.ID, Amount: money.New(-fromMinor, from.Currency), TransferAccountID: to.ID, Date: when, Payee: to.Name, Desc: "Transfer to " + to.Name}
				in := domain.Transaction{ID: id.New(), AccountID: to.ID, Amount: toMoney, TransferAccountID: from.ID, Date: when, Payee: from.Name, Desc: "Transfer from " + from.Name}
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
				return fmt.Sprintf("Set %s balance to %s", a.Account, fmtMoney(money.New(majorToMinor(a.Balance, base), base)))
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
				target := majorToMinor(a.Balance, acc.Currency)
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
	}
}

// majorToMinor converts a major-unit amount (e.g. dollars) to minor units for a
// currency, honoring its decimal places.
func majorToMinor(major float64, cur string) int64 {
	return int64(math.Round(major * math.Pow10(currency.Decimals(cur))))
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
