// SPDX-License-Identifier: MIT
package assistantdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestQueryReadsBeyondPromptLimitWithExactMoneyAndStableContinuation(t *testing.T) {
	rows := make([]domain.Transaction, 113)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprintf("t%03d", 112-i), AccountID: "eur", Date: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), Amount: money.New(math.MaxInt64, "EUR"), Desc: "café grocery"}
	}
	c := NewCollection("transactions", "id", func() ([]domain.Transaction, error) { return rows, nil })
	m, e := Describe(c)
	if e != nil {
		t.Fatal(e)
	}
	if m.Total != 113 {
		t.Fatal(m)
	}
	for _, want := range []string{"amount.Amount", "amount.Currency", "date", "splits[].amount.Amount"} {
		found := false
		for _, f := range m.Fields {
			if f.Path == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing exact field %s", want)
		}
	}
	q := Query{Collection: c.Name, Limit: 17, Equals: map[string]json.RawMessage{"amount.Currency": json.RawMessage(`"EUR"`)}, From: "2026-10-19", Through: "2026-10-19", DateField: "date", Search: "CAFÉ"}
	seen := map[string]bool{}
	for {
		p, e := ReadPage(c, q)
		if e != nil {
			t.Fatal(e)
		}
		if p.Total != 113 || p.FilteredTotal != 113 {
			t.Fatal(p)
		}
		for _, r := range p.Records {
			if seen[r.ID] {
				t.Fatal("duplicate page", r.ID)
			}
			seen[r.ID] = true
			if !strings.Contains(string(r.Data), `"Amount":9223372036854775807`) {
				t.Fatalf("money lost precision: %s", r.Data)
			}
		}
		if p.NextOffset == nil {
			if p.Truncated {
				t.Fatal("false truncation")
			}
			break
		}
		if !p.Truncated {
			t.Fatal("missing truncation")
		}
		q.Offset = *p.NextOffset
		q.Snapshot = p.Snapshot
	}
	if len(seen) != 113 {
		t.Fatal(len(seen))
	}
	// Collection enumeration order is not a financial snapshot change.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if _, e := ReadPage(c, q); e != nil {
		t.Fatal(e)
	}
	rows[0].Desc = "external edit"
	if _, e := ReadPage(c, q); e == nil || !strings.Contains(e.Error(), "snapshot changed") {
		t.Fatalf("stale continuation accepted: %v", e)
	}
}

func TestQueryExplicitFiltersDatesArchivedAndFailures(t *testing.T) {
	rows := []domain.Account{{ID: "a", Archived: true, Currency: "USD", Name: "retained"}, {ID: "b", Currency: "EUR", Name: "live"}}
	c := NewCollection("accounts", "id", func() ([]domain.Account, error) { return rows, nil })
	p, e := ReadPage(c, Query{Collection: "accounts"})
	if e != nil || p.Total != 2 || len(p.Records) != 2 {
		t.Fatalf("archived omitted: %+v %v", p, e)
	}
	p, e = ReadPage(c, Query{Collection: "accounts", Equals: map[string]json.RawMessage{"archived": json.RawMessage(`true`)}})
	if e != nil || p.FilteredTotal != 1 || p.Records[0].ID != "a" {
		t.Fatalf("archive filter: %+v %v", p, e)
	}
	for _, q := range []Query{{Collection: "accounts", Offset: 1}, {Collection: "accounts", Limit: 51}, {Collection: "accounts", Offset: -1}, {Collection: "accounts", From: "2026-02-30", DateField: "balanceAsOf"}, {Collection: "accounts", From: "2026-01-01"}, {Collection: "accounts", Equals: map[string]json.RawMessage{"typo": json.RawMessage(`true`)}}, {Collection: "accounts", Equals: map[string]json.RawMessage{"currency": json.RawMessage(`{}`)}}} {
		if _, err := ReadPage(c, q); err == nil {
			t.Errorf("invalid request accepted: %+v", q)
		}
	}
	fault := errors.New("read unavailable")
	bad := NewCollection("accounts", "id", func() ([]domain.Account, error) { return nil, fault })
	if _, e := Describe(bad); !errors.Is(e, fault) {
		t.Fatal(e)
	}
	if _, e := ReadPage(bad, Query{Collection: "accounts"}); !errors.Is(e, fault) {
		t.Fatal(e)
	}
	dup := NewCollection("accounts", "id", func() ([]domain.Account, error) { return []domain.Account{{ID: "a"}, {ID: "a"}}, nil })
	if _, e := Describe(dup); e == nil {
		t.Fatal("duplicate identity accepted")
	}
}

