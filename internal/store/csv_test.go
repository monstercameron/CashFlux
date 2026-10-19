// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTransactionsCSVRoundTrip(t *testing.T) {
	day, _ := dateutil.ParseDate("2026-06-03")
	day2, _ := dateutil.ParseDate("2026-06-05")

	txns := []domain.Transaction{
		{
			ID: "t1", AccountID: "a1", Date: day, Payee: "Store, Inc.", Desc: "Groceries",
			CategoryID: "food", Amount: money.New(-24055, "USD"), Cleared: true,
			Tags: []string{"weekly", "essentials"}, MemberIDs: []string{"m1", "m2"},
		},
		{
			ID: "t2", AccountID: "a1", Date: day2, Desc: "Transfer to savings",
			Amount: money.New(-50000, "USD"), TransferAccountID: "a2",
		},
	}

	data, err := TransactionsToCSV(txns)
	if err != nil {
		t.Fatalf("to csv: %v", err)
	}
	if !strings.HasPrefix(string(data), "id,date,account_id") {
		t.Errorf("missing/wrong header: %q", string(data)[:40])
	}

	got, err := TransactionsFromCSV(data, "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != len(txns) {
		t.Fatalf("round trip rows = %d, want %d", len(got), len(txns))
	}
	for i := range got {
		want := txns[i]
		want.NormalizeOwners()
		if got[i].ID != want.ID || got[i].AccountID != want.AccountID || !got[i].Date.Equal(want.Date) ||
			got[i].Payee != want.Payee || got[i].Desc != want.Desc || got[i].CategoryID != want.CategoryID ||
			got[i].Amount != want.Amount || got[i].TransferAccountID != want.TransferAccountID ||
			got[i].Cleared != want.Cleared || strings.Join(got[i].Tags, ",") != strings.Join(want.Tags, ",") ||
			strings.Join(got[i].Owners(), ",") != strings.Join(want.Owners(), ",") {
			t.Errorf("round trip row %d mismatch:\n got: %+v\nwant: %+v", i, got[i], want)
		}
	}
}

