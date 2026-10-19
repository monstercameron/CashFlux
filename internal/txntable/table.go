// SPDX-License-Identifier: MIT

// Package txntable derives the transaction table without browser or UI dependencies.
// Inputs are snapshots; selection, indexes and display decisions never mutate them.
package txntable

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
)

// VirtualizeThreshold is the All-view size above which rows are windowed.
const VirtualizeThreshold = 100

// Page describes the current page and its half-open row and selection ranges.
type Page struct {
	Number, Size, Start, End, SelectionStart, SelectionEnd int
	Virtual, OnePage                                       bool
}

// Paginate clamps a page without overflowing even for the largest native int.
func Paginate(number, size, total int) Page {
	if total < 0 {
		total = 0
	}
	if size == 0 {
		size = txnfilter.DefaultPageSize
	}
	p := Page{Number: 1, Size: size, End: total}
	if size > 0 {
		pages := 1
		if total > 0 {
			pages = 1 + (total-1)/size
		}
		if number < 1 {
			number = 1
		}
		if number > pages {
			number = pages
		}
		p.Number = number
		p.Start = (number - 1) * size
		p.End = p.Start + min(size, total-p.Start)
	}
	p.Virtual = size <= 0 && total > VirtualizeThreshold
	p.OnePage = total > 0 && total <= txnfilter.PageSizes[0]
	p.SelectionStart, p.SelectionEnd = p.Start, p.End
	if p.Virtual {
		p.SelectionStart, p.SelectionEnd = 0, total
	}
	return p
}

// SelectionScope discards ordering and paging, which do not hide selected rows.
func SelectionScope(f txnfilter.Criteria) txnfilter.Criteria {
	f.Page, f.PageSize, f.Sort, f.Dir = 0, 0, "", ""
	return f
}

// EffectiveCriteria applies register ordering before hidden-column fallback.
func EffectiveCriteria(f txnfilter.Criteria, register bool, hidden map[string]bool) txnfilter.Criteria {
	if register {
		f.Sort, f.Dir = "date", txnfilter.Asc
	}
	f.Sort, f.Dir = txnfilter.EffectiveSort(f, hidden)
	return f
}

// SortKey removes sorting affordances while register order is fixed.
func SortKey(register bool, key string) string {
	if register {
		return ""
	}
	return key
}

// SortBy returns the next stored sort, leaving register mode unchanged.
func SortBy(f txnfilter.Criteria, key string, register bool) txnfilter.Criteria {
	if register {
		return f
	}
	if f.Sort != key {
		f.Sort, f.Dir = key, txnfilter.DefaultDir(key)
	} else if f.Dir == txnfilter.Asc {
		f.Dir = txnfilter.Desc
	} else {
		f.Dir = txnfilter.Asc
	}
	return f
}

// ToggleSelection returns a fresh true-only selection, extending a valid shift range.
func ToggleSelection(selected map[string]bool, anchor, id string, shift bool, order []string) map[string]bool {
	next := make(map[string]bool, len(selected)+1)
	for k, v := range selected {
		if v {
			next[k] = true
		}
	}
	if shift && anchor != "" && anchor != id {
		a, b := -1, -1
		for i, k := range order {
			if k == anchor {
				a = i
			}
			if k == id {
				b = i
			}
		}
		if a >= 0 && b >= 0 {
			if a > b {
				a, b = b, a
			}
			for _, k := range order[a : b+1] {
				next[k] = true
			}
			return next
		}
	}
	if next[id] {
		delete(next, id)
	} else {
		next[id] = true
	}
	return next
}

// IDs reads a bounded frame range in display order.
func IDs(field domain.Field, start, end int) []string {
	start = max(0, min(start, len(field.Values)))
	end = max(start, min(end, len(field.Values)))
	ids := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		ids = append(ids, field.Str(i))
	}
	return ids
}

// Indices returns the half-open range used by keyed paginated rows.
func Indices(start, end int) []int {
	start = max(0, start)
	end = max(start, end)
	result := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, i)
	}
	return result
}

