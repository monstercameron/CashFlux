// SPDX-License-Identifier: MIT

package loadgen

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
)

// deterministicRNG is a SplitMix64 stream for reproducible load-shape
// simulation. It is deliberately not used for credentials, tokens, IDs, or
// any other security-sensitive value.
type deterministicRNG struct {
	state uint64
}

func newDeterministicRNG(seed int64) *deterministicRNG {
	encoded := sha256.Sum256([]byte(strconv.FormatInt(seed, 10)))
	return &deterministicRNG{state: binary.LittleEndian.Uint64(encoded[:8])}
}

func (r *deterministicRNG) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *deterministicRNG) Float64() float64 {
	return float64(r.next()>>11) * (1.0 / (1 << 53))
}

func (r *deterministicRNG) Int63() int64 {
	return int64(r.next() >> 1)
}

func (r *deterministicRNG) Int63n(n int64) int64 {
	if n <= 0 {
		panic("loadgen: invalid random bound")
	}
	return r.Int63() % n
}

func (r *deterministicRNG) Read(dst []byte) {
	for len(dst) > 0 {
		value := r.next()
		for i := 0; i < 8 && len(dst) > 0; i++ {
			dst[0] = byte(value)
			dst = dst[1:]
			value >>= 8
		}
	}
}
