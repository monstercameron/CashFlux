// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/csvsafe"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

func firstOwner(t domain.Transaction) string {
	owners := t.Owners()
	if len(owners) == 0 {
		return ""
	}
	return owners[0]
}

func ownerList(id string) []string {
	if id == "" {
		return nil
	}
	return []string{id}
}

func parseCSVOwners(encoded, legacy string) ([]string, error) {
	if strings.TrimSpace(encoded) == "" {
		return ownerList(legacy), nil
	}
	var owners []string
	if err := json.Unmarshal([]byte(encoded), &owners); err != nil {
		return nil, err
	}
	txn := domain.Transaction{MemberIDs: owners}
	return txn.Owners(), nil
}

// csvHeader is the stable column order for transaction CSV export.
var csvHeader = []string{
	"id", "date", "account_id", "payee", "desc", "category_id",
	"amount", "currency", "transfer_account_id", "cleared", "tags", "member_id", "source",
	// C680 appended after the published columns on purpose: other tools read them
	// positionally, so inserting beside transfer_account_id would shift their fields.
	"transfer_group_id", "cashflux_export", "member_ids",
	// v2-only fields are appended so established positional columns stay stable.
	"splits", "cleared_at", "legacy_member_id", "source_doc_id", "attachments", "custom",
	"reviewed", "exclude_from_reports", "balance_checkpoint_at", "note", "holding_id", "goal_id",
	"bill_account_id", "subscription_name",
}

// csvTextColumns names every exported column whose values are text. Keep this
// classification explicit so a new text field cannot become spreadsheet
// formula input merely because its position changed.
var csvTextColumns = map[string]struct{}{
	"id": {}, "account_id": {}, "payee": {}, "desc": {}, "category_id": {},
	"currency": {}, "transfer_account_id": {}, "tags": {}, "member_id": {},
	"source": {}, "transfer_group_id": {}, "cashflux_export": {}, "member_ids": {},
	"splits": {}, "legacy_member_id": {}, "source_doc_id": {}, "attachments": {},
	"custom": {}, "note": {}, "holding_id": {}, "goal_id": {},
	"bill_account_id": {}, "subscription_name": {},
}

// csvGuardedColumnIndexes is derived once from the stable export header. The
// exporter applies it to each row, so column order changes cannot bypass the
// text-column guard.
var csvGuardedColumnIndexes = guardedCSVColumnIndexes(csvHeader, csvTextColumns)

func guardedCSVColumnIndexes(header []string, textColumns map[string]struct{}) map[int]struct{} {
	indexes := make(map[int]struct{}, len(textColumns))
	for i, name := range header {
		if _, ok := textColumns[name]; ok {
			indexes[i] = struct{}{}
		}
	}
	return indexes
}

func guardCSVRow(row []string, indexes map[int]struct{}) {
	for i := range row {
		if _, ok := indexes[i]; ok {
			row[i] = csvsafe.Guard(row[i])
		}
	}
}

const cashFluxCSVExportMarker = "cashflux-csv-v2"

const cashFluxCSVExportMarkerV1 = "cashflux-csv-v1"

// TransactionsToCSV serializes transactions to CSV with a header row. Amounts
// are written as plain decimals in the transaction's currency.
func TransactionsToCSV(txns []domain.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, t := range txns {
		jsonCell := func(name string, value any) (string, error) {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("store: csv transaction %s %s: %w", t.ID, name, err)
			}
			return string(encoded), nil
		}
		splits, err := jsonCell("splits", t.Splits)
		if err != nil {
			return nil, err
		}
		owners, err := jsonCell("owners", t.MemberIDs)
		if err != nil {
			return nil, err
		}
		tags, err := jsonCell("tags", t.Tags)
		if err != nil {
			return nil, err
		}
		attachments, err := jsonCell("attachments", t.Attachments)
		if err != nil {
			return nil, err
		}
		custom, err := jsonCell("custom", t.Custom)
		if err != nil {
			return nil, err
		}
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
			tags,
			firstOwner(t), // Compatibility column for single-owner importers.
			string(t.Source),
			t.TransferGroupID,
			cashFluxCSVExportMarker,
			owners,
			splits,
			formatCSVTime(t.ClearedAt),
			t.MemberID,
			t.SourceDocID,
			attachments,
			custom,
			strconv.FormatBool(t.Reviewed),
			strconv.FormatBool(t.ExcludeFromReports),
			formatCSVTime(t.BalanceCheckpointAt),
			t.Note,
			t.HoldingID,
			t.GoalID,
			t.BillAccountID,
			t.SubscriptionName,
		}
		guardCSVRow(row, csvGuardedColumnIndexes)
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

