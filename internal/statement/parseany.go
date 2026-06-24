// SPDX-License-Identifier: MIT

package statement

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/docx"
	"github.com/monstercameron/CashFlux/internal/ofx"
	"github.com/monstercameron/CashFlux/internal/pdftext"
	"github.com/monstercameron/CashFlux/internal/xlsx"
)

// ParseAny detects the format of r and parses it into rows.
//
// Detection order (by magic bytes / prefix):
//  1. PK\x03\x04  → ZIP-based format; sniff central directory for "xl/" (XLSX) or "word/" (DOCX).
//  2. %PDF        → PDF text extraction then delimited parse.
//  3. OFX markers → OFX 1.x SGML or OFX 2.x XML.
//  4. Anything else → delimited (CSV/TSV/etc.) via Parse.
func ParseAny(r io.Reader, decimals int) ([]Row, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM before any inspection.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// ── ZIP-based formats (XLSX / DOCX) ─────────────────────────────────────
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return parseZipFormat(data, decimals)
	}

	// ── PDF ──────────────────────────────────────────────────────────────────
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return parsePDF(data, decimals)
	}

	// ── OFX / QFX ────────────────────────────────────────────────────────────
	trimmed := strings.TrimSpace(string(data))
	isOFX := strings.HasPrefix(trimmed, "OFXHEADER:") ||
		strings.HasPrefix(trimmed, "<?xml") ||
		strings.HasPrefix(trimmed, "<?OFX") ||
		strings.HasPrefix(trimmed, "<OFX")
	if isOFX {
		ofxRows, err := ofx.Parse(bytes.NewReader([]byte(trimmed)), decimals)
		if err != nil {
			return nil, err
		}
		rows := make([]Row, 0, len(ofxRows))
		for _, or_ := range ofxRows {
			rows = append(rows, Row{
				Date:        or_.Date,
				Description: or_.Description,
				Amount:      or_.Amount,
			})
		}
		return rows, nil
	}

	// ── Delimited (CSV / TSV / etc.) ─────────────────────────────────────────
	stmt, err := Parse(trimmed, decimals)
	if err != nil {
		return nil, err
	}
	if len(stmt.Errors) > 0 && len(stmt.Rows) == 0 {
		return nil, fmt.Errorf("statement: parse errors: %v", stmt.Errors[0])
	}
	return stmt.Rows, nil
}

// parseZipFormat opens data as a ZIP, detects whether it is an XLSX or DOCX,
// parses accordingly, and feeds the resulting grid through the column mapper.
func parseZipFormat(data []byte, decimals int) ([]Row, error) {
	isXLSX, isDocx := sniffZip(data)

	switch {
	case isXLSX:
		grid, err := xlsx.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("statement: xlsx: %w", err)
		}
		return gridToRows(grid, decimals)

	case isDocx:
		grid, err := docx.ParseTables(data)
		if err != nil {
			return nil, fmt.Errorf("statement: docx: %w", err)
		}
		return gridToRows(grid, decimals)

	default:
		return nil, fmt.Errorf("statement: ZIP archive is neither XLSX nor DOCX (no xl/ or word/ entries detected)")
	}
}

// sniffZip scans the local-file-header chain of a ZIP archive to detect
// whether it contains XLSX (entries under xl/) or DOCX (entries under word/).
// It does not use archive/zip so it adds no import overhead over what is
// already present; any malformed header silently ends the scan.
func sniffZip(data []byte) (isXLSX, isDocx bool) {
	rest := data
	for len(rest) >= 30 {
		if !bytes.HasPrefix(rest, []byte("PK\x03\x04")) {
			break
		}
		fnLen := int(rest[26]) | int(rest[27])<<8
		exLen := int(rest[28]) | int(rest[29])<<8
		headerLen := 30 + fnLen + exLen
		if len(rest) < headerLen {
			break
		}
		name := string(rest[30 : 30+fnLen])
		if strings.HasPrefix(name, "xl/") {
			isXLSX = true
			return
		}
		if strings.HasPrefix(name, "word/") {
			isDocx = true
			return
		}
		// Advance past this entry (header + compressed data).
		compSize := int(rest[18]) | int(rest[19])<<8 | int(rest[20])<<16 | int(rest[21])<<24
		next := headerLen + compSize
		if next <= 0 || next >= len(rest) {
			break
		}
		rest = rest[next:]
	}
	return
}

// parsePDF extracts text from a PDF and feeds it through the delimited parser.
// Many bank PDFs embed tab- or comma-separated transaction tables in their
// content streams; this path handles those. Image-only and encrypted PDFs
// propagate the pdftext sentinel errors (ErrNoText / ErrEncrypted).
func parsePDF(data []byte, decimals int) ([]Row, error) {
	text, err := pdftext.ExtractText(data)
	if err != nil {
		return nil, fmt.Errorf("statement: pdf: %w", err)
	}
	stmt, err := Parse(text, decimals)
	if err != nil {
		return nil, fmt.Errorf("statement: pdf → delimited: %w", err)
	}
	if len(stmt.Errors) > 0 && len(stmt.Rows) == 0 {
		return nil, fmt.Errorf("statement: pdf → delimited parse errors: %v", stmt.Errors[0])
	}
	return stmt.Rows, nil
}

// gridToRows maps a [][]string grid (header row + data rows) through the
// statement column mapper and normaliser by converting it to a tab-separated
// string and re-using Parse. Tabs inside cell values are replaced with spaces
// so they do not disrupt the delimiter.
func gridToRows(grid [][]string, decimals int) ([]Row, error) {
	if len(grid) == 0 {
		return nil, nil
	}
	var sb strings.Builder
	for ri, row := range grid {
		for ci, cell := range row {
			if ci > 0 {
				sb.WriteByte('\t')
			}
			sb.WriteString(strings.ReplaceAll(cell, "\t", " "))
		}
		if ri < len(grid)-1 {
			sb.WriteByte('\n')
		}
	}
	stmt, err := Parse(sb.String(), decimals)
	if err != nil {
		return nil, err
	}
	if len(stmt.Errors) > 0 && len(stmt.Rows) == 0 {
		return nil, fmt.Errorf("statement: grid parse errors: %v", stmt.Errors[0])
	}
	return stmt.Rows, nil
}