func TestTransactionsCSVFormulaNeutralizationRoundTrip(t *testing.T) {
	dangerous := []string{
		`=HYPERLINK("https://attacker.invalid","open")`,
		"+1+1",
		"-1+1",
		"@SUM(A1:A2)",
		"\t=1+1",
		"\r=1+1",
	}
	fields := []struct {
		name string
		put  func(*domain.Transaction, string)
		get  func(domain.Transaction) string
		col  string
	}{
		{name: "payee", put: func(tx *domain.Transaction, value string) { tx.Payee = value }, get: func(tx domain.Transaction) string { return tx.Payee }, col: "payee"},
		{name: "description", put: func(tx *domain.Transaction, value string) { tx.Desc = value }, get: func(tx domain.Transaction) string { return tx.Desc }, col: "desc"},
		{name: "tag", put: func(tx *domain.Transaction, value string) { tx.Tags = []string{value} }, get: func(tx domain.Transaction) string { return tx.Tags[0] }, col: "tags"},
		{name: "transaction id", put: func(tx *domain.Transaction, value string) { tx.ID = value }, get: func(tx domain.Transaction) string { return tx.ID }, col: "id"},
		{name: "account id", put: func(tx *domain.Transaction, value string) { tx.AccountID = value }, get: func(tx domain.Transaction) string { return tx.AccountID }, col: "account_id"},
		{name: "category id", put: func(tx *domain.Transaction, value string) { tx.CategoryID = value }, get: func(tx domain.Transaction) string { return tx.CategoryID }, col: "category_id"},
		{name: "currency", put: func(tx *domain.Transaction, value string) { tx.Amount.Currency = value }, get: func(tx domain.Transaction) string { return tx.Amount.Currency }, col: "currency"},
		{name: "transfer account id", put: func(tx *domain.Transaction, value string) { tx.TransferAccountID = value }, get: func(tx domain.Transaction) string { return tx.TransferAccountID }, col: "transfer_account_id"},
		{name: "transfer group id", put: func(tx *domain.Transaction, value string) { tx.TransferGroupID = value }, get: func(tx domain.Transaction) string { return tx.TransferGroupID }, col: "transfer_group_id"},
		{name: "member id", put: func(tx *domain.Transaction, value string) { tx.MemberIDs = []string{value} }, get: func(tx domain.Transaction) string { return tx.Owners()[0] }, col: "member_id"},
	}
	for _, field := range fields {
		for _, value := range dangerous {
			t.Run(field.name+"/"+value, func(t *testing.T) {
				tx := domain.Transaction{ID: "formula", Amount: money.New(-450, "USD")}
				field.put(&tx, value)
				data, err := TransactionsToCSV([]domain.Transaction{tx})
				if err != nil {
					t.Fatalf("export: %v", err)
				}
				reader := csv.NewReader(strings.NewReader(string(data)))
				records, err := reader.ReadAll()
				if err != nil {
					t.Fatalf("read exported CSV: %v", err)
				}
				columns := make(map[string]int, len(records[0]))
				for i, header := range records[0] {
					columns[header] = i
				}
				cell := records[1][columns[field.col]]
				if field.name == "tag" {
					var tags []string
					if err := json.Unmarshal([]byte(cell), &tags); err != nil || len(tags) != 1 || tags[0] != value {
						t.Errorf("exported tags JSON = %q, want one original tag %q (err=%v)", cell, value, err)
					}
				} else if cell != "'"+value {
					t.Errorf("exported %s = %q, want guarded %q", field.col, cell, "'"+value)
				}
				if amount := records[1][columns["amount"]]; amount != "-4.50" {
					t.Errorf("amount = %q, want numeric -4.50", amount)
				}
				got, err := TransactionsFromCSV(data, "")
				if err != nil {
					t.Fatalf("import exported CSV: %v", err)
				}
				if len(got) != 1 {
					t.Fatalf("round trip returned %d transactions, want 1", len(got))
				}
				wantValue := value
				if field.name == "currency" {
					wantValue = strings.ToUpper(strings.TrimSpace(value))
				}
				if roundTripped := field.get(got[0]); roundTripped != wantValue {
					t.Errorf("round trip text = %q, want %q", roundTripped, wantValue)
				}
				resilient, skipped, err := TransactionsFromCSVResilient(data, "")
				if err != nil || len(skipped) != 0 || len(resilient) != 1 {
					t.Fatalf("resilient import = %v, skipped=%v, err=%v; want one transaction", resilient, skipped, err)
				}
				if roundTripped := field.get(resilient[0]); roundTripped != wantValue {
					t.Errorf("resilient round trip text = %q, want %q", roundTripped, wantValue)
				}
			})
		}
	}
}

func TestTransactionsCSVV2RoundTripsEveryField(t *testing.T) {
	day, err := dateutil.ParseDate("2026-10-18")
	if err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{
		ID: "txn-1", AccountID: "account-1", Date: day, Payee: "Cafe, Inc.", Desc: "Lunch",
		CategoryID: "food", Amount: money.New(-12345, "USD"),
		Splits:            []domain.CategorySplit{{CategoryID: "dining", Amount: money.New(-10000, "USD"), MemberID: "member-1"}},
		TransferAccountID: "account-2", TransferGroupID: "transfer-1", Cleared: true,
		ClearedAt: time.Date(2026, 10, 19, 12, 30, 45, 123, time.UTC),
		Tags:      []string{"food; work", "=formula-looking tag"}, MemberIDs: []string{"member-1", "member-2"},
		MemberID: "legacy-member", Source: domain.TxnSourceScanned, SourceDocID: "doc-1",
		Attachments: []domain.AttachmentRef{{ArtifactID: "artifact-1", Name: "receipt.pdf", Kind: "receipt", MIME: "application/pdf"}},
		Custom:      map[string]any{"merchantCode": "=raw text", "reviewCount": float64(2), "enabled": true},
		Reviewed:    true, ExcludeFromReports: true,
		BalanceCheckpointAt: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC),
		Note:                "split with Alex; settle Friday", HoldingID: "holding-1", GoalID: "goal-1",
		BillAccountID: "liability-1", SubscriptionName: "Music plan",
	}

	data, err := TransactionsToCSV([]domain.Transaction{txn})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(string(data), cashFluxCSVExportMarker) {
		t.Fatalf("export missing v2 marker: %q", data)
	}
	got, err := TransactionsFromCSV(data, "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("import returned %d rows, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], txn) {
		t.Errorf("round trip mismatch:\n got: %#v\nwant: %#v", got[0], txn)
	}
	resilient, skipped, err := TransactionsFromCSVResilient(data, "")
	if err != nil || len(skipped) != 0 || len(resilient) != 1 {
		t.Fatalf("resilient import = %v, skipped=%v, err=%v", resilient, skipped, err)
	}
	if !reflect.DeepEqual(resilient[0], txn) {
		t.Errorf("resilient round trip mismatch:\n got: %#v\nwant: %#v", resilient[0], txn)
	}
}

