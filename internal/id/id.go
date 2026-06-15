// Package id generates stable, collision-resistant identifiers for CashFlux
// entities. IDs are random 128-bit values rendered as hex, optionally prefixed
// (e.g. "acc_1a2b…") for readability.
//
// The package is pure Go. Production uses crypto/rand; tests inject a
// deterministic source via Generator for reproducible output.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const byteLen = 16 // 128 bits

// Generator produces IDs from a configurable randomness source.
type Generator struct {
	src io.Reader
}

// NewGenerator returns a Generator reading from src. A nil src uses crypto/rand.
func NewGenerator(src io.Reader) *Generator {
	if src == nil {
		src = rand.Reader
	}
	return &Generator{src: src}
}

// New returns a new random hex ID, or an error if the source fails.
func (g *Generator) New() (string, error) {
	b := make([]byte, byteLen)
	if _, err := io.ReadFull(g.src, b); err != nil {
		return "", fmt.Errorf("id: read random source: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewWithPrefix returns a new ID prefixed with prefix and an underscore
// (e.g. "acc_<hex>"). An empty prefix behaves like New.
func (g *Generator) NewWithPrefix(prefix string) (string, error) {
	core, err := g.New()
	if err != nil {
		return "", err
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return core, nil
	}
	return prefix + "_" + core, nil
}

// defaultGen is the package-level generator backed by crypto/rand.
var defaultGen = NewGenerator(nil)

// New returns a new random hex ID using crypto/rand. It panics only if the
// system randomness source fails, which is treated as unrecoverable.
func New() string {
	s, err := defaultGen.New()
	if err != nil {
		panic(err)
	}
	return s
}

// NewWithPrefix returns a new prefixed ID using crypto/rand. It panics only if
// the system randomness source fails.
func NewWithPrefix(prefix string) string {
	s, err := defaultGen.NewWithPrefix(prefix)
	if err != nil {
		panic(err)
	}
	return s
}
