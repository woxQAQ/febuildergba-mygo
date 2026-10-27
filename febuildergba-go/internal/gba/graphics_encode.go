package gba

import (
	"encoding/binary"
	"image"
	"image/color"
)

// EncodeTiles4BPP requires binary transparency and exact colors from the supplied palette.
func EncodeTiles4BPP(src image.Image, palette []byte) ([]byte, error) {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 8 || height < 8 || width%8 != 0 || height%8 != 0 || width > 2048 || height > 2048 || len(palette) != 32 {
		return nil, ErrGraphics
	}
	colors := make(map[color.NRGBA]byte)
	for i := 15; i > 0; i-- {
		v := binary.LittleEndian.Uint16(palette[i*2:])
		expand := func(v uint16) uint8 { return uint8((v<<3 | v>>2) & 255) }
		colors[color.NRGBA{R: expand(v & 31), G: expand(v >> 5 & 31), B: expand(v >> 10 & 31), A: 255}] = byte(i)
	}
	out := make([]byte, width*height/2)
	for y := range height {
		for x := range width {
			raw := src.At(bounds.Min.X+x, bounds.Min.Y+y)
			_, _, _, alpha := raw.RGBA()
			if alpha != 0 && alpha != 0xFFFF {
				return nil, ErrGraphics
			}
			pixel := color.NRGBAModel.Convert(raw).(color.NRGBA)
			var index byte
			if pixel.A != 0 {
				var ok bool
				index, ok = colors[pixel]
				if !ok {
					return nil, ErrGraphics
				}
			}
			at := ((y/8)*(width/8)+x/8)*32 + (y%8)*4 + (x%8)/2
			out[at] |= index << ((x % 2) * 4)
		}
	}
	return out, nil
}

// Palette4BPP quantizes to the GBA's BGR555 colors; entry zero remains transparent.
// The resulting pixels are rendered for review before an edit can be applied.
func Palette4BPP(src image.Image, background []byte) ([]byte, error) {
	palette := make([]byte, 32)
	copy(palette, background[:min(2, len(background))])
	colors := make(map[uint16]bool)
	count := 1
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			raw := src.At(x, y)
			_, _, _, alpha := raw.RGBA()
			if alpha != 0 && alpha != 0xFFFF {
				return nil, ErrGraphics
			}
			pixel := color.NRGBAModel.Convert(raw).(color.NRGBA)
			if pixel.A == 0 {
				continue
			}
			v := bgr555(pixel)
			if colors[v] {
				continue
			}
			if count == 16 {
				return nil, ErrGraphics
			}
			colors[v] = true
			binary.LittleEndian.PutUint16(palette[count*2:], v)
			count++
		}
	}
	return palette, nil
}

func bgr555(pixel color.NRGBA) uint16 {
	quantise := func(v uint8) uint16 { return (uint16(v)*31 + 127) / 255 }
	return quantise(pixel.R) | quantise(pixel.G)<<5 | quantise(pixel.B)<<10
}

// Quantise4BPP converts opaque pixels to the exact colors represented by BGR555.
func Quantise4BPP(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			pixel := color.NRGBAModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if pixel.A != 0 {
				v := bgr555(pixel)
				expand := func(v uint16) uint8 { return uint8((v<<3 | v>>2) & 255) }
				pixel.R, pixel.G, pixel.B = expand(v&31), expand(v>>5&31), expand(v>>10&31)
			}
			out.SetNRGBA(x, y, pixel)
		}
	}
	return out
}

// CompressLZ77 writes BIOS-compatible 0x10 streams with a 4 KiB search window.
func CompressLZ77(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > 0xFFFFFF {
		return nil, ErrGraphics
	}
	size := uint32(len(data)) // #nosec G115 -- length is bounded by the 24-bit stream header.
	out := binary.LittleEndian.AppendUint32(nil, size<<8|0x10)
	positions := make(map[[3]byte][]int)
	for at := 0; at < len(data); {
		flag := len(out)
		out = append(out, 0)
		for bit := 7; bit >= 0 && at < len(data); bit-- {
			count, distance := lzMatch(data, at, positions)
			consumed := 1
			if count >= 3 {
				out[flag] |= 1 << bit
				packed := uint16((count-3)<<12 | (distance - 1)) // #nosec G115 -- match length is 3..18 and distance 2..4096.
				out = append(out, byte(packed>>8), byte(packed&255))
				consumed = count
			} else {
				out = append(out, data[at])
			}
			for i := at; i < at+consumed && i+2 < len(data); i++ {
				key := [3]byte{data[i], data[i+1], data[i+2]}
				list := positions[key]
				first := 0
				for first < len(list) && i-list[first] > 4096 {
					first++
				}
				positions[key] = append(list[first:], i)
			}
			at += consumed
		}
	}
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out, nil
}

func lzMatch(data []byte, at int, positions map[[3]byte][]int) (int, int) {
	if at+2 >= len(data) {
		return 0, 0
	}
	list := positions[[3]byte{data[at], data[at+1], data[at+2]}]
	best, distance := 0, 0
	for i := len(list) - 1; i >= 0 && at-list[i] <= 4096; i-- {
		start := list[i]
		// VRAM decompression cannot use a one-byte displacement.
		if at-start < 2 {
			continue
		}
		count := 0
		for count < 18 && at+count < len(data) && data[start+count] == data[at+count] {
			count++
		}
		if count > best {
			best, distance = count, at-start
			if best == 18 {
				break
			}
		}
	}
	return best, distance
}
