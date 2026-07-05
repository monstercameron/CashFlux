// SPDX-License-Identifier: MIT

package artifacts

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestParseCSV(t *testing.T) {
	cols, rows, err := ParseCSV([]byte("date,amount,note\n2026-06-01,12.50,coffee\n2026-06-02,3\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(cols, []string{"date", "amount", "note"}) {
		t.Errorf("cols = %v", cols)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// Ragged row is padded to header width.
	if !reflect.DeepEqual(rows[1], []string{"2026-06-02", "3", ""}) {
		t.Errorf("ragged row = %v", rows[1])
	}
	if _, _, err := ParseCSV(nil); err == nil {
		t.Error("empty csv should error")
	}
}

func TestDataURL(t *testing.T) {
	if got := DataURL("image/png", []byte("hi")); got != "data:image/png;base64,aGk=" {
		t.Errorf("DataURL = %q", got)
	}
	if got := DataURL("", []byte{}); got != "data:application/octet-stream;base64," {
		t.Errorf("default mime = %q", got)
	}
}

func TestSizeAndHuman(t *testing.T) {
	a := domain.Artifact{Bytes: []byte("abcd"), Columns: []string{"x"}, Rows: [][]string{{"yy"}}}
	if Size(a) != 7 { // 4 + 1 + 2
		t.Errorf("Size = %d, want 7", Size(a))
	}
	cases := map[int]string{500: "500 B", 2048: "2.0 KB", 3 << 20: "3.0 MB"}
	for n, want := range cases {
		if got := HumanSize(n); got != want {
			t.Errorf("HumanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := domain.Artifact{ID: "a1", Name: "Logo", Kind: KindImage, Bytes: []byte{1}}
	if errs := Validate(ok); errs != nil {
		t.Errorf("valid image flagged: %v", errs)
	}
	ds := domain.Artifact{ID: "a2", Name: "Data", Kind: KindCSV, Columns: []string{"x"}}
	if errs := Validate(ds); errs != nil {
		t.Errorf("valid dataset flagged: %v", errs)
	}
	if errs := Validate(domain.Artifact{Kind: "bogus"}); len(errs) == 0 {
		t.Error("bogus artifact should be invalid")
	}
	if errs := Validate(domain.Artifact{ID: "a", Name: "n", Kind: KindImage}); len(errs) == 0 {
		t.Error("image with no bytes should be invalid")
	}
}

func TestCSVBytesRoundTrip(t *testing.T) {
	cols := []string{"date", "payee", "amount"}
	rows := [][]string{{"2026-01-02", "Corner, Cafe", "-4.50"}, {"2026-01-03", "Acme \"Payroll\"", "4000"}}
	out := CSVBytes(cols, rows)
	gotCols, gotRows, err := ParseCSV(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(gotCols) != 3 || gotCols[1] != "payee" {
		t.Fatalf("columns round-trip = %v", gotCols)
	}
	if len(gotRows) != 2 || gotRows[0][1] != "Corner, Cafe" || gotRows[1][1] != `Acme "Payroll"` {
		t.Fatalf("rows round-trip = %v", gotRows)
	}
}