func TestReadPageBoundsCompleteEnvelopeAndContinuesAcrossRedactedUnicodeRows(t *testing.T) {
	type note struct {
		ID     string            `json:"id"`
		Text   string            `json:"text"`
		Custom map[string]string `json:"custom"`
	}
	rows := make([]note, 50)
	for i := range rows {
		custom := make(map[string]string, 32)
		for field := range 32 {
			custom[fmt.Sprintf("api_key_%03d", field)] = "secret"
		}
		rows[i] = note{ID: fmt.Sprintf("n%03d", i), Text: "café 🪙", Custom: custom}
	}
	c := NewCollection("notes", "id", func() ([]note, error) { return rows, nil })
	q := Query{Collection: c.Name, Limit: 50}
	seen := make(map[string]bool, len(rows))
	var snapshot string
	for {
		p, err := ReadPage(c, q)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(p)
		if err != nil || len(encoded) > maxOutput {
			t.Fatalf("encoded page bytes=%d err=%v budget=%d", len(encoded), err, maxOutput)
		}
		if p.Total != len(rows) || p.FilteredTotal != len(rows) || p.Offset != q.Offset {
			t.Fatalf("page totals or offset changed: %+v", p)
		}
		if snapshot != "" && p.Snapshot != snapshot {
			t.Fatal("snapshot changed between pages")
		}
		snapshot = p.Snapshot
		for _, row := range p.Records {
			if seen[row.ID] {
				t.Fatalf("duplicate row across pages: %s", row.ID)
			}
			seen[row.ID] = true
			if len(row.RedactedPaths) != 32 || !strings.Contains(string(row.Data), `"text":"café 🪙"`) {
				t.Fatalf("redacted metadata or Unicode row data lost: %+v", row)
			}
		}
		if p.NextOffset == nil {
			if p.Truncated {
				t.Fatal("last page marked truncated")
			}
			break
		}
		if len(p.Records) == 0 || !p.Truncated || *p.NextOffset != p.Offset+len(p.Records) {
			t.Fatalf("invalid empty or discontinuous continuation: %+v", p)
		}
		q.Offset, q.Snapshot = *p.NextOffset, p.Snapshot
	}
	if len(seen) != len(rows) {
		t.Fatalf("read %d of %d rows", len(seen), len(rows))
	}
}

func TestReadPageOversizedFirstRowOffersDetailRecovery(t *testing.T) {
	type note struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	c := NewCollection("notes", "id", func() ([]note, error) {
		return []note{{ID: "large-content", Text: strings.Repeat("x", maxOutput-500)}}, nil
	})
	p, err := ReadPage(c, Query{Collection: c.Name})
	if err != nil || len(p.Records) != 1 || !p.Records[0].DetailRequired || len(p.Records[0].Data) != 0 || p.Records[0].JSONCharacters == 0 {
		t.Fatalf("near-budget first row did not return a detail placeholder: page=%+v err=%v", p, err)
	}
	detail, err := ReadRecord(c, DetailRequest{Collection: c.Name, ID: p.Records[0].ID, Field: "text"})
	if err != nil || detail.JSONText == "" || detail.TotalCharacters != maxOutput-498 {
		t.Fatalf("exact record detail did not recover first row: characters=%d text=%d err=%v", detail.TotalCharacters, len(detail.JSONText), err)
	}
}

func TestReadPageOversizedRedactionMetadataErrorIdentifiesRecord(t *testing.T) {
	type note struct {
		ID     string            `json:"id"`
		Custom map[string]string `json:"custom"`
	}
	custom := make(map[string]string, 1800)
	for i := range 1800 {
		custom[fmt.Sprintf("api_key_%04d", i)] = "secret"
	}
	c := NewCollection("notes", "id", func() ([]note, error) {
		return []note{{ID: "large-metadata", Custom: custom}}, nil
	})
	if _, err := ReadPage(c, Query{Collection: c.Name}); err == nil || !strings.Contains(err.Error(), `collection "notes" record "large-metadata"`) || !strings.Contains(err.Error(), "record detail read") {
		t.Fatalf("oversized redaction metadata error lacked an actionable record identity: %v", err)
	}
}

