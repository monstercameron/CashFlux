// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via
//   tools = append(tools, agToolsCapture(app, base, rates)...)
//   tools = append(tools, agToolsTax(app, base, rates)...)
//   tools = append(tools, agToolsDocQA(app, base, rates)...)
// in buildChatTools (internal/screens/chat_agent.go), after the base tool slice.
//
// Agent D tools: rapid capture (AG11), tax-season gather (AG14), and grounded
// document Q&A (AG13). All three are READ tools — they parse/gather/retrieve and
// return a plain-text scaffold the model presents; the actual writes flow through
// the existing preview-approved add_transaction / add_task tools.

package screens

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/docqa"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rapidcapture"
	"github.com/monstercameron/CashFlux/internal/taxgather"
)

// agToolsCapture returns the rapid-capture tool (AG11): it parses a free-typed or
// dictated capture line into draft transactions for bulk review, flagging splits
// and likely duplicates, so the model can present them and add each via
// add_transaction on approval.
func agToolsCapture(app *appstate.App, base string, rates currency.Rates) []chatTool {
	fmtM := func(minor int64) string { return fmtMoney(money.New(minor, base)) }
	return []chatTool{
		{
			spec: ai.FunctionTool("parse_rapid_capture",
				"Parse a quick capture line the user typed or dictated — e.g. 'coffee 4.50, gas 38, costco 122 split with priya' — into DRAFT transactions for bulk review. Returns one draft per item with its label, amount, and a split flag, and badges any that look like a duplicate of an existing entry. Present the drafts to the user, then record each with add_transaction on approval (a split draft is shared — halve it or note the counterpart as the user prefers). Amounts are expenses unless clearly income.",
				json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","description":"the raw capture line, items separated by commas or newlines"}},"required":["text"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Text) == "" {
					return "Give me a capture line, e.g. 'coffee 4.50, gas 38'."
				}
				drafts := rapidcapture.Parse(a.Text)
				if len(drafts) == 0 {
					return "I couldn't find any amounts in that. Try 'label amount' pairs, e.g. 'lunch 12, parking 8'."
				}
				txns := app.Transactions()
				defAccount := ""
				for _, ac := range app.Accounts() {
					if !ac.Archived {
						defAccount = ac.Name
						break
					}
				}
				var b strings.Builder
				fmt.Fprintf(&b, "%d draft %s to review:\n", len(drafts), pluralWord(len(drafts), "transaction"))
				for i, d := range drafts {
					minor := currency.MinorFromMajor(atof(d.MajorString), base)
					label := d.Label
					if label == "" {
						label = "(no label)"
					}
					line := fmt.Sprintf("%d. %s — %s", i+1, label, fmtM(minor))
					if d.Split {
						if d.SplitWith != "" {
							line += " · split with " + d.SplitWith
						} else {
							line += " · split"
						}
					}
					if captureLooksDuplicate(txns, label, minor) {
						line += " · ⚠ possible duplicate of an existing entry"
					}
					b.WriteString(line + "\n")
				}
				if defAccount != "" {
					fmt.Fprintf(&b, "\nDefault account: %s. Confirm the account and any category, then I'll add each with add_transaction.", defAccount)
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
	}
}

// captureLooksDuplicate reports whether an existing non-transfer transaction has
// the same absolute amount and a payee/description that contains the draft label —
// a cheap "you may have already logged this" badge for the review list.
func captureLooksDuplicate(txns []domain.Transaction, label string, minor int64) bool {
	q := strings.ToLower(strings.TrimSpace(label))
	if q == "" {
		return false
	}
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if t.Amount.Abs().Amount == absMinor(minor) && strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
			return true
		}
	}
	return false
}