// csvReader accepts UTF-8 CSV exported by CashFlux and common bank downloads.
// The delimiter is selected from the header line so quoted data rows are parsed
// by encoding/csv without guessing based on their contents.
func csvReader(data []byte) *csv.Reader {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	if header, _, err := bytes.Cut(data, []byte("\n")); err {
		if bytes.Count(header, []byte(";")) > bytes.Count(header, []byte(",")) {
			reader.Comma = ';'
		}
	}
	return reader
}

func isCashFluxCSVMarker(marker string) bool {
	marker = strings.TrimSpace(marker)
	return marker == cashFluxCSVExportMarker || marker == cashFluxCSVExportMarkerV1
}

func formatCSVTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func decodeCSVJSON(row []string, idx map[string]int, name string, native bool, target any) error {
	i, ok := idx[name]
	if !ok || i >= len(row) || strings.TrimSpace(row[i]) == "" {
		return nil
	}
	value := row[i]
	if native {
		value = csvsafe.Restore(value)
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return err
	}
	return nil
}

func applyCSVV2Fields(tx *domain.Transaction, row []string, idx map[string]int, native bool) error {
	jsonColumns := []struct {
		name   string
		target any
	}{
		{name: "splits", target: &tx.Splits},
		{name: "member_ids", target: &tx.MemberIDs},
		{name: "tags", target: &tx.Tags},
		{name: "attachments", target: &tx.Attachments},
		{name: "custom", target: &tx.Custom},
	}
	for _, column := range jsonColumns {
		if err := decodeCSVJSON(row, idx, column.name, native, column.target); err != nil {
			return fmt.Errorf("invalid %s JSON: %w", column.name, err)
		}
	}
	tx.MemberID = csvText(row, idx, "legacy_member_id", native)
	tx.SourceDocID = csvText(row, idx, "source_doc_id", native)
	var err error
	if tx.Reviewed, err = parseCSVBool(row, idx, "reviewed", native); err != nil {
		return err
	}
	if tx.ExcludeFromReports, err = parseCSVBool(row, idx, "exclude_from_reports", native); err != nil {
		return err
	}
	tx.Note = csvText(row, idx, "note", native)
	tx.HoldingID = csvText(row, idx, "holding_id", native)
	tx.GoalID = csvText(row, idx, "goal_id", native)
	tx.BillAccountID = csvText(row, idx, "bill_account_id", native)
	tx.SubscriptionName = csvText(row, idx, "subscription_name", native)
	timeColumns := []struct {
		name   string
		target *time.Time
	}{
		{name: "cleared_at", target: &tx.ClearedAt},
		{name: "balance_checkpoint_at", target: &tx.BalanceCheckpointAt},
	}
	for _, column := range timeColumns {
		value := csvText(row, idx, column.name, native)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return fmt.Errorf("invalid %s %q: %w", column.name, value, err)
		}
		*column.target = parsed
	}
	return nil
}

func parseCSVBool(row []string, idx map[string]int, name string, native bool) (bool, error) {
	value := csvText(row, idx, name, native)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}

func csvText(row []string, idx map[string]int, name string, native bool) string {
	i, ok := idx[name]
	if !ok || i >= len(row) {
		return ""
	}
	value := row[i]
	if native {
		return csvsafe.Restore(value)
	}
	return strings.TrimSpace(value)
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
	return TransactionsFromCSVResilientForAccounts(data, defaultCurrency, "", nil)
}

