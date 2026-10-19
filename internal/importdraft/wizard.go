// SPDX-License-Identifier: MIT

package importdraft

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importai"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/pdftext"
	"github.com/monstercameron/CashFlux/internal/statement"
)

// WizardSelection is the editable column and locale mapping of a statement.
type WizardSelection struct{ Date, Description, Amount, Debit, Credit, NumberFormat, DateOrder string }

// Profile builds the same manual/saved profile, including legacy invalid-index defaults.
func (s WizardSelection) Profile(name string, decimals int) importmap.Profile {
	atoi := func(s string) int { v, _ := strconv.Atoi(s); return v }
	return importmap.Profile{Name: name, DateCol: atoi(s.Date), DescCol: atoi(s.Description), AmountCol: atoi(s.Amount), DebitCol: atoi(s.Debit), CreditCol: atoi(s.Credit), BalanceCol: -1, CurrencyCol: -1, Decimals: decimals, NumberFormat: NumberFormatChoice(s.NumberFormat), DateOrder: statement.DateOrder(s.DateOrder)}
}

// FlattenRecords unwraps logical records without reordering or copying their fields.
func FlattenRecords(groups [][][]string) [][]string {
	flat := make([][]string, 0)
	for _, group := range groups {
		flat = append(flat, group...)
	}
	return flat
}

// ApplyProfile maps source records to editable rows with the profile's precision.
// Mapped categories remain empty, matching the column wizard and saved mappings.
func ApplyProfile(profile importmap.Profile, groups [][][]string) ([]extract.Row, []importmap.RowError, []importmap.RowWarning) {
	mapped, failures, warnings := importmap.ApplyWithDiagnostics(profile, FlattenRecords(groups))
	rows := StatementRows(mapped, profile.Decimals)
	for i := range rows {
		rows[i].Category = ""
	}
	return rows, failures, warnings
}

// StructuredSource identifies the existing PDF, spreadsheet and OFX parser routes.
func StructuredSource(data string) bool {
	upper := strings.ToUpper(data)
	return strings.HasPrefix(data, "%PDF") || strings.HasPrefix(data, "PK\x03\x04") || strings.HasPrefix(upper, "OFXHEADER:") || strings.HasPrefix(upper, "<?XML") || strings.HasPrefix(upper, "<?OFX") || strings.HasPrefix(upper, "<OFX")
}

// StatementFailure classifies parser failures for the existing recovery controls.
func StatementFailure(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, pdftext.ErrNoText) || errors.Is(err, pdftext.ErrEncrypted) || strings.Contains(err.Error(), "no text found") || strings.Contains(err.Error(), "PDF is encrypted") {
		return "scanned"
	}
	if strings.Contains(err.Error(), "could not find") {
		return "mapping"
	}
	return "error"
}

// Reminder builds the existing monthly task, preserving calendar normalization.
func Reminder(id, title string, now time.Time) domain.Task {
	return domain.Task{ID: id, Title: title, Due: dateutil.AddMonths(now, 1), Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceNudge, Recurrence: domain.CadenceMonthly}
}

// CategoryNames retains the household's category order for extraction prompts.
func CategoryNames(categories []domain.Category) []string {
	var names []string
	for _, category := range categories {
		names = append(names, category.Name)
	}
	return names
}

// CategoryName resolves the first category ID without changing its text.
func CategoryName(categories []domain.Category, id string) (string, bool) {
	for _, category := range categories {
		if category.ID == id {
			return category.Name, true
		}
	}
	return "", false
}

// CategoryInputs binds uncategorized rows to stable identities for an AI request.
func CategoryInputs(rows []extract.Row) ([]importai.SuggestionInput, []string) {
	var inputs []importai.SuggestionInput
	var ids []string
	for _, row := range rows {
		if row.Category != "" {
			continue
		}
		inputs = append(inputs, importai.SuggestionInput{Index: len(ids), Description: row.Description})
		ids = append(ids, row.ID)
	}
	return inputs, ids
}

// ValidateExtraction supplies missing currencies and validates without mutating input.
func ValidateExtraction(rows []extract.Row, accounts []domain.Account, cur string, today time.Time) []importai.ValidatedRow {
	known := map[string]bool{}
	for _, account := range accounts {
		known[account.ID] = true
	}
	rows = append([]extract.Row(nil), rows...)
	for i := range rows {
		if rows[i].Currency == "" {
			rows[i].Currency = cur
		}
	}
	return importai.ValidateRows(rows, importai.ValidationOptions{AccountIDs: known, Today: today})
}
