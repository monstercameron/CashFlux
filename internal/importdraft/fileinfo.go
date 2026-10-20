// SPDX-License-Identifier: MIT

package importdraft

import (
	"fmt"
	"strings"
)

// SourceFileError validates a PDF or image before its bytes enter the draft.
// The result is a translation key, or empty when the file can be read.
func SourceFileError(source, name, mime string, size int) string {
	if source != "pdf" {
		return ImageError(mime, size, 10*1024*1024)
	}
	if mime != "application/pdf" && (mime != "" || !strings.HasSuffix(strings.ToLower(name), ".pdf")) {
		return "importux.pdfRequired"
	}
	if size > 10*1024*1024 {
		return "importux.pdfLimit"
	}
	return ""
}

// FileSizeLabel formats a selected source's byte count without hiding small files.
func FileSizeLabel(size int) string {
	if size < 0 {
		size = 0
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	unit, divisor := "KB", 1024
	if size >= 1024*1024 {
		unit, divisor = "MB", 1024*1024
	}
	whole, tenth := size/divisor, (size%divisor)*10/divisor
	return fmt.Sprintf("%d.%d %s", whole, tenth, unit)
}