func TestQueryLongRecordIsFullyReachableInUnicodeChunks(t *testing.T) {
	type item struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	text := strings.Repeat("café🪙\"\\\n\t<&>", 8000)
	c := NewCollection("notes", "id", func() ([]item, error) { return []item{{"long", text}}, nil })
	p, e := ReadPage(c, Query{Collection: "notes"})
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Records) != 1 || !p.Records[0].DetailRequired || len(p.Records[0].Data) != 0 {
		t.Fatal("large record silently truncated", p)
	}
	q := DetailRequest{Collection: "notes", ID: "long", Field: "text", Limit: maxOutput}
	var b strings.Builder
	expectedOffset := 0
	fieldJSON, _ := json.Marshal(text)
	expectedCharacters := len([]rune(string(fieldJSON)))
	var snapshot string
	for {
		d, e := ReadRecord(c, q)
		if e != nil {
			t.Fatal(e)
		}
		encoded, err := json.Marshal(d)
		if err != nil || len(encoded) > maxDetailBytes {
			t.Fatalf("detail envelope bytes=%d err=%v budget=%d", len(encoded), err, maxDetailBytes)
		}
		if d.Offset != expectedOffset || len([]rune(d.JSONText)) > q.Limit || d.TotalCharacters != expectedCharacters {
			t.Fatalf("invalid chunk offsets/limit/total: offset=%d expected=%d chars=%d limit=%d total=%d expectedTotal=%d", d.Offset, expectedOffset, len([]rune(d.JSONText)), q.Limit, d.TotalCharacters, expectedCharacters)
		}
		if snapshot != "" && d.Snapshot != snapshot {
			t.Fatal("snapshot changed between detail chunks")
		}
		snapshot = d.Snapshot
		b.WriteString(d.JSONText)
		if d.NextOffset == nil {
			break
		}
		expectedOffset = *d.NextOffset
		q.Offset = *d.NextOffset
		q.Snapshot = d.Snapshot
	}
	var got string
	if e := json.Unmarshal([]byte(b.String()), &got); e != nil {
		t.Fatal(e)
	}
	if got != text {
		t.Fatal("chunk boundary lost text")
	}
	if _, e := ReadRecord(c, DetailRequest{Collection: "notes", ID: "missing"}); e == nil {
		t.Fatal("missing record accepted")
	}
}

