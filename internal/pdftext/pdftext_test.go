package pdftext_test

import (
	"bytes"
	"compress/zlib"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/pdftext"
)

// buildPDF constructs a minimal valid-enough PDF byte slice.
// streams is a list of raw content stream bytes (already the stream body).
// Each entry may optionally be zlib-compressed if compressStreams=true.
func buildPDF(streams [][]byte, compressStreams bool) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	for _, s := range streams {
		body := s
		filter := ""
		if compressStreams {
			var zbuf bytes.Buffer
			w := zlib.NewWriter(&zbuf)
			w.Write(s)
			w.Close()
			body = zbuf.Bytes()
			filter = "/Filter /FlateDecode\n"
		}
		buf.WriteString("1 0 obj\n<<\n")
		if filter != "" {
			buf.WriteString(filter)
		}
		buf.WriteString("/Length ")
		buf.WriteString(itoa(len(body)))
		buf.WriteString("\n>>\nstream\n")
		buf.Write(body)
		buf.WriteString("\nendstream\nendobj\n")
	}
	buf.WriteString("%%EOF\n")
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// buildPDFStr is a helper that wraps buildPDF for string streams.
func buildPDFStr(streams []string, compressStreams bool) []byte {
	bs := make([][]byte, len(streams))
	for i, s := range streams {
		bs[i] = []byte(s)
	}
	return buildPDF(bs, compressStreams)
}

func TestExtractText_SimpleTj(t *testing.T) {
	stream := "BT\n(Hello World) Tj\nET\n"
	data := buildPDFStr([]string{stream}, false)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hello World") {
		t.Errorf("want 'Hello World' in %q", text)
	}
}

func TestExtractText_TJArray(t *testing.T) {
	// TJ with kerning: large negative number inserts a space.
	stream := "BT\n[(Hello) -300 (World)] TJ\nET\n"
	data := buildPDFStr([]string{stream}, false)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Errorf("want Hello and World in %q", text)
	}
}

func TestExtractText_HexString(t *testing.T) {
	// <48656c6c6f> = "Hello"
	stream := "BT\n<48656c6c6f> Tj\nET\n"
	data := buildPDFStr([]string{stream}, false)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hello") {
		t.Errorf("want Hello in %q", text)
	}
}

func TestExtractText_FlateCompressed(t *testing.T) {
	stream := "BT\n(Bank Statement) Tj\nET\n"
	data := buildPDFStr([]string{stream}, true)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Bank Statement") {
		t.Errorf("want 'Bank Statement' in %q", text)
	}
}

func TestExtractText_MultipleStreams(t *testing.T) {
	s1 := "BT\n(Line one) Tj\nET\n"
	s2 := "BT\n(Line two) Tj\nET\n"
	data := buildPDFStr([]string{s1, s2}, false)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Line one") {
		t.Errorf("want 'Line one' in %q", text)
	}
	if !strings.Contains(text, "Line two") {
		t.Errorf("want 'Line two' in %q", text)
	}
}

func TestExtractText_Encrypted(t *testing.T) {
	data := []byte("%PDF-1.4\n/Encrypt << >>\n")
	_, err := pdftext.ExtractText(data)
	if err != pdftext.ErrEncrypted {
		t.Errorf("want ErrEncrypted, got %v", err)
	}
}

func TestExtractText_NotAPDF(t *testing.T) {
	_, err := pdftext.ExtractText([]byte("this is not a pdf"))
	if err == nil {
		t.Fatal("expected error for non-PDF data")
	}
}

func TestExtractText_NoText(t *testing.T) {
	// A valid PDF header but no BT…ET blocks (image-only simulation).
	data := []byte("%PDF-1.4\n% image-only placeholder\n%%EOF\n")
	_, err := pdftext.ExtractText(data)
	if err != pdftext.ErrNoText {
		t.Errorf("want ErrNoText, got %v", err)
	}
}

func TestExtractText_EscapeSequences(t *testing.T) {
	// Backslash escapes inside literal strings.
	stream := "BT\n(Hello\\nWorld) Tj\nET\n"
	data := buildPDFStr([]string{stream}, false)

	text, err := pdftext.ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Errorf("want Hello and World in %q", text)
	}
}
