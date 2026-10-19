// SPDX-License-Identifier: MIT

package reviewview

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func clone[K comparable, V any](in map[K]V) map[K]V {
	out := maps.Clone(in)
	if out == nil {
		out = make(map[K]V)
	}
	return out
}

// Toggle returns an independent selection with key toggled, including false entries.
func Toggle(in map[string]bool, key string) map[string]bool {
	out := clone(in)
	out[key] = !out[key]
	return out
}

// Choose returns an independent category map with the explicit choice recorded.
func Choose(in map[string]string, key, category string) map[string]string {
	out := clone(in)
	out[key] = category
	return out
}

// Dismiss returns an independent dismissal map with the transaction suppressed.
func Dismiss(in map[string]bool, key string) map[string]bool {
	out := clone(in)
	out[key] = true
	return out
}

// Category gives explicit edits precedence over local evidence and then AI.
// An explicitly empty edit and conflicting local evidence both suppress AI.
func Category(row Row, manual, proposals map[string]string) string {
	if value, ok := manual[row.Group.Key]; ok {
		return value
	}
	if row.HasSugg {
		return row.Suggestion.CategoryID
	}
	if row.SuggestionConflict {
		return ""
	}
	return proposals[row.Group.Key]
}

// MerchantName is the cleaned name used in scope labels, confirmations and rules.
// The card itself still displays the original grouped merchant string.
func MerchantName(row Row) string { return strings.TrimSpace(row.Group.Merchant) }

// Assignments builds one atomic request for the selected merchants in scope.
func Assignments(rows []Row, selected map[string]bool, manual, proposals map[string]string) map[string]string {
	out := map[string]string{}
	for _, row := range rows {
		category := Category(row, manual, proposals)
		if !selected[row.Group.Key] || category == "" {
			continue
		}
		for _, id := range ChargeIDs(row) {
			out[id] = category
		}
	}
	return out
}

// Gaps returns merchants local evidence cannot answer and their charge count.
func Gaps(rows []Row) ([]Row, int) {
	out := make([]Row, 0)
	charges := 0
	for _, row := range rows {
		if !row.HasSugg && !row.SuggestionConflict {
			out = append(out, row)
			charges += len(row.Group.Items)
		}
	}
	return out, charges
}

// ScanDraft is a bounded merchant sample and its sign-aware prompt references.
type ScanDraft struct {
	Rows        []Row
	Lines       string
	IncomeByRef map[int]bool
}

// DraftScan samples one representative per merchant, preserving queue order.
// Empty groups are ignored; a nonpositive cap produces an empty request.
func DraftScan(rows []Row, cap int, format func(money.Money) string) ScanDraft {
	out := ScanDraft{IncomeByRef: map[int]bool{}}
	var lines strings.Builder
	for _, row := range rows {
		if len(out.Rows) >= cap {
			break
		}
		if len(row.Group.Items) == 0 {
			continue
		}
		out.Rows = append(out.Rows, row)
		ref, txn := len(out.Rows), row.Group.Items[0]
		lines.WriteString(strconv.Itoa(ref) + " | " + MerchantName(row) + " | " + format(txn.Amount) + "\n")
		out.IncomeByRef[ref] = txn.Amount.Amount > 0
	}
	out.Lines = lines.String()
	return out
}

// ScanResult merges valid, sign-matched responses without changing older proposals.
func ScanResult(draft ScanDraft, answer string, catalog smartai.Catalog, previous map[string]string) (map[string]string, [2]int) {
	parsed := smartai.RejectSignMismatches(smartai.ParseCategoryAssignments(answer, len(draft.Rows), catalog), draft.IncomeByRef)
	out, filled := clone(previous), 0
	for _, assignment := range parsed {
		if assignment.Ref < 1 || assignment.Ref > len(draft.Rows) || assignment.CategoryID == "" {
			continue
		}
		out[draft.Rows[assignment.Ref-1].Group.Key] = assignment.CategoryID
		filled++
	}
	return out, [2]int{filled, len(draft.Rows) - filled}
}

// AcceptScan stages proposals and selects them, preserving every explicit edit.
func AcceptScan(manual map[string]string, selected map[string]bool, proposals map[string]string) (map[string]string, map[string]bool) {
	out, sel := clone(manual), clone(selected)
	for key, category := range proposals {
		if _, edited := manual[key]; !edited {
			out[key], sel[key] = category, true
		}
	}
	return out, sel
}

