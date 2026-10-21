// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantloop"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/toolperm"
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

type transferToolArgs struct {
	From        string          `json:"from_account"`
	To          string          `json:"to_account"`
	Amount      json.RawMessage `json:"amount"`
	Received    json.RawMessage `json:"received_amount"`
	OperationID string          `json:"operation_id"`
	Date        string          `json:"date"`
}

type transferToolBuild struct {
	params        appstate.TransferParams
	from, to      domain.Account
	converted     int64
	hasConversion bool
}

func parseTransferMinor(raw json.RawMessage, code string, optional bool) (int64, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		if optional {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("amount is required")
	}
	minor, err := money.ParseMinor(string(raw), currency.Decimals(code))
	if err != nil {
		return 0, false, err
	}
	if minor <= 0 {
		return 0, false, fmt.Errorf("amount must be greater than zero")
	}
	return minor, true, nil
}

func validTransferOperationID(s string) bool {
	if len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func buildAssistantTransfer(raw json.RawMessage, from, to domain.Account, rates currency.Rates, now time.Time, desc string) (transferToolBuild, error) {
	var a transferToolArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return transferToolBuild{}, fmt.Errorf("read transfer details: %w", err)
	}
	if a.OperationID != "" && !validTransferOperationID(a.OperationID) {
		return transferToolBuild{}, fmt.Errorf("operation id must be at most 64 letters, numbers, hyphens, or underscores")
	}
	amount, _, err := parseTransferMinor(a.Amount, from.Currency, false)
	if err != nil {
		return transferToolBuild{}, fmt.Errorf("invalid source amount: %w", err)
	}
	received, hasReceived, err := parseTransferMinor(a.Received, to.Currency, true)
	if err != nil {
		return transferToolBuild{}, fmt.Errorf("invalid received amount: %w", err)
	}
	when := now
	if a.Date != "" {
		parsed, parseErr := dateutil.ParseDate(a.Date)
		if parseErr != nil {
			return transferToolBuild{}, fmt.Errorf("invalid transfer date: %w", parseErr)
		}
		when = parsed
	}
	converted, conversionErr := rates.Convert(money.New(amount, from.Currency), to.Currency)
	build := transferToolBuild{from: from, to: to, converted: converted.Amount, hasConversion: conversionErr == nil}
	if hasReceived {
		build.params.ReceivedMinor = received
	} else if conversionErr == nil {
		build.params.ReceivedMinor = converted.Amount
	}
	opID := a.OperationID
	if opID == "" {
		opID = generatedAssistantTransferOperationID(raw, from, to, rates, now, desc)
	}
	build.params.OperationID = opID
	build.params.FromAccountID = from.ID
	build.params.ToAccountID = to.ID
	build.params.AmountMinor = amount
	build.params.Date = when
	build.params.Desc = desc
	build.params.Source = domain.TxnSourceAssistant
	if from.ID == to.ID {
		return transferToolBuild{}, fmt.Errorf("the two accounts must be different")
	}
	return build, nil
}

func generatedAssistantTransferOperationID(raw json.RawMessage, from, to domain.Account, rates currency.Rates, now time.Time, desc string) string {
	key, ok := assistantTransferBuildCacheKey(raw, from, to, rates, now, desc)
	if !ok {
		key = fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s", string(raw), from.ID, to.ID, rates.Base, now.UnixNano(), desc)
	}
	sum := sha256.Sum256([]byte(key))
	return "transfer_" + hex.EncodeToString(sum[:24])
}

func assistantTransferPreview(build transferToolBuild) string {
	fromAmount := fmtMoney(money.New(build.params.AmountMinor, build.from.Currency))
	toAmount := ""
	if build.params.ReceivedMinor > 0 {
		toAmount = fmtMoney(money.New(build.params.ReceivedMinor, build.to.Currency))
	}
	preview := fmt.Sprintf("Transfer %s (%s) from %s to %s (%s). Destination amount to record: %s",
		fromAmount, build.from.Currency, build.from.Name, build.to.Name, build.to.Currency, toAmount)
	if build.params.ReceivedMinor == 0 && !build.hasConversion {
		return preview + ". No exchange rate — this transfer will be refused."
	}
	if build.hasConversion && build.params.ReceivedMinor > 0 && build.converted > 0 {
		pctExact := math.Abs(float64(build.params.ReceivedMinor)/float64(build.converted)-1) * 100
		if pctExact <= 5 {
			return preview
		}
		pct := math.Round(pctExact)
		preview += fmt.Sprintf("\nWARNING: destination amount differs from today's rate by %.0f%%.", pct)
	}
	return preview
}

