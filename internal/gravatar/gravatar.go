// Package gravatar builds Gravatar avatar URLs from a member's email — the pure
// half of member avatars (C88). A Gravatar URL is the MD5 hash of the trimmed,
// lowercased email, with an identicon fallback so an address without a Gravatar
// still gets a stable generated image.
//
// Pure Go, no platform dependencies (crypto/md5 works under GOOS=js GOARCH=wasm);
// unit-tested on native Go.
package gravatar

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"strings"
)

const (
	base        = "https://www.gravatar.com/avatar/"
	defaultSize = 80
	maxSize     = 2048
)

// Hash returns the Gravatar hash for an email: the hex MD5 of the address after
// trimming surrounding whitespace and lowercasing.
func Hash(email string) string {
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// URL returns the Gravatar image URL for an email at the given pixel size, with an
// identicon as the default image for addresses that have no Gravatar. size is
// clamped to [1, 2048]; a non-positive size uses the default (80).
func URL(email string, size int) string {
	switch {
	case size <= 0:
		size = defaultSize
	case size > maxSize:
		size = maxSize
	}
	return base + Hash(email) + "?s=" + strconv.Itoa(size) + "&d=identicon"
}
