// Package portrait decodes and edits native portrait resources.
package portrait

import (
	"image"
	"image/color"
	"image/draw"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Editor owns portrait discovery, decoded frames and metadata edits. Record
// references remain inputs to portrait-table discovery, including damaged rows.
type Editor struct {
	context         *edit.Context
	format          Format
	generic         GenericFormat
	records         *records.Editor
	protected       func([]byte) []space.Range
	image           rom.Snapshot
	binding         edit.Stamp
	portraitEntries []Entry
}

func New(
	context *edit.Context,
	format Format,
	generic GenericFormat,
	records *records.Editor,
	protected func([]byte) []space.Range,
) *Editor {
	return &Editor{
		context:   context,
		format:    format,
		generic:   generic,
		records:   records,
		protected: protected,
	}
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
		s.portraitEntries = nil
	}
}

func (f Format) Image(data []byte, id int) (*image.RGBA, error) {
	sheet, err := f.sheet(data, id)
	return sheet.image, err
}

func (f Format) Encode(
	data []byte,
	id int,
	bitmap image.Image,
) (graphics.Encoding, error) {
	sheet, err := f.sheet(data, id)
	if err != nil {
		return graphics.Encoding{}, err
	}
	parts, err := sheet.encode(bitmap)
	return graphics.Encoding{
		Parts:    parts,
		Expected: gba.Quantise4BPP(bitmap),
	}, err
}

// Portrait is a revision-bound view of one shared resource. Expressions are
// composed from decoded parts; changing a preview frame never edits the record.
type Portrait struct {
	Entry                      Entry
	Mouth, Eyes                image.Point
	Blink                      int
	HasEyes                    bool
	Fields                     []records.Field
	Resources                  []records.RecordResource
	Palette                    []color.RGBA
	Main, Mini, Sheet          *image.RGBA
	MainErr, MiniErr, SheetErr error
	binding                    edit.Stamp
	id, address                int
	writable                   bool
	eyes                       [2]*image.RGBA
	mouths                     []*image.RGBA
}

func (p *Portrait) Revision() uint64 { return p.binding.Revision() }