func TestTransactionsCSVV1RemainsReadable(t *testing.T) {
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)
	if err := w.Write([]string{"id", "date", "account_id", "payee", "desc", "category_id", "amount", "currency", "transfer_account_id", "cleared", "tags", "member_id", "source", "transfer_group_id", "cashflux_export", "member_ids"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Write([]string{"old", "2026-06-03", "a1", "Coffee", "Coffee", "food", "-4.50", "USD", "", "true", "weekly", "m1", "manual", "tg-1", cashFluxCSVExportMarkerV1, `["m1"]`}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	got, err := TransactionsFromCSV(buffer.Bytes(), "")
	if err != nil {
		t.Fatalf("read v1 export: %v", err)
	}
	if len(got) != 1 || got[0].ID != "old" || got[0].TransferGroupID != "tg-1" || got[0].Source != domain.TxnSourceManual || !got[0].Cleared || !got[0].HasOwner("m1") {
		t.Fatalf("v1 import mismatch: %+v", got)
	}
	resilient, skipped, err := TransactionsFromCSVResilient(buffer.Bytes(), "")
	if err != nil || len(skipped) != 0 || len(resilient) != 1 || resilient[0].ID != "old" || resilient[0].TransferGroupID != "tg-1" {
		t.Fatalf("resilient v1 import = %+v, skipped=%+v, err=%v", resilient, skipped, err)
	}
}

func TestCSVImportBOMAndDelimiters(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "comma bank CSV with BOM", data: "\ufeffdate,payee,amount,currency\n2026-06-01,\"Coffee, Inc.\",-4.50,USD\n", want: "Coffee, Inc."},
		{name: "semicolon bank CSV with BOM", data: "\ufeffdate;payee;amount;currency\n2026-06-01;\"Coffee; Shop\";-4.50;USD\n", want: "Coffee; Shop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransactionsFromCSV([]byte(tt.data), "")
			if err != nil {
				t.Fatalf("strict import: %v", err)
			}
			if len(got) != 1 || got[0].Payee != tt.want || got[0].Amount.Amount != -450 {
				t.Fatalf("strict import = %+v", got)
			}
			resilient, skipped, err := TransactionsFromCSVResilient([]byte(tt.data), "")
			if err != nil || len(skipped) != 0 || len(resilient) != 1 || resilient[0].Payee != tt.want {
				t.Fatalf("resilient import = %+v, skipped=%+v, err=%v", resilient, skipped, err)
			}
		})
	}
}

func TestTransactionsCSVGuardDoesNotChangeNonTextColumns(t *testing.T) {
	day, _ := dateutil.ParseDate("2026-06-03")
	tx := domain.Transaction{ID: "t1", Date: day, Amount: money.New(-450, "USD"), Cleared: true}
	data, err := TransactionsToCSV([]domain.Transaction{tx})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	columns := make(map[string]int, len(records[0]))
	for i, header := range records[0] {
		columns[header] = i
	}
	row := records[1]
	if got := row[columns["amount"]]; got != "-4.50" {
		t.Errorf("amount = %q, want unguarded numeric -4.50", got)
	}
	if got := row[columns["date"]]; got != "2026-06-03" {
		t.Errorf("date = %q, want unguarded date", got)
	}
	if got := row[columns["cleared"]]; got != "true" {
		t.Errorf("cleared = %q, want unguarded boolean", got)
	}
}

