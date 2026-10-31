// Package sprite reads native standing map sprites and weapon-type icons.
package sprite

import (
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type format struct {
	standing               []int
	palette                int
	weapons, weaponPalette int
	weaponX, weaponY       int
}

func forVersion(id string) format {
	switch id {
	case version.FE6:
		return format{[]int{0x21C74}, 0x100968, 0x750DC, 0x750E4, 6, 8}
	case version.FE7U:
		return format{[]int{0x24DA0}, 0x194594, 0x965F4, 0x90530, 3, 0}
	case version.FE7J:
		return format{[]int{0x2522C}, 0x1900E8, 0x96D78, 0x90E70, 3, 0}
	case version.FE8U:
		return format{
			[]int{0x26730, 0x267B0, 0x26838, 0x26C88, 0x28064, 0x27F74},
			0x59EE20,
			0x9DC5C,
			0x91178,
			3,
			0,
		}
	case version.FE8J:
		return format{
			[]int{
				0x266D4,
				0x26754,
				0x267DC,
				0x26C2C,
				0x27274,
				0x27B44,
				0x27C30,
				0x27FF8,
			},
			0x5C7340,
			0x9FEFC,
			0x93450,
			3,
			0,
		}
	default:
		return format{}
	}
}

type Reader struct {
	context *edit.Context
	format  format
	image   rom.Snapshot
	binding edit.Stamp
}

func New(context *edit.Context, version string) *Reader {
	return &Reader{context: context, format: forVersion(version)}
}

func (s *Reader) refresh() error {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
	}
	return s.context.Validate(s.image.Data)
}

// Preview binds even an unreadable resource to its document interpretation.
// Callers can cache the failure until the source changes.
type Preview struct {
	Image   *image.RGBA
	Err     error
	binding edit.Stamp
}

func (p Preview) Current(s *Reader) bool { return p.binding.Current(s.context) }

func (s *Reader) Standing(id int) Preview {
	err := s.refresh()
	result := Preview{binding: s.binding, Err: err}
	if err == nil {
		result.Image, result.Err = s.format.standingImage(s.image.Data, id)
	}
	return result
}

func (s *Reader) Weapon(id int) Preview {
	err := s.refresh()
	result := Preview{binding: s.binding, Err: err}
	if err == nil {
		result.Image, result.Err = s.format.weaponImage(s.image.Data, id)
	}
	return result
}

func (f format) standingImage(data []byte, id int) (*image.RGBA, error) {
	if id < 0 || id > 255 || f.palette < 0xC0 ||
		!gba.Within(data, f.palette, 32) {
		return nil, graphics.ErrImage
	}
	// Native bit 7 disables animation; it is not a second bank of sprite IDs.
	id &= 0x7F
	base := 0
	for _, site := range f.standing {
		at, ok := gba.Pointer(data, site)
		if ok && at >= 0xC0 && gba.Within(data, at+id*8, 8) {
			base = at
			break
		}
	}
	if base == 0 {
		return nil, graphics.ErrImage
	}
	row := base + id*8
	width, height := 16, 16
	switch gba.Uint16(data, row+2) {
	case 0:
	case 1:
		height = 32
	case 2:
		width, height = 32, 32
	default:
		return nil, graphics.ErrImage
	}
	address, ok := gba.Pointer(data, row+4)
	if !ok || address < 0xC0 || !gba.Within(data, address, 4) {
		return nil, graphics.ErrImage
	}
	tiles, err := gba.DecompressLZ77(data[address:], 32*32*3/2)
	if err != nil {
		return nil, err
	}
	// The first standing frame is row-major, including the full 16x32 canvas.
	return gba.Tiles4BPP(tiles, data[f.palette:f.palette+32], width, height)
}

func (f format) weaponImage(data []byte, id int) (*image.RGBA, error) {
	if id < 0 || id >= 8 || f.weapons < 0xC0 {
		return nil, graphics.ErrImage
	}
	address, ok := gba.Pointer(data, f.weapons)
	palette, palOK := gba.Pointer(data, f.weaponPalette)
	if !ok || !palOK || address < 0xC0 || palette < 0xC0 ||
		!gba.Within(data, address, 4) ||
		!gba.Within(data, palette, 32) {
		return nil, graphics.ErrImage
	}
	tiles, err := gba.DecompressLZ77(data[address:], 4096)
	if err != nil {
		return nil, err
	}
	sheet, err := gba.Tiles4BPP(
		tiles,
		data[palette:palette+32],
		256,
		16+f.weaponY,
	)
	if err != nil {
		return nil, err
	}
	result := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(
		result,
		result.Bounds(),
		sheet,
		image.Pt((id+f.weaponX)*16, f.weaponY),
		draw.Src,
	)
	return result, nil
}