// CSVAccount supplies the identity and currency needed to parse rows for an account.
type CSVAccount struct {
	ID       string
	Name     string
	Currency string
}

// TransactionsFromCSVResilientForAccounts parses amounts using the currency of
// the resolved account. It preserves row failures for the import preflight.
func TransactionsFromCSVResilientForAccounts(data []byte, defaultCurrency, fallbackAccountID string, accountCurrencies map[string]string) (txns []domain.Transaction, skipped []CSVRowError, err error) {
	accounts := make([]CSVAccount, 0, len(accountCurrencies))
	for accountID, curr := range accountCurrencies {
		accounts = append(accounts, CSVAccount{ID: accountID, Currency: curr})
	}
	return parseCSVForAccountList(data, defaultCurrency, fallbackAccountID, accounts, false)
}

// TransactionsFromCSVResilientForAccountList parses CSV amounts using each row's
// explicit currency or its resolved account currency before converting to minor units.
// Account names are resolved case-insensitively; blank account cells use fallbackAccountID.
func TransactionsFromCSVResilientForAccountList(data []byte, defaultCurrency, fallbackAccountID string, accounts []CSVAccount) (txns []domain.Transaction, skipped []CSVRowError, err error) {
	return parseCSVForAccountList(data, defaultCurrency, fallbackAccountID, accounts, true)
}

