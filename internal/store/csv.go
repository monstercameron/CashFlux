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

// TransactionsFromCSV parses transactions from CSV. Columns are matched by their
// header name (case-insensitive), so column order and extra columns are
// tolerated. Rows missing an id get a fresh one. amount + currency are required.
func TransactionsFromCSV(data []byte) ([]domain.Transaction, error) {
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

	out := make([]domain.Transaction, 0, len(records)-1)
	for n, row := range records[1:] {
		line := n + 2

		curr := col(row, "currency")
		amtStr := col(row, "amount")
		if amtStr == "" || curr == "" {
			return nil, fmt.Errorf("store: csv line %d: amount and currency are required", line)
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

		out = append(out, domain.Transaction{
			ID:                tid,
			AccountID:         col(row, "account_id"),
			Date:              date,
			Payee:             col(row, "payee"),
			Desc:              col(row, "desc"),
			CategoryID:        col(row, "category_id"),
			Amount:            money.New(amt, curr),
			TransferAccountID: col(row, "transfer_account_id"),
			Cleared:           cleared,
			Tags:              tags,
			MemberID:          col(row, "member_id"),
		})
	}
	return out, nil
}
