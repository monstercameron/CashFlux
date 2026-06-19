package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: "proto/cashflux/v1/cashflux.proto",
			want: []string{
				"package cashflux.v1;",
				`option go_package = "github.com/monstercameron/CashFlux/internal/backendrpc/pb;backendrpcpb";`,
				"service SyncService",
				"service AIService",
			},
		},
		{
			path: "proto/README.md",
			want: []string{
				"## Versioning Policy",
				"## Deprecation Windows",
				"## Future Codegen",
				"Do not renumber existing fields.",
				"reserve them in the message before deletion",
			},
		},
		{
			path: "internal/server/config.go",
			want: []string{
				`APIVersion          = "v1"`,
				`MinClientAPIVersion = "v1"`,
			},
		},
		{
			path: ".github/workflows/ci.yml",
			want: []string{
				"go run ./cmd/api_compat_guard",
			},
		},
	}

	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			fail("read %s: %v", check.path, err)
		}
		text := string(data)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				fail("%s missing %q", check.path, want)
			}
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "api compatibility guard: "+format+"\n", args...)
	os.Exit(1)
}