func assistantTransferBuildCacheKey(raw json.RawMessage, from, to domain.Account, rates currency.Rates, now time.Time, desc string) (string, bool) {
	key, err := json.Marshal(struct {
		Args  string
		From  domain.Account
		To    domain.Account
		Rates currency.Rates
		Now   time.Time
		Desc  string
	}{string(raw), from, to, rates, now, desc})
	if err != nil {
		return "", false
	}
	return string(key), true
}

// approvalReq is a pending mutating tool awaiting the user's yes/no in the chat.
// preview is the tool's own plain-English sentence; perm is the same call read
// structurally (what it reads, what it changes, whether it can be undone) so the
// card can be scanned rather than parsed (C388).
type approvalReq struct {
	tool     string
	preview  string
	perm     toolperm.Permission
	resp     chan bool
	decision *assistantloop.Approval
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
func buildChatTools(app *appstate.App, base string, rates currency.Rates, tier aicontext.ConversationTier, currentScope ...func() scope.ReportScope) []chatTool {
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
			spec: ai.FunctionTool("list_tasks", "List the user's to-do tasks (title, status, priority, due date).", json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				ts := app.Tasks()
				if len(ts) == 0 {
					return "No tasks."
				}
				var b strings.Builder
				for _, t := range ts {
					line := fmt.Sprintf("[%s] %s (ID %s, %s priority)", string(t.Status), t.Title, t.ID, string(t.Priority))
					if !t.Due.IsZero() {
						line += ", due " + t.Due.Format("Jan 2")
					}
					b.WriteString(line + "\n")
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
						acc = acc.WithRateAPR(a.APR)
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
	}
	// AG series — the assistant tool groups built in the ag_*.go / chat_agent_*.go
	// sidecar files (registered here so those files never touched this one):
	// trust (remember facts), the auditor, natural-language rule/workflow authoring,
	// web-grounded benchmarks, rapid capture, tax gather, and document Q&A.
	tools = append(tools, agToolsTasks(app, base, rates)...)
	tools = append(tools, agToolsData(app)...)
	tools = append(tools, agToolsPlanning(app, base, rates)...)
	tools = append(tools, agToolsFlags(app, base, rates)...)
	tools = append(tools, agToolsFinance(app, base, rates, currentScope...)...)
	tools = append(tools, agToolsGoalWrites(app, base, rates)...)
	tools = append(tools, agToolsTransactionWrites(app)...)
	tools = append(tools, agToolsFormula(app, base, rates)...)
	tools = append(tools, agToolsAccountWrites(app)...)
	tools = append(tools, agToolsTrust(app, base, rates)...)
	tools = append(tools, agToolsAuditor(app, base, rates)...)
	tools = append(tools, agToolsAuthoring(app, base, rates)...)
	tools = append(tools, agToolsBenchmark(app, base, rates)...)
	tools = append(tools, agToolsCapture(app, base, rates)...)
	tools = append(tools, agToolsTax(app, base, rates)...)
	tools = append(tools, agToolsDocQA(app, base, rates)...)
	tools = append(tools, agToolsCoach(app, base, rates)...)
	tools = append(tools, agToolsImage(app, base, rates)...)
	tools = append(tools, agToolsSandbox(app, base, rates)...)
	tools = append(tools, agToolsSpotlight(app, base, rates)...)
	tools = append(tools, agToolsReference(app, base, rates)...)
	tools = append(tools, agToolsReports(app, base, rates, tier, currentScope...)...)
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
		seen[dedupe.Key(t)] = true
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