func (p *Portrait) Current(s *Editor) bool {
	return p != nil && p.binding.Current(s.context)
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
		return nil, edit.ErrValue
	}
	result := portraitCrop(p.Main, p.Main.Bounds())
	if eye > 0 {
		if !p.HasEyes || p.eyes[eye-1] == nil {
			return nil, graphics.ErrImage
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
			return nil, graphics.ErrImage
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
	if !gba.Within(data, at, 4) || gba.Uint32(data, at) == 0 {
		return nil, nil
	}
	part, err := readAssetPart(data, at, width, size, compressed, nil)
	if err != nil {
		return nil, err
	}
	return gba.Tiles4BPP(part.data, palette, width, size*2/width)
}

func portraitResource(
	data []byte,
	label string,
	at, size int,
) records.RecordResource {
	r := records.RecordResource{Label: label, Pointer: at, Length: size}
	if !gba.Within(data, at, 4) {
		r.Err = graphics.ErrImage
		return r
	}
	if gba.Uint32(data, at) == 0 {
		r.Empty = true
		return r
	}
	target, ok := gba.Pointer(data, at)
	if !ok || target < 0xC0 || !gba.Within(data, target, size) {
		r.Err = graphics.ErrImage
		return r
	}
	r.Address = target
	return r
}

func (s *Editor) Read(id int) (*Portrait, error) {
	s.Refresh()
	entry, err := s.Entry(id)
	if err != nil {
		return nil, err
	}
	g := s.format
	if s.context.Validate(s.image.Data) != nil {
		return nil, edit.ErrVersion
	}
	f, data, at := g, s.image.Data, entry.Address
	p := &Portrait{
		Entry:   entry,
		binding: s.context.Bind(s.image.Revision),
		id:      id,
		address: at,
	}
	miniLength := 4
	if f.Stride == 16 {
		miniLength = 512
	}
	p.Resources = []records.RecordResource{
		portraitResource(data, "portrait_main", at, 4),
		portraitResource(data, "portrait_mini", at+4, miniLength),
		portraitResource(data, "portrait_palette", at+8, 32),
	}
	if entry.ClassCard && f.Stride == 16 {
		p.Resources[0].Label = "portrait_card"
	}
	if f.Stride != 16 {
		p.Resources = append(
			p.Resources,
			portraitResource(data, "portrait_mouth_tiles", at+12, 1536),
			portraitResource(data, "portrait_card", at+16, 4),
		)
	}
	if entry.Empty {
		return p, nil
	}
	standard := f.Mode == nil || f.Mode(data) == Standard
	if !entry.ClassCard && standard {
		offset := 20
		if f.Stride == 16 {
			offset = 12
		}
		p.Mouth = image.Pt(int(data[at+offset]), int(data[at+offset+1]))
		p.writable = true
		p.Fields = portraitFields(f)
		if f.Stride != 16 {
			p.HasEyes = true
			p.Eyes = image.Pt(int(data[at+22]), int(data[at+23]))
			p.Blink = int(data[at+24])
		}
	}
	palette, valid := gba.Pointer(data, at+8)
	if !valid || palette < 0xC0 || !gba.Within(data, palette, 32) {
		p.MainErr, p.MiniErr, p.SheetErr = graphics.ErrImage, graphics.ErrImage, graphics.ErrImage
		return p, nil
	}
	pal := data[palette : palette+32]
	for index := range 16 {
		value := gba.Uint16(pal, index*2)
		r, g, b := byte(value&31), byte(value>>5&31), byte(value>>10&31)
		p.Palette = append(
			p.Palette,
			color.RGBA{R: r<<3 | r>>2, G: g<<3 | g>>2, B: b<<3 | b>>2, A: 255},
		)
	}
	if entry.ClassCard || !standard {
		preview, readErr := f.Preview(data, id)
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
			f.Stride != 16,
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
	f Format,
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
	if f.Stride == 16 {
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

func portraitFields(f Format) []records.Field {
	offset := 20
	if f.Stride == 16 {
		offset = 12
	}
	fields := []records.Field{
		records.NumericField(
			"portrait_mouth_x",
			records.GroupGeneral,
			offset,
			1,
		),
		records.NumericField(
			"portrait_mouth_y",
			records.GroupGeneral,
			offset+1,
			1,
		),
	}
	if f.Stride != 16 {
		fields = append(
			fields,
			records.NumericField("portrait_eye_x", records.GroupGeneral, 22, 1),
			records.NumericField("portrait_eye_y", records.GroupGeneral, 23, 1),
			records.NumericField("portrait_blink", records.GroupGeneral, 24, 1),
		)
	}
	return fields
}

// SetPortrait applies confirmed fields from the displayed revision together.
// Unknown bytes and all image pointers remain untouched.
func (s *Editor) Set(p *Portrait, values map[string]int64) error {
	if !p.Current(s) {
		return rom.ErrStale
	}
	s.Refresh()
	current, err := s.Entry(p.id)
	if err != nil || current.Address != p.address {
		return rom.ErrStale
	}
	g := s.format
	if s.context.Validate(s.image.Data) != nil {
		return edit.ErrVersion
	}
	if !p.writable {
		return edit.ErrField
	}
	fields := portraitFields(g)
	var changes []rom.Change
	for name, value := range values {
		index := slices.IndexFunc(
			fields,
			func(f records.Field) bool { return f.ID == name },
		)
		if index < 0 {
			return edit.ErrField
		}
		f := fields[index]
		if value < 0 || value > 255 {
			return edit.ErrValue
		}
		address := current.Address + f.Offset
		if s.image.Data[address] == byte(value) {
			continue
		}
		for _, table := range s.records.Tables() {
			if table.Err == nil &&
				address < table.Address+table.Stride*table.Count &&
				table.Address < address+1 {
				return edit.ErrLayout
			}
		}
		for _, region := range s.protected(s.image.Data) {
			if address < region.End && region.Start < address+1 {
				return edit.ErrLayout
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
	if err := s.context.Document().Apply(
		p.binding.Revision(),
		"portrait.fields",
		changes,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
