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
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// chatTool pairs an OpenAI tool spec (what the model sees) with its handler (what
// runs locally when the model calls it). Handlers return a short plain-text result
// fed back to the model; they read only aggregates from the user's own data.
type chatTool struct {
	spec ai.Tool
	run  func(args json.RawMessage) string
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
const defaultChatSystemPrompt = `You are CashFlux, a friendly, concise personal-finance assistant built into the user's own budgeting app. You can call tools to read the user's real, on-device figures — ALWAYS use a tool for any specific figure (a category total, an account balance, net worth, affordability) instead of guessing. If unsure which category a question means, call list_categories first. You may also COMBINE your own general knowledge (tax brackets, rates, formulas) with the calculator tool and the user's figures to ESTIMATE things the data doesn't directly contain (e.g. taxes) — never refuse; compute a clear estimate and state your assumptions. Use web_search for current or external facts (tax brackets, prices, rates). Use the calculator tool for any arithmetic. Never invent the user's own numbers. Answer in plain English as short Markdown. The user's money is private and never leaves their device except for these requests.`

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
	}
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
