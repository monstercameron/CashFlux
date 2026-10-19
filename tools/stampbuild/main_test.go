// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/monstercameron/CashFlux/internal/buildinfo"
)

func TestRunWritesMatchingBuildJSONAndLDFlags(t *testing.T) {
	dir := t.TempDir()
	buildJSON := filepath.Join(dir, "build.json")
	ldflags := filepath.Join(dir, "ldflags.txt")

	if err := run(buildJSON, ldflags, "", "v1.20.0-abc1234", 7); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	rawJSON, err := os.ReadFile(buildJSON)
	if err != nil {
		t.Fatalf("read build.json: %v", err)
	}
	info, err := buildinfo.ParseBuildJSON(rawJSON)
	if err != nil {
		t.Fatalf("ParseBuildJSON() error = %v", err)
	}
	if info.BuildID != "v1.20.0-abc1234" {
		t.Fatalf("build ID = %q, want exact release tag and commit", info.BuildID)
	}
	if info.Schema != 7 {
		t.Fatalf("schema = %d, want 7", info.Schema)
	}

	rawFlags, err := os.ReadFile(ldflags)
	if err != nil {
		t.Fatalf("read ldflags: %v", err)
	}
	wantFlags := buildinfo.LDFlags(info.BuildID) + "\n"
	if string(rawFlags) != wantFlags {
		t.Fatalf("ldflags = %q, want %q", rawFlags, wantFlags)
	}
}

func TestRunUsesDistinctIDsForRepeatedBuildsOfSameCommit(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	if err := run(first, "", "abc1234", "", 7); err != nil {
		t.Fatalf("first run() error = %v", err)
	}
	if err := run(second, "", "abc1234", "", 7); err != nil {
		t.Fatalf("second run() error = %v", err)
	}

	firstJSON, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first build.json: %v", err)
	}
	secondJSON, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second build.json: %v", err)
	}
	firstInfo, err := buildinfo.ParseBuildJSON(firstJSON)
	if err != nil {
		t.Fatalf("parse first build.json: %v", err)
	}
	secondInfo, err := buildinfo.ParseBuildJSON(secondJSON)
	if err != nil {
		t.Fatalf("parse second build.json: %v", err)
	}
	if firstInfo.BuildID == secondInfo.BuildID {
		t.Fatalf("repeated build ID collision: %q", firstInfo.BuildID)
	}
}