// RuleDrafts builds merchant rules in queue order; IDs and successful-write order
// are assigned by the persistence adapter, so failed writes do not leave gaps.
func RuleDrafts(rows []Row, selected map[string]bool, manual, proposals map[string]string) []rules.Rule {
	var out []rules.Rule
	for _, row := range rows {
		category, match := Category(row, manual, proposals), MerchantName(row)
		if selected[row.Group.Key] && len(row.Group.Items) > 0 && category != "" && match != "" {
			out = append(out, rules.Rule{Match: match, SetCategoryID: category})
		}
	}
	return out
}

// SelectTier adds actionable merchants of a tier to an independent selection.
func SelectTier(rows []Row, selected map[string]bool, tier Tier, manual, proposals map[string]string) map[string]bool {
	out := clone(selected)
	for _, row := range rows {
		if row.Tier == tier && Category(row, manual, proposals) != "" {
			out[row.Group.Key] = true
		}
	}
	return out
}

// SelectionCount counts selected merchants and charges in the current population.
func SelectionCount(rows []Row, selected map[string]bool) (merchants, charges int) {
	for _, row := range rows {
		if selected[row.Group.Key] {
			merchants++
			charges += len(row.Group.Items)
		}
	}
	return
}

// TierRows returns a stable tier subset and the global keyboard focus key.
func TierRows(rows []Row, tier Tier, focused int) ([]Row, string) {
	out, key := make([]Row, 0), ""
	if focused >= 0 && focused < len(rows) {
		key = rows[focused].Group.Key
	}
	for _, row := range rows {
		if row.Tier == tier {
			out = append(out, row)
		}
	}
	return out, key
}

// Focus wraps the keyboard focus at either end of the merchant list.
func Focus(position, length int) int {
	if length <= 0 || position >= length {
		return 0
	}
	if position < 0 {
		return length - 1
	}
	return position
}

// Card is the current merchant, its displayed charge, and its effective scope.
type Card struct {
	At      int
	Row     Row
	Txn     domain.Transaction
	Commit  reviewscope.CommitScope
	IDs     []string
	Targets []string
}

// CurrentCard holds the merchant identity ahead of its changing rank. A commit
// scope is honored only for the merchant for which it was explicitly chosen.
func CurrentCard(rows []Row, cursor int, cursorKey, commitKey string, commit reviewscope.CommitScope) Card {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Group.Key)
	}
	out := Card{At: reviewscope.CardIndex(keys, cursor, cursorKey), Commit: reviewscope.CommitCharge}
	if len(rows) > 0 && len(rows[out.At].Group.Items) > 0 {
		out.Row = rows[out.At]
		out.Txn = out.Row.Group.Items[0]
	}
	out.IDs = ChargeIDs(out.Row)
	if commitKey != "" && commitKey == out.Row.Group.Key && len(out.IDs) > 1 {
		out.Commit = reviewscope.NormalizeCommit(commit)
	}
	out.Targets = reviewscope.Targets(out.Commit, out.Txn.ID, out.IDs)
	return out
}

// MoveCard clamps a requested position and returns its merchant identity.
func MoveCard(rows []Row, position int) (int, string) {
	if len(rows) == 0 {
		return 0, ""
	}
	if position < 0 {
		position = 0
	}
	if position >= len(rows) {
		position = len(rows) - 1
	}
	return position, rows[position].Group.Key
}

// ActiveScope follows the ledger until the user explicitly chooses a scope.
func ActiveScope(ledger txnfilter.Criteria, chosen bool, scope reviewscope.Scope) (reviewscope.Scope, bool) {
	filtered := len(ledger.PageFilters()) > 0
	if chosen {
		return scope, filtered
	}
	return reviewscope.Default(filtered), filtered
}

// Kind returns the category direction, treating zero as expense as before.
func Kind(txn domain.Transaction) domain.CategoryKind {
	if txn.Amount.Amount > 0 {
		return domain.KindIncome
	}
	return domain.KindExpense
}

