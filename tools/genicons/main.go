// SPDX-License-Identifier: MIT

// genicons deterministically renders CashFlux's favicon and install icons.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

var (
	greenColor = color.NRGBA{R: 0x2e, G: 0x8b, B: 0x57, A: 255}
	inkColor   = color.NRGBA{R: 0x05, G: 0x2e, B: 0x13, A: 255}
)

type icon struct {
	name string
	size int
	mode string
}

var icons = []icon{
	{name: "favicon-16.png", size: 16, mode: "rounded"},
	{name: "favicon-32.png", size: 32, mode: "rounded"},
	{name: "icon-192.png", size: 192, mode: "rounded"},
	{name: "icon-512.png", size: 512, mode: "rounded"},
	{name: "icon-maskable-192.png", size: 192, mode: "maskable"},
	{name: "icon-maskable-512.png", size: 512, mode: "maskable"},
	{name: "apple-touch-icon.png", size: 180, mode: "apple"},
}

// glyphPath is the canonical cubic path also written to web/favicon.svg.
var glyphPath = [][4]image.Point{
	{{X: 77, Y: 30}, {X: 70, Y: 22}, {X: 59, Y: 18}, {X: 48, Y: 20}},
	{{X: 48, Y: 20}, {X: 32, Y: 23}, {X: 23, Y: 35}, {X: 23, Y: 50}},
	{{X: 23, Y: 50}, {X: 23, Y: 65}, {X: 32, Y: 77}, {X: 48, Y: 80}},
	{{X: 48, Y: 80}, {X: 59, Y: 82}, {X: 70, Y: 78}, {X: 77, Y: 70}},
}

func main() {
	root := filepath.Join("web")
	for _, asset := range icons {
		data, err := renderPNG(asset.size, asset.mode)
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(root, asset.name), data, 0o644); err != nil {
			fail(fmt.Errorf("write %s: %w", asset.name, err))
		}
	}
	ico, err := renderICO()
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(root, "favicon.ico"), ico, 0o644); err != nil {
		fail(fmt.Errorf("write favicon.ico: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func renderPNG(size int, mode string) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = 4
	paths := make([][]point, len(glyphPath))
	for i, segment := range glyphPath {
		paths[i] = flatten(segment)
	}
	stroke := 14.0
	if mode == "maskable" {
		stroke *= .8
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := (float64(x) + (float64(sx)+.5)/samples) / float64(size) * 100
					py := (float64(y) + (float64(sy)+.5)/samples) / float64(size) * 100
					inside := mode != "rounded" || roundedRectContains(px, py, 100, 100, 22)
					if mode == "apple" || mode == "maskable" {
						inside = true
					}
					if inside {
						r += int(greenColor.R)
						g += int(greenColor.G)
						b += int(greenColor.B)
						a += 255
					}
					gx, gy := px, py
					if mode == "maskable" {
						gx = (gx-50)/.8 + 50
						gy = (gy-50)/.8 + 50
					}
					if inside && strokeContains(paths, gx, gy, stroke) {
						r += int(inkColor.R) - int(greenColor.R)
						g += int(inkColor.G) - int(greenColor.G)
						b += int(inkColor.B) - int(greenColor.B)
					}
				}
			}
			count := samples * samples
			if a == 0 {
				img.SetNRGBA(x, y, color.NRGBA{})
				continue
			}
			covered := a / 255
			// #nosec G115 -- covered averages of 8-bit channels stay in [0,255]; a/count is also in that range.
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(r / covered), G: uint8(g / covered), B: uint8(b / covered), A: uint8(a / count)})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode %dx%d PNG: %w", size, size, err)
	}
	return out.Bytes(), nil
}

type point struct{ x, y float64 }

func flatten(segment [4]image.Point) []point {
	const steps = 64
	out := make([]point, steps+1)
	for i := range out {
		t := float64(i) / steps
		u := 1 - t
		out[i] = point{
			x: u*u*u*float64(segment[0].X) + 3*u*u*t*float64(segment[1].X) + 3*u*t*t*float64(segment[2].X) + t*t*t*float64(segment[3].X),
			y: u*u*u*float64(segment[0].Y) + 3*u*u*t*float64(segment[1].Y) + 3*u*t*t*float64(segment[2].Y) + t*t*t*float64(segment[3].Y),
		}
	}
	return out
}

func strokeContains(paths [][]point, x, y, width float64) bool {
	limit := width / 2
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			if segmentDistance(x, y, path[i-1], path[i]) <= limit {
				return true
			}
		}
	}
	return false
}

func segmentDistance(x, y float64, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	length := dx*dx + dy*dy
	t := 0.0
	if length > 0 {
		t = math.Max(0, math.Min(1, ((x-a.x)*dx+(y-a.y)*dy)/length))
	}
	return math.Hypot(x-(a.x+t*dx), y-(a.y+t*dy))
}

func roundedRectContains(x, y, w, h, radius float64) bool {
	cx := math.Max(radius, math.Min(w-radius, x))
	cy := math.Max(radius, math.Min(h-radius, y))
	return math.Hypot(x-cx, y-cy) <= radius
}

func renderICO() ([]byte, error) {
	var entries [][]byte
	for _, size := range []int{16, 32, 48} {
		data, err := renderPNG(size, "rounded")
		if err != nil {
			return nil, err
		}
		entries = append(entries, data)
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	// #nosec G115 -- entries is the fixed three-element set of 16, 32 and 48 px icons.
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(entries)))
	// #nosec G115 -- the fixed-size ICO directory is 6 + 16*3 bytes.
	offset := uint32(6 + 16*len(entries))
	for i, data := range entries {
		size := []byte{16, 32, 48}[i]
		_ = out.WriteByte(size)
		_ = out.WriteByte(size)
		_ = out.WriteByte(0)
		_ = out.WriteByte(0)
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		// #nosec G115 -- each data slice is a PNG rendered at one of the fixed 16, 32 or 48 px sizes.
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
		_ = binary.Write(&out, binary.LittleEndian, offset)
		// #nosec G115 -- cumulative fixed-size icon PNG payloads fit in the ICO uint32 offset field.
		offset += uint32(len(data))
	}
	for _, data := range entries {
		_, _ = out.Write(data)
	}
	return out.Bytes(), nil
}
