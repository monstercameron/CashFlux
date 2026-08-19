// Command cfdecrypt opens a CashFlux encrypted dataset envelope with your App
// Lock passcode and reports what is inside it.
//
//	go run ./tools/cfdecrypt <envelope-file> [--write plain.json]
//
// The passcode is read from the terminal, never from an argument (arguments end
// up in shell history and process listings) and never printed. Nothing is sent
// anywhere: this runs entirely on your machine against a file you already have.
//
// By default it prints only a SUMMARY — counts and date ranges — so a recovery
// check does not spray a household's finances across a terminal. Pass --write to
// save the decrypted JSON to a file you name.
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/store"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"

	"crypto/sha256"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: cfdecrypt <envelope-file> [--write plain.json]")
		os.Exit(2)
	}
	path := os.Args[1]
	var writeTo string
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--write" {
			writeTo = os.Args[i+1]
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		fail("read %s: %v", path, err)
	}
	if !cryptobox.IsEnvelope(raw) {
		fail("%s is not an encrypted CashFlux envelope", path)
	}
	env, ok := cryptobox.Parse(raw)
	if !ok {
		fail("could not parse the envelope header")
	}
	if env.Alg != cryptobox.AlgAESGCM {
		fail("unexpected algorithm %q", env.Alg)
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		fail("salt: %v", err)
	}
	iv, err := base64.StdEncoding.DecodeString(env.IV)
	if err != nil {
		fail("iv: %v", err)
	}
	ct, err := base64.StdEncoding.DecodeString(env.Cipher)
	if err != nil {
		fail("cipher: %v", err)
	}

	// A terminal gets a hidden prompt. When stdin is a pipe (scripted recovery)
	// the passcode is read from it instead — still never an argument, which is
	// the thing that would leak into shell history and process listings.
	var pass []byte
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "App Lock passcode: ")
		pass, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fail("read passcode: %v", err)
		}
	} else {
		line, rerr := bufio.NewReader(os.Stdin).ReadString(0x0A)
		if rerr != nil && line == "" {
			fail("read passcode from stdin: %v", rerr)
		}
		pass = []byte(strings.TrimSpace(line))
	}

	// Exactly what the browser does: PBKDF2-SHA256 at cryptobox.PBKDF2Iterations
	// into a 256-bit AES-GCM key. Slow on purpose — this takes a few seconds.
	fmt.Fprintln(os.Stderr, "deriving key (600k PBKDF2 rounds, this takes a moment)...")
	key := pbkdf2.Key(pass, salt, cryptobox.PBKDF2Iterations, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		fail("aes: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		fail("gcm: %v", err)
	}
	plain, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		fail("decryption failed — wrong passcode, or this envelope was sealed with a different one")
	}

	fmt.Printf("decrypted %d bytes\n", len(plain))

	ds, err := store.Import(plain)
	if err != nil {
		fmt.Printf("decrypted, but it did not parse as a dataset: %v\n", err)
		fmt.Printf("first bytes: %q\n", strings.TrimSpace(string(plain[:min(120, len(plain))])))
		os.Exit(1)
	}

	first, last := "", ""
	for _, t := range ds.Transactions {
		d := t.Date.Format("2006-01-02")
		if first == "" || d < first {
			first = d
		}
		if d > last {
			last = d
		}
	}
	fmt.Println()
	fmt.Printf("  transactions      %d  (%s -> %s)\n", len(ds.Transactions), first, last)
	fmt.Printf("  accounts          %d\n", len(ds.Accounts))
	fmt.Printf("  categories        %d\n", len(ds.Categories))
	fmt.Printf("  budgets           %d\n", len(ds.Budgets))
	fmt.Printf("  goals             %d\n", len(ds.Goals))
	fmt.Printf("  balance snapshots %d\n", len(ds.BalanceSnapshots))
	fmt.Printf("  holdings          %d\n", len(ds.Holdings))
	fmt.Printf("  artifacts         %d\n", len(ds.Artifacts))

	if writeTo != "" {
		if err := os.WriteFile(writeTo, plain, 0o600); err != nil {
			fail("write %s: %v", writeTo, err)
		}
		fmt.Printf("\nwrote decrypted dataset to %s (mode 0600)\n", writeTo)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
