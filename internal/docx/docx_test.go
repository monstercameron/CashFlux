// SPDX-License-Identifier: MIT

package docx_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/docx"
)

// The w: namespace URI used by WordprocessingML.
const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// buildDocx constructs a minimal .docx zip in memory containing a single
// word/document.xml whose body holds the provided tables.
// Each table is a [][]string (rows × cells).
func buildDocx(tables [][][]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(fmt.Sprintf(`<w:document xmlns:w=%q><w:body>`, wNS))

	for _, tbl := range tables {
		sb.WriteString(`<w:tbl>`)
		for _, row := range tbl {
			sb.WriteString(`<w:tr>`)
			for _, cell := range row {
				sb.WriteString(`<w:tc><w:p><w:r><w:t>`)
				sb.WriteString(xmlEscape(cell))
				sb.WriteString(`</w:t></w:r></w:p></w:tc>`)
			}
			sb.WriteString(`</w:tr>`)
		}
		sb.WriteString(`</w:tbl>`)
	}
	sb.WriteString(`</w:body></w:document>`)

	f, _ := w.Create("word/document.xml")
	f.Write([]byte(sb.String()))
	w.Close()
	return buf.Bytes()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func TestParseTables_SingleTable(t *testing.T) {
	table := [][]string{
		{"Date", "Description", "Amount"},
		{"2024-01-15", "Grocery store", "45.99"},
		{"2024-01-16", "Coffee shop", "3.50"},
	}
	data := buildDocx([][][]string{table})

	rows, err := docx.ParseTables(data)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	cases := []struct {
		r, c int
		want string
	}{
		{0, 0, "Date"},
		{0, 1, "Description"},
		{0, 2, "Amount"},
		{1, 1, "Grocery store"},
		{1, 2, "45.99"},
		{2, 0, "2024-01-16"},
	}
	for _, tc := range cases {
		if rows[tc.r][tc.c] != tc.want {
			t.Errorf("row %d col %d: want %q, got %q", tc.r, tc.c, tc.want, rows[tc.r][tc.c])
		}
	}
}

func TestParseTables_MultipleTables(t *testing.T) {
	tbl1 := [][]string{{"A", "B"}, {"1", "2"}}
	tbl2 := [][]string{{"X", "Y", "Z"}, {"7", "8", "9"}}
	data := buildDocx([][][]string{tbl1, tbl2})

	rows, err := docx.ParseTables(data)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	// 2 rows from tbl1 + 2 rows from tbl2 = 4 total.
	if len(rows) != 4 {
		t.Fatalf("want 4 rows, got %d", len(rows))
	}
	if rows[2][0] != "X" {
		t.Errorf("want X, got %q", rows[2][0])
	}
	if rows[3][2] != "9" {
		t.Errorf("want 9, got %q", rows[3][2])
	}
}

func TestParseTables_NoTables(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("word/document.xml")
	f.Write([]byte(fmt.Sprintf(`<?xml version="1.0"?><w:document xmlns:w=%q><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`, wNS)))
	w.Close()

	rows, err := docx.ParseTables(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no rows, got %d", len(rows))
	}
}

func TestParseTables_MissingDocument(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("word/styles.xml")
	f.Write([]byte("<styles/>"))
	w.Close()

	_, err := docx.ParseTables(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for missing word/document.xml")
	}
}

func TestParseTables_NotAZip(t *testing.T) {
	_, err := docx.ParseTables([]byte("not a zip"))
	if err == nil {
		t.Fatal("expected error for non-zip data")
	}
}

func TestParseTables_ZipBombEntryCount(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < 1001; i++ {
		f, _ := w.Create(fmt.Sprintf("junk/%d.xml", i))
		f.Write([]byte("<x/>"))
	}
	w.Close()

	_, err := docx.ParseTables(buf.Bytes())
	if err == nil {
		t.Fatal("expected zip-bomb entry-count error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("unexpected error: %v", err)
	}
}
