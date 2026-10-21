// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"bytes"
	"testing"
)

func TestLocalArtifactCipherBindsFileToID(t *testing.T) {
	initFixture(t)
	cipher := LocalArtifactCipher{}
	plain := []byte("application/pdf\x00private statement")
	sealed, err := cipher.SealArtifact("pdf-a", plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("private statement")) || bytes.Contains(sealed, []byte("application/pdf")) {
		t.Fatal("sealed local artifact exposed its MIME or contents")
	}
	opened, err := cipher.OpenArtifact("pdf-a", sealed)
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatalf("open local artifact: %v", err)
	}
	if _, err := cipher.OpenArtifact("pdf-b", sealed); err == nil {
		t.Fatal("local artifact opened under another ID")
	}
}
