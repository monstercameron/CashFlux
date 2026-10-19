// SPDX-License-Identifier: MIT

package importdraft

import (
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importmap"
)

func TestStatusHelpers(t *testing.T) {
	for _, tc := range []struct{ message, want string }{{"", ""}, {" \n ", ""}, {"raw failure", "imb.aiFailed"}} {
		if got := FriendlyError(tc.message, translation); got != tc.want {
			t.Fatal(got)
		}
	}
	project := func(pair [2]string) (string, string) { return pair[0], pair[1] }
	if ImportedTransactionIDs([][2]string(nil), project) != nil {
		t.Fatal("nil")
	}
	if got := ImportedTransactionIDs([][2]string{{"failed", "x"}, {"imported", "a"}, {"duplicate", "b"}, {"imported", "c"}}, project); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		failures         []importmap.RowError
		warnings         []importmap.RowWarning
		failure, warning bool
	}{{nil, nil, false, false}, {[]importmap.RowError{{Field: "date"}}, nil, true, false}, {nil, []importmap.RowWarning{{Field: "amount"}}, false, true}} {
		got := ParsedMessage(2, tc.failures, tc.warnings, translation)
		if !strings.HasPrefix(got, "documents.stmtParsedimb.rowMany2") || strings.Contains(got, "skipLine") != tc.failure || strings.Contains(got, "ambiguousAmounts") != tc.warning {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		rows    []extract.Row
		current string
		enabled bool
		want    string
	}{{nil, "", true, "0.00"}, {[]extract.Row{{Amount: "3"}, {Amount: "-1"}}, "", true, "2.00"}, {nil, " 4 ", true, " 4 "}, {nil, "", false, ""}, {[]extract.Row{{Amount: "bad"}}, " ", true, " "}, {[]extract.Row{{Amount: "92233720368547758.07"}, {Amount: "1"}}, "", true, ""}} {
		if got := ReceiptTotal(tc.rows, tc.current, 2, tc.enabled); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		mime string
		data []byte
		want string
	}{{"", nil, "data:application/pdf;base64,"}, {"application/custom", []byte{0, 1, 2}, "data:application/custom;base64,AAEC"}} {
		if got := PDFDataURL(tc.mime, tc.data); got != tc.want {
			t.Fatal(got)
		}
	}
}