func TestTransactionsCSVGuardsEveryNamedTextColumn(t *testing.T) {
	nonTextColumns := map[string]struct{}{
		"date": {}, "amount": {}, "cleared": {}, "cleared_at": {}, "reviewed": {},
		"exclude_from_reports": {}, "balance_checkpoint_at": {},
	}
	seenText := make(map[string]struct{}, len(csvTextColumns))
	fixture := make([]string, len(csvHeader))
	for i, header := range csvHeader {
		_, text := csvTextColumns[header]
		_, nonText := nonTextColumns[header]
		if text == nonText {
			t.Errorf("header %q must be classified exactly once as text or non-text", header)
		}
		if text {
			seenText[header] = struct{}{}
			fixture[i] = `=HYPERLINK("x")`
		}
	}
	if len(seenText) != len(csvTextColumns) {
		t.Errorf("text column set has %d entries, but header accounts for %d", len(csvTextColumns), len(seenText))
	}

	indexes := guardedCSVColumnIndexes(csvHeader, csvTextColumns)
	guardCSVRow(fixture, indexes)
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)
	if err := w.Write(fixture); err != nil {
		t.Fatalf("write guarded fixture: %v", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatalf("flush guarded fixture: %v", err)
	}
	reader := csv.NewReader(&buffer)
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("read guarded fixture: %v", err)
	}
	for i, header := range csvHeader {
		if _, text := csvTextColumns[header]; text && row[i] != "'=HYPERLINK(\"x\")" {
			t.Errorf("exported fixture cell %q = %q, want guarded formula", header, row[i])
		}
		if _, nonText := nonTextColumns[header]; nonText && row[i] != "" {
			t.Errorf("non-text fixture cell %q was unexpectedly guarded: %q", header, row[i])
		}
	}
}

func TestTransactionsCSVImportKeepsUnmarkedGuardQuote(t *testing.T) {
	input := "payee,desc,amount,currency\n'=1+1,\"'@SUM(A1:A2)\",-4.50,USD\n"
	got, err := TransactionsFromCSV([]byte(input), "")
	if err != nil {
		t.Fatalf("import foreign CSV: %v", err)
	}
	if got[0].Payee != "'=1+1" || got[0].Desc != "'@SUM(A1:A2)" {
		t.Errorf("foreign text changed: payee=%q desc=%q", got[0].Payee, got[0].Desc)
	}
	if got[0].Amount.Amount != -450 {
		t.Errorf("negative amount changed: %d", got[0].Amount.Amount)
	}
}

func TestCSVImportGeneratesMissingID(t *testing.T) {
	in := "date,account_id,desc,amount,currency\n2026-06-03,a1,Coffee,-4.50,USD\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d transactions, want 1", len(got))
	}
	if got[0].ID == "" {
		t.Error("expected a generated id")
	}
	if got[0].Amount.Amount != -450 || got[0].Amount.Currency != "USD" {
		t.Errorf("amount = %v, want -450 USD", got[0].Amount)
	}
}

func TestCSVImportColumnOrderTolerant(t *testing.T) {
	// Reordered + extra column.
	in := "amount,currency,account_id,extra,id\n-4.50,USD,a1,ignored,tx9\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got[0].ID != "tx9" || got[0].AccountID != "a1" || got[0].Amount.Amount != -450 {
		t.Errorf("parsed wrong: %+v", got[0])
	}
}