func parseCSVForAccountList(data []byte, defaultCurrency, fallbackAccountID string, accounts []CSVAccount, allowExplicitCurrencyMismatch bool) (txns []domain.Transaction, skipped []CSVRowError, err error) {
	accountByName := make(map[string]CSVAccount, len(accounts)*2)
	accountByID := make(map[string]CSVAccount, len(accounts))
	for _, account := range accounts {
		accountByID[account.ID] = account
		accountByName[strings.ToLower(strings.TrimSpace(account.ID))] = account
		if account.Name != "" {
			accountByName[strings.ToLower(strings.TrimSpace(account.Name))] = account
		}
	}
	r := csvReader(data)
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
	_, srcCol := idx["source"] // whether the file carries provenance (vs a foreign CSV)
	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	isCashFluxExportRow := func(row []string) bool {
		i, ok := idx["cashflux_export"]
		return ok && i < len(row) && isCashFluxCSVMarker(row[i])
	}
	isV2ExportRow := func(row []string) bool {
		i, ok := idx["cashflux_export"]
		return ok && i < len(row) && strings.TrimSpace(row[i]) == cashFluxCSVExportMarker
	}
	fieldCol := func(row []string, name string, nativeExport bool) string {
		if i, ok := idx[name]; ok && i < len(row) {
			value := row[i]
			if nativeExport {
				return csvsafe.Restore(value)
			}
			return strings.TrimSpace(value)
		}
		return ""
	}
	colID := func(row []string, base string) string {
		if v := fieldCol(row, base+"_id", isCashFluxExportRow(row)); v != "" {
			return v
		}
		return fieldCol(row, base, isCashFluxExportRow(row))
	}
	textCol := func(row []string, name string, nativeExport bool) string {
		if i, ok := idx[name]; ok && i < len(row) {
			value := row[i]
			if nativeExport {
				return csvsafe.Restore(value)
			}
			return strings.TrimSpace(value)
		}
		return ""
	}

	out := make([]domain.Transaction, 0, len(records)-1)
	for n, row := range records[1:] {
		line := n + 2
		nativeExport := isCashFluxExportRow(row)

		accountID := colID(row, "account")
		if account := accountByName[strings.ToLower(strings.TrimSpace(accountID))]; account.ID != "" {
			accountID = account.ID
		} else if accountID != "" && fallbackAccountID != "" {
			if _, known := accountByID[accountID]; !known {
				accountID = fallbackAccountID
			}
		}
		accountForCurrency := accountByID[accountID]
		if accountID == "" {
			accountForCurrency = accountByID[fallbackAccountID]
		}
		amtStr := col(row, "amount")
		if amtStr == "" {
			skipped = append(skipped, CSVRowError{Line: line, Reason: "amount is required"})
			continue
		}
		curr := fieldCol(row, "currency", nativeExport)
		if curr == "" {
			curr = accountForCurrency.Currency
			if curr == "" {
				curr = defaultCurrency
			}
		}
		if curr == "" {
			skipped = append(skipped, CSVRowError{Line: line, Reason: "currency is required (add a currency column or set a base currency)"})
			continue
		}
		if !allowExplicitCurrencyMismatch && accountForCurrency.Currency != "" && !strings.EqualFold(curr, accountForCurrency.Currency) {
			skipped = append(skipped, CSVRowError{Line: line, Reason: fmt.Sprintf("currency %s does not match account currency %s", curr, accountForCurrency.Currency)})
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
		if ts := textCol(row, "tags", nativeExport); ts != "" {
			for _, tg := range strings.Split(ts, ";") {
				if !nativeExport {
					tg = strings.TrimSpace(tg)
				}
				if tg != "" {
					tags = append(tags, tg)
				}
			}
		}

		tid := fieldCol(row, "id", nativeExport)
		if tid == "" {
			tid = id.New()
		}

		// The documented "date,payee,amount,account" shape (C27) has no desc
		// column, but the ledger requires a description — so fall back to the
		// payee, which is the human-facing label, when desc is absent. An explicit
		// desc column still takes precedence.
		payee := textCol(row, "payee", nativeExport)
		desc := textCol(row, "desc", nativeExport)
		if desc == "" {
			desc = payee
		}

		var owners []string
		var ownerErr error
		if isV2ExportRow(row) {
			ownerErr = decodeCSVJSON(row, idx, "member_ids", true, &owners)
		} else {
			owners, ownerErr = parseCSVOwners(fieldCol(row, "member_ids", nativeExport), colID(row, "member"))
		}
		if ownerErr != nil {
			skipped = append(skipped, CSVRowError{Line: line, Reason: fmt.Sprintf("invalid member_ids: %v", ownerErr)})
			continue
		}
		txn := domain.Transaction{
			ID:                tid,
			AccountID:         accountID,
			Date:              date,
			Payee:             payee,
			Desc:              desc,
			CategoryID:        colID(row, "category"),
			Amount:            money.New(amt, curr),
			TransferAccountID: colID(row, "transfer_account"),
			TransferGroupID:   colID(row, "transfer_group"),
			Cleared:           cleared,
			Tags:              tags,
			MemberIDs:         owners,
			Source:            csvSource(fieldCol(row, "source", nativeExport), srcCol),
		}
		if txn.AccountID == "" {
			txn.AccountID = fallbackAccountID
		}
		if isV2ExportRow(row) {
			if err := applyCSVV2Fields(&txn, row, idx, true); err != nil {
				skipped = append(skipped, CSVRowError{Line: line, Reason: err.Error()})
				continue
			}
		}
		out = append(out, txn)
	}
	return out, skipped, nil
}

// csvSource reads the provenance from a CSV row. A valid source value is preserved
// (so an export round-trips losslessly, including an empty source). columnPresent is
// whether the CSV had a "source" header at all: when it did not, the file is a
// foreign upload whose provenance IS "imported"; when it did, an empty/unknown cell
// is kept empty so a round-tripped untagged row stays untagged.
func csvSource(raw string, columnPresent bool) domain.TxnSource {
	if s := domain.TxnSource(strings.TrimSpace(raw)); s.Valid() {
		return s
	}
	if !columnPresent {
		return domain.TxnSourceImported
	}
	return ""
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
// TransactionsFromCSV is the fail-fast compatibility wrapper around the resilient parser.
func TransactionsFromCSV(data []byte, defaultCurrency string) ([]domain.Transaction, error) {
	txns, skipped, err := TransactionsFromCSVResilient(data, defaultCurrency)
	if err != nil {
		return nil, err
	}
	if len(skipped) > 0 {
		return nil, fmt.Errorf("store: csv line %d: %s", skipped[0].Line, skipped[0].Reason)
	}
	return txns, nil
}