// agToolsTax returns the tax-season gather tool (AG14): it sweeps a tax year for
// deductible spending, charitable donations, and interest paid, and lists the gaps
// (entries missing a receipt). It gathers the user's own records for their own
// filing — it gives no tax advice.
func agToolsTax(app *appstate.App, base string, rates currency.Rates) []chatTool {
	return []chatTool{
		{
			spec: ai.FunctionTool("gather_tax_records",
				"Gather the user's records for a tax year: totals for deductible-flagged categories, charitable donations, and interest paid, plus a list of gaps (deductible/donation entries with no receipt attached). Returns a plain summary and a CSV block the user can save. It GATHERS the user's own records — it is not tax advice and makes no completeness or deductibility claim; the user (or their preparer) decides what actually applies.",
				json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","description":"tax year, e.g. 2025; defaults to the previous calendar year"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Year int `json:"year"`
				}
				_ = json.Unmarshal(raw, &a)
				year := a.Year
				if year == 0 {
					year = time.Now().Year() - 1
				}
				start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
				cats := app.Categories()
				catName := make(map[string]string, len(cats))
				for _, c := range cats {
					catName[c.ID] = c.Name
				}
				s, err := taxgather.Gather(app.Transactions(), cats, year, start, end, rates)
				if err != nil {
					return "Couldn't gather tax records: " + err.Error()
				}
				fmtM := func(minor int64) string { return fmtMoney(money.New(minor, base)) }
				var b strings.Builder
				fmt.Fprintf(&b, "Tax-year %d gathering (your records, not tax advice — you or your preparer decide what applies):\n", year)
				fmt.Fprintf(&b, "- Deductible-category spending: %s across %s\n", fmtM(s.Deductible.Total), pluralWord(len(s.Deductible.Rows), "category"))
				for _, r := range s.Deductible.Rows {
					n := catName[r.CategoryID]
					if n == "" {
						n = "Uncategorized"
					}
					fmt.Fprintf(&b, "    • %s: %s\n", n, fmtM(r.Amount))
				}
				fmt.Fprintf(&b, "- Charitable donations: %s (%d)\n", fmtM(s.Charitable.Total), s.Charitable.Count)
				fmt.Fprintf(&b, "- Interest paid: %s (%d)\n", fmtM(s.InterestPaid.Total), s.InterestPaid.Count)
				if len(s.Gaps) == 0 {
					b.WriteString("- Gaps: none — every deductible/donation entry has a receipt attached.\n")
				} else {
					fmt.Fprintf(&b, "- Gaps (%d entries missing a receipt):\n", len(s.Gaps))
					for i, g := range s.Gaps {
						if i >= 10 {
							fmt.Fprintf(&b, "    • …and %d more\n", len(s.Gaps)-10)
							break
						}
						fmt.Fprintf(&b, "    • %s — %s on %s (%s)\n", g.Label, fmtM(g.Amount), g.Date.Format("Jan 2"), g.Reason)
					}
					b.WriteString("Offer to file each gap as a to-do (add_task) so it resolves when the receipt is attached.\n")
				}
				name := func(id string) string {
					if n := catName[id]; n != "" {
						return n
					}
					return "Uncategorized"
				}
				amt := func(v int64) string { return strconv.FormatFloat(currency.MajorFromMinor(v, base), 'f', 2, 64) }
				csv := taxgather.GatherCSV(s, name, amt)
				b.WriteString("\nCSV export:\n```\n")
				b.Write(csv)
				b.WriteString("```")
				return b.String()
			},
		},
	}
}

// agToolsDocQA returns the document Q&A tool (AG13): it retrieves the most relevant
// attached artifacts/documents for a question and returns a cited scaffold, or a
// graceful refusal when the corpus doesn't contain the answer.
func agToolsDocQA(app *appstate.App, base string, rates currency.Rates) []chatTool {
	return []chatTool{
		{
			spec: ai.FunctionTool("search_documents",
				"Search the user's attached documents and artifacts (statements, policies, receipts) for the answer to a question like 'what was on the March statement?' or 'when does my insurance renew?'. Returns the most relevant source(s) with their text and a citation link to open each. Ground your answer ONLY in the returned text and cite the source with its Open link; if nothing relevant is returned, tell the user you don't see a document covering it rather than guessing.",
				json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"}},"required":["question"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Question string `json:"question"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Question) == "" {
					return "What would you like me to look up in your documents?"
				}
				corpus := docqa.BuildCorpus(app.Documents(), app.Artifacts())
				if len(corpus) == 0 {
					return "You don't have any documents or attachments yet, so there's nothing to search. Add a statement or receipt first."
				}
				res := docqa.Query(corpus, a.Question, 3)
				if !res.Grounded {
					return "I don't see a document that covers that. I searched your attachments and none matched — I won't guess."
				}
				var b strings.Builder
				b.WriteString("Relevant sources (ground your answer in these and cite the Open link):\n")
				for _, h := range res.Hits {
					text := strings.TrimSpace(h.Source.Text)
					if r := []rune(text); len(r) > 1200 {
						text = string(r[:1200]) + "…"
					}
					fmt.Fprintf(&b, "\n### %s%s\n%s\n", h.Source.Title, openLink(h.Source.Route, h.Source.ID), text)
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
	}
}

// atof parses a cleaned major-unit amount string to float64 (0 on error).
func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// pluralWord returns "word" or "words" for a count (a simple English pluralizer
// that also handles "category" → "categories").
func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	if strings.HasSuffix(word, "y") {
		return word[:len(word)-1] + "ies"
	}
	return word + "s"
}
