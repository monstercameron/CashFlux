// SPDX-License-Identifier: MIT

package id

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNewDeterministicWithSource(t *testing.T) {
	// 16 fixed bytes -> known hex string.
	src := bytes.NewReader([]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	})
	g := NewGenerator(src)
	got, err := g.New()
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	want := "00112233445566778899aabbccddeeff"
	if got != want {
		t.Errorf("New = %q, want %q", got, want)
	}
}

func TestNewLength(t *testing.T) {
	g := NewGenerator(repeatReader(0xAB))
	got, err := g.New()
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if len(got) != byteLen*2 {
		t.Errorf("len = %d, want %d", len(got), byteLen*2)
	}
}

func TestNewWithPrefix(t *testing.T) {
	g := NewGenerator(repeatReader(0x00))
	got, err := g.NewWithPrefix(" acc ")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.HasPrefix(got, "acc_") {
		t.Errorf("prefix missing: %q", got)
	}
	bare, _ := NewGenerator(repeatReader(0x00)).NewWithPrefix("")
	if strings.Contains(bare, "_") {
		t.Errorf("empty prefix should not add underscore: %q", bare)
	}
}

func TestNewUniqueAcrossCalls(t *testing.T) {
	// Uses the package default (crypto/rand) to exercise real uniqueness.
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		s := New()
		if seen[s] {
			t.Fatalf("collision at %d: %s", i, s)
		}
		seen[s] = true
	}
}

func TestNewErrorOnShortRead(t *testing.T) {
	g := NewGenerator(bytes.NewReader([]byte{0x01, 0x02})) // fewer than 16 bytes
	if _, err := g.New(); err == nil {
		t.Error("expected error on short read")
	}
}

func TestNewErrorPropagates(t *testing.T) {
	g := NewGenerator(errReader{})
	_, err := g.New()
	if !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

// --- helpers ---

func repeatReader(b byte) io.Reader { return &repeating{b: b} }

type repeating struct{ b byte }

func (r *repeating) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}

var errBoom = errors.New("boom")

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errBoom }
