package portrait

import (
	"bytes"
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type Mode uint8

const (
	Standard Mode = iota
	Exceed
	HalfBody
)

func FE8UMode(data []byte) Mode {
	if gba.Within(data, 0x55D2, 6) &&
		bytes.Equal(
			data[0x55D2:0x55D8],
			[]byte{0xC0, 0x46, 0x01, 0xB0, 0x03, 0x4B},
		) {
		return Exceed
	}
	if gba.Within(data, 0x8540, 2) &&
		(gba.Uint16(data, 0x8540) == 0x1C0A || gba.Uint16(data, 0x8540) == 0x3A01) {
		return HalfBody
	}
	return Standard
}

type Format struct {
	Pointer, Stride          int
	CardMarker, CardPointer  int
	StateOffset, MouthOffset int
	Mode                     func([]byte) Mode
}

func (f Format) Preview(data []byte, id int) (graphics.Preview, error) {
	if id == 0 {
		return graphics.Preview{}, nil
	}
	if id < 0 || id > 0xFFFF {
		return graphics.Preview{}, graphics.ErrImage
	}
	base, ok := gba.Pointer(data, f.Pointer)
	if !ok || !gba.Within(data, base+id*f.Stride, f.Stride) {
		return graphics.Preview{}, graphics.ErrImage
	}
	at := base + id*f.Stride
	result := graphics.Preview{
		Regions: []graphics.Region{
			{Label: "portrait_record", Address: at, Length: f.Stride},
		},
	}
	if gba.Uint32(data, at) == 0 && gba.Uint32(data, at+f.CardPointer) == 0 {
		return result, nil
	}
	palette, ok := gba.Pointer(data, at+8)
	if !ok || !gba.Within(data, palette, 32) {
		return result, graphics.ErrImage
	}
	result.Regions = append(
		result.Regions,
		graphics.Region{Label: "image_palette", Address: palette, Length: 32},
	)
	if gba.Uint32(data, at+f.CardMarker) == 0 {
		return readClassCard(data, at+f.CardPointer, palette, result)
	}
	graphic, ok := gba.Pointer(data, at)
	if !ok || !gba.Within(data, graphic, 4) {
		return result, graphics.ErrImage
	}
	mode := Standard
	if f.Mode != nil {
		mode = f.Mode(data)
	}
	halfBody := false
	var tiles []byte
	if data[graphic] == 0x10 {
		var err error
		tiles, err = gba.DecompressLZ77(data[graphic:], 8192)
		// Static portraits use the first four tile rows; FE6 stores mouth tiles
		// below them.
		if err != nil || len(tiles) < 4096 {
			return result, fault.New(
				graphics.ErrImage,
				"game.decompress_portrait",
				fault.Context{
					Range: &fault.Range{
						Offset: graphic,
						Length: 4096,
						Size:   len(data),
					},
				},
				err,
			)
		}
	} else {
		header := gba.Uint32(data, graphic)
		length := 4096
		switch header {
		case 0x00100400:
		case 0x00200400:
			length = 8192
			halfBody = mode == HalfBody
		default:
			return result, graphics.ErrImage
		}
		if !gba.Within(data, graphic+4, length) {
			return result, graphics.ErrImage
		}
		tiles = data[graphic+4 : graphic+4+length]
	}
	height := 32
	if halfBody {
		height = 64
	}
	sheet, err := gba.Tiles4BPP(tiles, data[palette:palette+32], 256, height)
	if err != nil {
		return result, fault.New(
			graphics.ErrImage,
			"game.render_portrait",
			fault.Context{},
			err,
		)
	}
	face := image.NewRGBA(image.Rect(0, 0, 96, 80))
	// ImagePortraitForm's standard portrait sheet placement, in pixels.
	for _, piece := range portraitFacePieces {
		blitPortrait(face, sheet, piece, draw.Src)
	}
	if mode == Exceed && data[at+26] == 1 {
		for i := range 2 {
			x, y := int(data[at+16+i*2])*8, int(data[at+17+i*2])*8
			blitPortrait(
				face,
				sheet,
				[6]int{x, y, 16, 16, 224 + i*16, 16},
				draw.Over,
			)
		}
	}
	if f.StateOffset >= 0 && data[at+f.StateOffset] == 6 {
		blitPortrait(
			face,
			sheet,
			[6]int{
				int(data[at+f.MouthOffset]) * 8,
				int(data[at+f.MouthOffset+1]) * 8,
				32,
				16,
				192,
				16,
			},
			draw.Over,
		)
	}
	if halfBody {
		body := image.NewRGBA(image.Rect(0, 0, 128, 128))
		draw.Draw(
			body,
			image.Rect(16, 0, 112, 80),
			face,
			image.Point{},
			draw.Src,
		)
		for _, piece := range [][6]int{
			{0, 80, 128, 32, 0, 32},
			{16, 112, 32, 16, 224, 16},
			{0, 112, 16, 16, 128, 32},
			{112, 112, 16, 16, 128, 48},
			{16, 16, 16, 32, 144, 32},
			{96, 16, 16, 32, 160, 32},
			{112, 48, 16, 32, 176, 32},
			{0, 48, 16, 32, 192, 32},
			{48, 112, 32, 16, 224, 32},
			{80, 112, 32, 16, 224, 48},
			{96, 0, 8, 16, 208, 48},
			{24, 0, 8, 16, 216, 32},
		} {
			blitPortrait(body, sheet, piece, draw.Src)
		}
		face = body
	}
	result.Image = face
	return result, nil
}

func readClassCard(
	data []byte,
	at, palette int,
	result graphics.Preview,
) (graphics.Preview, error) {
	card, ok := gba.Pointer(data, at)
	if !ok {
		return result, graphics.ErrImage
	}
	tiles, err := gba.DecompressLZ77(data[card:], 80*128/2)
	if err != nil || len(tiles)%320 != 0 {
		return result, fault.New(
			graphics.ErrImage,
			"game.decompress_class_card",
			fault.Context{
				Range: &fault.Range{
					Offset: card,
					Length: 80 * 128 / 2,
					Size:   len(data),
				},
			},
			err,
		)
	}
	result.Image, err = gba.Tiles4BPP(
		tiles,
		data[palette:palette+32],
		80,
		len(tiles)/40,
	)
	if err != nil {
		return result, fault.New(
			graphics.ErrImage,
			"game.render_class_card",
			fault.Context{},
			err,
		)
	}
	return result, nil
}

func blitPortrait(dst, src *image.RGBA, piece [6]int, op draw.Op) {
	draw.Draw(
		dst,
		image.Rect(piece[0], piece[1], piece[0]+piece[2], piece[1]+piece[3]),
		src,
		image.Pt(piece[4], piece[5]),
		op,
	)
}
