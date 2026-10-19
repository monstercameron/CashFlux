// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestSV05RestoreReloadWaitsForWriteCallback(t *testing.T) {
	var finishWrite func()
	var adopted, reloaded bool
	reloadAfterDurableWrites(func(done func()) { finishWrite = done }, func() { adopted = true }, func() { reloaded = true })
	if adopted || reloaded {
		t.Fatal("restore advanced before its write callback")
	}
	finishWrite()
	if !adopted || !reloaded {
		t.Fatalf("completion did not adopt and reload: adopted=%v reloaded=%v", adopted, reloaded)
	}
}

func TestSV05WorkspaceSwitchBumpsGenerationBeforeReload(t *testing.T) {
	generation := "old"
	var reloaded bool
	bumpAndReloadWorkspace(func() string { return "new" }, func(next string) { generation = next }, func() {
		if generation != "new" {
			t.Error("workspace reloaded before adopting its new dataset generation")
		}
		reloaded = true
	})
	if !reloaded || generation != "new" {
		t.Fatalf("workspace switch did not adopt generation and reload: generation=%q reloaded=%v", generation, reloaded)
	}
}
