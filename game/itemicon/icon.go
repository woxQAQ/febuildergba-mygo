// Package itemicon encodes native icons while preserving their shared palette.
package itemicon

import (
	"bytes"
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type Format struct {
	Pointer, Palette  int
	Original, Maximum int
}

func (f Format) Preview(data []byte, id int) (graphics.Preview, error) {
	base, ok := gba.Pointer(data, f.Pointer)
	if !ok || id < 0 || id >= 0xFF || (base == f.Original && id > f.Maximum) {
		return graphics.Preview{}, graphics.ErrImage
	}
	palette, ok := gba.Pointer(data, f.Palette)
	if !ok || !gba.Within(data, palette, 32) ||
		!gba.Within(data, base+id*128, 128) {
		return graphics.Preview{}, graphics.ErrImage
	}
	at := base + id*128
	bitmap, err := gba.Tiles4BPP(
		data[at:at+128],
		data[palette:palette+32],
		16,
		16,
	)
	if err != nil {
		return graphics.Preview{}, fault.New(
			graphics.ErrImage,
			"game.render_icon",
			fault.Context{
				Range: &fault.Range{Offset: at, Length: 128, Size: len(data)},
			},
			err,
		)
	}
	return graphics.Preview{Image: bitmap, Regions: []graphics.Region{
		{Label: "image_tiles", Address: at, Length: 128},
		{Label: "image_palette", Address: palette, Length: 32},
	}}, nil
}

func (f Format) encodeParts(
	data []byte,
	id int,
	bitmap image.Image,
) ([]graphics.Replacement, error) {
	preview, err := f.Preview(data, id)
	if err != nil {
		return nil, err
	}
	palette := preview.Regions[1].Address
	tiles, err := gba.EncodeTiles4BPP(bitmap, data[palette:palette+32])
	if err != nil {
		return nil, fault.New(
			graphics.ErrColors,
			"game.encode_icon",
			fault.Context{
				Resource:   string(graphics.ItemIcon),
				ResourceID: new(id),
			},
			err,
		)
	}
	base, ok := gba.Pointer(data, f.Pointer)
	// The byte-sized ID reserves FF. Preserve the full addressable window so
	// a relocated table retains all IDs, including those used by patched games.
	const capacity = 0xFF * 128
	if !ok || base < 0xC0 || base%4 != 0 || !gba.Within(data, base, capacity) {
		return nil, graphics.ErrImage
	}
	body := bytes.Clone(data[base : base+capacity])
	copy(body[id*128:], tiles)
	return []graphics.Replacement{{Pointer: f.Pointer, Data: body}}, nil
}

func (f Format) Image(data []byte, id int) (*image.RGBA, error) {
	preview, err := f.Preview(data, id)
	return preview.Image, err
}

func (f Format) Encode(
	data []byte,
	id int,
	bitmap image.Image,
) (graphics.Encoding, error) {
	parts, err := f.encodeParts(data, id, bitmap)
	return graphics.Encoding{Parts: parts, Expected: bitmap}, err
}
