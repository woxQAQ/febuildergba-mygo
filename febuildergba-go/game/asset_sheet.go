package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
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

type assetResource struct {
	pointer int
	data    []byte
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

func readAssetPart(data []byte, address, width, length int, compressed bool, pieces [][6]int) (assetPart, error) {
	part := assetPart{pointer: address, width: width, compressed: compressed, pieces: pieces}
	if !within(data, address, 4) {
		return part, ErrImage
	}
	if u32(data, address) == 0 {
		part.data = make([]byte, length)
		return part, nil
	}
	at, ok := pointer(data, address)
	if !ok || at < 0xC0 {
		return part, ErrImage
	}
	switch {
	case compressed:
		var err error
		part.data, err = gba.DecompressLZ77(data[at:], 8192)
		if err != nil || len(part.data) < length {
			return part, ErrImage
		}
	case within(data, at, length):
		part.data = bytes.Clone(data[at : at+length])
	default:
		return part, ErrImage
	}
	return part, nil
}

func (f portraitFormat) sheet(data []byte, id int) (assetSheet, error) {
	var sheet assetSheet
	base, ok := pointer(data, f.pointer)
	at := base + id*f.stride
	if !ok || base < 0xC0 || base%4 != 0 || id < 1 || id > 0xFFFF || !within(data, at, f.stride) {
		return sheet, ErrImage
	}
	palette, ok := pointer(data, at+8)
	if !ok || !within(data, palette, 32) {
		return sheet, ErrImage
	}
	sheet.palette, sheet.palettePointer = bytes.Clone(data[palette:palette+32]), at+8
	if u32(data, at+f.cardMarker) == 0 {
		part, err := readAssetPart(data, at+f.cardPointer, 80, 1, true, nil)
		if err != nil || u32(data, at+f.cardPointer) == 0 || len(part.data)%320 != 0 || len(part.data) > 80*128/2 {
			return sheet, ErrImage
		}
		height := len(part.data) / 40
		part.pieces = [][6]int{{0, 0, 80, height, 0, 0}}
		sheet.image = image.NewRGBA(image.Rect(0, 0, 80, height))
		sheet.parts = []assetPart{part}
		return sheet, sheet.render()
	}
	if f.mode != nil && f.mode(data) != portraitStandard {
		return sheet, ErrAssetFormat
	}
	main, err := f.facePart(data, at)
	if err != nil {
		return sheet, err
	}
	mini, err := readAssetPart(data, at+4, 32, 512, f.stride != 16, [][6]int{{96, 16, 32, 32, 0, 0}})
	if err != nil {
		return sheet, err
	}
	sheet.image = image.NewRGBA(image.Rect(0, 0, 128, 112))
	sheet.parts = []assetPart{main, mini}
	if f.stride != 16 {
		mouthPieces := make([][6]int, 6)
		for i := range mouthPieces {
			mouthPieces[i] = [6]int{(i % 3) * 32, 80 + (i/3)*16, 32, 16, 0, i * 16}
		}
		mouth, readErr := readAssetPart(data, at+12, 32, 1536, false, mouthPieces)
		if readErr != nil {
			return sheet, readErr
		}
		sheet.parts = append(sheet.parts, mouth)
	}
	return sheet, sheet.render()
}

func (f portraitFormat) facePart(data []byte, at int) (assetPart, error) {
	graphic, ok := pointer(data, at)
	if !ok || !within(data, graphic, 4) {
		return assetPart{}, ErrImage
	}
	pieces := append([][6]int(nil), portraitFacePieces...)
	length := 4096
	if f.stride == 16 {
		length = 4352
		pieces = append(pieces, [6]int{0, 80, 64, 32, 192, 0}, [6]int{96, 80, 32, 8, 0, 32}, [6]int{96, 88, 32, 8, 32, 32})
	} else {
		pieces = append(pieces, [6]int{96, 48, 32, 16, 192, 0}, [6]int{96, 64, 32, 16, 192, 16}, [6]int{96, 80, 32, 16, 224, 0}, [6]int{96, 96, 32, 16, 224, 16})
	}
	if data[graphic] == 0x10 {
		return readAssetPart(data, at, 256, length, true, pieces)
	}
	if f.stride == 16 || u32(data, graphic) != 0x00100400 || !within(data, graphic+4, length) {
		return assetPart{}, ErrAssetFormat
	}
	return assetPart{pointer: at, width: 256, data: bytes.Clone(data[graphic+4 : graphic+4+length]), header: bytes.Clone(data[graphic : graphic+4]), pieces: pieces}, nil
}

func (s *assetSheet) render() error {
	for _, part := range s.parts {
		height := ((len(part.data) + part.width*4 - 1) / (part.width * 4)) * 8
		padded := make([]byte, part.width*height/2)
		copy(padded, part.data)
		tiles, err := gba.Tiles4BPP(padded, s.palette, part.width, height)
		if err != nil {
			return ErrImage
		}
		for _, piece := range part.pieces {
			blitPortrait(s.image, tiles, piece, draw.Src)
		}
	}
	return nil
}

func (s assetSheet) encode(src image.Image) ([]assetResource, error) {
	palette := s.palette
	if _, err := gba.EncodeTiles4BPP(src, palette); err != nil {
		var paletteErr error
		palette, paletteErr = gba.Palette4BPP(src, palette)
		if paletteErr != nil {
			return nil, ErrAssetColors
		}
		src = gba.Quantise4BPP(src)
	}
	// Pixels outside the documented sheet rectangles have no storage location.
	covered := image.NewAlpha(s.image.Bounds())
	for _, part := range s.parts {
		for _, p := range part.pieces {
			draw.Draw(covered, image.Rect(p[0], p[1], p[0]+p[2], p[1]+p[3]), image.Opaque, image.Point{}, draw.Src)
		}
	}
	for y := range covered.Bounds().Dy() {
		for x := range covered.Bounds().Dx() {
			_, _, _, alpha := src.At(x, y).RGBA()
			if covered.AlphaAt(x, y).A == 0 && alpha != 0 {
				return nil, ErrAssetPixels
			}
		}
	}
	var resources []assetResource
	for _, part := range s.parts {
		body := bytes.Clone(part.data)
		for _, p := range part.pieces {
			region := image.NewRGBA(image.Rect(0, 0, p[2], p[3]))
			draw.Draw(region, region.Bounds(), src, image.Pt(p[0], p[1]), draw.Src)
			tiles, err := gba.EncodeTiles4BPP(region, palette)
			if err != nil {
				return nil, ErrAssetColors
			}
			for row := range p[3] / 8 {
				offset := ((p[5]/8+row)*(part.width/8) + p[4]/8) * 32
				length := p[2] / 8 * 32
				if offset < 0 || offset+length > len(body) {
					return nil, ErrImage
				}
				copy(body[offset:offset+length], tiles[row*length:(row+1)*length])
			}
		}
		content := append(bytes.Clone(part.header), body...)
		if part.compressed {
			var err error
			content, err = gba.CompressLZ77(body)
			if err != nil {
				return nil, ErrImage
			}
		}
		// Allocation is completed by the adapter after all pixels are validated.
		resources = append(resources, assetResource{pointer: part.pointer, data: content})
	}
	resources = append(resources, assetResource{pointer: s.palettePointer, data: bytes.Clone(palette)})
	return resources, nil
}

func appendAssetResources(size int, resources []assetResource) (space.Plan, error) {
	plan := space.Plan{Edit: rom.Edit{Size: size}}
	for _, resource := range resources {
		allocation, err := space.PlanAppend(plan.Edit.Size, resource.data)
		if err != nil {
			return space.Plan{}, err
		}
		address := uint32(0x08000000 + allocation.Destination.Start) // #nosec G115 -- PlanAppend enforces the ROM size limit.
		plan.Edit.Size = allocation.Edit.Size
		plan.Edit.Changes = append(plan.Edit.Changes, allocation.Edit.Changes...)
		plan.Edit.Changes = append(plan.Edit.Changes, rom.Change{Offset: resource.pointer, Data: binary.LittleEndian.AppendUint32(nil, address)})
		if plan.Destination.Start == 0 {
			plan.Destination.Start = allocation.Destination.Start
		}
		plan.Destination.End = allocation.Destination.End
	}
	return plan, nil
}
