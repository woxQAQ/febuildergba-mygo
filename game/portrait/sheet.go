package portrait

import (
	"bytes"
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Each piece maps a rectangle in the exported sheet to its original tile sheet.
// Coordinates are {sheet x, sheet y, width, height, resource x, resource y}.
var portraitFacePieces = [][6]int{
	{16, 0, 64, 32, 0, 0},
	{16, 32, 64, 32, 64, 0},
	{16, 64, 32, 16, 128, 0},
	{0, 48, 16, 32, 160, 0},
	{48, 64, 32, 16, 128, 16},
	{80, 48, 16, 32, 176, 0},
}

type assetPart struct {
	pointer, width int
	data, header   []byte
	compressed     bool
	pieces         [][6]int
}

type assetSheet struct {
	image          *image.RGBA
	palette        []byte
	palettePointer int
	parts          []assetPart
}

func readAssetPart(
	data []byte,
	address, width, length int,
	compressed bool,
	pieces [][6]int,
) (assetPart, error) {
	part := assetPart{
		pointer:    address,
		width:      width,
		compressed: compressed,
		pieces:     pieces,
	}
	if !gba.Within(data, address, 4) {
		return part, graphics.ErrImage
	}
	if gba.Uint32(data, address) == 0 {
		part.data = make([]byte, length)
		return part, nil
	}
	at, ok := gba.Pointer(data, address)
	if !ok || at < 0xC0 {
		return part, graphics.ErrImage
	}
	switch {
	case compressed:
		var err error
		part.data, err = gba.DecompressLZ77(data[at:], 8192)
		if err != nil || len(part.data) < length {
			return part, fault.New(
				graphics.ErrImage,
				"game.decompress_asset",
				fault.Context{
					Range: &fault.Range{
						Offset: at,
						Length: length,
						Size:   len(data),
					},
				},
				err,
			)
		}
	case gba.Within(data, at, length):
		part.data = bytes.Clone(data[at : at+length])
	default:
		return part, graphics.ErrImage
	}
	return part, nil
}

func (f Format) sheet(data []byte, id int) (assetSheet, error) {
	var sheet assetSheet
	base, ok := gba.Pointer(data, f.Pointer)
	at := base + id*f.Stride
	if !ok || base < 0xC0 || base%4 != 0 || id < 1 || id > 0xFFFF ||
		!gba.Within(data, at, f.Stride) {
		return sheet, graphics.ErrImage
	}
	palette, ok := gba.Pointer(data, at+8)
	if !ok || !gba.Within(data, palette, 32) {
		return sheet, graphics.ErrImage
	}
	sheet.palette, sheet.palettePointer = bytes.Clone(
		data[palette:palette+32],
	), at+8
	if gba.Uint32(data, at+f.CardMarker) == 0 {
		part, err := readAssetPart(data, at+f.CardPointer, 80, 1, true, nil)
		if err != nil || gba.Uint32(data, at+f.CardPointer) == 0 ||
			len(part.data)%320 != 0 ||
			len(part.data) > 80*128/2 {
			return sheet, fault.New(
				graphics.ErrImage,
				"game.read_class_card",
				fault.Context{ResourceID: new(id)},
				err,
			)
		}
		height := len(part.data) / 40
		part.pieces = [][6]int{{0, 0, 80, height, 0, 0}}
		sheet.image = image.NewRGBA(image.Rect(0, 0, 80, height))
		sheet.parts = []assetPart{part}
		return sheet, sheet.render()
	}
	if f.Mode != nil && f.Mode(data) != Standard {
		return sheet, graphics.ErrFormat
	}
	main, err := f.facePart(data, at)
	if err != nil {
		return sheet, err
	}
	mini, err := readAssetPart(
		data,
		at+4,
		32,
		512,
		f.Stride != 16,
		[][6]int{{96, 16, 32, 32, 0, 0}},
	)
	if err != nil {
		return sheet, err
	}
	sheet.image = image.NewRGBA(image.Rect(0, 0, 128, 112))
	sheet.parts = []assetPart{main, mini}
	if f.Stride != 16 {
		mouthPieces := make([][6]int, 6)
		for i := range mouthPieces {
			mouthPieces[i] = [6]int{
				(i % 3) * 32,
				80 + (i/3)*16,
				32,
				16,
				0,
				i * 16,
			}
		}
		mouth, readErr := readAssetPart(
			data,
			at+12,
			32,
			1536,
			false,
			mouthPieces,
		)
		if readErr != nil {
			return sheet, readErr
		}
		sheet.parts = append(sheet.parts, mouth)
	}
	return sheet, sheet.render()
}

func (f Format) facePart(data []byte, at int) (assetPart, error) {
	graphic, ok := gba.Pointer(data, at)
	if !ok || !gba.Within(data, graphic, 4) {
		return assetPart{}, graphics.ErrImage
	}
	pieces := append([][6]int(nil), portraitFacePieces...)
	length := 4096
	if f.Stride == 16 {
		length = 4352
		pieces = append(
			pieces,
			[6]int{0, 80, 64, 32, 192, 0},
			[6]int{96, 80, 32, 8, 0, 32},
			[6]int{96, 88, 32, 8, 32, 32},
		)
	} else {
		pieces = append(
			pieces,
			[6]int{96, 48, 32, 16, 192, 0},
			[6]int{96, 64, 32, 16, 192, 16},
			[6]int{96, 80, 32, 16, 224, 0},
			[6]int{96, 96, 32, 16, 224, 16},
		)
	}
	if data[graphic] == 0x10 {
		return readAssetPart(data, at, 256, length, true, pieces)
	}
	if f.Stride == 16 || gba.Uint32(data, graphic) != 0x00100400 ||
		!gba.Within(data, graphic+4, length) {
		return assetPart{}, graphics.ErrFormat
	}
	return assetPart{
		pointer: at,
		width:   256,
		data:    bytes.Clone(data[graphic+4 : graphic+4+length]),
		header:  bytes.Clone(data[graphic : graphic+4]),
		pieces:  pieces,
	}, nil
}

func (s *assetSheet) render() error {
	for _, part := range s.parts {
		height := ((len(part.data) + part.width*4 - 1) / (part.width * 4)) * 8
		padded := make([]byte, part.width*height/2)
		copy(padded, part.data)
		tiles, err := gba.Tiles4BPP(padded, s.palette, part.width, height)
		if err != nil {
			return fault.New(
				graphics.ErrImage,
				"game.render_asset",
				fault.Context{},
				err,
			)
		}
		for _, piece := range part.pieces {
			blitPortrait(s.image, tiles, piece, draw.Src)
		}
	}
	return nil
}

func (s assetSheet) encode(src image.Image) ([]graphics.Replacement, error) {
	palette := s.palette
	if _, err := gba.EncodeTiles4BPP(src, palette); err != nil {
		var paletteErr error
		palette, paletteErr = gba.Palette4BPP(src, palette)
		if paletteErr != nil {
			return nil, fault.New(
				graphics.ErrColors,
				"game.encode_palette",
				fault.Context{},
				paletteErr,
			)
		}
		src = gba.Quantise4BPP(src)
	}
	// Pixels outside the documented sheet rectangles have no storage location.
	covered := image.NewAlpha(s.image.Bounds())
	for _, part := range s.parts {
		for _, p := range part.pieces {
			draw.Draw(
				covered,
				image.Rect(p[0], p[1], p[0]+p[2], p[1]+p[3]),
				image.Opaque,
				image.Point{},
				draw.Src,
			)
		}
	}
	for y := range covered.Bounds().Dy() {
		for x := range covered.Bounds().Dx() {
			_, _, _, alpha := src.At(x, y).RGBA()
			if covered.AlphaAt(x, y).A == 0 && alpha != 0 {
				return nil, graphics.ErrPixels
			}
		}
	}
	var resources []graphics.Replacement
	for _, part := range s.parts {
		body := bytes.Clone(part.data)
		for _, p := range part.pieces {
			region := image.NewRGBA(image.Rect(0, 0, p[2], p[3]))
			draw.Draw(
				region,
				region.Bounds(),
				src,
				image.Pt(p[0], p[1]),
				draw.Src,
			)
			tiles, err := gba.EncodeTiles4BPP(region, palette)
			if err != nil {
				return nil, fault.New(
					graphics.ErrColors,
					"game.encode_asset",
					fault.Context{},
					err,
				)
			}
			for row := range p[3] / 8 {
				offset := ((p[5]/8+row)*(part.width/8) + p[4]/8) * 32
				length := p[2] / 8 * 32
				if offset < 0 || offset+length > len(body) {
					return nil, graphics.ErrImage
				}
				copy(
					body[offset:offset+length],
					tiles[row*length:(row+1)*length],
				)
			}
		}
		content := append(bytes.Clone(part.header), body...)
		if part.compressed {
			var err error
			content, err = gba.CompressLZ77(body)
			if err != nil {
				return nil, fault.New(
					graphics.ErrImage,
					"game.compress_asset",
					fault.Context{},
					err,
				)
			}
		}
		// Allocation is completed by the adapter after all pixels are
		// validated.
		resources = append(
			resources,
			graphics.Replacement{Pointer: part.pointer, Data: content},
		)
	}
	resources = append(
		resources,
		graphics.Replacement{
			Pointer: s.palettePointer,
			Data:    bytes.Clone(palette),
		},
	)
	return resources, nil
}
