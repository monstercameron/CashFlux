// SPDX-License-Identifier: MIT

package ofx

import (
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// Row is one normalized OFX transaction. Amount is signed minor units.
type Row struct {
	Date        time.Time
	Description string
	Amount      int64 // signed minor units, credit positive, debit negative
	FITID       string
}

// RowError identifies an OFX transaction that could not be normalized.
// Index is one-based and follows the transaction order in the source file.
type RowError struct {
	Index int
	Err   string
}

// Result contains valid transactions and errors for rejected transactions.
type Result struct {
	Rows   []Row
	Errors []RowError
}

// Parse parses OFX 1.x (SGML) or OFX 2.x (XML) content from r.
// Decimals is the minor-unit precision (e.g. 2 for USD/GBP).
func Parse(r io.Reader, decimals int) ([]Row, error) {
	result, err := ParseDetailed(r, decimals)
	if err != nil {
		return nil, err
	}
	if len(result.Errors) != 0 {
		return result.Rows, fmt.Errorf("ofx: %d transaction(s) could not be parsed; first error: transaction %d: %s", len(result.Errors), result.Errors[0].Index, result.Errors[0].Err)
	}
	return result.Rows, nil
}

// ParseDetailed parses OFX and returns valid rows together with per-transaction
// errors. Invalid amounts are rejected instead of being rounded or replaced by zero.
func ParseDetailed(r io.Reader, decimals int) (Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}, fmt.Errorf("ofx: read input: %w", err)
	}
	content := string(data)
	trimmed := strings.TrimSpace(content)

	if strings.HasPrefix(trimmed, "OFXHEADER:") {
		return parseSGML(trimmed, decimals)
	}
	if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<?OFX") || strings.HasPrefix(trimmed, "<OFX") {
		return parseXML(trimmed, decimals)
	}
	return Result{}, fmt.Errorf("ofx: unrecognized format")
}

// parseSGML parses OFX 1.x SGML line-by-line.
func parseSGML(content string, decimals int) (Result, error) {
	var result Result
	var cur *Row
	index := 0
	var amountErr string
	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Closing aggregate tag: </TAG>
		if strings.HasPrefix(line, "</") {
			tag := strings.ToUpper(strings.Trim(line, "</>"))
			if tag == "STMTTRN" && cur != nil {
				if amountErr != "" {
					result.Errors = append(result.Errors, RowError{Index: index, Err: amountErr})
				} else {
					result.Rows = append(result.Rows, *cur)
				}
				cur = nil
				amountErr = ""
			}
			continue
		}
		// Opening or leaf tag: <TAG>VALUE or <TAG>
		if !strings.HasPrefix(line, "<") {
			continue
		}
		closeAngle := strings.Index(line, ">")
		if closeAngle < 0 {
			continue
		}
		tag := strings.ToUpper(line[1:closeAngle])
		value := strings.TrimSpace(line[closeAngle+1:])

		if tag == "STMTTRN" {
			index++
			c := &Row{}
			cur = c
			continue
		}
		if cur == nil || value == "" {
			continue
		}
		switch tag {
		case "DTPOSTED":
			t, err := parseOFXDate(value)
			if err == nil {
				cur.Date = t
			}
		case "TRNAMT":
			amt, err := parseAmount(value, decimals)
			if err != nil {
				amountErr = fmt.Sprintf("invalid TRNAMT %q: %v", value, err)
			} else {
				cur.Amount = amt
			}
		case "FITID":
			cur.FITID = value
		case "NAME":
			if cur.Description == "" {
				cur.Description = value
			}
		case "MEMO":
			if cur.Description == "" {
				cur.Description = value
			}
		}
	}
	if cur != nil {
		if amountErr != "" {
			result.Errors = append(result.Errors, RowError{Index: index, Err: amountErr})
		} else {
			result.Rows = append(result.Rows, *cur)
		}
	}
	return result, nil
}

// XML structs for OFX 2.x
type ofxXML struct {
	XMLName xml.Name   `xml:"OFX"`
	Bank    bankMsgXML `xml:"BANKMSGSRSV1"`
	CC      ccMsgXML   `xml:"CREDITCARDMSGSRSV1"`
}