func TestCSVImportDefaultsCurrencyAndFriendlyColumns(t *testing.T) {
	// The documented hand-written shape: no currency column, friendly column
	// names (account/category/member, not *_id). Currency defaults to the
	// caller-supplied base; the friendly columns are read into the id fields
	// (appstate resolves any names to ids). (C27)
	in := "date,payee,amount,account,category,member\n2026-06-10,Coffee Bar,-4.50,Checking,Food,Alex\n"
	got, err := TransactionsFromCSV([]byte(in), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	g := got[0]
	if g.Amount.Amount != -450 || g.Amount.Currency != "USD" {
		t.Errorf("amount = %v, want -450 USD (defaulted)", g.Amount)
	}
	if g.Payee != "Coffee Bar" || g.AccountID != "Checking" || g.CategoryID != "Food" || !g.HasOwner("Alex") {
		t.Errorf("friendly columns not read: %+v", g)
	}
	// The payee fills the (required) description when no desc column is present,
	// so the documented shape actually imports rather than failing validation.
	if g.Desc != "Coffee Bar" {
		t.Errorf("Desc = %q, want the payee fallback %q", g.Desc, "Coffee Bar")
	}
}

func TestCSVImportDescFallsBackToPayee(t *testing.T) {
	// No desc column (the documented date,payee,amount,account shape): the ledger
	// requires a description, so it falls back to the payee.
	got, err := TransactionsFromCSV([]byte("date,payee,amount,account\n2026-06-05,Rent,-1200.00,Checking\n"), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 || got[0].Desc != "Rent" {
		t.Errorf("Desc = %q, want payee fallback \"Rent\"", got[0].Desc)
	}
	// An explicit desc column takes precedence over the payee.
	got2, err := TransactionsFromCSV([]byte("date,payee,desc,amount,account\n2026-06-05,Rent,Monthly rent,-1200.00,Checking\n"), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got2[0].Desc != "Monthly rent" {
		t.Errorf("Desc = %q, want explicit \"Monthly rent\"", got2[0].Desc)
	}
	if got2[0].Payee != "Rent" {
		t.Errorf("Payee = %q, want \"Rent\"", got2[0].Payee)
	}
}

func TestCSVImportExportIDColumnsWinOverFriendly(t *testing.T) {
	// When both account_id and account are present, the explicit id wins.
	in := "amount,currency,account_id,account\n-1.00,USD,acc-123,Checking\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got[0].AccountID != "acc-123" {
		t.Errorf("account_id should win: got %q", got[0].AccountID)
	}
}

func TestCSVImportErrors(t *testing.T) {
	cases := []string{
		"date,account_id,amount\n2026-06-03,a1,-4.50\n", // no currency column AND no default → error
		"amount,currency\nnotnumber,USD\n",              // bad amount
		"amount,currency,date\n-4.50,USD,nope\n",        // bad date
		"date,account_id\n2026-06-03,a1\n",              // missing amount
	}
	for i, in := range cases {
		if _, err := TransactionsFromCSV([]byte(in), ""); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestTransactionsFromCSVResilient(t *testing.T) {
	t.Run("all_valid", func(t *testing.T) {
		in := "date,payee,amount,currency\n2026-06-01,Coffee,-4.50,USD\n2026-06-02,Salary,2000.00,USD\n"
		txns, skipped, err := TransactionsFromCSVResilient([]byte(in), "")
		if err != nil {
			t.Fatalf("unexpected structural error: %v", err)
		}
		if len(txns) != 2 {
			t.Errorf("got %d txns, want 2", len(txns))
		}
		if len(skipped) != 0 {
			t.Errorf("got %d skipped, want 0: %+v", len(skipped), skipped)
		}
	})

	t.Run("some_bad_rows", func(t *testing.T) {
		// Row on line 2 = missing amount, line 3 = valid, line 4 = non-numeric amount, lines 5+6 = valid.
		in := strings.Join([]string{
			"date,payee,amount,currency",
			"2026-06-01,Bad1,,USD",           // line 2: missing amount
			"2026-06-02,Good1,-4.50,USD",     // line 3: valid
			"2026-06-03,Bad2,notanumber,USD", // line 4: non-numeric amount
			"2026-06-04,Good2,100.00,USD",    // line 5: valid
			"2026-06-05,Good3,-22.00,USD",    // line 6: valid
		}, "\n") + "\n"

		txns, skipped, err := TransactionsFromCSVResilient([]byte(in), "")
		if err != nil {
			t.Fatalf("unexpected structural error: %v", err)
		}
		if len(txns) != 3 {
			t.Errorf("got %d valid txns, want 3", len(txns))
		}
		if len(skipped) != 2 {
			t.Errorf("got %d skipped, want 2: %+v", len(skipped), skipped)
		}
		// Check line numbers.
		lines := make(map[int]bool, len(skipped))
		for _, s := range skipped {
			lines[s.Line] = true
		}
		if !lines[2] {
			t.Errorf("expected line 2 in skipped; got %+v", skipped)
		}
		if !lines[4] {
			t.Errorf("expected line 4 in skipped; got %+v", skipped)
		}
		// Reasons must be non-empty.
		for _, s := range skipped {
			if s.Reason == "" {
				t.Errorf("skipped row at line %d has empty reason", s.Line)
			}
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		txns, skipped, err := TransactionsFromCSVResilient([]byte(""), "USD")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(txns) != 0 || len(skipped) != 0 {
			t.Errorf("expected empty results, got txns=%d skipped=%d", len(txns), len(skipped))
		}
	})

	t.Run("header_only", func(t *testing.T) {
		txns, skipped, err := TransactionsFromCSVResilient([]byte("date,payee,amount,currency\n"), "USD")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(txns) != 0 || len(skipped) != 0 {
			t.Errorf("expected empty results for header-only, got txns=%d skipped=%d", len(txns), len(skipped))
		}
	})

	t.Run("totally_malformed", func(t *testing.T) {
		// A bare quote triggers a CSV parse error — structural failure, not a skipped row.
		_, _, err := TransactionsFromCSVResilient([]byte("date,amount\n\"unclosed\n"), "USD")
		if err == nil {
			t.Error("expected structural error for malformed CSV")
		}
	})
}

// TestCSVResilientScale verifies that TransactionsFromCSVResilient handles a
// 600-row corpus at scale (L23): all valid rows are imported, scattered malformed
// rows are skipped with reasons, and the function never aborts early. This guards
// against any regression that silently returns a partial result or panics on bulk
// data.
func TestCSVResilientScale(t *testing.T) {
	const totalRows = 600
	// Malformed rows at a spread of positions (1-based data row number).
	malformedAt := map[int]bool{1: true, 50: true, 200: true, 350: true, 599: true}
	wantSkipped := len(malformedAt)
	wantImported := totalRows - wantSkipped

	var sb strings.Builder
	sb.WriteString("date,payee,amount,currency\n")
	for i := 1; i <= totalRows; i++ {
		day := fmt.Sprintf("2024-%02d-%02d", (i%12)+1, (i%28)+1)
		if malformedAt[i] {
			// Missing amount — guaranteed skip by the resilient parser.
			fmt.Fprintf(&sb, "%s,Malformed row %d,,USD\n", day, i)
		} else {
			fmt.Fprintf(&sb, "%s,Payee %d,-%.2f,USD\n", day, i, float64(i)*1.23)
		}
	}

	txns, skipped, err := TransactionsFromCSVResilient([]byte(sb.String()), "")
	if err != nil {
		t.Fatalf("unexpected structural error on 600-row corpus: %v", err)
	}
	if len(txns) != wantImported {
		t.Errorf("imported %d rows, want %d", len(txns), wantImported)
	}
	if len(skipped) != wantSkipped {
		t.Errorf("skipped %d rows, want %d: %+v", len(skipped), wantSkipped, skipped)
	}
	for _, s := range skipped {
		if s.Reason == "" {
			t.Errorf("skipped row at line %d has empty reason", s.Line)
		}
		if s.Line < 2 || s.Line > totalRows+1 {
			t.Errorf("skipped line number %d out of expected range [2, %d]", s.Line, totalRows+1)
		}
	}
	// Every imported transaction must have a non-empty ID (generated when absent)
	// and a valid non-zero amount.
	for _, tx := range txns {
		if tx.ID == "" {
			t.Error("imported transaction missing ID")
		}
		if tx.Amount.Amount == 0 {
			t.Errorf("imported transaction %s has zero amount", tx.ID)
		}
	}
}
