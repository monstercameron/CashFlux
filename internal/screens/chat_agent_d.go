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

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
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

const (
	taxProseCategoryLimit = 8
	taxProseLabelRunes    = 120
	taxCSVChunkCharacters = 24000
	taxToolOutputMaxBytes = 32 * 1024
)

type taxCSVRecord struct {
	ID  string `json:"id"`
	CSV string `json:"csv"`
}

type taxGatherCounts struct {
	DeductibleCategories int `json:"deductibleCategories"`
	DonationMatches      int `json:"donationMatches"`
	InterestMatches      int `json:"interestMatches"`
	ReceiptGaps          int `json:"receiptGaps"`
}

type taxCSVChunkOutput struct {
	Year            int                  `json:"year"`
	Counts          taxGatherCounts      `json:"counts"`
	Summary         string               `json:"summary"`
	CSV             assistantdata.Detail `json:"csv"`
	CSVInstructions string               `json:"csvInstructions"`
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
				"Gather the household's retained records for a UTC calendar tax year: checked totals for deductible-flagged category lines, donation/interest keyword matches, and receipt gaps. Uses current stored base currency/FX and explicit refund links; excludes transfers and report-excluded rows. Buckets may overlap and are not additive. Small results include complete exact-decimal CSV. Large CSV is returned in bounded chunks with a snapshot and nextOffset. Continue with the same year and csv_snapshot, setting csv_offset to nextOffset and optionally csv_limit (maximum 24000 characters). Concatenate each csv.jsonText slice, then JSON-decode the concatenated text to recover the exact CSV. A changed snapshot requires restarting at offset 0; never combine chunks from different snapshots. Unavailable inputs or ambiguous partially refunded split allocation are explicit errors. This is record gathering, not tax advice or a completeness/deductibility claim.",
				json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","description":"tax year, e.g. 2025; defaults to the previous calendar year"},"csv_offset":{"type":"integer","minimum":0,"description":"CSV JSON-character offset from the prior response's nextOffset; requires csv_snapshot when greater than zero"},"csv_limit":{"type":"integer","minimum":0,"maximum":24000,"description":"maximum CSV JSON characters in this response; default 24000 for chunked output"},"csv_snapshot":{"type":"string","description":"snapshot from the prior CSV chunk; reuse unchanged for every continuation"}},"additionalProperties":false}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Year        *int   `json:"year"`
					CSVOffset   int    `json:"csv_offset,omitempty"`
					CSVLimit    int    `json:"csv_limit,omitempty"`
					CSVSnapshot string `json:"csv_snapshot,omitempty"`
				}
				if err := assistantDataArgs(raw, &a); err != nil {
					return "Couldn't gather tax records: " + err.Error()
				}
				if app == nil || app.Store() == nil {
					return "Couldn't gather tax records: local dataset is unavailable."
				}
				sourceRevision := app.Rev()
				year := app.ClockNow().UTC().Year() - 1
				if a.Year != nil {
					year = *a.Year
				} else {
					// A pointer distinguishes explicit zero from omission; reject
					// explicit null as well rather than silently choosing a year.
					var supplied map[string]json.RawMessage
					_ = json.Unmarshal(raw, &supplied) // already strictly decoded
					if _, hasYear := supplied["year"]; hasYear {
						return "Couldn't gather tax records: year must be an integer from 1 through 9999."
					}
				}
				if a.CSVOffset < 0 || a.CSVLimit < 0 || a.CSVLimit > taxCSVChunkCharacters {
					return "Couldn't gather tax records: csv_offset must be nonnegative and csv_limit must be 0 through 24000."
				}
				if a.CSVOffset > 0 && a.CSVSnapshot == "" {
					return "Couldn't gather tax records: csv_snapshot is required when continuing beyond offset 0."
				}
				if len(a.CSVSnapshot) > 128 {
					return "Couldn't gather tax records: csv_snapshot is invalid."
				}
				if (a.CSVOffset > 0 || a.CSVSnapshot != "") && a.Year == nil {
					return "Couldn't gather tax records: include the same year when continuing a CSV snapshot."
				}
				start, end, err := taxgather.YearBounds(year)
				if err != nil {
					return "Couldn't gather tax records: " + err.Error()
				}
				settings, err := app.Store().GetSettings()
				if err != nil {
					return "Couldn't gather tax records: financial settings are unavailable."
				}
				currentBase := strings.ToUpper(strings.TrimSpace(settings.BaseCurrency))
				if currentBase == "" {
					currentBase = "USD"
				}
				currentRates := currency.Rates{Base: currentBase, Rates: settings.FXRates}
				cats, err := app.CategoriesChecked()
				if err != nil {
					return "Couldn't gather tax records: categories are unavailable."
				}
				txns, err := app.Store().ListTransactions()
				if err != nil {
					return "Couldn't gather tax records: transactions are unavailable."
				}
				links, err := app.Store().ListTxnLinks()
				if err != nil {
					return "Couldn't gather tax records: refund links are unavailable."
				}
				catName := make(map[string]string, len(cats))
				for _, c := range cats {
					catName[c.ID] = c.Name
				}
				s, err := taxgather.GatherWithLinks(txns, cats, links, year, start, end, currentRates)
				if err != nil {
					return "Couldn't gather tax records: " + err.Error()
				}
				fmtM := func(minor int64) string { return fmtMoney(money.New(minor, currentBase)) }
				var b strings.Builder
				fmt.Fprintf(&b, "Tax-year %d gathering (your records, not tax advice — you or your preparer decide what applies):\n", year)
				fmt.Fprintf(&b, "Household scope; UTC calendar year; amounts in %s using current stored FX. Negative category split lines follow the checked report rules; positive split credits are omitted. Deductible, charitable and interest buckets can overlap and must not be added together.\n", currentBase)
				if !currency.Valid(currentBase) {
					b.WriteString("This currency is not in the registry; CSV amounts use the app's canonical two-decimal fallback assumption.\n")
				}
				fmt.Fprintf(&b, "- Deductible-category spending: %s across %d %s\n", fmtM(s.Deductible.Total), len(s.Deductible.Rows), pluralWord(len(s.Deductible.Rows), "category"))
				for i, r := range s.Deductible.Rows {
					if i >= taxProseCategoryLimit {
						fmt.Fprintf(&b, "    • …and %d more categories; exact full category detail is in the CSV export.\n", len(s.Deductible.Rows)-taxProseCategoryLimit)
						break
					}
					n := catName[r.CategoryID]
					if n == "" {
						n = "Uncategorized"
					}
					fmt.Fprintf(&b, "    • %s: %s\n", taxProseLabel(n), fmtM(r.Amount))
				}
				fmt.Fprintf(&b, "- Charitable donations: %s (%d)\n", fmtM(s.Charitable.Total), s.Charitable.Count)
				fmt.Fprintf(&b, "- Interest paid: %s (%d)\n", fmtM(s.InterestPaid.Total), s.InterestPaid.Count)
				if len(s.Gaps) == 0 {
					b.WriteString("- Receipt gaps: none among the matched retained deductible/donation records. This does not establish filing completeness.\n")
				} else {
					fmt.Fprintf(&b, "- Gaps (%d entries missing a receipt):\n", len(s.Gaps))
					for i, g := range s.Gaps {
						if i >= 10 {
							fmt.Fprintf(&b, "    • …and %d more; exact gap details are in the CSV export.\n", len(s.Gaps)-10)
							break
						}
						fmt.Fprintf(&b, "    • %s — %s on %s (%s)\n", taxProseLabel(g.Label), fmtM(g.Amount), g.Date.Format("Jan 2"), g.Reason)
					}
					b.WriteString("Offer to file each gap as a to-do (add_task) so it resolves when the receipt is attached.\n")
				}
				name := func(id string) string {
					if n := catName[id]; n != "" {
						return n
					}
					return "Uncategorized"
				}
				csv, err := taxgather.GatherCSVExact(s, name, currentBase)
				if err != nil {
					return "Couldn't gather tax records: " + err.Error()
				}
				if app.Rev() != sourceRevision {
					return "Couldn't gather tax records: local data changed during collection; retry the tax-year request."
				}
				summary := b.String()
				chunkRequested := a.CSVOffset > 0 || a.CSVSnapshot != "" || a.CSVLimit > 0
				if !chunkRequested {
					legacy := summary + "\nCSV export:\n```\n" + string(csv) + "```"
					if len(legacy) < taxToolOutputMaxBytes {
						return legacy
					}
				}
				chunkLimit := a.CSVLimit
				if chunkLimit == 0 {
					chunkLimit = taxCSVChunkCharacters
				}
				collectionID := fmt.Sprintf("tax-year-%d", year)
				csvCollection := assistantdata.NewCollection("tax_csv", "id", func() ([]taxCSVRecord, error) {
					return []taxCSVRecord{{ID: collectionID, CSV: string(csv)}}, nil
				})
				// Assistant usage and unrelated household writes can advance the app
				// revision without changing this CSV. Keep the snapshot identity tied to
				// the app/store instance and tax query; ReadRecord fingerprints the full
				// CSV itself so relevant changes still invalidate continuations.
				csvCollection.Revision = fmt.Sprintf("tax-csv:v1;app:%p;store:%p;year:%d;base:%s", app, app.Store(), year, currentBase)
				counts := taxGatherCounts{DeductibleCategories: len(s.Deductible.Rows), DonationMatches: s.Charitable.Count,
					InterestMatches: s.InterestPaid.Count, ReceiptGaps: len(s.Gaps)}
				for {
					detail, detailErr := assistantdata.ReadRecord(csvCollection, assistantdata.DetailRequest{
						Collection: "tax_csv", ID: collectionID, Field: "csv", Offset: a.CSVOffset,
						Limit: chunkLimit, Snapshot: a.CSVSnapshot,
					})
					if detailErr != nil {
						return assistantDataJSON(nil, detailErr)
					}
					output := taxCSVChunkOutput{
						Year: year, Counts: counts, Summary: summary, CSV: detail,
						CSVInstructions: "csv.jsonText is a slice of the JSON serialization of the CSV string. Append slices in offset order through nextOffset, then JSON-decode the concatenated text once to recover the exact CSV. Keep csv.snapshot unchanged; if it changes, restart at offset 0.",
					}
					encoded := assistantDataJSON(output, nil)
					if len(encoded) < taxToolOutputMaxBytes {
						return encoded
					}
					if chunkLimit <= 1 {
						return "Couldn't gather tax records: bounded CSV chunk metadata exceeds the response limit."
					}
					chunkLimit /= 2
				}
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
				"Search all retained extracted document rows and artifact table rows (statements, policies, receipts) for a question. Returns up to three matching sources with the matching row number and a bounded excerpt, plus the total matching source count. Ground your answer ONLY in returned excerpts and cite the source with its Open link. For exact or additional rows, use data_manifest then query_data/read_record. If a read fails, the result reports unavailable; do not claim that no document matched.",
				json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"}},"required":["question"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Question string `json:"question"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Question) == "" {
					return "What would you like me to look up in your documents?"
				}
				docs, err := app.Store().ListDocuments()
				if err != nil {
					return "Document search is unavailable because retained documents could not be read."
				}
				arts, err := app.Store().ListArtifacts()
				if err != nil {
					return "Document search is unavailable because retained attachments could not be read."
				}
				corpus := docqa.BuildCorpus(docs, arts)
				if len(corpus) == 0 {
					return "You don't have any documents or attachments yet, so there's nothing to search. Add a statement or receipt first."
				}
				res := docqa.Query(corpus, a.Question, 3)
				if !res.Grounded {
					return "I don't see a document that covers that. I searched your attachments and none matched — I won't guess."
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Relevant sources (%d matching sources; showing %d. Ground your answer in these excerpts and cite each Open link):\n", res.TotalHits, len(res.Hits))
				for _, h := range res.Hits {
					row := ""
					if h.Row > 0 {
						row = fmt.Sprintf(" — row %d", h.Row)
					}
					if h.ExcerptTruncated {
						row += " (excerpt shortened)"
					}
					fmt.Fprintf(&b, "\n### %s%s%s\n%s\n", h.Source.Title, row, openLink(h.Source.Route, h.Source.ID), h.Excerpt)
				}
				if res.Truncated {
					b.WriteString("\nMore matching sources were omitted from this short result. Use data_manifest, query_data, or read_record to inspect additional persisted rows.")
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

func taxProseLabel(label string) string {
	runes := []rune(label)
	if len(runes) <= taxProseLabelRunes {
		return label
	}
	return string(runes[:taxProseLabelRunes-1]) + "…"
}