type bankMsgXML struct {
	Stmts []stmtXML `xml:"STMTTRNRS>STMTRS"`
}

type ccMsgXML struct {
	Stmts []stmtXML `xml:"CCSTMTTRNRS>CCSTMTRS"`
}

type stmtXML struct {
	Transactions []trnXML `xml:"BANKTRANLIST>STMTTRN"`
}

type trnXML struct {
	DTPosted string `xml:"DTPOSTED"`
	TrnAmt   string `xml:"TRNAMT"`
	FITID    string `xml:"FITID"`
	Name     string `xml:"NAME"`
	Memo     string `xml:"MEMO"`
}

// parseXML parses OFX 2.x XML content.
func parseXML(content string, decimals int) (Result, error) {
	// Strip <?OFX ...?> processing instruction if present (not valid XML PI)
	if strings.HasPrefix(content, "<?OFX") {
		end := strings.Index(content, "?>")
		if end >= 0 {
			content = strings.TrimSpace(content[end+2:])
		}
	}

	var envelope ofxXML
	if err := xml.Unmarshal([]byte(content), &envelope); err != nil {
		return Result{}, fmt.Errorf("ofx: xml parse error: %w", err)
	}

	var trns []trnXML
	for _, s := range envelope.Bank.Stmts {
		trns = append(trns, s.Transactions...)
	}
	for _, s := range envelope.CC.Stmts {
		trns = append(trns, s.Transactions...)
	}

	result := Result{Rows: make([]Row, 0, len(trns))}
	for i, t := range trns {
		date, err := parseOFXDate(t.DTPosted)
		if err != nil {
			result.Errors = append(result.Errors, RowError{Index: i + 1, Err: fmt.Sprintf("invalid DTPOSTED %q: %v", t.DTPosted, err)})
			continue
		}
		amt, err := parseAmount(t.TrnAmt, decimals)
		if err != nil {
			result.Errors = append(result.Errors, RowError{Index: i + 1, Err: fmt.Sprintf("invalid TRNAMT %q: %v", t.TrnAmt, err)})
			continue
		}
		desc := t.Name
		if desc == "" {
			desc = t.Memo
		}
		result.Rows = append(result.Rows, Row{
			Date:        date,
			Description: desc,
			Amount:      amt,
			FITID:       t.FITID,
		})
	}
	return result, nil
}

// parseOFXDate parses an OFX date string, stripping the [tz] annotation.
// Supports YYYYMMDDHHMMSS and YYYYMMDD formats. Returns UTC time.
func parseOFXDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Strip [tz] annotation like [0:GMT] or [-5:EST]
	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	layouts := []string{"20060102150405", "20060102"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("ofx: cannot parse date %q", s)
}

// parseAmount parses a decimal amount string to signed minor units.
func parseAmount(s string, decimals int) (int64, error) {
	s = strings.TrimSpace(s)
	if decimals < 0 || decimals > 18 {
		return 0, fmt.Errorf("unsupported currency precision %d", decimals)
	}
	parsed, err := money.ParseMinor(s, decimals)
	if err != nil {
		return 0, err
	}
	// ParseMinor is exact, but check the scaled integer explicitly so overflow
	// cannot wrap before it is returned as an int64.
	clean := strings.TrimSpace(s)
	negative := strings.HasPrefix(clean, "-")
	clean = strings.TrimLeft(clean, "+-")
	parts := strings.Split(clean, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid decimal amount %q", s)
	}
	whole := new(big.Int)
	if _, ok := whole.SetString(parts[0], 10); !ok {
		return 0, fmt.Errorf("invalid decimal amount %q", s)
	}
	whole.Mul(whole, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	frac += strings.Repeat("0", decimals-len(frac))
	if frac != "" {
		fraction := new(big.Int)
		if _, ok := fraction.SetString(frac, 10); !ok {
			return 0, fmt.Errorf("invalid decimal amount %q", s)
		}
		whole.Add(whole, fraction)
	}
	if negative {
		whole.Neg(whole)
	}
	if !whole.IsInt64() {
		return 0, money.ErrOverflow
	}
	return parsed, nil
}
