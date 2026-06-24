// SPDX-License-Identifier: MIT

// Package docx provides a minimal pure-Go reader for .docx files (Office Open XML
// WordprocessingML). It opens the zip archive, reads word/document.xml, and
// extracts all tables (<w:tbl>) as a flat slice of rows, each row being a slice
// of cell strings joined from <w:t> elements inside <w:tc>.
//
// Zip-bomb guard: total decompressed bytes per entry are capped at 50 MB and the
// number of zip entries is capped at 1 000; either limit exceeded returns an error.
//
// Pure Go, stdlib only (archive/zip, encoding/xml, bytes). No syscall/js.
package docx

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

// ParseTables opens data as a .docx zip archive and returns all table rows found
// in word/document.xml. Rows from every <w:tbl> are concatenated in document
// order; each row is a slice of cell text strings in column order.
func ParseTables(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx: open zip: %w", err)
	}
	if len(zr.File) > maxZipEntries {
		return nil, fmt.Errorf("docx: zip entry count %d exceeds limit %d", len(zr.File), maxZipEntries)
	}

	// Locate word/document.xml.
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("docx: word/document.xml not found in archive")
	}

	b, err := readZipEntry(docFile)
	if err != nil {
		return nil, fmt.Errorf("docx: read word/document.xml: %w", err)
	}

	rows, err := extractTableRows(b)
	if err != nil {
		return nil, fmt.Errorf("docx: parse word/document.xml: %w", err)
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

// ── XML extraction ─────────────────────────────────────────────────────────────

// extractTableRows scans the XML token stream for w:tbl / w:tr / w:tc / w:t
// elements and assembles table rows. Using a token-stream parser avoids the
// complexity of deeply nested struct tags across arbitrary namespaces.
func extractTableRows(data []byte) ([][]string, error) {
	dec := xml.NewTokenDecoder(xml.NewDecoder(bytes.NewReader(data)))

	var (
		rows    [][]string
		inTable bool
		inRow   bool
		inCell  bool
		inText  bool
		curRow  []string
		curCell strings.Builder
	)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch localName(t.Name) {
			case "tbl":
				inTable = true
			case "tr":
				if inTable {
					inRow = true
					curRow = nil
				}
			case "tc":
				if inRow {
					inCell = true
					curCell.Reset()
				}
			case "t":
				if inCell {
					inText = true
				}
			}
		case xml.EndElement:
			switch localName(t.Name) {
			case "tbl":
				inTable = false
				inRow = false
				inCell = false
				inText = false
			case "tr":
				if inRow {
					rows = append(rows, curRow)
					curRow = nil
					inRow = false
				}
			case "tc":
				if inCell {
					curRow = append(curRow, curCell.String())
					curCell.Reset()
					inCell = false
				}
			case "t":
				inText = false
			}
		case xml.CharData:
			if inText {
				curCell.Write(t)
			}
		}
	}
	return rows, nil
}

// localName returns the local part of an XML name, stripping any namespace prefix.
func localName(name xml.Name) string {
	if name.Local != "" {
		return name.Local
	}
	// Fallback: strip prefix from Space if Local is empty (shouldn't happen with
	// Go's encoding/xml, but defensive).
	if idx := strings.LastIndex(name.Space, ":"); idx >= 0 {
		return name.Space[idx+1:]
	}
	return name.Space
}
