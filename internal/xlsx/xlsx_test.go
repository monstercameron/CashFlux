package xlsx_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/xlsx"
)

// buildXLSX constructs a minimal .xlsx byte slice in memory.
// shared is the list of shared strings; cells is a slice of (col, row, type, value) tuples
// where type "s" means shared-string index in value, "n" means numeric, "i" means inlineStr.
func buildXLSX(shared []string, rows [][]testCell) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// [Content_Types].xml — required by the zip spec but not parsed by our reader.
	addEntry(w, "[Content_Types].xml", `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
</Types>`)

	// xl/sharedStrings.xml
	if len(shared) > 0 {
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		sb.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
		for _, s := range shared {
			sb.WriteString(fmt.Sprintf("<si><t>%s</t></si>", xmlEscape(s)))
		}
		sb.WriteString(`</sst>`)
		addEntry(w, "xl/sharedStrings.xml", sb.String())
	}

	// xl/worksheets/sheet1.xml
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, ri+1))
		for _, c := range row {
			ref := fmt.Sprintf("%s%d", colLetter(c.col), ri+1)
			switch c.typ {
			case "s":
				sb.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%s</v></c>`, ref, c.val))
			case "i":
				sb.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xmlEscape(c.val)))
			default: // numeric
				sb.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, c.val))
			}
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)
	addEntry(w, "xl/worksheets/sheet1.xml", sb.String())

	w.Close()
	return buf.Bytes()
}

type testCell struct {
	col int    // 0-based
	typ string // "s", "i", or "" (numeric)
	val string
}

func addEntry(w *zip.Writer, name, content string) {
	f, _ := w.Create(name)
	f.Write([]byte(content))
}

func colLetter(col int) string {
	// Single-letter only (A–Z) sufficient for tests.
	return string(rune('A' + col))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func TestParse_SharedStrings(t *testing.T) {
	shared := []string{"Date", "Description", "Amount", "2024-01-15", "Grocery store", "Coffee shop"}
	rows := [][]testCell{
		{{0, "s", "0"}, {1, "s", "1"}, {2, "s", "2"}},
		{{0, "s", "3"}, {1, "s", "4"}, {2, "", "4599"}},
		{{0, "s", "3"}, {1, "s", "5"}, {2, "", "350"}},
	}
	data := buildXLSX(shared, rows)

	got, err := xlsx.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d", len(got))
	}

	cases := []struct {
		row, col int
		want     string
	}{
		{0, 0, "Date"},
		{0, 1, "Description"},
		{0, 2, "Amount"},
		{1, 1, "Grocery store"},
		{1, 2, "4599"},
		{2, 1, "Coffee shop"},
	}
	for _, tc := range cases {
		if got[tc.row][tc.col] != tc.want {
			t.Errorf("row %d col %d: want %q, got %q", tc.row, tc.col, tc.want, got[tc.row][tc.col])
		}
	}
}

func TestParse_InlineStrings(t *testing.T) {
	rows := [][]testCell{
		{{0, "i", "Name"}, {1, "i", "Score"}},
		{{0, "i", "Alice"}, {1, "", "95"}},
	}
	data := buildXLSX(nil, rows)

	got, err := xlsx.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0][0] != "Name" {
		t.Errorf("want Name, got %q", got[0][0])
	}
	if got[1][1] != "95" {
		t.Errorf("want 95, got %q", got[1][1])
	}
}

func TestParse_NumericOnly(t *testing.T) {
	rows := [][]testCell{
		{{0, "", "100"}, {1, "", "200"}, {2, "", "300"}},
	}
	data := buildXLSX(nil, rows)

	got, err := xlsx.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || len(got[0]) != 3 {
		t.Fatalf("unexpected shape: %v", got)
	}
	if got[0][2] != "300" {
		t.Errorf("want 300, got %q", got[0][2])
	}
}

func TestParse_EmptyArchive(t *testing.T) {
	// Zip archive that has no sheet1.xml — should return an error.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	addEntry(w, "xl/workbook.xml", "<workbook/>")
	w.Close()

	_, err := xlsx.Parse(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for missing sheet1.xml")
	}
}

func TestParse_NotAZip(t *testing.T) {
	_, err := xlsx.Parse([]byte("not a zip"))
	if err == nil {
		t.Fatal("expected error for non-zip data")
	}
}

func TestParse_ZipBombEntryCount(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Write 1001 entries (one over the cap of 1000).
	for i := 0; i < 1001; i++ {
		addEntry(w, fmt.Sprintf("junk/%d.xml", i), "<x/>")
	}
	w.Close()

	_, err := xlsx.Parse(buf.Bytes())
	if err == nil {
		t.Fatal("expected zip-bomb entry-count error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("unexpected error: %v", err)
	}
}
