// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedIconsAreByteIdentical(t *testing.T) {
	for _, asset := range icons {
		want, err := renderPNG(asset.size, asset.mode)
		if err != nil {
			t.Fatalf("render %s: %v", asset.name, err)
		}
		got, err := os.ReadFile(filepath.Join("..", "..", "web", asset.name))
		if err != nil {
			t.Fatalf("read %s: %v", asset.name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from deterministic generator output", asset.name)
		}
	}
	want, err := renderICO()
	if err != nil {
		t.Fatalf("render favicon.ico: %v", err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "web", "favicon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("favicon.ico differs from deterministic generator output")
	}
}

func TestICOContainsValid16_32And48PixelImages(t *testing.T) {
	data, err := renderICO()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 || binary.LittleEndian.Uint16(data[4:6]) != 3 {
		t.Fatalf("ICO directory header is invalid: %x", data[:min(len(data), 6)])
	}
	for i, wantSize := range []int{16, 32, 48} {
		entry := data[6+i*16 : 6+(i+1)*16]
		width, height := int(entry[0]), int(entry[1])
		if width != wantSize || height != wantSize {
			t.Errorf("entry %d size = %dx%d, want %dx%d", i, width, height, wantSize, wantSize)
		}
		length := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if offset < 54 || length < 1 || offset+length > len(data) {
			t.Fatalf("entry %d has invalid PNG bounds: offset=%d length=%d", i, offset, length)
		}
		img, err := png.Decode(bytes.NewReader(data[offset : offset+length]))
		if err != nil {
			t.Errorf("entry %d PNG decode: %v", i, err)
		} else if img.Bounds().Dx() != wantSize || img.Bounds().Dy() != wantSize {
			t.Errorf("entry %d PNG dimensions = %v", i, img.Bounds())
		}
	}
}

func TestSVGUsesTheCanonicalPathWithoutFonts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "web", "favicon.svg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]byte{[]byte("role=\"img\""), []byte("aria-label=\"CashFlux\""), []byte("<path d=\"M 77 30")} {
		if !bytes.Contains(data, want) {
			t.Errorf("favicon.svg missing %q", want)
		}
	}
	if bytes.Contains(data, []byte("<text")) || bytes.Contains(data, []byte("font-family")) {
		t.Error("favicon.svg must draw its C as a path without fonts")
	}
}
