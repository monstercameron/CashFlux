package statement

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ofx"
)

// ParseAny detects the format of r and parses it into rows.
// It peeks at the content to decide:
//   - If it starts with "OFXHEADER:" or "<?OFX" or "<OFX" or "<?xml" → OFX path (decimals=2)
//   - Otherwise → delimited (CSV/TSV) path using Parse
func ParseAny(r io.Reader, decimals int) ([]Row, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

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

	// Delimited CSV/TSV path
	stmt, err := Parse(trimmed, decimals)
	if err != nil {
		return nil, err
	}
	if len(stmt.Errors) > 0 && len(stmt.Rows) == 0 {
		return nil, fmt.Errorf("statement: parse errors: %v", stmt.Errors[0])
	}
	return stmt.Rows, nil
}
