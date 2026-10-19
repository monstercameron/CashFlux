// SPDX-License-Identifier: MIT

package importdraft

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/textdecode"
)

// CleanCSV decodes source text and sanitizes imported text cells while preserving
// logical records, delimiters, quoted values and all non-text transaction fields.
func CleanCSV(data []byte) ([]byte, error) {
	text, err := textdecode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("read CSV encoding: %w", err)
	}
	r := csvReader(text)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV records: %w", err)
	}
	if len(records) == 0 {
		return data, nil
	}
	for i, name := range records[0] {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "desc", "description", "payee", "category", "category_id", "account", "account_id":
			for _, record := range records[1:] {
				if i < len(record) {
					record[i] = Clean(record[i])
				}
			}
		}
	}
	var output bytes.Buffer
	w := csv.NewWriter(&output)
	w.Comma = r.Comma
	if err := w.WriteAll(records); err != nil {
		return nil, fmt.Errorf("write sanitized CSV: %w", err)
	}
	return output.Bytes(), nil
}

// ValidateCSVLimit counts logical CSV records, including multiline quoted fields.
func ValidateCSVLimit(data []byte, limit int) error {
	if limit <= 0 {
		return nil
	}
	r := csvReader(string(data))
	if _, err := r.Read(); err != nil {
		return err
	}
	for count := 0; ; {
		_, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		count++
		if count > limit {
			return errors.New("too many CSV rows")
		}
	}
}

func csvReader(data string) *csv.Reader {
	text := strings.TrimPrefix(data, "\ufeff")
	first := strings.SplitN(text, "\n", 2)[0]
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = statement.DetectDelimiter(first)
	r.FieldsPerRecord = -1
	return r
}

// CSVFailureRows retains the original editable fields for failed CSV outcomes.
// Lines refer to logical records, with the header at one. The full source remains
// visible in the CSV textarea for columns outside the draft editor's core fields.
func CSVFailureRows(data string, outcomes []Outcome, lines map[string]int) []extract.Row {
	r := csvReader(data)
	header, err := r.Read()
	if err != nil {
		return nil
	}
	byLine := map[int]string{}
	for _, o := range outcomes {
		if o.Status == "failed" {
			byLine[lines[o.ID]] = o.ID
		}
	}
	var rows []extract.Row
	for line := 2; ; line++ {
		cells, err := r.Read()
		if err != nil {
			break
		}
		rowID := byLine[line]
		if rowID == "" {
			continue
		}
		row := extract.Row{ID: rowID}
		for i, name := range header {
			if i >= len(cells) {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "date":
				row.Date = cells[i]
			case "description", "desc", "payee":
				if row.Description == "" {
					row.Description = Clean(cells[i])
				}
			case "amount":
				row.Amount = cells[i]
			case "category", "category_id":
				row.Category = cells[i]
			case "account", "account_id":
				row.AccountID = cells[i]
			case "currency":
				row.Currency = cells[i]
			case "status":
				row.Status = cells[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}
