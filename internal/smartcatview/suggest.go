// SPDX-License-Identifier: MIT

package smartcatview

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/txnreview"
)

// CategoryByID resolves an existing nonempty identifier, with first occurrence winning.
func CategoryByID(cats []domain.Category, id string) (domain.Category, bool) {
	if id != "" {
		for _, c := range cats {
			if c.ID == id {
				return c, true
			}
		}
	}
	return domain.Category{}, false
}

// CategoryKind treats a zero amount as an expense, matching the category picker.
func CategoryKind(txn domain.Transaction) domain.CategoryKind {
	if txn.Amount.IsPositive() {
		return domain.KindIncome
	}
	return domain.KindExpense
}

// MatchesDirection prevents income/expense category mismatches.
func MatchesDirection(txn domain.Transaction, id string, cats []domain.Category) bool {
	c, ok := CategoryByID(cats, id)
	return ok && c.Kind == CategoryKind(txn)
}

// ChipVisible requires an uncategorized non-transfer and at least medium confidence.
func ChipVisible(txn domain.Transaction, s catsuggest.Suggestion, found bool, cats []domain.Category) bool {
	return EligibleTransaction(txn, true) && found && s.Confidence >= catsuggest.ConfMedium && MatchesDirection(txn, s.CategoryID, cats)
}

// IndexTransactions indexes transactions in their input order; the last duplicate wins.
func IndexTransactions(txns []domain.Transaction) map[string]domain.Transaction {
	out := make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		out[t.ID] = t
	}
	return out
}

// SeedCategory retains a filed category ahead of a local suggestion.
func SeedCategory(txn domain.Transaction, s catsuggest.Suggestion, has bool) string {
	if txn.CategoryID != "" {
		return txn.CategoryID
	}
	if has {
		return s.CategoryID
	}
	return ""
}

func merchantPeer(txn, t domain.Transaction, key string) bool {
	return t.ID != txn.ID && t.CategoryID == "" && !t.IsTransfer() && reviewqueue.MerchantKey(t) == key
}

// MerchantOthers counts other uncategorized non-transfer charges for the same merchant.
func MerchantOthers(txn domain.Transaction, txns []domain.Transaction) int {
	key := reviewqueue.MerchantKey(txn)
	n := 0
	if key != "" {
		for _, t := range txns {
			if merchantPeer(txn, t, key) {
				n++
			}
		}
	}
	return n
}

// MerchantWrites confirms the current charge plus matching uncategorized merchant charges.
func MerchantWrites(txn domain.Transaction, txns []domain.Transaction, categoryID string) ([]domain.Transaction, []string) {
	var writes []domain.Transaction
	var ids []string
	key := reviewqueue.MerchantKey(txn)
	for _, t := range txns {
		if t.IsTransfer() || (t.ID != txn.ID && (key == "" || !merchantPeer(txn, t, key))) {
			continue
		}
		writes = append(writes, txnreview.Confirm(t, categoryID))
		ids = append(ids, t.ID)
	}
	return writes, ids
}

// Option is a plain picker option, independent of the rendering framework.
type Option struct{ Value, Label string }

// CategoryOptions builds the placeholder and qualified options in category-tree order.
func CategoryOptions(cats []domain.Category, kind domain.CategoryKind, placeholder string) []Option {
	out := []Option{{Label: placeholder}}
	for _, f := range categorytree.Flatten(cats) {
		if f.Category.Kind == kind {
			out = append(out, Option{Value: f.Category.ID, Label: catname.Label(cats, f.Category)})
		}
	}
	return out
}

// TransactionLabel prefers a trimmed payee and otherwise uses the description.
func TransactionLabel(txn domain.Transaction) string {
	if s := strings.TrimSpace(txn.Payee); s != "" {
		return s
	}
	return strings.TrimSpace(txn.Desc)
}

// CategorizeRequest prepares a single-charge AI request with the real catalog.
func CategorizeRequest(txn domain.Transaction, cats []domain.Category) smartai.Request {
	return smartai.Categorize(TransactionLabel(txn)+" "+formatMoney(txn.Amount), Catalog(cats).Prompt())
}

// CategoryAnswer resolves a model response and returns the exact source/unmatched translation key.
func CategoryAnswer(text string, cats []domain.Category) (id, key, answer string) {
	answer = strings.TrimSpace(text)
	if hit, ok := Catalog(cats).Lookup(answer); ok {
		return hit.ID, "catSuggest.aiSource", answer
	}
	return "", "catSuggest.aiUnmatched", answer
}

// FiledLabelKey chooses the undo label for the actual number of writes.
func FiledLabelKey(n int) string {
	if n > 1 {
		return "catSuggest.filedBatchUndo"
	}
	return "catSuggest.filedUndo"
}

// RevisionCache caches a derived value by source identity and both data revisions.
// Values may contain slices: their contents never serve as memo dependencies.
type RevisionCache[K comparable, V any] struct {
	identity      K
	revision      int
	storeRevision uint64
	value         V
	valid         bool
}

// Get rebuilds only after the identity or either revision changes.
func (c *RevisionCache[K, V]) Get(identity K, revision int, storeRevision uint64, build func() V) V {
	if !c.valid || c.identity != identity || c.revision != revision || c.storeRevision != storeRevision {
		c.identity = identity
		c.revision = revision
		c.storeRevision = storeRevision
		c.value = build()
		c.valid = true
	}
	return c.value
}

// CategoryAskKey selects the single-charge AI action's translation key.
func CategoryAskKey(loading bool) string {
	if loading {
		return "catSuggest.asking"
	}
	return "catSuggest.suggestAI"
}

// TransactionAmount formats an individual charge without converting currencies.
func TransactionAmount(txn domain.Transaction) string { return formatMoney(txn.Amount) }

// EligibleTransaction reports whether the inline chip may resolve an unfiled charge.
func EligibleTransaction(txn domain.Transaction, found bool) bool {
	return found && txn.CategoryID == "" && !txn.IsTransfer()
}

// Memo returns a cached value or resolves it once; a nil cache simply skips storage.
func Memo[K comparable, V any](values map[K]V, key K, resolve func() V) V {
	if cached, ok := values[key]; ok {
		return cached
	}
	value := resolve()
	if values != nil {
		values[key] = value
	}
	return value
}

// FiledLabel formats the undo description with the original singular/batch arguments.
func FiledLabel(n int, name string, translate func(string, ...any) string) string {
	if n > 1 {
		return translate(FiledLabelKey(n), n, name)
	}
	return translate(FiledLabelKey(n), name)
}
