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

func TestQueryLongRecordIsFullyReachableInUnicodeChunks(t *testing.T) {
	type item struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	text := strings.Repeat("café🪙\n", 8000)
	c := NewCollection("notes", "id", func() ([]item, error) { return []item{{"long", text}}, nil })
	p, e := ReadPage(c, Query{Collection: "notes"})
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Records) != 1 || !p.Records[0].DetailRequired || len(p.Records[0].Data) != 0 {
		t.Fatal("large record silently truncated", p)
	}
	q := DetailRequest{Collection: "notes", ID: "long", Field: "text", Limit: 137}
	var b strings.Builder
	for {
		d, e := ReadRecord(c, q)
		if e != nil {
			t.Fatal(e)
		}
		b.WriteString(d.JSONText)
		if d.NextOffset == nil {
			break
		}
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
