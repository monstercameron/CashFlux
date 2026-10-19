// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputPathCleansAndRequiresRegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "envelope.json")
	if err := os.WriteFile(file, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := inputPath(filepath.Join(dir, "sub", "..", "envelope.json"))
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

func TestOutputPathCleansAndRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := outputPath(filepath.Join(dir, "sub", "..", "plain.json"))
	if err != nil {
		t.Fatalf("outputPath: %v", err)
	}
	want, err := filepath.Abs(filepath.Join(dir, "plain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("outputPath = %q, want %q", got, want)
	}
	if _, err := outputPath(dir); err == nil {
		t.Error("outputPath accepted a directory")
	}
}