// Transactions indexes the supplied snapshot, with later duplicate IDs winning.
func Transactions(rows []domain.Transaction) map[string]domain.Transaction {
	result := make(map[string]domain.Transaction, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result
}

// GroupMembers collects only the members named by groups from the complete ledger.
func GroupMembers(groups map[string]domain.TxnLink, all []domain.Transaction) map[string]domain.Transaction {
	out := make(map[string]domain.Transaction)
	if len(groups) == 0 {
		return out
	}
	wanted := make(map[string]bool)
	for _, link := range groups {
		for _, id := range link.TxnIDs {
			wanted[id] = true
		}
	}
	for _, row := range all {
		if wanted[row.ID] {
			out[row.ID] = row
		}
	}
	return out
}

// RegisterBalances returns nil when disabled, unresolved, or invalid; no partial balance escapes.
func RegisterBalances(active bool, id string, accounts []domain.Account, all []domain.Transaction) map[string]money.Money {
	if active {
		for _, account := range accounts {
			if account.ID == id {
				result, err := ledger.RegisterBalances(account, all)
				if err == nil {
					return result
				}
				break
			}
		}
	}
	return nil
}

// Index contains immutable row lookup data, derived once per ledger revision.
type Index struct {
	Transactions                                           map[string]domain.Transaction
	Accounts                                               map[string]domain.Account
	AccountNames, CategoryNames, MemberNames, MemberColors map[string]string
	ReconciledThrough                                      map[string]time.Time
	Refund, Refunded, BillMatched                          map[string]bool
	EventNames                                             map[string]string
}

// BuildIndex resolves names, reconciliation boundaries and links in one pass per collection.
func BuildIndex(rows []domain.Transaction, accounts []domain.Account, categories []domain.Category, members []domain.Member, links []domain.TxnLink, events []domain.Event) Index {
	x := Index{Transactions: Transactions(rows), Accounts: map[string]domain.Account{},
		AccountNames: map[string]string{}, CategoryNames: map[string]string{}, MemberNames: map[string]string{}, MemberColors: map[string]string{},
		ReconciledThrough: map[string]time.Time{}, Refund: map[string]bool{}, Refunded: map[string]bool{}, BillMatched: map[string]bool{}, EventNames: map[string]string{}}
	for _, a := range accounts {
		x.Accounts[a.ID] = a
		x.AccountNames[a.ID] = a.Name
		if through, ok := reconcile.Through(a.Reconciliations); ok {
			x.ReconciledThrough[a.ID] = through
		}
	}
	for _, c := range categories {
		x.CategoryNames[c.ID] = c.Name
	}
	for _, m := range members {
		x.MemberNames[m.ID] = m.Name
		x.MemberColors[m.ID] = m.Color
	}
	names := map[string]string{}
	for _, e := range events {
		names[e.ID] = e.Name
	}
	for _, link := range links {
		if link.Kind == domain.TxnLinkRefundPair && len(link.TxnIDs) == 2 {
			x.Refunded[link.TxnIDs[0]] = true
			x.Refund[link.TxnIDs[1]] = true
		}
		if link.Kind == domain.TxnLinkBillMatch && len(link.TxnIDs) == 1 {
			x.BillMatched[link.TxnIDs[0]] = true
		}
		if link.Kind == domain.TxnLinkEventTxn && len(link.TxnIDs) == 1 && link.EventID != "" {
			if _, ok := x.EventNames[link.TxnIDs[0]]; !ok {
				x.EventNames[link.TxnIDs[0]] = names[link.EventID]
			}
		}
	}
	return x
}

// GroupSum adds signed minor units with checked arithmetic and explicit FX conversion.
// Same-currency groups retain their original currency; missing FX or overflow returns no partial total.
func GroupSum(rows []domain.Transaction, rates currency.Rates) (money.Money, error) {
	if len(rows) == 0 {
		return money.Money{}, nil
	}
	total := money.Zero(rows[0].Amount.Currency)
	for _, row := range rows {
		amount := row.Amount
		if amount.Currency != total.Currency {
			converted, err := rates.Convert(amount, total.Currency)
			if err != nil {
				return money.Money{}, fmt.Errorf("group total: %w", err)
			}
			amount = converted
		}
		next, err := total.Add(amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("group total: %w", err)
		}
		total = next
	}
	return total, nil
}

// Description chooses the learned merchant or the original statement description.
func Description(desc, payee string, learned func(string) bool, resolve func(string) string) string {
	if resolve == nil {
		resolve = func(s string) string { return s }
	}
	if strings.TrimSpace(payee) != "" && learned != nil && learned(payee) {
		return resolve(payee)
	}
	if strings.TrimSpace(desc) == "" {
		return resolve(payee)
	}
	return desc
}

// DeriveRows filters and orders descriptions by their displayed text; ID breaks ties ascending.
func DeriveRows(rows []domain.Transaction, f txnfilter.Criteria, labels txnfilter.Labels, learned func(string) bool, resolve func(string) string) ([]domain.Transaction, []txnfilter.ConversionIssue) {
	if labels.Payee != nil {
		original := labels.Payee
		cache := map[string]string{}
		labels.Payee = func(raw string) string {
			if v, ok := cache[raw]; ok {
				return v
			}
			v := original(raw)
			cache[raw] = v
			return v
		}
	}
	result, issues := txnfilter.ApplyWithLabelsChecked(rows, f, labels)
	return OrderDescriptions(result, f, learned, resolve), issues
}

// OrderDescriptions orders an already filtered snapshot by its displayed description.
// It never filters again or mutates the shared input. Other sort orders are retained.
func OrderDescriptions(rows []domain.Transaction, f txnfilter.Criteria, learned func(string) bool, resolve func(string) string) []domain.Transaction {
	result := append([]domain.Transaction(nil), rows...)
	normalized := f.Normalize()
	if normalized.Sort == "payee" {
		keys := map[string]string{}
		for _, row := range result {
			keys[row.ID] = strings.ToLower(Description(row.Desc, row.Payee, learned, resolve))
		}
		sort.SliceStable(result, func(i, j int) bool {
			cmp := strings.Compare(keys[result[i].ID], keys[result[j].ID])
			if cmp == 0 {
				return result[i].ID < result[j].ID
			}
			if normalized.Dir == txnfilter.Desc {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	return result
}

// CategoryCriteria removes category and direction choices for category summary context.
func CategoryCriteria(f txnfilter.Criteria) txnfilter.Criteria {
	return txntoolbar.CategoryScope(f)
}

// OpenTaskCount counts unfinished tasks linked to a transaction.
func OpenTaskCount(tasks []domain.Task, id string) int {
	count := 0
	for _, task := range tasks {
		if task.RelatedType == domain.RelatedTransaction && task.RelatedID == id && task.Status != domain.StatusDone {
			count++
		}
	}
	return count
}

// RulePhrase preserves payee-first rule draft prefill, trimming after choosing a field.
func RulePhrase(row domain.Transaction) string {
	if row.Payee != "" {
		return strings.TrimSpace(row.Payee)
	}
	return strings.TrimSpace(row.Desc)
}

// VisibleTransfers retains one row per household movement while keeping the
// matching leg in single-account register views. It preserves source order.
func VisibleTransfers(rows []domain.Transaction, criteria txnfilter.Criteria, rates currency.Rates) []domain.Transaction {
	_, single := (txnfilter.Criteria{Account: strings.TrimSpace(criteria.Account), Accounts: criteria.Accounts}).SingleAccount()
	if single || len(rows) < 2 {
		return rows
	}
	summary := txnclassify.SummarizeVisibleTransfers(rows, rates)
	kept := make([]domain.Transaction, 0, len(rows))
	for i, row := range rows {
		if !row.IsTransfer() || summary.CountedRows[i] {
			kept = append(kept, row)
		}
	}
	return kept
}

// RowIndex supplies an absolute ARIA row position only for a windowed body.
// Zero omits the attribute for ordinary pages or unrepresentable positions.
func (p Page) RowIndex(index int) int {
	if !p.Virtual || index < 0 {
		return 0
	}
	row := index + 2
	if row < 2 {
		return 0
	}
	return row
}
