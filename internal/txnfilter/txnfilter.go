// Package txnfilter holds the transaction list's filter/sort criteria and the
// pure logic that applies them. Keeping it platform-free means the filtering —
// a core behavior — is unit-tested on native Go, while the wasm screen and the
// localStorage atom just hold and persist a Criteria value.
package txnfilter

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Criteria is the transaction list's filter and sort selection. It persists to
// localStorage (hence the JSON tags); Cleared is "", "yes", or "no".
type Criteria struct {
	Text     string `json:"text,omitempty"`
	Account  string `json:"account,omitempty"`
	Category string `json:"category,omitempty"`
	Member   string `json:"member,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Cleared  string `json:"cleared,omitempty"`
}

// Normalize fills defaults (sort defaults to newest-first by date).
func (c Criteria) Normalize() Criteria {
	if c.Sort == "" {
		c.Sort = "date"
	}
	return c
}

// Apply returns the transactions matching c, sorted per c.Sort (newest-first by
// date by default). The input slice is not mutated.
func Apply(txns []domain.Transaction, c Criteria) []domain.Transaction {
	c = c.Normalize()
	sorted := append([]domain.Transaction(nil), txns...)
	// Newest-first, breaking ties on ID so equal-dated rows order deterministically.
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].Date.Equal(sorted[j].Date) {
			return sorted[i].Date.After(sorted[j].Date)
		}
		return sorted[i].ID < sorted[j].ID
	})

	ft := strings.ToLower(strings.TrimSpace(c.Text))
	var fromT, toT time.Time
	if s := strings.TrimSpace(c.From); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			fromT = d
		}
	}
	if s := strings.TrimSpace(c.To); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			toT = d
		}
	}

	out := make([]domain.Transaction, 0, len(sorted))
	for _, t := range sorted {
		switch {
		case c.Account != "" && t.AccountID != c.Account:
		case c.Category != "" && t.CategoryID != c.Category:
		case c.Member != "" && t.MemberID != c.Member:
		case !fromT.IsZero() && t.Date.Before(fromT):
		case !toT.IsZero() && t.Date.After(toT):
		case ft != "" && !matchText(t, ft):
		case c.Cleared == "yes" && !t.Cleared:
		case c.Cleared == "no" && t.Cleared:
		default:
			out = append(out, t)
		}
	}

	// Secondary sorts break ties on ID so the order is deterministic; the default
	// "date" sort is already applied above.
	switch c.Sort {
	case "amount":
		sort.Slice(out, func(i, j int) bool {
			if ai, aj := AbsAmount(out[i]), AbsAmount(out[j]); ai != aj {
				return ai > aj
			}
			return out[i].ID < out[j].ID
		})
	case "payee":
		sort.Slice(out, func(i, j int) bool {
			di, dj := strings.ToLower(out[i].Desc), strings.ToLower(out[j].Desc)
			if di != dj {
				return di < dj
			}
			return out[i].ID < out[j].ID
		})
	}
	return out
}

// AbsAmount returns the absolute minor-unit amount of a transaction (for sorting
// by size regardless of income/expense sign).
func AbsAmount(t domain.Transaction) int64 {
	if a := t.Amount.Amount; a < 0 {
		return -a
	}
	return t.Amount.Amount
}

// matchText reports whether the (already-lowercased) query appears in a
// transaction's description or any of its tags.
func matchText(t domain.Transaction, q string) bool {
	if strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}