// PickerKind resolves the first charge's direction, defaulting to expense.
func PickerKind(index Index, key string) domain.CategoryKind {
	if row, ok := index.Byid[key]; ok && len(row.Group.Items) > 0 {
		return Kind(row.Group.Items[0])
	}
	return domain.KindExpense
}

// ConfirmDecision describes the next explicit confirmation step.
type ConfirmDecision struct {
	Ask       bool
	Commit    bool
	NoticeKey string
}

// Confirm validates the current card before requesting or performing a write.
func Confirm(txn domain.Transaction, category string, targets int, pending, merchant string) ConfirmDecision {
	if txn.ID == "" {
		return ConfirmDecision{}
	}
	if !reviewqueue.CanAssignCategory(txn) {
		return ConfirmDecision{NoticeKey: "review.splitNeeded"}
	}
	if targets > 1 && pending != merchant {
		if category == "" {
			return ConfirmDecision{NoticeKey: "review.chooseFirst"}
		}
		return ConfirmDecision{Ask: true}
	}
	return ConfirmDecision{Commit: true}
}

// PendingConfirmation rejects a confirmation inherited from another or shrunken group.
func PendingConfirmation(merchant, pending string, targets int) bool {
	return merchant != "" && pending == merchant && targets > 1
}

// CanScan prevents overlapping requests and requests with no local gaps.
func CanScan(gaps int, state string) bool { return gaps > 0 && state != "scanning" }

// SnoozeUntil preserves the current wall-calendar time seven days later, even
// across month ends and daylight-saving changes; it does not arbitrate writes.
func SnoozeUntil(now time.Time) time.Time { return now.AddDate(0, 0, 7) }

// GroupSummary sums one currency with checked integer operations and retains
// the original instant-based date ordering. No FX rate is guessed or required:
// a mixed-currency group returns ErrCurrencyMismatch instead of a false total.
func GroupSummary(row Row) (money.Money, time.Time, time.Time, error) {
	if len(row.Group.Items) == 0 {
		return money.Money{}, time.Time{}, time.Time{}, nil
	}
	items := row.Group.Items
	total := money.Money{Currency: items[0].Amount.Currency}
	first, last := items[0].Date, items[0].Date
	for _, txn := range items {
		var err error
		total, err = total.Add(txn.Amount)
		if err != nil {
			return money.Money{}, time.Time{}, time.Time{}, fmt.Errorf("review group total: %w", err)
		}
		if txn.Date.Before(first) {
			first = txn.Date
		}
		if txn.Date.After(last) {
			last = txn.Date
		}
	}
	return total, first, last, nil
}

// GroupFacts formats a valid confirmation summary with the existing copy. An
// invalid total is omitted rather than presenting wrapped or mislabeled money.
func (text Text) GroupFacts(row Row, pr prefs.Prefs, format func(money.Money) string) string {
	if len(row.Group.Items) == 0 {
		return ""
	}
	total, first, last, err := GroupSummary(row)
	if err != nil {
		return ""
	}
	span := pr.FormatDate(first)
	if !first.Equal(last) {
		span = text("review.dateRange", pr.FormatDate(first), pr.FormatDate(last))
	}
	return text("review.applyAllFacts", format(total), span)
}

// ActionLabels selects the existing per-charge or per-merchant action labels.
func (text Text) ActionLabels(targets int) (confirm, snooze, dismiss string) {
	if targets > 1 {
		return text("review.categorizeAllN", targets), text("review.snoozeN", targets), text("review.dismissN", targets)
	}
	return text("review.categorizeNext"), text("review.snooze"), text("review.dismiss")
}

// KeyboardHint selects the existing keyboard help for the active mode.
func (text Text) KeyboardHint(bulk bool) string {
	if bulk {
		return text("review.keyboardHintBulkPlain")
	}
	return text("review.keyboardHintSinglePlain")
}

// SelectionLabels selects the existing footer copy and enabled state.
func (text Text) SelectionLabels(merchants, charges int) (label, sub, confirm string) {
	if merchants > 0 {
		return text("review.selectionCountPlain", textutil.Plural(merchants, "merchant"), textutil.Plural(charges, "charge")), text("review.undoAvailable"), text("review.confirmN", charges)
	}
	return text("review.nothingPicked"), text("review.pickHint"), text("review.confirm")
}
