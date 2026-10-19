// SPDX-License-Identifier: MIT

package importdraft

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// Summary contains checked totals and identity-indexed review decisions.
type Summary struct {
	Indices                                   map[string]int
	Income, Expense, Net                      int64
	Importable, Duplicates, Invalid, Repeated int
	Errors                                    map[string]string
	DuplicateIDs, RepeatedIDs                 map[string]bool
	Overflow                                  bool
}

// Summarize calculates review facts once per staged batch, in linear time.
func Summarize(batch staging.Batch) Summary {
	s := Summary{Indices: map[string]int{}, Errors: map[string]string{}, DuplicateIDs: map[string]bool{}, RepeatedIDs: map[string]bool{}}
	for i, row := range batch.Rows {
		s.Indices[row.ID] = i
		switch row.Verdict {
		case staging.Duplicate:
			s.Duplicates++
			s.DuplicateIDs[row.ID] = true
		case staging.Unusable:
			s.Invalid++
			s.Errors[row.ID] = row.Reason
		default:
			if !row.Approved() {
				continue
			}
			s.Importable++
			if row.PossibleDuplicate {
				s.Repeated++
				s.RepeatedIDs[row.ID] = true
			}
			var err error
			s.Net, err = add(s.Net, row.AmountMinor)
			if err != nil {
				s.Overflow = true
			}
			if row.AmountMinor >= 0 {
				s.Income, err = add(s.Income, row.AmountMinor)
			} else {
				var positive int64
				positive, err = money.Neg(row.AmountMinor)
				if err == nil {
					s.Expense, err = add(s.Expense, positive)
				}
			}
			if err != nil {
				s.Overflow = true
			}
		}
	}
	return s
}

// RowError prefers a failed-commit or source-validation reason over staging.
func RowError(id string, staged, failed map[string]string) string {
	if reason := failed[id]; reason != "" {
		return reason
	}
	return staged[id]
}

// CountKey chooses a translated count label without assembling English words.
func CountKey(n int, noun string) string {
	base := "imb.row"
	switch noun {
	case "transaction":
		base = "imb.transaction"
	case "category":
		base = "imb.category"
	case "new transaction":
		base = "imb.newTransaction"
	}
	if n == 1 {
		return base + "One"
	}
	return base + "Many"
}

func add(a, b int64) (int64, error) { return money.Add(a, b) }

// Window returns a bounded one-based page for a possibly changed draft.
func Window(total, page, size int) (start, end, current, pages int) {
	if total <= 0 || size <= 0 {
		return 0, 0, 1, 1
	}
	pages = 1 + (total-1)/size
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	start = (page - 1) * size
	end = min(start+size, total)
	return start, end, page, pages
}

// ReceiptDate uses the first valid parsed date, falling back only if none exists.
func ReceiptDate(rows []extract.Row, fallback time.Time) time.Time {
	for _, r := range rows {
		if d, err := dateutil.ParseDate(r.Date); err == nil {
			return d
		}
	}
	return fallback
}

// Receipt keeps signed discount/refund lines when building the reviewed receipt.
func Receipt(rows []extract.Row, merchant, total string) extract.Receipt {
	r := extract.Receipt{Merchant: Clean(merchant), Total: total, Date: dateutil.FormatDate(ReceiptDate(rows, time.Time{}))}
	for _, row := range rows {
		r.Lines = append(r.Lines, extract.ReceiptLine{Description: Clean(row.Description), Category: Clean(row.Category), Amount: row.Amount})
	}
	return r
}

// FooterAction determines what the pinned primary may do from the visible stage.
// It can only commit rows on review, or a visible CSV preflight for this account.
func FooterAction(stage, source string, rows int, preflight, busy bool) string {
	if busy {
		return "wait"
	}
	if stage == "review" && rows > 0 {
		return "review"
	}
	if source == "csv" {
		if preflight {
			return "csv-confirm"
		}
		return "csv-preview"
	}
	if rows > 0 {
		return "show-review"
	}
	return "done"
}
