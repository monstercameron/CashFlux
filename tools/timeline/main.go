// Command timeline reconstructs every distinct dataset CashFlux has ever stored
// on the server — across the live database and every backup, decrypting
// App-Lock-encrypted snapshots — and prints them in time order with the figures
// that matter for spotting where data was lost.
//
//	CFPASS=<passcode> go run ./tools/timeline <dump.b64>
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/checkpoints"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/store"
	"golang.org/x/crypto/pbkdf2"
)

type row struct {
	User string `json:"user"`
	Ver  int64  `json:"ver"`
	Upd  string `json:"upd"`
	Src  string `json:"src"`
	Enc  bool   `json:"enc"`
	B64  string `json:"b64"`
}

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	s := string(raw)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	dec, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	var rows []row
	if err := json.Unmarshal(dec, &rows); err != nil {
		panic(err)
	}
	pass := []byte(os.Getenv("CFPASS"))
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)

	type out struct {
		when, user, src, note string
		ver                   int64
		txns, cats, snaps     int
		last                  string
		net, checking, apple  int64
		enc                   bool
	}
	var results []out

	for _, r := range rows {
		blob, err := base64.StdEncoding.DecodeString(r.B64)
		if err != nil {
			continue
		}
		note := ""
		if r.Enc {
			plain, derr := decrypt(blob, pass)
			if derr != nil {
				results = append(results, out{when: r.Upd, user: r.User, src: r.Src, ver: r.Ver, enc: true, note: "ENCRYPTED (undecryptable)"})
				continue
			}
			blob = plain
			note = "was encrypted"
		}
		ds, err := store.Import(blob)
		if err != nil {
			continue
		}
		var assets, liab, checking, apple int64
		for _, a := range ds.Accounts {
			if a.Archived {
				continue
			}
			bal := checkpoints.BalanceMinorAt(a, ds.Transactions, checkpoints.ForAccount(ds.BalanceSnapshots, a.ID), day)
			switch a.Name {
			case "SCCU Checkings":
				checking = bal
			case "Apple Credit Card":
				apple = bal
			}
			if a.IsLiability() {
				liab += bal
			} else {
				assets += bal
			}
		}
		last := ""
		for _, t := range ds.Transactions {
			if d := t.Date.Format("2006-01-02"); d > last {
				last = d
			}
		}
		results = append(results, out{
			when: r.Upd, user: r.User, src: r.Src, ver: r.Ver, note: note,
			txns: len(ds.Transactions), cats: len(ds.Categories), snaps: len(ds.BalanceSnapshots),
			last: last, net: assets - liab, checking: checking, apple: apple, enc: r.Enc,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].when < results[j].when })

	fmt.Printf("%-21s %-24s %-5s %6s %5s %-11s %12s %10s %9s  %s\n",
		"UPDATED (UTC)", "ACCOUNT", "VER", "TXNS", "CATS", "LAST TXN", "NET WORTH", "CHECKING", "APPLE CC", "NOTE")
	for _, r := range results {
		if r.note == "ENCRYPTED (undecryptable)" {
			fmt.Printf("%-21s %-24s v%-4d %6s %5s %-11s %12s %10s %9s  %s\n",
				r.when[:19], trunc(r.user, 23), r.ver, "?", "?", "?", "?", "?", "?", r.note)
			continue
		}
		fmt.Printf("%-21s %-24s v%-4d %6d %5d %-11s %12.2f %10.2f %9.2f  %s\n",
			r.when[:19], trunc(r.user, 23), r.ver, r.txns, r.cats, r.last,
			float64(r.net)/100, float64(r.checking)/100, float64(r.apple)/100, r.note)
	}
}

func decrypt(blob, pass []byte) ([]byte, error) {
	env, ok := cryptobox.Parse(blob)
	if !ok {
		return nil, fmt.Errorf("not an envelope")
	}
	salt, _ := base64.StdEncoding.DecodeString(env.Salt)
	iv, _ := base64.StdEncoding.DecodeString(env.IV)
	ct, _ := base64.StdEncoding.DecodeString(env.Cipher)
	key := pbkdf2.Key(pass, salt, cryptobox.PBKDF2Iterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ct, nil)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