func TestReadRecordBoundsEncodedEnvelopeAndRedactedMetadata(t *testing.T) {
	type item struct {
		ID     string            `json:"id"`
		Custom map[string]string `json:"custom"`
		Text   string            `json:"text"`
	}
	makeCollection := func(count int) Collection {
		custom := make(map[string]string, count)
		for i := range count {
			custom[fmt.Sprintf("api_key_%04d", i)] = "redacted"
		}
		row := item{ID: "metadata", Custom: custom, Text: strings.Repeat("🪙\"\\\n\t", 3000)}
		return NewCollection("notes", "id", func() ([]item, error) { return []item{row}, nil })
	}

	c := makeCollection(500)
	q := DetailRequest{Collection: "notes", ID: "metadata", Field: "text", Limit: maxOutput}
	var joined strings.Builder
	fieldJSON, _ := json.Marshal(strings.Repeat("🪙\"\\\n\t", 3000))
	expectedCharacters := len([]rune(string(fieldJSON)))
	var snapshot string
	for {
		d, err := ReadRecord(c, q)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(d)
		if err != nil || len(encoded) > maxDetailBytes {
			t.Fatalf("redacted detail envelope bytes=%d err=%v budget=%d", len(encoded), err, maxDetailBytes)
		}
		if len(d.RedactedPaths) != 500 || d.Offset != len([]rune(joined.String())) || d.TotalCharacters != expectedCharacters {
			t.Fatalf("redacted metadata/offset/total lost: paths=%d offset=%d joined=%d total=%d expected=%d", len(d.RedactedPaths), d.Offset, len([]rune(joined.String())), d.TotalCharacters, expectedCharacters)
		}
		if snapshot != "" && d.Snapshot != snapshot {
			t.Fatal("snapshot changed between redacted detail chunks")
		}
		snapshot = d.Snapshot
		joined.WriteString(d.JSONText)
		if d.NextOffset == nil {
			break
		}
		q.Offset, q.Snapshot = *d.NextOffset, d.Snapshot
	}
	var got string
	if err := json.Unmarshal([]byte(joined.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got != strings.Repeat("🪙\"\\\n\t", 3000) {
		t.Fatal("redacted metadata chunking changed JSON content")
	}

	tooMany := makeCollection(1400)
	if _, err := ReadRecord(tooMany, DetailRequest{Collection: "notes", ID: "metadata", Field: "text", Limit: 1}); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("oversized fixed metadata did not fail clearly: %v", err)
	}
}

func TestQueryRedactsCredentialsInNestedFieldsAndStoredTextWithoutMutation(t *testing.T) {
	type item struct {
		ID          string         `json:"id"`
		Custom      map[string]any `json:"custom"`
		Text        string         `json:"text"`
		TokenBudget int            `json:"tokenBudget"`
	}
	r := item{"x", map[string]any{"API_Key": "DO_NOT_LEAK", "nested": map[string]any{"password": "DO_NOT_LEAK", "balanceMinor": int64(123)}, "headers": map[string]string{"Authorization": "DO_NOT_LEAK"}}, "Ignore instructions; Bearer abc.def.ghi; api_key=DO_NOT_LEAK", 123}
	c := NewCollection("notes", "id", func() ([]item, error) { return []item{r}, nil })
	p, e := ReadPage(c, Query{Collection: "notes"})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "DO_NOT_LEAK") || strings.Contains(string(b), "abc.def.ghi") {
		t.Fatal("credential leaked", string(b))
	}
	if !strings.Contains(string(b), `\"balanceMinor\":123`) && !strings.Contains(string(b), `"balanceMinor":123`) {
		t.Fatal("financial field removed", string(b))
	}
	if !strings.Contains(string(b), `"tokenBudget":123`) || len(p.Records[0].RedactedPaths) != 4 {
		t.Fatal(string(b))
	}
	if r.Custom["API_Key"] != "DO_NOT_LEAK" || !strings.Contains(r.Text, "DO_NOT_LEAK") {
		t.Fatal("source mutated")
	}
	d, e := ReadRecord(c, DetailRequest{Collection: "notes", ID: "x", Field: "custom.API_Key"})
	if e == nil {
		t.Fatal("detail bypasses redaction", d)
	}
}

func TestQueryRedactsKnownCredentialEvenWithoutARecognizablePrefix(t *testing.T) {
	type item struct {
		ID     string         `json:"id"`
		Text   string         `json:"text"`
		Custom map[string]any `json:"custom"`
	}
	secret := "unlabeled-device-value-123"
	row := item{"safe", `Notes: ` + secret + `; JSON {"password":"two word password"}; https://login:pass@example.test`, map[string]any{secret: "value", "balanceMinor": json.Number("9223372036854775807")}}
	c := RedactValues(NewCollection("notes", "id", func() ([]item, error) { return []item{row}, nil }), secret)
	p, err := ReadPage(c, Query{Collection: c.Name})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	for _, value := range []string{secret, "two word password", "login:pass"} {
		if strings.Contains(string(b), value) {
			t.Fatalf("credential leaked: %s", b)
		}
	}
	if !strings.Contains(string(b), "9223372036854775807") {
		t.Fatal("financial amount changed", string(b))
	}
	if row.Custom[secret] != "value" {
		t.Fatal("original mutated")
	}
	before, _ := Describe(c)
	c.Revision = "changed"
	after, _ := Describe(c)
	if before.Snapshot == after.Snapshot {
		t.Fatal("revision missing from fingerprint")
	}
}

