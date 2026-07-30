// SPDX-License-Identifier: MIT

package architecture_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRenderWASMDoesNotLinkGRPC(t *testing.T) {
	root := moduleRoot(t)
	renderDeps := goListDeps(t, root, ".")
	for _, forbidden := range []string{
		"github.com/monstercameron/GoGRPCBridge/",
		"github.com/monstercameron/CashFlux/internal/syncbridge",
		"google.golang.org/grpc",
	} {
		for _, dep := range renderDeps {
			if dep == forbidden || strings.HasPrefix(dep, forbidden) {
				t.Fatalf("render WASM links forbidden worker dependency %q", dep)
			}
		}
	}
}

func TestServicesWASMOwnsGRPC(t *testing.T) {
	root := moduleRoot(t)
	deps := goListDeps(t, root, "./cmd/cashflux-services")
	required := []string{
		"github.com/monstercameron/CashFlux/internal/syncbridge",
		"google.golang.org/grpc",
	}
	for _, want := range required {
		found := false
		for _, dep := range deps {
			if dep == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("services WASM does not link required dependency %q", want)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func goListDeps(t *testing.T, root, target string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", target)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", target, err)
	}
	return strings.Fields(string(out))
}
