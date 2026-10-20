// SPDX-License-Identifier: MIT

package importdraft

import "testing"

func TestFileSizeLabel(t *testing.T) {
	for _, tc := range []struct {
		size int
		want string
	}{{-1, "0 B"}, {0, "0 B"}, {12, "12 B"}, {1023, "1023 B"}, {1024, "1.0 KB"}, {1536, "1.5 KB"}, {1048575, "1023.9 KB"}, {1048576, "1.0 MB"}, {10 * 1048576, "10.0 MB"}} {
		if got := FileSizeLabel(tc.size); got != tc.want {
			t.Errorf("FileSizeLabel(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

func TestSourceFileError(t *testing.T) {
	for _, tc := range []struct {
		source, name, mime string
		size               int
		want               string
	}{
		{"pdf", "statement.pdf", "application/pdf", 1, ""},
		{"pdf", "statement.PDF", "", 1, ""},
		{"pdf", "statement.csv", "", 1, "importux.pdfRequired"},
		{"pdf", "statement.pdf", "text/plain", 1, "importux.pdfRequired"},
		{"pdf", "statement.pdf", "application/pdf", 10 * 1024 * 1024, ""},
		{"pdf", "statement.pdf", "application/pdf", 10*1024*1024 + 1, "importux.pdfLimit"},
		{"receipt", "receipt.png", "image/png", 1, ""},
		{"receipt", "receipt.txt", "text/plain", 1, "documents.imageTypeInvalid"},
	} {
		if got := SourceFileError(tc.source, tc.name, tc.mime, tc.size); got != tc.want {
			t.Errorf("SourceFileError(%+v) = %q", tc, got)
		}
	}
}
