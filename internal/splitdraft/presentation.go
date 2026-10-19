// SPDX-License-Identifier: MIT

package splitdraft

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
)

// View holds the live arithmetic, draft shape, and untranslated remainder text.
type View struct {
	Total, Target, Remainder int64
	ParseError, CanSave      bool
	Shape                    split.Draft
	Tone, Key, Value, Hint   string
	PickerOpen               bool
	PickerCategory           string
}

// View computes the exact remainder and validation presentation. Incomplete rows
// are ignored on save as before; their shape still informs the live status.
func (s State) View(tx domain.Transaction) View {
	total, err := money.Abs(tx.Amount.Amount)
	v := View{Target: total, ParseError: err != nil, Hint: accounting(total, tx.Amount.Currency)}
	if s.Percent {
		v.Target = split.PercentScale
	}
	for _, line := range s.Lines {
		if line.Amt == "" {
			continue
		}
		amount, err := money.ParseMinor(line.Amt, precision(tx, s.Percent))
		if err != nil || amount <= 0 {
			v.ParseError = true
			continue
		}
		next, err := money.Add(v.Total, amount)
		if err != nil {
			v.ParseError = true
			continue
		}
		v.Total = next
	}
	v.Remainder, _ = money.Sub(v.Target, v.Total) // Both operands are nonnegative int64.
	v.PickerOpen = s.PickerLine >= 0 && s.PickerLine < len(s.Lines)
	if v.PickerOpen {
		v.PickerCategory = s.Lines[s.PickerLine].Cat
	}
	return v
}

// Present attaches the verdict shared with internal/split and formats the
// remainder copy. The caller supplies the shared shape and Saveable result.
func (s State) Present(v View, shape split.Draft, canSave bool, currency string) View {
	v.Shape, v.CanSave = shape, canSave
	v.Tone, v.Key, v.Value = s.remainderText(v, currency)
	return v
}

func (s State) remainderText(v View, code string) (tone, key, value string) {
	switch {
	case s.SaveAttempted && v.ParseError:
		return "neg", badAmountKey(s.Percent), ""
	case s.SaveAttempted && v.Remainder == 0 && !v.CanSave && v.Shape.Incomplete > 0:
		return "neg", "splitEditor.incomplete", ""
	case s.SaveAttempted && v.Remainder == 0 && !v.CanSave:
		return "neg", "splitEditor.needTwoShort", ""
	case v.Remainder != 0:
		key = "splitEditor.left"
		if s.Percent {
			key = "splitEditor.pctLeft"
		}
		if v.Remainder < 0 {
			key = "splitEditor.over"
			if s.Percent {
				key = "splitEditor.pctOver"
			}
		}
		amount, _ := money.Abs(v.Remainder) // The difference cannot be MinInt64.
		value = accounting(amount, code)
		if s.Percent {
			value = money.FormatMinor(amount, 2)
		}
		return RemainderTone(s.SaveAttempted), key, value
	default:
		return "pos", "splitEditor.balanced", ""
	}
}

func badAmountKey(percent bool) string {
	if percent {
		return "splitEditor.badPercent"
	}
	return "splitEditor.badAmount"
}

func accounting(amount int64, code string) string {
	return money.FormatAccounting(amount, currency.Decimals(code), currency.Symbol(code))
}

// Magnitude retains the legacy helper's value for an unrepresentable MinInt64
// magnitude. Draft arithmetic instead checks money.Abs and refuses to save it.
func Magnitude(amount int64) int64 {
	v, err := money.Abs(amount)
	if err != nil {
		return amount
	}
	return v
}

// Columns returns the original split grid template.
func Columns(showOwner bool) string {
	if showOwner {
		return "minmax(180px, 1fr) 160px 120px 32px"
	}
	return "minmax(180px, 1fr) 120px 32px"
}

// RemainderTone keeps unfinished arithmetic neutral until saving is attempted.
func RemainderTone(attempted bool) string {
	if attempted {
		return "neg"
	}
	return "split-remainder-neutral"
}

// CategoryKind selects income only for a positive parent amount.
func CategoryKind(tx domain.Transaction) domain.CategoryKind {
	if tx.Amount.Amount > 0 {
		return domain.KindIncome
	}
	return domain.KindExpense
}

// CategoriesForKind filters categories without changing their order or input.
func CategoriesForKind(categories []domain.Category, kind domain.CategoryKind) []domain.Category {
	out := make([]domain.Category, 0, len(categories))
	for _, category := range categories {
		if category.Kind == kind {
			out = append(out, category)
		}
	}
	return out
}

// CellSummary counts distinct categories and formats each split in its own
// currency for the tooltip. It does not total or convert mixed currencies.
func CellSummary(tx domain.Transaction) (count int, tooltip string) {
	if !tx.HasSplits() {
		return 0, ""
	}
	ids := make(map[string]struct{}, len(tx.Splits))
	amounts := make([]string, 0, len(tx.Splits))
	for _, line := range tx.Splits {
		ids[line.CategoryID] = struct{}{}
		amounts = append(amounts, accounting(line.Amount.Amount, line.Amount.Currency))
	}
	return len(ids), strings.Join(amounts, ", ")
}

// AmountKey selects the amount input's untranslated label.
func AmountKey(percent bool) string {
	if percent {
		return "splitEditor.percent"
	}
	return "splitEditor.amount"
}

// ToggleKey selects the untranslated split action label.
func ToggleKey(open, hasSplits bool) string {
	switch {
	case open:
		return "splitEditor.hide"
	case hasSplits:
		return "splitEditor.edit"
	default:
		return "splitEditor.toggle"
	}
}

// ShowOwner reports whether the household roster exposes per-line attribution.
func ShowOwner(members []domain.Member) bool { return len(members) > 0 }

// ModeClass returns the original segmented button class for its active state.
func ModeClass(active bool) string {
	if active {
		return "btn btn-sm btn-primary"
	}
	return "btn btn-sm"
}
