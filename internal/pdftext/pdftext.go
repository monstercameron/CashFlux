// Package pdftext provides a minimal pure-Go text extractor for simple PDFs.
// It handles:
//   - Uncompressed content streams
//   - FlateDecode (zlib/deflate) compressed content streams
//   - BT…ET text blocks with Tj (single string) and TJ (array) operators
//   - Both literal strings (...) and hex strings <…>
//
// Limitations (documented):
//   - Does not handle encrypted PDFs — returns [ErrEncrypted].
//   - Does not handle image-only (scanned) PDFs — returns [ErrNoText].
//   - Does not decode font encodings; multi-byte / CID-keyed fonts (e.g. CJK,
//     embedded Unicode maps) may produce garbled output. For those files an
//     AI/vision fallback (C74 Tier 3) is recommended.
//   - ASCIIHexDecode and ASCII85Decode streams are not decoded; they appear only
//     in very old PDFs and are skipped silently.
//   - Does not follow cross-reference streams (PDF 1.5+ compressed xref) for
//     locating streams; it scans the raw bytes instead.
//
// Pure Go, stdlib only (compress/flate, bytes, strings). No syscall/js.
package pdftext

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrEncrypted is returned when the PDF is encrypted.
var ErrEncrypted = errors.New("pdftext: PDF is encrypted; cannot extract text")

// ErrNoText is returned when no text operators were found (image-only or empty PDF).
var ErrNoText = errors.New("pdftext: no text found; PDF may be image-only")

// ExtractText extracts all text from a simple PDF. It returns the concatenated
// text from all BT…ET blocks across all content streams found in the file.
// Words are separated by a single space; blocks are separated by a newline.
func ExtractText(data []byte) (string, error) {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return "", fmt.Errorf("pdftext: not a PDF (missing %%PDF header)")
	}

	// Encrypted PDFs cannot be decoded.
	if bytes.Contains(data, []byte("/Encrypt")) {
		return "", ErrEncrypted
	}

	streams := extractStreams(data)

	var sb strings.Builder
	found := false

	for _, raw := range streams {
		text := extractTextFromStream(raw)
		if text != "" {
			found = true
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(text)
		}
	}

	if !found {
		return "", ErrNoText
	}
	return strings.TrimSpace(sb.String()), nil
}

// extractStreams scans the PDF bytes for all stream…endstream pairs and returns
// their decoded contents. FlateDecode streams are decompressed; others are
// returned as-is (they may not contain text operators but won't cause errors).
func extractStreams(data []byte) [][]byte {
	var out [][]byte
	pos := 0
	for {
		// Find next "stream" keyword.
		idx := bytes.Index(data[pos:], []byte("stream"))
		if idx < 0 {
			break
		}
		streamStart := pos + idx + len("stream")
		// Skip the mandatory CR, LF, or CRLF after "stream".
		if streamStart < len(data) && data[streamStart] == '\r' {
			streamStart++
		}
		if streamStart < len(data) && data[streamStart] == '\n' {
			streamStart++
		}

		// Find the matching "endstream".
		endIdx := bytes.Index(data[streamStart:], []byte("endstream"))
		if endIdx < 0 {
			break
		}
		raw := data[streamStart : streamStart+endIdx]

		// Look backwards from "stream" to find the stream dictionary and detect
		// /Filter /FlateDecode (or /Fl short form).
		dictRegion := data[pos : pos+idx]
		isFlate := bytes.Contains(dictRegion, []byte("/FlateDecode")) ||
			bytes.Contains(dictRegion, []byte("/Fl\n")) ||
			bytes.Contains(dictRegion, []byte("/Fl ")) ||
			bytes.Contains(dictRegion, []byte("/Fl/")) ||
			bytes.Contains(dictRegion, []byte("/Fl>"))

		if isFlate {
			if decoded, err := zlibDecode(raw); err == nil {
				out = append(out, decoded)
			}
			// If decoding fails (e.g. corrupted stream), skip it.
		} else {
			out = append(out, raw)
		}

		pos = streamStart + endIdx + len("endstream")
	}
	return out
}

