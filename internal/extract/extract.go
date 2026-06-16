// Package extract turns an AI vision model's free-form JSON reply into tidy,
// reviewable transaction rows. Models are inconsistent — a top-level array or an
// object wrapper, amounts as numbers or strings, varied field names, sometimes
// wrapped in a ```json code fence — so parsing is deliberately tolerant. The
// result is intentionally slim (strings only): the UI maps rows to real
// transactions against a chosen account at import time.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package extract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Row is one extracted transaction candidate, before it is mapped to a real
// transaction. Fields are strings (as the user will review/edit them).
type Row struct {
	Date        string
	Description string
	Amount      string
	Category    string
}

// ParseRows reads a model reply and returns the extracted rows. It accepts a bare
// JSON array, or an object with a transactions/rows/items/data array, optionally
// wrapped in a Markdown code fence. Rows with neither a description nor an amount
// are skipped. An unreadable reply is an error.
func ParseRows(reply string) ([]Row, error) {
	s := stripFence(strings.TrimSpace(reply))

	var list []map[string]any
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		// Maybe it's an object wrapping the array under a common key.
		var obj map[string]json.RawMessage
		if err2 := json.Unmarshal([]byte(s), &obj); err2 != nil {
			return nil, fmt.Errorf("extract: could not read the response as JSON: %w", err)
		}
		found := false
		for _, key := range []string{"transactions", "rows", "items", "data", "results"} {
			if v, ok := obj[key]; ok {
				if json.Unmarshal(v, &list) == nil {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, errors.New("extract: no transaction list found in the response")
		}
	}

	rows := make([]Row, 0, len(list))
	for _, m := range list {
		r := Row{
			Date:        firstString(m, "date", "Date"),
			Description: firstString(m, "description", "desc", "Description", "merchant", "payee", "name"),
			Category:    firstString(m, "category", "Category"),
			Amount:      amountString(m, "amount", "Amount", "value", "total"),
		}
		if r.Description == "" && r.Amount == "" {
			continue
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// stripFence removes a leading/trailing Markdown code fence (```json … ```), which
// models often add despite being told not to.
func stripFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// firstString returns the first key whose value is a non-empty string.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// amountString returns the first amount key coerced to a clean numeric string
// (JSON numbers arrive as float64; strings pass through trimmed).
func amountString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		case string:
			if t := strings.TrimSpace(x); t != "" {
				return t
			}
		}
	}
	return ""
}