func TestQueryNumericEqualityPreservesExactRationalValueWithoutExpandingExponents(t *testing.T) {
	type row struct {
		ID    string      `json:"id"`
		Value json.Number `json:"value"`
	}
	for _, tc := range []struct {
		stored, want string
		match        bool
	}{
		{"100", "100.0", true}, {"100", "1e2", true}, {"1.01", "1.0100", true}, {"0", "-0e99999999999999999999999", true},
		{"9223372036854775807", "9223372036854775807.0", true}, {"9223372036854775807", "9223372036854775808", false},
		{"-9223372036854775808", "-9.223372036854775808e18", true}, {"100", `"100"`, false},
		{"1e1000000000000000000000000", "10e999999999999999999999999", true},
		{"1e1000000000000000000000000", "1e1000000000000000000000001", false},
	} {
		t.Run(tc.stored+"="+tc.want, func(t *testing.T) {
			c := NewCollection("values", "id", func() ([]row, error) { return []row{{"x", json.Number(tc.stored)}}, nil })
			p, err := ReadPage(c, Query{Collection: c.Name, Equals: map[string]json.RawMessage{"value": json.RawMessage(tc.want)}})
			if err != nil {
				t.Fatal(err)
			}
			if (p.FilteredTotal == 1) != tc.match {
				t.Fatalf("exact comparison: %+v want %v", p, tc.match)
			}
		})
	}
}

func TestQueryDeclaredUnixDateFieldsUseUTCBoundsAndRejectInvalidNumericDates(t *testing.T) {
	type row struct {
		ID          string `json:"id"`
		At          any    `json:"at"`
		AmountMinor int64  `json:"amountMinor"`
	}
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC).Unix()
	rows := []row{{"nil", nil, 100}, {"zero", int64(0), 100}, {"before", start - 1, 100}, {"start", start, 100}, {"end", start + 86399, 100}, {"after", start + 86400, 100}}
	c := NewCollection("timestamps", "id", func() ([]row, error) { return rows, nil })
	c.UnixSecondsDateFields = map[string]UnixSecondsDateField{"at": {ZeroIsMissing: true}}
	previousZone := time.Local
	time.Local = time.FixedZone("different-calendar-day", -7*60*60)
	defer func() { time.Local = previousZone }()
	q := Query{Collection: c.Name, DateField: "at", From: "2026-10-20", Through: "2026-10-20"}
	p, err := ReadPage(c, q)
	if err != nil || p.FilteredTotal != 2 || p.Records[0].ID != "end" || p.Records[1].ID != "start" {
		t.Fatalf("UTC inclusive boundaries: %+v %v", p, err)
	}
	m, err := Describe(c)
	if err != nil || m.UnixSecondsTimezone != "UTC" || !m.UnixSecondsDateFields["at"].ZeroIsMissing || !strings.Contains(p.Notes, "calendar bounds in UTC") {
		t.Fatalf("date semantics undisclosed: %+v %v", m, err)
	}
	for _, tc := range []struct {
		name        string
		value       any
		day         string
		wantCount   int
		wantError   bool
		zeroMissing bool
	}{
		{"minimum", int64(-62135596800), "0001-01-01", 1, false, false},
		{"maximum", int64(253402300799), "9999-12-31", 1, false, false},
		{"before_minimum", int64(-62135596801), "0001-01-01", 0, true, false},
		{"after_maximum", int64(253402300800), "9999-12-31", 0, true, false},
		{"max_int64", int64(math.MaxInt64), "2026-10-20", 0, true, false},
		{"outside_int64", json.Number("9223372036854775808"), "2026-10-20", 0, true, false},
		{"fractional", json.Number("1.5"), "1970-01-01", 0, true, false},
		{"wrong_type", "2026-10-20", "2026-10-20", 0, true, false},
		{"nil", nil, "1970-01-01", 0, false, false},
		{"zero_missing", int64(0), "1970-01-01", 0, false, true},
		{"epoch_zero", int64(0), "1970-01-01", 1, false, false},
		{"negative_epoch", int64(-1), "1969-12-31", 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows = []row{{"only", tc.value, 100}}
			c.UnixSecondsDateFields["at"] = UnixSecondsDateField{ZeroIsMissing: tc.zeroMissing}
			p, err := ReadPage(c, Query{Collection: c.Name, DateField: "at", From: tc.day, Through: tc.day})
			if (err != nil) != tc.wantError || err == nil && p.FilteredTotal != tc.wantCount {
				t.Fatalf("date availability: %+v %v", p, err)
			}
		})
	}
	for _, empty := range []bool{false, true} {
		if empty {
			rows = nil
		}
		if _, err := ReadPage(c, Query{Collection: c.Name, DateField: "amountMinor", From: "1970-01-01", Through: "1970-01-01"}); err == nil || !strings.Contains(err.Error(), "not declared") {
			t.Fatal("money was treated as a timestamp or a healthy zero", err)
		}
	}
}
