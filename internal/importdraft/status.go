// SPDX-License-Identifier: MIT

package importdraft

import (
	"encoding/base64"
	"strings"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/money"
)

// FriendlyError preserves the import region's generic AI failure disclosure.
func FriendlyError(message string, translate Translate) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return translate("imb.aiFailed")
}

// ImportedTransactionIDs selects durable imported identities in outcome order.
// The projection keeps the persistence result type outside the pure draft model.
func ImportedTransactionIDs[T any](outcomes []T, project func(T) (status, id string)) []string {
	var ids []string
	for _, outcome := range outcomes {
		status, id := project(outcome)
		if status == "imported" {
			ids = append(ids, id)
		}
	}
	return ids
}

// ParsedMessage describes mapped rows and their capped failure/ambiguity details.
func ParsedMessage(count int, failures []importmap.RowError, warnings []importmap.RowWarning, translate Translate) string {
	text := translate("documents.stmtParsed", translate(CountKey(count, "row"), count))
	if len(failures) > 0 {
		text += " " + CSVSkipDetail(MapErrors(failures, translate), translate)
	}
	if len(warnings) > 0 {
		text += " " + MapWarningSummary(warnings, translate)
	}
	return text
}

// ReceiptTotal initializes only an empty total when receipt mode is enabled.
func ReceiptTotal(rows []extract.Row, current string, decimals int, enabled bool) string {
	if enabled && strings.TrimSpace(current) == "" {
		if sum, err := Receipt(rows, "", "").LinesTotalMinor(decimals); err == nil {
			return money.FormatMinor(sum, decimals)
		}
	}
	return current
}

// PDFDataURL retains the selected MIME type and losslessly encodes source bytes.
func PDFDataURL(mime string, data []byte) string {
	if mime == "" {
		mime = "application/pdf"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
