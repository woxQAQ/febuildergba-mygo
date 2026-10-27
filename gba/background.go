package gba

import (
	"encoding/binary"
	"image"
	"image/color"
)

// Background4BPP renders a row-major background tilemap. Width and height count
// 8x8 tiles. Each word selects a 10-bit tile index, two flip bits and one of 16
// BGR555 palette banks. Background colors, including index zero, are opaque.
func Background4BPP(
	data, palette []byte,
	tiles []uint16,
	width, height int,
) (*image.RGBA, error) {
	if width < 1 || height < 1 || width > 256 || height > 256 ||
		len(tiles) != width*height ||
		len(palette) < 512 ||
		len(data)%32 != 0 {
		return nil, ErrGraphics
	}
	var colors [256]color.RGBA
	for i := range colors {
		v := binary.LittleEndian.Uint16(palette[i*2:])
		expand := func(v uint16) uint8 { return uint8((v<<3 | v>>2) & 0xFF) }
		colors[i] = color.RGBA{
			R: expand(v & 31),
			G: expand((v >> 5) & 31),
			B: expand((v >> 10) & 31),
			A: 255,
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, width*8, height*8))
	for i, word := range tiles {
		base, bank := int(word&0x3FF)*32, int(word>>12)*16
		if base+32 > len(data) {
			return nil, ErrGraphics
		}
		x, y := (i%width)*8, (i/width)*8
		for dy := range 8 {
			for dx := range 8 {
				sx, sy := dx, dy
				if word&0x400 != 0 {
					sx = 7 - sx
				}
				if word&0x800 != 0 {
					sy = 7 - sy
				}
				index := data[base+sy*4+sx/2] >> ((sx % 2) * 4) & 15
				out.SetRGBA(x+dx, y+dy, colors[bank+int(index)])
			}
		}
	}
	return out, nil
}
