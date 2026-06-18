package id

import (
	"errors"
	"strings"
	"testing"
)

// TestPackageNewWithPrefix covers the package-level NewWithPrefix (crypto/rand
// backed) happy path. The panic-on-failure branch isn't reachable here since the
// default generator always uses crypto/rand.
func TestPackageNewWithPrefix(t *testing.T) {
	got := NewWithPrefix("acc")
	if !strings.HasPrefix(got, "acc_") {
		t.Errorf("NewWithPrefix(%q) = %q, want an acc_ prefix", "acc", got)
	}
	if len(got) != len("acc_")+byteLen*2 {
		t.Errorf("len = %d, want %d", len(got), len("acc_")+byteLen*2)
	}
}

// TestGeneratorNewWithPrefixError covers the method's error path when the
// randomness source fails.
func TestGeneratorNewWithPrefixError(t *testing.T) {
	g := NewGenerator(errReader{})
	if _, err := g.NewWithPrefix("x"); !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}