// zlibDecode decompresses a FlateDecode stream (zlib format).
func zlibDecode(data []byte) ([]byte, error) {
	// Trim any trailing whitespace that might have been included.
	data = bytes.TrimRight(data, "\r\n \t")
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// extractTextFromStream extracts the text from a single decoded content stream
// by scanning BT…ET blocks and processing Tj and TJ operators.
func extractTextFromStream(data []byte) string {
	s := string(data)
	var sb strings.Builder

	// Scan for BT…ET blocks.
	for {
		btIdx := strings.Index(s, "BT")
		if btIdx < 0 {
			break
		}
		etIdx := strings.Index(s[btIdx:], "ET")
		if etIdx < 0 {
			break
		}
		block := s[btIdx+2 : btIdx+etIdx]
		text := parseTextBlock(block)
		if text != "" {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(text)
		}
		s = s[btIdx+etIdx+2:]
	}
	return sb.String()
}

// parseTextBlock processes the content of a BT…ET block and returns concatenated
// text from Tj and TJ operators.
func parseTextBlock(block string) string {
	var sb strings.Builder
	tokens := tokenizePDF(block)

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok {
		case "Tj":
			// Previous token should be a string operand.
			if i > 0 {
				if text, ok := decodePDFString(tokens[i-1]); ok {
					sb.WriteString(text)
				}
			}
		case "TJ":
			// Previous token should be an array operand.
			if i > 0 {
				text := decodeTJArray(tokens[i-1])
				sb.WriteString(text)
			}
		case "'", `"`:
			// Move to next line and show string.
			if i > 0 {
				if text, ok := decodePDFString(tokens[i-1]); ok {
					if sb.Len() > 0 {
						sb.WriteByte(' ')
					}
					sb.WriteString(text)
				}
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// tokenizePDF splits a PDF content stream fragment into tokens: string literals
// (including their delimiters), array literals (including their delimiters), and
// bare PDF keywords/numbers. This is intentionally simple — it handles the subset
// of operators used inside BT…ET blocks.
func tokenizePDF(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		ch := s[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
		case ch == '(':
			// Literal string: scan to matching unescaped ')'.
			end, ok := scanLiteralString(s, i)
			if !ok {
				i++
				continue
			}
			tokens = append(tokens, s[i:end])
			i = end
		case ch == '<' && i+1 < len(s) && s[i+1] != '<':
			// Hex string.
			end := strings.Index(s[i+1:], ">")
			if end < 0 {
				i++
				continue
			}
			tokens = append(tokens, s[i:i+1+end+1])
			i = i + 1 + end + 1
		case ch == '[':
			// Array (for TJ).
			end := strings.Index(s[i+1:], "]")
			if end < 0 {
				i++
				continue
			}
			tokens = append(tokens, s[i:i+1+end+1])
			i = i + 1 + end + 1
		case ch == '%':
			// Comment: skip to end of line.
			nl := strings.IndexByte(s[i:], '\n')
			if nl < 0 {
				i = len(s)
			} else {
				i += nl + 1
			}
		default:
			// Keyword or number.
			end := i
			for end < len(s) {
				c := s[end]
				if c == ' ' || c == '\t' || c == '\r' || c == '\n' ||
					c == '(' || c == ')' || c == '<' || c == '>' ||
					c == '[' || c == ']' || c == '{' || c == '}' || c == '/' || c == '%' {
					break
				}
				end++
			}
			if end > i {
				tokens = append(tokens, s[i:end])
			}
			i = end
		}
	}
	return tokens
}

// scanLiteralString scans from position start (which must be '(') and returns
// the end position (exclusive, i.e. past the closing ')') and whether it succeeded.
// It handles nested parentheses and backslash escapes.
func scanLiteralString(s string, start int) (int, bool) {
	depth := 0
	for i := start; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' {
			i++ // skip escaped character
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return len(s), false
}

// decodePDFString decodes a PDF literal string token (parenthesised) or a hex
// string token (<…>) into a plain Go string. Returns ("", false) if tok is
// not a string token.
func decodePDFString(tok string) (string, bool) {
	if len(tok) < 2 {
		return "", false
	}
	if tok[0] == '(' && tok[len(tok)-1] == ')' {
		return decodeLiteralString(tok[1 : len(tok)-1]), true
	}
	if tok[0] == '<' && tok[len(tok)-1] == '>' {
		return decodeHexString(tok[1 : len(tok)-1]), true
	}
	return "", false
}

// decodeLiteralString handles PDF escape sequences inside a literal string.
func decodeLiteralString(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch != '\\' {
			sb.WriteByte(ch)
			i++
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		switch s[i] {
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case '(', ')', '\\':
			sb.WriteByte(s[i])
		case '\r':
			// Line continuation: skip optional following '\n'.
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case '\n':
			// Line continuation.
		default:
			// Octal sequence: up to 3 digits.
			if s[i] >= '0' && s[i] <= '7' {
				oct := string(s[i])
				if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
					i++
					oct += string(s[i])
					if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
						i++
						oct += string(s[i])
					}
				}
				v, _ := strconv.ParseUint(oct, 8, 8)
				sb.WriteByte(byte(v))
			} else {
				sb.WriteByte(s[i])
			}
		}
		i++
	}
	return sb.String()
}

// decodeHexString decodes a PDF hex string (pairs of hex digits, whitespace ignored).
func decodeHexString(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s)%2 != 0 {
		s += "0" // PDF spec: odd nibble padded with 0.
	}
	var sb strings.Builder
	for i := 0; i+1 < len(s); i += 2 {
		v, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			continue
		}
		sb.WriteByte(byte(v))
	}
	return sb.String()
}

// decodeTJArray decodes a TJ operand (an array like [(Hello) -250 (World)]) by
// concatenating the string elements and inserting a space for large negative
// kerning values (< −100 thousandths of a text unit).
func decodeTJArray(tok string) string {
	if len(tok) < 2 || tok[0] != '[' || tok[len(tok)-1] != ']' {
		return ""
	}
	inner := tok[1 : len(tok)-1]
	tokens := tokenizePDF(inner)
	var sb strings.Builder
	for _, t := range tokens {
		if text, ok := decodePDFString(t); ok {
			sb.WriteString(text)
		} else {
			// Numeric kerning: a large negative value means a word space.
			if v, err := strconv.ParseFloat(t, 64); err == nil && v < -100 {
				if sb.Len() > 0 {
					sb.WriteByte(' ')
				}
			}
		}
	}
	return sb.String()
}
