package game

import (
	"image"
	"image/color"
	"image/draw"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Portrait is a revision-bound view of one shared resource. Expressions are
// composed from decoded parts; changing a preview frame never edits the record.
type Portrait struct {
	Entry                      PortraitEntry
	Mouth, Eyes                image.Point
	Blink                      int
	HasEyes                    bool
	Fields                     []Field
	Resources                  []RecordResource
	Palette                    []color.RGBA
	Main, Mini, Sheet          *image.RGBA
	MainErr, MiniErr, SheetErr error
	document                   *rom.Document
	id, address                int
	writable                   bool
	revision                   uint64
	layouts                    map[Kind]Layout
	eyes                       [2]*image.RGBA
	mouths                     []*image.RGBA
}

func (p *Portrait) Revision() uint64 { return p.revision }
func (p *Portrait) Current(s *Session) bool {
	return p != nil && p.document == s.Document &&
		p.revision == s.Document.State().Revision &&
		maps.Equal(p.layouts, s.layouts)
}
func (p *Portrait) MouthFrames() int { return len(p.mouths) }

// Frame uses eye -1 for the saved blink configuration, 0 for the base image,
// and 1/2 for half/closed eyes. Mouth zero selects the base image.
func (p *Portrait) Frame(eye, mouth int) (*image.RGBA, error) {
	return p.FrameAt(eye, mouth, p.Mouth, p.Eyes)
}

func (p *Portrait) FrameAt(
	eye, mouth int,
	mouthAt, eyesAt image.Point,
) (*image.RGBA, error) {
	if p.Main == nil {
		return nil, p.MainErr
	}
	if eye == -1 {
		eye = 0
		if p.HasEyes && p.Blink == 6 {
			eye = 2
		}
	}
	if eye < 0 || eye > 2 || mouth < 0 || mouth > len(p.mouths) {
		return nil, ErrValue
	}
	result := portraitCrop(p.Main, p.Main.Bounds())
	if eye > 0 {
		if !p.HasEyes || p.eyes[eye-1] == nil {
			return nil, ErrImage
		}
		draw.Draw(
			result,
			image.Rect(eyesAt.X*8, eyesAt.Y*8, eyesAt.X*8+32, eyesAt.Y*8+16),
			p.eyes[eye-1],
			image.Point{},
			draw.Over,
		)
	}
	if mouth > 0 {
		if p.mouths[mouth-1] == nil {
			return nil, ErrImage
		}
		draw.Draw(
			result,
			image.Rect(
				mouthAt.X*8,
				mouthAt.Y*8,
				mouthAt.X*8+32,
				mouthAt.Y*8+16,
			),
			p.mouths[mouth-1],
			image.Point{},
			draw.Over,
		)
	}
	return result, nil
}

func portraitCrop(src image.Image, r image.Rectangle) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), src, r.Min, draw.Src)
	return out
}

func portraitPart(
	data []byte,
	at, width, size int,
	compressed bool,
	palette []byte,
) (*image.RGBA, error) {
	if !within(data, at, 4) || u32(data, at) == 0 {
		return nil, nil
	}
	part, err := readAssetPart(data, at, width, size, compressed, nil)
	if err != nil {
		return nil, err
	}
	return gba.Tiles4BPP(part.data, palette, width, size*2/width)
}

func portraitResource(data []byte, label string, at, size int) RecordResource {
	r := RecordResource{Label: label, Pointer: at, Length: size}
	if !within(data, at, 4) {
		r.Err = ErrImage
		return r
	}
	if u32(data, at) == 0 {
		r.Empty = true
		return r
	}
	target, ok := pointer(data, at)
	if !ok || target < 0xC0 || !within(data, target, size) {
		r.Err = ErrImage
		return r
	}
	r.Address = target
	return r
}

func (s *Session) Portrait(id int) (*Portrait, error) {
	s.Refresh()
	entry, err := s.PortraitEntry(id)
	if err != nil {
		return nil, err
	}
	g, ok := s.Adapter.(gameFormat)
	if !ok || !g.Match(s.image.Data) {
		return nil, ErrVersion
	}
	f, data, at := g.portraits, s.image.Data, entry.Address
	p := &Portrait{
		Entry:    entry,
		document: s.Document,
		id:       id,
		address:  at,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
	}
	miniLength := 4
	if f.stride == 16 {
		miniLength = 512
	}
	p.Resources = []RecordResource{
		portraitResource(data, "portrait_main", at, 4),
		portraitResource(data, "portrait_mini", at+4, miniLength),
		portraitResource(data, "portrait_palette", at+8, 32),
	}
	if entry.ClassCard && f.stride == 16 {
		p.Resources[0].Label = "portrait_card"
	}
	if f.stride != 16 {
		p.Resources = append(
			p.Resources,
			portraitResource(data, "portrait_mouth_tiles", at+12, 1536),
			portraitResource(data, "portrait_card", at+16, 4),
		)
	}
	if entry.Empty {
		return p, nil
	}
	standard := f.mode == nil || f.mode(data) == portraitStandard
	if !entry.ClassCard && standard {
		offset := 20
		if f.stride == 16 {
			offset = 12
		}
		p.Mouth = image.Pt(int(data[at+offset]), int(data[at+offset+1]))
		p.writable = true
		p.Fields = portraitFields(f)
		if f.stride != 16 {
			p.HasEyes = true
			p.Eyes = image.Pt(int(data[at+22]), int(data[at+23]))
			p.Blink = int(data[at+24])
		}
	}
	palette, valid := pointer(data, at+8)
	if !valid || palette < 0xC0 || !within(data, palette, 32) {
		p.MainErr, p.MiniErr, p.SheetErr = ErrImage, ErrImage, ErrImage
		return p, nil
	}
	pal := data[palette : palette+32]
	for index := range 16 {
		value := u16(pal, index*2)
		r, g, b := byte(value&31), byte(value>>5&31), byte(value>>10&31)
		p.Palette = append(
			p.Palette,
			color.RGBA{R: r<<3 | r>>2, G: g<<3 | g>>2, B: b<<3 | b>>2, A: 255},
		)
	}
	if entry.ClassCard || !standard {
		preview, readErr := f.read(data, id)
		p.Main, p.MainErr = preview.Image, readErr
	} else {
		p.MainErr = p.loadFace(f, data, at, pal)
	}
	if !entry.ClassCard {
		p.Mini, p.MiniErr = portraitPart(
			data,
			at+4,
			32,
			512,
			f.stride != 16,
			pal,
		)
	}
	sheet, sheetErr := f.sheet(data, id)
	p.SheetErr = sheetErr
	if sheetErr == nil {
		p.Sheet = sheet.image
	}
	return p, nil
}

