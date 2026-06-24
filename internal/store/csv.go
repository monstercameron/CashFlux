// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// csvHeader is the stable column order for transaction CSV export.
var csvHeader = []string{
	"id", "date", "account_id", "payee", "desc", "category_id",
	"amount", "currency", "transfer_account_id", "cleared", "tags", "member_id",
}

// TransactionsToCSV serializes transactions to CSV with a header row. Amounts
// are written as plain decimals in the transaction's currency.
func TransactionsToCSV(txns []domain.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, t := range txns {
		row := []string{
			t.ID,
			dateutil.FormatDate(t.Date),
			t.AccountID,
			t.Payee,
			t.Desc,
			t.CategoryID,
			money.FormatMinor(t.Amount.Amount, currency.Decimals(t.Amount.Currency)),
			t.Amount.Currency,
			t.TransferAccountID,
			strconv.FormatBool(t.Cleared),
			strings.Join(t.Tags, ";"),
			t.MemberID,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CSVRowError records a single row that could not be parsed during a resilient
// CSV import. Line is the 1-based CSV line number (header = 1).
type CSVRowError struct {
	Line   int
	Reason string
}

// TransactionsFromCSVResilient parses transactions from CSV without aborting on
// per-row problems. Structural failures (unreadable CSV, missing or empty file)
// are returned as err. Per-row problems (missing amount, bad number, bad date)
// are collected in skipped so the caller can report them without losing the
// valid rows. The semantics of column matching, id generation, and currency
// defaulting are identical to TransactionsFromCSV.
func TransactionsFromCSVResilient(data []byte, defaultCurrency string) (txns []domain.Transaction, skipped []CSVRowError, err error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	records, csvErr := r.ReadAll()
	if csvErr != nil {
		return nil, nil, fmt.Errorf("store: csv read: %w", csvErr)
	}
	if len(records) == 0 {
		return nil, nil, nil
	}

	idx := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		idx[strings.ToLower(strings.TrimSpace(name))] = i
	}
	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	colID := func(row []string, base string) string {
		if v := col(row, base+"_id"); v != "" {
			return v
		}
		return col(row, base)
	}

	out := make([]domain.Transaction, 0, len(records)-1)
	for n, row := range records[1:] {
		line := n + 2

		amtStr := col(row, "amount")
		if amtStr == "" {
			skipped = append(skipped, CSVRowError{Line: line, Reason: "amount is required"})
			continue
		}
		curr := col(row, "currency")
		if curr == "" {
			curr = defaultCurrency
		}
		if curr == "" {
			skipped = append(skipped, CSVRowError{Line: line, Reason: "currency is required (add a currency column or set a base currency)"})
			continue
		}
		amt, parseErr := money.ParseMinor(amtStr, currency.Decimals(curr))
		if parseErr != nil {
			skipped = append(skipped, CSVRowError{Line: line, Reason: parseErr.Error()})
			continue
		}

		var date time.Time
		if ds := col(row, "date"); ds != "" {
			var dateErr error
			if date, dateErr = dateutil.ParseDate(ds); dateErr != nil {
				skipped = append(skipped, CSVRowError{Line: line, Reason: dateErr.Error()})
				continue
			}
		}

		cleared := false
		if cs := col(row, "cleared"); cs != "" {
			var clearErr error
			if cleared, clearErr = strconv.ParseBool(cs); clearErr != nil {
				skipped = append(skipped, CSVRowError{Line: line, Reason: fmt.Sprintf("invalid cleared %q", cs)})
				continue
			}
		}

		var tags []string
		if ts := col(row, "tags"); ts != "" {
			for _, tg := range strings.Split(ts, ";") {
				if tg = strings.TrimSpace(tg); tg != "" {
					tags = append(tags, tg)
				}
			}
		}

		tid := col(row, "id")
		if tid == "" {
			tid = id.New()
		}

		// The documented "date,payee,amount,account" shape (C27) has no desc
		// column, but the ledger requires a description — so fall back to the
		// payee, which is the human-facing label, when desc is absent. An explicit
		// desc column still takes precedence.
		desc := col(row, "desc")
		if desc == "" {
			desc = col(row, "payee")
		}

		out = append(out, domain.Transaction{
			ID:                tid,
			AccountID:         colID(row, "account"),
			Date:              date,
			Payee:             col(row, "payee"),
			Desc:              desc,
			CategoryID:        colID(row, "category"),
			Amount:            money.New(amt, curr),
			TransferAccountID: colID(row, "transfer_account"),
			Cleared:           cleared,
			Tags:              tags,
			MemberID:          colID(row, "member"),
		})
	}
	return out, skipped, nil
}

// TransactionsFromCSV parses transactions from CSV. Columns are matched by their
// header name (case-insensitive), so column order and extra columns are
// tolerated. Rows missing an id get a fresh one. Only amount is required: when a
// row has no currency column/value, defaultCurrency is used (the caller passes
// the base currency), so the documented `date,payee,amount,account` shape works
// without an explicit currency column (C27). The account/category/member columns
// are read from either the export's `*_id` headers or the friendly `account`/
// `category`/`member` names; values given as names are resolved to ids by the
// caller (appstate), which has the entity lists.
func TransactionsFromCSV(data []byte, defaultCurrency string) ([]domain.Transaction, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("store: csv read: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	idx := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		idx[strings.ToLower(strings.TrimSpace(name))] = i
	}
	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	// colID reads an entity reference from either the export header (`<base>_id`)
	// or the friendly documented name (`<base>`), preferring the explicit id.
	colID := func(row []string, base string) string {
		if v := col(row, base+"_id"); v != "" {
			return v
		}
		return col(row, base)
	}

	out := make([]domain.Transaction, 0, len(records)-1)
	for n, row := range records[1:] {
		line := n + 2

		amtStr := col(row, "amount")
		if amtStr == "" {
			return nil, fmt.Errorf("store: csv line %d: amount is required", line)
		}
		curr := col(row, "currency")
		if curr == "" {
			curr = defaultCurrency
		}
		if curr == "" {
			return nil, fmt.Errorf("store: csv line %d: currency is required (add a currency column or set a base currency)", line)
		}
		amt, err := money.ParseMinor(amtStr, currency.Decimals(curr))
		if err != nil {
			return nil, fmt.Errorf("store: csv line %d: %w", line, err)
		}

		var date time.Time
		if ds := col(row, "date"); ds != "" {
			if date, err = dateutil.ParseDate(ds); err != nil {
				return nil, fmt.Errorf("store: csv line %d: %w", line, err)
			}
		}

		cleared := false
		if cs := col(row, "cleared"); cs != "" {
			if cleared, err = strconv.ParseBool(cs); err != nil {
				return nil, fmt.Errorf("store: csv line %d: invalid cleared %q", line, cs)
			}
		}

		var tags []string
		if ts := col(row, "tags"); ts != "" {
			for _, tg := range strings.Split(ts, ";") {
				if tg = strings.TrimSpace(tg); tg != "" {
					tags = append(tags, tg)
				}
			}
		}

		tid := col(row, "id")
		if tid == "" {
			tid = id.New()
		}

		// The documented "date,payee,amount,account" shape (C27) has no desc
		// column, but the ledger requires a description — so fall back to the
		// payee, which is the human-facing label, when desc is absent. An explicit
		// desc column still takes precedence.
		desc := col(row, "desc")
		if desc == "" {
			desc = col(row, "payee")
		}

		out = append(out, domain.Transaction{
			ID:                tid,
			AccountID:         colID(row, "account"),
			Date:              date,
			Payee:             col(row, "payee"),
			Desc:              desc,
			CategoryID:        colID(row, "category"),
			Amount:            money.New(amt, curr),
			TransferAccountID: colID(row, "transfer_account"),
			Cleared:           cleared,
			Tags:              tags,
			MemberID:          colID(row, "member"),
		})
	}
	return out, nil
}
