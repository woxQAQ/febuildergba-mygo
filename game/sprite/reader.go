// Package sprite reads native standing map sprites and weapon-type icons.
package sprite

import (
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Format holds the confirmed native sprite and weapon-icon locations.
// WeaponX/WeaponY are the icon's cell position within the weapon sheet.
type Format struct {
	Standing               []int
	Palette                int
	Weapons, WeaponPalette int
	WeaponX, WeaponY       int
}

type Reader struct {
	context *edit.Context
	format  Format
	image   rom.Snapshot
	binding edit.Stamp
}

func New(context *edit.Context, format Format) *Reader {
	return &Reader{context: context, format: format}
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

func (f Format) standingImage(data []byte, id int) (*image.RGBA, error) {
	if id < 0 || id > 255 || f.Palette < 0xC0 ||
		!gba.Within(data, f.Palette, 32) {
		return nil, graphics.ErrImage
	}
	// Native bit 7 disables animation; it is not a second bank of sprite IDs.
	id &= 0x7F
	base := 0
	for _, site := range f.Standing {
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
	return gba.Tiles4BPP(tiles, data[f.Palette:f.Palette+32], width, height)
}

func (f Format) weaponImage(data []byte, id int) (*image.RGBA, error) {
	if id < 0 || id >= 8 || f.Weapons < 0xC0 {
		return nil, graphics.ErrImage
	}
	address, ok := gba.Pointer(data, f.Weapons)
	palette, palOK := gba.Pointer(data, f.WeaponPalette)
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
		16+f.WeaponY,
	)
	if err != nil {
		return nil, err
	}
	result := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(
		result,
		result.Bounds(),
		sheet,
		image.Pt((id+f.WeaponX)*16, f.WeaponY),
		draw.Src,
	)
	return result, nil
}