func (p *Portrait) loadFace(
	f portraitFormat,
	data []byte,
	at int,
	pal []byte,
) error {
	part, err := f.facePart(data, at)
	if err != nil {
		return err
	}
	height := ((len(part.data) + 1023) / 1024) * 8
	padded := make([]byte, 256*height/2)
	copy(padded, part.data)
	tiles, err := gba.Tiles4BPP(padded, pal, 256, height)
	if err != nil {
		return err
	}
	p.Main = image.NewRGBA(image.Rect(0, 0, 96, 80))
	for _, piece := range portraitFacePieces {
		blitPortrait(p.Main, tiles, piece, draw.Src)
	}
	if f.stride == 16 {
		p.loadFE6Frames(tiles)
		return nil
	}
	p.eyes = [2]*image.RGBA{
		portraitCrop(tiles, image.Rect(192, 0, 224, 16)),
		portraitCrop(tiles, image.Rect(192, 16, 224, 32)),
	}
	p.mouths = make([]*image.RGBA, 7)
	p.mouths[6] = portraitCrop(tiles, image.Rect(224, 0, 256, 16))
	mouth, err := portraitPart(data, at+12, 32, 1536, false, pal)
	if err != nil || mouth == nil {
		return nil
	}
	for index := range 6 {
		p.mouths[index] = portraitCrop(
			mouth,
			image.Rect(0, index*16, 32, index*16+16),
		)
	}
	return nil
}

func (p *Portrait) loadFE6Frames(tiles *image.RGBA) {
	p.mouths = make([]*image.RGBA, 5)
	for index, point := range []image.Point{{X: 192}, {X: 224}, {X: 192, Y: 16}, {X: 224, Y: 16}} {
		p.mouths[index] = portraitCrop(
			tiles,
			image.Rectangle{Min: point, Max: point.Add(image.Pt(32, 16))},
		)
	}
	last := image.NewRGBA(image.Rect(0, 0, 32, 16))
	draw.Draw(last, image.Rect(0, 0, 32, 8), tiles, image.Pt(0, 32), draw.Src)
	draw.Draw(last, image.Rect(0, 8, 32, 16), tiles, image.Pt(32, 32), draw.Src)
	p.mouths[4] = last
}

func portraitFields(f portraitFormat) []Field {
	offset := 20
	if f.stride == 16 {
		offset = 12
	}
	fields := []Field{
		field("portrait_mouth_x", GroupGeneral, offset, 1),
		field("portrait_mouth_y", GroupGeneral, offset+1, 1),
	}
	if f.stride != 16 {
		fields = append(
			fields,
			field("portrait_eye_x", GroupGeneral, 22, 1),
			field("portrait_eye_y", GroupGeneral, 23, 1),
			field("portrait_blink", GroupGeneral, 24, 1),
		)
	}
	return fields
}

// SetPortrait applies confirmed fields from the displayed revision together.
// Unknown bytes and all image pointers remain untouched.
func (s *Session) SetPortrait(p *Portrait, values map[string]int64) error {
	if !p.Current(s) {
		return rom.ErrStale
	}
	s.Refresh()
	current, err := s.PortraitEntry(p.id)
	if err != nil || current.Address != p.address {
		return rom.ErrStale
	}
	g, ok := s.Adapter.(gameFormat)
	if !ok || !g.Match(s.image.Data) {
		return ErrVersion
	}
	if !p.writable {
		return ErrField
	}
	fields := portraitFields(g.portraits)
	var changes []rom.Change
	for name, value := range values {
		index := slices.IndexFunc(
			fields,
			func(f Field) bool { return f.ID == name },
		)
		if index < 0 {
			return ErrField
		}
		f := fields[index]
		if value < 0 || value > 255 {
			return ErrValue
		}
		address := current.Address + f.Offset
		if s.image.Data[address] == byte(value) {
			continue
		}
		for _, table := range s.Tables {
			if table.Err == nil &&
				address < table.Address+table.Stride*table.Count &&
				table.Address < address+1 {
				return ErrLayout
			}
		}
		for _, region := range g.deploymentRanges(s.image.Data) {
			if address < region.End && region.Start < address+1 {
				return ErrLayout
			}
		}
		changes = append(
			changes,
			rom.Change{Offset: address, Data: []byte{byte(value)}},
		)
	}
	if len(changes) == 0 {
		return nil
	}
	if err := s.Document.Apply(
		p.revision,
		"portrait.fields",
		changes,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
