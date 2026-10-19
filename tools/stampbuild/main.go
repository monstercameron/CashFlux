// SPDX-License-Identifier: MIT

// Command stampbuild stamps one release build identity into the two places
// that must agree (SV7a): web/build.json, which the service worker and open
// tabs read to learn the active build, and — via the -ldflags fragment it
// prints (or writes with -ldflags-out) — the wasm binary itself, so a
// running tab knows which build it is.
//
// Usage:
//
//	go run ./tools/stampbuild -out web/build.json -ldflags-out web/bin/ldflags.txt
//	GOOS=js GOARCH=wasm go build -ldflags "-s -w $(cat web/bin/ldflags.txt)" -o web/bin/main.wasm .
//
// A build that skips this step keeps build ID "dev": the service worker
// behaves as it always has (immediate activation, no update banner), which
// is what `gwc dev` live reload needs.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/buildinfo"
	"github.com/monstercameron/CashFlux/internal/store"
)

func main() {
	out := flag.String("out", "web/build.json", "path of the build.json to write")
	ldflagsOut := flag.String("ldflags-out", "", "optional path to also write the -ldflags fragment to")
	sha := flag.String("sha", "", "git short SHA to stamp (default: git rev-parse --short HEAD)")
	schema := flag.Int("schema", store.SchemaVersion, "dataset schema version this build writes")
	flag.Parse()

	if err := run(*out, *ldflagsOut, *sha, *schema); err != nil {
		slog.Error("stampbuild failed", "error", err)
		os.Exit(1)
	}
}

func run(out, ldflagsOut, sha string, schema int) error {
	if strings.TrimSpace(sha) == "" {
		sha = gitShortSHA()
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("stampbuild: generate unique build ID: %w", err)
	}
	info := buildinfo.Info{
		BuildID: buildinfo.FormatBuildID(sha+"-"+hex.EncodeToString(nonce[:]), time.Now().UnixNano()),
		Schema:  schema,
	}
	raw, err := buildinfo.MarshalBuildJSON(info)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		return fmt.Errorf("stampbuild: write %s: %w", out, err)
	}
	lf := buildinfo.LDFlags(info.BuildID)
	if ldflagsOut != "" {
		if err := os.WriteFile(ldflagsOut, []byte(lf+"\n"), 0o644); err != nil {
			return fmt.Errorf("stampbuild: write %s: %w", ldflagsOut, err)
		}
	}
	// The build ID and the exact flag are the contract with the calling
	// build script, so they go to stdout in a stable, greppable shape.
	fmt.Printf("buildId=%s\nschema=%d\nldflags=%s\n", info.BuildID, info.Schema, lf)
	slog.Info("stamped build", "buildId", info.BuildID, "schema", info.Schema, "out", out)
	return nil
}

// gitShortSHA asks git for the current commit. Any failure yields "" and
// FormatBuildID stamps "nogit-…" rather than failing the release build.
func gitShortSHA() string {
	raw, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		slog.Warn("stampbuild: git rev-parse failed, stamping nogit", "error", err)
		return ""
	}
	return strings.TrimSpace(string(raw))
}
