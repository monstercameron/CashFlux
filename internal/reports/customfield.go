// SPDX-License-Identifier: MIT

package reports

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// CustomFieldSpend is the total expense for one custom-field value over the
// reporting period, in base-currency minor units. Value is the field's display
// string after normalisation; an empty Value means the field was absent or nil
// on the transaction (the caller labels this bucket, e.g. "(no value)").
type CustomFieldSpend struct {
	Value  string
	Amount int64
}

// normaliseCustomValue converts a raw Custom-map value to a stable, display-safe
// string for grouping and CSV export. Rules:
//
//   - bool  → "Yes" or "No"
//   - number (float64 / json.Number) → decimal with trailing-zero stripping,
//     e.g. 3.0→"3", 3.14→"3.14". This avoids float noise (never "3.0000001").
//   - string / date → as-is
//   - nil / missing → ""
func normaliseCustomValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case float64:
		return trimFloat(val)
	case json.Number:
		// JSON decoded with UseNumber; keep the original string if it's clean,
		// but run through float to normalise any "-0" / scientific notation.
		if f, err := val.Float64(); err == nil {
			return trimFloat(f)
		}
		return val.String()
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// trimFloat formats f as a decimal string with no trailing zeros.
func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 10, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// ByCustomField totals expenses by the normalised value of Custom[fieldKey] over
// the half-open period [start, end) in the base currency, largest first (ties
// broken by Value for determinism). Transfers and income are excluded, matching
// the rest of the reporting core. Transactions where the field is absent or nil
// are grouped under the empty-string Value; the caller labels that bucket.
func ByCustomField(txns []domain.Transaction, fieldKey string, start, end time.Time, rates currency.Rates) ([]CustomFieldSpend, error) {
	totals := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		key := normaliseCustomValue(t.Custom[fieldKey])
		totals[key] += conv.Abs().Amount
	}

	out := make([]CustomFieldSpend, 0, len(totals))
	for val, amt := range totals {
		out = append(out, CustomFieldSpend{Value: val, Amount: amt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Value < out[j].Value
	})
	return out, nil
}
