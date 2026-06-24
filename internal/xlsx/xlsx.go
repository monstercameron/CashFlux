// SPDX-License-Identifier: MIT

// Package xlsx provides a minimal pure-Go reader for .xlsx files (Office Open XML
// SpreadsheetML). It opens the zip archive, reads the shared-strings table and the
// first worksheet, and returns all rows as [][]string.
//
// Zip-bomb guard: total decompressed bytes per entry are capped at 50 MB and the
// number of zip entries is capped at 1 000; either limit exceeded returns an error.
//
// Pure Go, stdlib only (archive/zip, encoding/xml, bytes). No syscall/js.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	// maxDecompressedBytes is the zip-bomb size cap per entry (50 MB).
	maxDecompressedBytes = 50 << 20
	// maxZipEntries is the zip-bomb entry-count cap.
	maxZipEntries = 1_000
)

// Parse opens data as an .xlsx zip archive and returns the rows of the first
// worksheet as cell strings. Cells are in column order; sparse rows are padded
// to the last non-empty column with empty strings.
func Parse(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx: open zip: %w", err)
	}
	if len(zr.File) > maxZipEntries {
		return nil, fmt.Errorf("xlsx: zip entry count %d exceeds limit %d", len(zr.File), maxZipEntries)
	}

	// Locate the files we need.
	var sharedStringsFile, sheet1File *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case "xl/sharedStrings.xml":
			sharedStringsFile = f
		case "xl/worksheets/sheet1.xml":
			sheet1File = f
		}
	}
	if sheet1File == nil {
		return nil, fmt.Errorf("xlsx: xl/worksheets/sheet1.xml not found in archive")
	}

	// Read shared strings (absent when all cells are inline or numeric).
	var shared []string
	if sharedStringsFile != nil {
		b, err := readZipEntry(sharedStringsFile)
		if err != nil {
			return nil, fmt.Errorf("xlsx: read sharedStrings.xml: %w", err)
		}
		shared, err = parseSharedStrings(b)
		if err != nil {
			return nil, fmt.Errorf("xlsx: parse sharedStrings.xml: %w", err)
		}
	}

	// Read and parse the worksheet.
	b, err := readZipEntry(sheet1File)
	if err != nil {
		return nil, fmt.Errorf("xlsx: read sheet1.xml: %w", err)
	}
	rows, err := parseSheet(b, shared)
	if err != nil {
		return nil, fmt.Errorf("xlsx: parse sheet1.xml: %w", err)
	}
	return rows, nil
}

// readZipEntry decompresses one zip entry, enforcing the size cap.
func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	lr := io.LimitReader(rc, maxDecompressedBytes+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxDecompressedBytes {
		return nil, fmt.Errorf("decompressed size exceeds %d byte limit", maxDecompressedBytes)
	}
	return b, nil
}

// ── Shared strings ────────────────────────────────────────────────────────────

type sst struct {
	Items []siItem `xml:"si"`
}

type siItem struct {
	T    string  `xml:"t"` // plain string
	Runs []rtRun `xml:"r"` // rich-text runs
}

type rtRun struct {
	T string `xml:"t"`
}

// text returns the full string value, merging rich-text runs when present.
func (si siItem) text() string {
	if si.T != "" {
		return si.T
	}
	var sb strings.Builder
	for _, r := range si.Runs {
		sb.WriteString(r.T)
	}
	return sb.String()
}

func parseSharedStrings(data []byte) ([]string, error) {
	var s sst
	if err := xml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	out := make([]string, len(s.Items))
	for i, item := range s.Items {
		out[i] = item.text()
	}
	return out, nil
}

// ── Worksheet ─────────────────────────────────────────────────────────────────

type sheetData struct {
	Rows []wsRow `xml:"sheetData>row"`
}

type wsRow struct {
	Cells []wsCell `xml:"c"`
}

type wsCell struct {
	Ref  string  `xml:"r,attr"` // e.g. "B3"
	Type string  `xml:"t,attr"` // "s"=shared-string, "inlineStr", "str", "b", ""=number
	V    string  `xml:"v"`      // numeric value or shared-string index
	IS   isValue `xml:"is"`     // inline string container
}

type isValue struct {
	T    string  `xml:"t"`
	Runs []rtRun `xml:"r"`
}

func (is isValue) text() string {
	if is.T != "" {
		return is.T
	}
	var sb strings.Builder
	for _, r := range is.Runs {
		sb.WriteString(r.T)
	}
	return sb.String()
}

// colLetters extracts the leading letter(s) from a cell ref like "AB12".
func colLetters(ref string) string {
	for i, ch := range ref {
		if ch >= '0' && ch <= '9' {
			return ref[:i]
		}
	}
	return ref
}

// colIndex converts column letters (A, B, … Z, AA, AB, …) to a 0-based index.
func colIndex(letters string) int {
	idx := 0
	for _, ch := range strings.ToUpper(letters) {
		idx = idx*26 + int(ch-'A'+1)
	}
	return idx - 1
}

func parseSheet(data []byte, shared []string) ([][]string, error) {
	var sd sheetData
	if err := xml.Unmarshal(data, &sd); err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(sd.Rows))
	for _, wsrow := range sd.Rows {
		if len(wsrow.Cells) == 0 {
			rows = append(rows, []string{})
			continue
		}

		// Find the rightmost column in this row.
		maxCol := 0
		for _, c := range wsrow.Cells {
			if ci := colIndex(colLetters(c.Ref)); ci > maxCol {
				maxCol = ci
			}
		}

		row := make([]string, maxCol+1)
		for _, c := range wsrow.Cells {
			if ci := colIndex(colLetters(c.Ref)); ci >= 0 && ci < len(row) {
				row[ci] = cellString(c, shared)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// cellString resolves a cell to its display string.
func cellString(c wsCell, shared []string) string {
	switch c.Type {
	case "s":
		// Shared string: V is the 0-based index.
		idx := 0
		for _, ch := range c.V {
			if ch < '0' || ch > '9' {
				break
			}
			idx = idx*10 + int(ch-'0')
		}
		if idx < len(shared) {
			return shared[idx]
		}
		return c.V
	case "inlineStr":
		return c.IS.text()
	case "str":
		return c.V // formula result cached as string
	case "b":
		if c.V == "1" {
			return "TRUE"
		}
		return "FALSE"
	default:
		return c.V // numeric, date serial, etc.
	}
}
