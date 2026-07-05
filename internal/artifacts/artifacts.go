// SPDX-License-Identifier: MIT

// Package artifacts is the pure logic for user-stored assets (uploaded images and
// imported CSV/JSON datasets): the kinds, parsing a CSV into columns + rows,
// building a data URL for an image, byte-size accounting, and validation. It has
// no platform dependencies, so it unit-tests on native Go; the wasm UI handles
// the file picker and persistence.
package artifacts

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Artifact kinds, stored in domain.Artifact.Kind.
const (
	KindImage = "image"
	KindCSV   = "csv"
	KindJSON  = "json"
)

// ParseCSV reads CSV bytes into a header row and the remaining data rows. Rows are
// padded/truncated to the header width so the table is rectangular. An empty input
// is an error (nothing to import); a header-only file yields no rows.
func ParseCSV(data []byte) (columns []string, rows [][]string, err error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1 // tolerate ragged rows; we normalize below
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("artifacts: parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("artifacts: empty csv")
	}
	columns = records[0]
	width := len(columns)
	for _, rec := range records[1:] {
		row := make([]string, width)
		for i := 0; i < width; i++ {
			if i < len(rec) {
				row[i] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, row)
	}
	return columns, rows, nil
}

// DataURL builds a `data:` URL for raw bytes with the given MIME type, suitable
// for an <img src>. An empty MIME defaults to application/octet-stream.
func DataURL(mime string, data []byte) string {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// Size reports the storage footprint of an artifact in bytes: the raw bytes plus
// the length of the parsed cells (an estimate good enough for a storage meter).
func Size(a domain.Artifact) int {
	n := len(a.Bytes)
	for _, c := range a.Columns {
		n += len(c)
	}
	for _, row := range a.Rows {
		for _, cell := range row {
			n += len(cell)
		}
	}
	return n
}

// HumanSize renders a byte count as a compact human string (B, KB, MB).
func HumanSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// Validate reports human-readable problems with an artifact, or nil if valid: it
// needs an ID, a name, a known kind, and content appropriate to that kind.
func Validate(a domain.Artifact) []string {
	var errs []string
	if a.ID == "" {
		errs = append(errs, "An artifact needs an id.")
	}
	if strings.TrimSpace(a.Name) == "" {
		errs = append(errs, "An artifact needs a name.")
	}
	switch a.Kind {
	case KindImage:
		if len(a.Bytes) == 0 {
			errs = append(errs, "An image artifact needs image data.")
		}
	case KindCSV, KindJSON:
		if len(a.Columns) == 0 && len(a.Rows) == 0 && len(a.Bytes) == 0 {
			errs = append(errs, "A dataset artifact needs data.")
		}
	default:
		errs = append(errs, "Unknown artifact kind.")
	}
	return errs
}

// CSVBytes serializes a parsed CSV artifact (header + rows) back into CSV file
// bytes — the inverse of ParseCSV, for downloading a stored dataset. The
// round-trip is lossless for rectangular data (ParseCSV pads/truncates rows to
// the header width on the way in).
func CSVBytes(columns []string, rows [][]string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if len(columns) > 0 {
		_ = w.Write(columns)
	}
	for _, r := range rows {
		_ = w.Write(r)
	}
	w.Flush()
	return buf.Bytes()
}
