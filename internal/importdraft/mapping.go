// SPDX-License-Identifier: MIT

package importdraft

import (
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/store"
)

// Translate renders an existing translation key using the caller's locale.
type Translate func(string, ...any) string

// StatementSystemPrompt builds the existing statement extraction prompt with household categories.
func StatementSystemPrompt(catNames []string) string {
	base := "You extract EVERY transaction from an attached bank or credit-card statement (a PDF). " +
		"For each transaction return: date (YYYY-MM-DD), description (the merchant/payee, tidied), " +
		"amount as a string (negative for money out/expenses like \"-45.00\", positive for money in), and category. " +
		"Skip non-transaction lines (opening/closing balance, totals, interest summaries, payment-due notices). "
	if len(catNames) > 0 {
		base += "For category, use EXACTLY one of these existing categories when one clearly fits, " +
			"otherwise return an empty string \"\" — NEVER invent a category not in this list. " +
			"Existing categories: " + strings.Join(catNames, ", ") + "."
	} else {
		base += "Return an empty string \"\" for every category."
	}
	return base
}

// CSVSkipDetail limits localized CSV failure details to three rows and a remainder count.
func CSVSkipDetail(rows []store.CSVRowError, translate Translate) string {
	if len(rows) == 0 {
		return ""
	}
	const maxShown = 3
	parts := make([]string, 0, maxShown)
	for i, r := range rows {
		if i >= maxShown {
			break
		}
		reason := CSVReason(strings.TrimSpace(r.Reason), translate)
		if reason == "" {
			reason = translate("documents.skipReasonGeneric")
		}
		parts = append(parts, translate("documents.skipLine", r.Line, reason))
	}
	detail := strings.Join(parts, "; ")
	if len(rows) > maxShown {
		detail += " " + translate("documents.skipMore", len(rows)-maxShown)
	}
	return detail
}

// CSVReason selects the existing localized explanation for a CSV row failure.
func CSVReason(reason string, translate Translate) string {
	lower := strings.ToLower(reason)
	switch {
	case reason == "amount is required":
		return translate("tc4n6.amountRequired")
	case strings.Contains(lower, "possible duplicate"):
		return translate("tc4n6.possibleDuplicate")
	case strings.Contains(lower, "too many decimal places"):
		return translate("tc4n6.amountPrecision")
	case strings.Contains(lower, "invalid amount"):
		return translate("tc4n6.amountInvalid")
	case strings.Contains(lower, "invalid cleared"):
		return translate("tc4n6.clearedInvalid")
	case strings.Contains(lower, "member_ids"):
		return translate("tc4n6.memberIDsInvalid")
	case strings.Contains(lower, "date"):
		return translate("tc4n6.dateInvalid")
	case strings.Contains(lower, "currency"):
		return translate("tc4n6.currencyInvalid")
	case strings.Contains(lower, "repeated"):
		return translate("tc4n6.repeatedID")
	default:
		return translate("documents.skipReasonGeneric")
	}
}

// MapWarningSummary counts date and amount ambiguity warnings in the existing display order.
func MapWarningSummary(warnings []importmap.RowWarning, translate Translate) string {
	dateCount, amountCount := 0, 0
	for _, warning := range warnings {
		switch warning.Field {
		case "amount":
			amountCount++
		default:
			dateCount++
		}
	}
	var parts []string
	if dateCount > 0 {
		parts = append(parts, translate("tc4n6.ambiguousDates", dateCount))
	}
	if amountCount > 0 {
		parts = append(parts, translate("tc4n6.ambiguousAmounts", amountCount))
	}
	return strings.Join(parts, " ")
}

// StatementRows formats parsed statement rows without losing categories or local calendar dates.
func StatementRows(rows []statement.Row, decimals int) []extract.Row {
	out := make([]extract.Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, extract.Row{
			Date: dateutil.FormatDate(row.Date), Description: row.Description,
			Amount: money.FormatMinor(row.Amount, decimals), Category: row.Category,
		})
	}
	return out
}

// StatementOptions maps the date and number selectors to parser options.
func StatementOptions(dateOrder, numberFormat string) statement.ParseOptions {
	options := statement.ParseOptions{DateOrder: statement.DateOrder(dateOrder)}
	switch numberFormat {
	case "dot":
		options.NumberFormat = statement.NumberFormat{DecimalSep: '.', GroupSep: ','}
	case "comma":
		options.NumberFormat = statement.NumberFormat{DecimalSep: ',', GroupSep: '.'}
	}
	return options
}

// NumberFormatChoice maps a number selector to its import mapping format.
func NumberFormatChoice(choice string) importmap.NumberFormat {
	switch choice {
	case "dot":
		return importmap.NumberFormatDotDecimal
	case "comma":
		return importmap.NumberFormatCommaDecimal
	default:
		return importmap.NumberFormatAuto
	}
}

// MapErrors maps parser failures to one-based localized CSV row failures.
func MapErrors(rows []importmap.RowError, translate Translate) []store.CSVRowError {
	out := make([]store.CSVRowError, 0, len(rows))
	for _, row := range rows {
		field := row.Field
		switch field {
		case "date":
			field = translate("tc4n6.fieldDate")
		case "amount":
			field = translate("tc4n6.fieldAmount")
		case "debit":
			field = translate("tc4n6.fieldDebit")
		case "credit":
			field = translate("tc4n6.fieldCredit")
		case "balance":
			field = translate("tc4n6.fieldBalance")
		}
		out = append(out, store.CSVRowError{Line: row.Index + 1, Reason: translate("tc4n6.rowFieldFailure", row.Index+1, field)})
	}
	return out
}

// DocumentRows copies editable rows into the persisted document history shape.
func DocumentRows(rows []extract.Row) []domain.DocumentRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.DocumentRow, len(rows))
	for i, r := range rows {
		out[i] = domain.DocumentRow{Date: r.Date, Description: r.Description, Amount: r.Amount, Category: r.Category}
	}
	return out
}

// RawHeaderAndRows reads logical delimited records, including quoted commas and newlines.
func RawHeaderAndRows(text string, delim rune) (header []string, rows [][][]string) {
	if delim == 0 {
		firstLine := text
		if i := strings.IndexAny(text, "\r\n"); i >= 0 {
			firstLine = text[:i]
		}
		delim = statement.DetectDelimiter(firstLine)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return nil, nil
	}
	header = records[0]
	for _, record := range records[1:] {
		rows = append(rows, [][]string{record})
	}
	return header, rows
}

// GuessField prefers a detected column, then the first keyword/header match.
func GuessField(header []string, keywords []string, detected int) string {
	if detected >= 0 {
		return strconv.Itoa(detected)
	}
	lower := make([]string, len(header))
	for i, h := range header {
		lower[i] = strings.ToLower(h)
	}
	for _, kw := range keywords {
		for i, h := range lower {
			if strings.Contains(h, kw) {
				return strconv.Itoa(i)
			}
		}
	}
	return "-1"
}
