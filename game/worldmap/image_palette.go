package worldmap

import (
	"encoding/binary"
	"image/color"

	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// nativePalette keeps duplicate colors and the transparent slot's RGB value.
func nativePalette(data []byte, transparent bool) color.Palette {
	colors := make(color.Palette, len(data)/2)
	for i := range colors {
		v := binary.LittleEndian.Uint16(data[i*2:])
		expand := func(v uint16) uint8 { return uint8((v<<3 | v>>2) & 255) }
		alpha := uint8(255)
		if transparent && i == 0 {
			alpha = 0
		}
		colors[i] = color.NRGBA{
			R: expand(v & 31),
			G: expand(v >> 5 & 31),
			B: expand(v >> 10 & 31),
			A: alpha,
		}
	}
	return colors
}

// putPalette changes only defined color bits. The transparent entry and absent
// entries retain all original bits; visible colors retain the unused high bit.
func putPalette(data []byte, colors color.Palette, transparent bool) error {
	for i := range min(len(data)/2, len(colors)) {
		pixel := color.NRGBAModel.Convert(colors[i]).(color.NRGBA)
		if transparent && i == 0 {
			if pixel.A != 0 {
				return gba.ErrGraphics
			}
			continue
		}
		if pixel.A != 255 {
			return gba.ErrGraphics
		}
		quantize := func(v uint8) uint16 { return (uint16(v)*31 + 127) / 255 }
		value := quantize(
			pixel.R,
		) | quantize(
			pixel.G,
		)<<5 | quantize(
			pixel.B,
		)<<10
		value |= binary.LittleEndian.Uint16(data[i*2:]) & 0x8000
		binary.LittleEndian.PutUint16(data[i*2:], value)
	}
	return nil
}
