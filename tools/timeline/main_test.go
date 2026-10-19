// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputPathCleansAndRequiresRegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "dump.b64")
	if err := os.WriteFile(file, []byte("e30="), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := inputPath(filepath.Join(dir, "sub", "..", "dump.b64"))
	if err != nil {
		t.Fatalf("inputPath: %v", err)
	}
	want, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("inputPath = %q, want %q", got, want)
	}
	if _, err := inputPath(dir); err == nil {
		t.Error("inputPath accepted a directory")
	}
	if _, err := inputPath("  "); err == nil {
		t.Error("inputPath accepted an empty path")
	}
}
