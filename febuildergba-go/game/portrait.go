package game

import (
	"bytes"
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type portraitMode uint8

const (
	portraitStandard portraitMode = iota
	portraitExceed
	portraitHalfBody
)

func fe8uPortraitMode(data []byte) portraitMode {
	if within(data, 0x55D2, 6) && bytes.Equal(data[0x55D2:0x55D8], []byte{0xC0, 0x46, 0x01, 0xB0, 0x03, 0x4B}) {
		return portraitExceed
	}
	if within(data, 0x8540, 2) && (u16(data, 0x8540) == 0x1C0A || u16(data, 0x8540) == 0x3A01) {
		return portraitHalfBody
	}
	return portraitStandard
}

type portraitFormat struct {
	pointer, stride          int
	cardMarker, cardPointer  int
	stateOffset, mouthOffset int
	mode                     func([]byte) portraitMode
}

func (f portraitFormat) read(data []byte, id int) (Preview, error) {
	if id == 0 {
		return Preview{}, nil
	}
	if id < 0 || id > 0xFFFF {
		return Preview{}, ErrImage
	}
	base, ok := pointer(data, f.pointer)
	if !ok || !within(data, base+id*f.stride, f.stride) {
		return Preview{}, ErrImage
	}
	at := base + id*f.stride
	result := Preview{Regions: []AssetRegion{{Label: "portrait_record", Address: at, Length: f.stride}}}
	if u32(data, at) == 0 && u32(data, at+f.cardPointer) == 0 {
		return result, nil
	}
	palette, ok := pointer(data, at+8)
	if !ok || !within(data, palette, 32) {
		return result, ErrImage
	}
	result.Regions = append(result.Regions, AssetRegion{Label: "image_palette", Address: palette, Length: 32})
	if u32(data, at+f.cardMarker) == 0 {
		return readClassCard(data, at+f.cardPointer, palette, result)
	}
	graphic, ok := pointer(data, at)
	if !ok || !within(data, graphic, 4) {
		return result, ErrImage
	}
	mode := portraitStandard
	if f.mode != nil {
		mode = f.mode(data)
	}
	halfBody := false
	var tiles []byte
	if data[graphic] == 0x10 {
		var err error
		tiles, err = gba.DecompressLZ77(data[graphic:], 8192)
		// Static portraits use the first four tile rows; FE6 stores mouth tiles below them.
		if err != nil || len(tiles) < 4096 {
			return result, ErrImage
		}
	} else {
		header := u32(data, graphic)
		length := 4096
		switch header {
		case 0x00100400:
		case 0x00200400:
			length = 8192
			halfBody = mode == portraitHalfBody
		default:
			return result, ErrImage
		}
		if !within(data, graphic+4, length) {
			return result, ErrImage
		}
		tiles = data[graphic+4 : graphic+4+length]
	}
	height := 32
	if halfBody {
		height = 64
	}
	sheet, err := gba.Tiles4BPP(tiles, data[palette:palette+32], 256, height)
	if err != nil {
		return result, ErrImage
	}
	face := image.NewRGBA(image.Rect(0, 0, 96, 80))
	// ImagePortraitForm's standard portrait sheet placement, in pixels.
	for _, piece := range portraitFacePieces {
		blitPortrait(face, sheet, piece, draw.Src)
	}
	if mode == portraitExceed && data[at+26] == 1 {
		for i := range 2 {
			x, y := int(data[at+16+i*2])*8, int(data[at+17+i*2])*8
			blitPortrait(face, sheet, [6]int{x, y, 16, 16, 224 + i*16, 16}, draw.Over)
		}
	}
	if f.stateOffset >= 0 && data[at+f.stateOffset] == 6 {
		blitPortrait(face, sheet, [6]int{int(data[at+f.mouthOffset]) * 8, int(data[at+f.mouthOffset+1]) * 8, 32, 16, 192, 16}, draw.Over)
	}
	if halfBody {
		body := image.NewRGBA(image.Rect(0, 0, 128, 128))
		draw.Draw(body, image.Rect(16, 0, 112, 80), face, image.Point{}, draw.Src)
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

func readClassCard(data []byte, at, palette int, result Preview) (Preview, error) {
	card, ok := pointer(data, at)
	if !ok {
		return result, ErrImage
	}
	tiles, err := gba.DecompressLZ77(data[card:], 80*128/2)
	if err != nil || len(tiles)%320 != 0 {
		return result, ErrImage
	}
	result.Image, err = gba.Tiles4BPP(tiles, data[palette:palette+32], 80, len(tiles)/40)
	if err != nil {
		return result, ErrImage
	}
	return result, nil
}

func blitPortrait(dst, src *image.RGBA, piece [6]int, op draw.Op) {
	draw.Draw(dst, image.Rect(piece[0], piece[1], piece[0]+piece[2], piece[1]+piece[3]), src, image.Pt(piece[4], piece[5]), op)
}
