// SPDX-License-Identifier: MIT

// Package toolcite names the work behind an assistant answer.
//
// When the assistant says "you spent $312 on groceries in August", that figure came
// from a specific tool run over a specific slice of the data. The conversation used
// to discard that: the tool results were fed to the model and thrown away, so the
// number arrived with nothing behind it and had to be taken on trust. This package
// turns a tool call into a citable Source — what was consulted, over what scope —
// and the caller attaches the tool's own result as the evidence.
//
// It is pure and table-driven, like internal/toolperm: a tool with no entry still
// cites, just in its own name rather than in a friendlier one.
package toolcite

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Source is one thing an answer was computed from.
type Source struct {
	// Tool is the tool's name, kept for the audit trail and for tests.
	Tool string
	// Label is what the tool did, in plain English ("Spending by category").
	Label string
	// Scope narrows it to the slice actually consulted ("August 2026"). Empty when
	// the call had no narrowing arguments.
	Scope string
	// Evidence is the tool's own result — the rows and totals the model was handed.
	// This is the citation's substance: everything else is a label for it.
	Evidence string
}

// Title renders the source's one-line heading.
func (s Source) Title() string {
	if strings.TrimSpace(s.Scope) == "" {
		return s.Label
	}
	return s.Label + " · " + s.Scope
}

// citable describes how one tool presents itself in a citation: a friendly label,
// and which of its arguments narrow the question.
type citable struct {
	label string
	// scopeKeys are argument names read in order and joined with " · ". A key may
	// carry a format ("month=%s"); a bare key prints its value.
	scopeKeys []string
}

// tools is the citation table. Only the tools that produce figures or rows need an
// entry — those are the answers a person wants to check.
var tools = map[string]citable{
	"financial_summary":               {label: "Your headline figures"},
	"spending_by_category":            {label: "Spending by category", scopeKeys: []string{"category", "period", "from", "to"}},
	"spending_breakdown":              {label: "Spending breakdown", scopeKeys: []string{"period", "from", "to", "limit"}},
	"account_balances":                {label: "Account balances"},
	"list_transactions":               {label: "Transactions", scopeKeys: []string{"category", "period", "limit"}},
	"list_uncategorized_transactions": {label: "Uncategorized transactions"},
	"list_budgets":                    {label: "Your budgets", scopeKeys: []string{"period", "from", "to"}},
	"list_goals":                      {label: "Your goals", scopeKeys: []string{"goal_id", "include_archived", "offset", "limit"}},
	"list_recurring":                  {label: "Recurring bills and subscriptions", scopeKeys: []string{"recurring_id", "account_id", "include_paused", "from", "before", "offset", "definitions_offset", "limit"}},
	"list_tasks":                      {label: "Your to-dos"},
	"list_members":                    {label: "Household members"},
	"list_categories":                 {label: "Your categories"},
	"find_duplicate_transactions":     {label: "Possible duplicate transactions"},
	"list_report_sections":            {label: "The report's sections"},
	"report_section":                  {label: "A section of your report", scopeKeys: []string{"section", "period", "from", "to", "field"}},
	"trace_report_row":                {label: "The transactions behind a report figure", scopeKeys: []string{"section", "row", "period", "from", "to", "field", "limit"}},
	"find_transactions":               {label: "Transactions", scopeKeys: []string{"contains", "category", "account", "member", "tag", "kind", "uncategorized", "min_amount", "max_amount", "period", "from", "to", "slices", "sort", "limit"}},
	"check_affordability":             {label: "Affordability check", scopeKeys: []string{"amount", "months", "reserve"}},
	"evaluate_formula":                {label: "A formula you defined", scopeKeys: []string{"expression", "snapshot"}},
	"list_formula_metrics":            {label: "Your formula metrics"},
	"gather_tax_records":              {label: "Tax-season gather", scopeKeys: []string{"year"}},
	"search_documents":                {label: "Your documents", scopeKeys: []string{"question"}},
	"web_search":                      {label: "A web search", scopeKeys: []string{"query"}},
	"fetch_webpage":                   {label: "A web page", scopeKeys: []string{"url"}},
	"list_remembered_facts":           {label: "What the assistant remembers"},
	"parse_rapid_capture":             {label: "The text you pasted"},
	"data_manifest":                   {label: "Your local data catalog", scopeKeys: []string{"collection"}},
	"query_data":                      {label: "Local records", scopeKeys: []string{"collection", "ids", "equals", "search", "dateField", "from", "through", "offset", "limit", "snapshot"}},
	"read_record":                     {label: "A local record", scopeKeys: []string{"collection", "id", "field", "offset", "limit", "snapshot"}},
	"get_account_details":             {label: "Account details", scopeKeys: []string{"account_id"}},
	"project_account":                 {label: "Account projection", scopeKeys: []string{"account_id", "horizon_days", "as_of"}},
	"goal_funding_plan":               {label: "Goal funding plan", scopeKeys: []string{"goal_id", "offset", "limit"}},
	"list_goal_reservations":          {label: "Goal reservations", scopeKeys: []string{"account_id", "offset", "limit"}},
}

// For returns the citation for one tool call. Evidence is left to the caller,
// which is the only place the tool's result exists.
func For(tool string, args json.RawMessage) Source {
	tool = strings.TrimSpace(tool)
	c, ok := tools[tool]
	if !ok {
		// An untabled tool still cites — under its own name, humanised — rather
		// than vanishing from the list. A missing citation reads as "the assistant
		// made this up"; an awkwardly-named one only reads as untidy.
		return Source{Tool: tool, Label: humanise(tool)}
	}
	return Source{Tool: tool, Label: c.label, Scope: scopeOf(args, c.scopeKeys)}
}

// scopeOf joins the named arguments into one scope phrase, skipping any that the
// call did not supply. The order of keys is the order they read best in.
func scopeOf(args json.RawMessage, keys []string) string {
	if len(keys) == 0 || len(args) == 0 {
		return ""
	}
	var decoded map[string]any
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return ""
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if v := valueOf(decoded[key], key); v != "" {
			parts = append(parts, key+"="+v)
		}
	}
	return strings.Join(parts, " · ")
}

// valueOf renders one JSON argument without rounding its numeric lexeme.
func valueOf(v any, key string) string {
	switch t := v.(type) {
	case string:
		if key == "snapshot" && strings.TrimSpace(t) != "" {
			return "continuation snapshot"
		}
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "yes"
		}
		return ""
	case []any, map[string]any:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return ""
	}
}

// humanise turns a snake_case tool name into a readable label ("list_transactions"
// → "List transactions").
func humanise(tool string) string {
	if tool == "" {
		return "A lookup"
	}
	words := strings.Split(tool, "_")
	out := strings.Join(words, " ")
	return strings.ToUpper(out[:1]) + out[1:]
}

// Numeric reports whether a reply makes a claim worth checking — that is, whether
// it contains a figure. A citation panel on an answer with no numbers in it is
// clutter; on one with numbers it is the difference between a claim and a result.
func Numeric(reply string) bool {
	for _, r := range reply {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}
