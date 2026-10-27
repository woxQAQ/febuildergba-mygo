// Package gba implements binary graphics formats shared by the game adapters.
package gba

import (
	"encoding/binary"
	"image"
	"image/color"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

var ErrGraphics = fault.Code{
	ID:   "gba.invalid_graphics",
	Kind: fault.InvalidData,
}

// DecompressLZ77 reads a GBA 0x10 stream, bounded by the caller's resource
// size.
// Back-references may overlap the output being produced.
func DecompressLZ77(data []byte, limit int) ([]byte, error) {
	if len(data) < 4 || data[0] != 0x10 {
		return nil, ErrGraphics
	}
	length := int(data[1]) | int(data[2])<<8 | int(data[3])<<16
	if length < 1 || length > limit {
		return nil, ErrGraphics
	}
	out := make([]byte, 0, length)
	at := 4
	for len(out) < length {
		if at >= len(data) {
			return nil, ErrGraphics
		}
		flags := data[at]
		at++
		for mask := byte(0x80); mask != 0 && len(out) < length; mask >>= 1 {
			if at >= len(data) {
				return nil, ErrGraphics
			}
			if flags&mask == 0 {
				out = append(out, data[at])
				at++
				continue
			}
			if at+1 >= len(data) {
				return nil, ErrGraphics
			}
			count := int(data[at]>>4) + 3
			distance := (int(data[at]&15)<<8 | int(data[at+1])) + 1
			at += 2
			if distance > len(out) {
				return nil, ErrGraphics
			}
			for range min(count, length-len(out)) {
				out = append(out, out[len(out)-distance])
			}
		}
	}
	return out, nil
}

// Tiles4BPP renders row-major 8x8 tiles with a BGR555 palette. Index zero is
// transparent.
func Tiles4BPP(data, palette []byte, width, height int) (*image.RGBA, error) {
	if width < 8 || height < 8 || width%8 != 0 || height%8 != 0 ||
		width > 2048 ||
		height > 2048 ||
		len(palette) < 32 ||
		len(data) < width*height/2 {
		return nil, ErrGraphics
	}
	var colors [16]color.RGBA
	for i := 1; i < 16; i++ {
		v := binary.LittleEndian.Uint16(palette[i*2:])
		expand := func(c uint16) uint8 { return uint8((c<<3 | c>>2) & 0xFF) }
		colors[i] = color.RGBA{
			R: expand(v & 31),
			G: expand((v >> 5) & 31),
			B: expand((v >> 10) & 31),
			A: 255,
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			index := ((y/8)*(width/8)+x/8)*32 + (y%8)*4 + (x%8)/2
			pixel := data[index] >> ((x % 2) * 4) & 15
			out.SetRGBA(x, y, colors[pixel])
		}
	}
	return out, nil
}
