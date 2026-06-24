// SPDX-License-Identifier: MIT

package ofx

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// Row is one normalized OFX transaction. Amount is signed minor units.
type Row struct {
	Date        time.Time
	Description string
	Amount      int64 // signed minor units, credit positive, debit negative
	FITID       string
}

// Parse parses OFX 1.x (SGML) or OFX 2.x (XML) content from r.
// Decimals is the minor-unit precision (e.g. 2 for USD/GBP).
func Parse(r io.Reader, decimals int) ([]Row, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := string(data)
	trimmed := strings.TrimSpace(content)

	if strings.HasPrefix(trimmed, "OFXHEADER:") {
		return parseSGML(trimmed, decimals)
	}
	if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<?OFX") || strings.HasPrefix(trimmed, "<OFX") {
		return parseXML(trimmed, decimals)
	}
	return nil, fmt.Errorf("ofx: unrecognized format")
}

// parseSGML parses OFX 1.x SGML line-by-line.
func parseSGML(content string, decimals int) ([]Row, error) {
	var rows []Row
	var cur *Row
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
				rows = append(rows, *cur)
				cur = nil
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
			if err == nil {
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
	return rows, nil
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
func parseXML(content string, decimals int) ([]Row, error) {
	// Strip <?OFX ...?> processing instruction if present (not valid XML PI)
	if strings.HasPrefix(content, "<?OFX") {
		end := strings.Index(content, "?>")
		if end >= 0 {
			content = strings.TrimSpace(content[end+2:])
		}
	}

	var envelope ofxXML
	if err := xml.Unmarshal([]byte(content), &envelope); err != nil {
		return nil, fmt.Errorf("ofx: xml parse error: %w", err)
	}

	var trns []trnXML
	for _, s := range envelope.Bank.Stmts {
		trns = append(trns, s.Transactions...)
	}
	for _, s := range envelope.CC.Stmts {
		trns = append(trns, s.Transactions...)
	}

	rows := make([]Row, 0, len(trns))
	for _, t := range trns {
		date, err := parseOFXDate(t.DTPosted)
		if err != nil {
			continue
		}
		amt, err := parseAmount(t.TrnAmt, decimals)
		if err != nil {
			continue
		}
		desc := t.Name
		if desc == "" {
			desc = t.Memo
		}
		rows = append(rows, Row{
			Date:        date,
			Description: desc,
			Amount:      amt,
			FITID:       t.FITID,
		})
	}
	return rows, nil
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
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	factor := math.Pow(10, float64(decimals))
	return int64(math.Round(f * factor)), nil
}
