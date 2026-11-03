package maps

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// PaletteSize is the native ten-bank map palette: five normal and five fogged.
const (
	PaletteSize   = 10 * 16 * 2
	MaxPalettePNG = 1 << 20
)

// PaletteSection selects color slots without changing their native order.
type PaletteSection int

const (
	NormalPalette PaletteSection = iota
	FogPalette
	AllPalettes
)

func (section PaletteSection) colors() (start, count int, ok bool) {
	switch section {
	case NormalPalette:
		return 0, 80, true
	case FogPalette:
		return 80, 80, true
	case AllPalettes:
		return 0, 160, true
	default:
		return 0, 0, false
	}
}

var (
	ErrPalettePNG = fault.Code{
		ID:   "game.map_palette_png",
		Kind: fault.InvalidInput,
	}
	ErrPaletteSize = fault.Code{
		ID:   "game.map_palette_size",
		Kind: fault.InvalidInput,
	}
	ErrPalettePixels = fault.Code{
		ID:   "game.map_palette_pixels",
		Kind: fault.InvalidInput,
	}
	ErrPaletteColors = fault.Code{
		ID:   "game.map_palette_colors",
		Kind: fault.InvalidInput,
	}
)

// Palette is an immutable source for PNG work. Its optional map resources are
// independent from the palette contract, so a broken map cannot block colors.
type Palette struct {
	binding                   edit.Stamp
	chapter, pointer, address int
	original                  []byte
	resources                 mapResources
	resourcesErr, layoutErr   error
}

func (s *Editor) Palette(chapter int) (result *Palette, err error) {
	defer func() { err = fault.Wrap(err, "game.read_map_palette", fault.Context{Resource: PaletteField}) }()
	s.Refresh()
	if err := s.context.Validate(s.image.Data); err != nil {
		return nil, err
	}
	_, row, _, err := s.records.Resolve(records.Chapters, chapter, PaletteField)
	if err != nil {
		return nil, err
	}
	list, ok := gba.Pointer(s.image.Data, s.format.Palette)
	if !ok || list < 0xC0 || list%4 != 0 {
		return nil, ErrMap
	}
	site := list + int(s.image.Data[row.Address+6])*4
	address, ok := gba.Pointer(s.image.Data, site)
	if !ok || address < 0xC0 || address%2 != 0 ||
		!gba.Within(s.image.Data, address, PaletteSize) {
		return nil, ErrMap
	}
	p := &Palette{
		binding:  s.image.Stamp(),
		chapter:  chapter,
		pointer:  site,
		address:  address,
		original: bytes.Clone(s.image.Data[address : address+PaletteSize]),
	}
	p.resources, p.resourcesErr = s.format.tilesetResources(
		s.image.Data,
		row.Address,
	)
	p.resources.palette = p.original
	if p.resourcesErr == nil {
		p.resources.layout, p.layoutErr = mapCompressed(
			s.image.Data,
			s.format.Layout,
			s.image.Data[row.Address+8],
			LayoutField,
			maxMapBytes,
		)
		p.resources.layoutID = int(s.image.Data[row.Address+8])
	}
	return p, nil
}

func (p *Palette) Current(
	s *Editor,
) bool {
	return p != nil && p.binding.Current(s.context)
}

func mapPaletteColor(data []byte, index int) color.RGBA {
	word := binary.LittleEndian.Uint16(data[index*2:])
	expand := func(value uint16) uint8 { return uint8((value<<3 | value>>2) & 255) }
	return color.RGBA{
		R: expand(word & 31),
		G: expand(word >> 5 & 31),
		B: expand(word >> 10 & 31),
		A: 255,
	}
}

// paletteChart gives each native slot its own index, including duplicate colors
// and color zero. Every patch is 8x8; all background colors are opaque.
func paletteChart(
	data []byte,
	section PaletteSection,
) (*image.Paletted, error) {
	start, count, ok := section.colors()
	if !ok {
		return nil, edit.ErrValue
	}
	colors := make(color.Palette, count)
	for i := range colors {
		colors[i] = mapPaletteColor(data, start+i)
	}
	out := image.NewPaletted(image.Rect(0, 0, 128, count/16*8), colors)
	for y := range out.Bounds().Dy() {
		for x := range out.Bounds().Dx() {
			out.SetColorIndex(x, y, byte(y/8*16+x/8))
		}
	}
	return out, nil
}

func (p *Palette) Image(section PaletteSection) (*image.Paletted, error) {
	return paletteChart(p.original, section)
}

func (p *Palette) PNG(section PaletteSection) ([]byte, error) {
	chart, err := p.Image(section)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := png.Encode(&output, chart); err != nil {
		return nil, fault.New(
			ErrPalettePNG,
			"game.encode_map_palette",
			fault.Context{},
			err,
		)
	}
	return output.Bytes(), nil
}

// PaletteDraft replaces only the chosen slots. The opposite tone, unused color
// bits and every source byte outside the 320-byte native palette remain intact.
type PaletteDraft struct {
	source  *Palette
	section PaletteSection
	content []byte
}

func (p *Palette) ImportPNG(
	section PaletteSection,
	content []byte,
) (*PaletteDraft, error) {
	start, count, ok := section.colors()
	if !ok {
		return nil, edit.ErrValue
	}
	if len(content) == 0 || len(content) > MaxPalettePNG {
		return nil, ErrPaletteSize
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrPalettePNG,
			"game.decode_map_palette",
			fault.Context{},
			err,
		)
	}
	if config.Width != 128 || config.Height != count/16*8 {
		return nil, ErrPaletteSize
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrPalettePNG,
			"game.decode_map_palette",
			fault.Context{},
			err,
		)
	}
	chart, ok := decoded.(*image.Paletted)
	if !ok {
		return nil, ErrPalettePNG
	}
	for y := range config.Height {
		for x := range config.Width {
			if chart.ColorIndexAt(x, y) != byte(y/8*16+x/8) {
				return nil, ErrPalettePixels
			}
		}
	}
	if len(chart.Palette) < count {
		return nil, ErrPaletteColors
	}
	result := bytes.Clone(p.original)
	for i := range count {
		pixel := color.NRGBAModel.Convert(chart.Palette[i]).(color.NRGBA)
		if pixel.A != 255 {
			return nil, ErrPaletteColors
		}
		quantize := func(value uint8) uint16 { return (uint16(value)*31 + 127) / 255 }
		word := quantize(pixel.R) | quantize(pixel.G)<<5 | quantize(pixel.B)<<10
		at := (start + i) * 2
		word |= binary.LittleEndian.Uint16(result[at:]) & 0x8000
		binary.LittleEndian.PutUint16(result[at:], word)
	}
	return &PaletteDraft{source: p, section: section, content: result}, nil
}

func (d *PaletteDraft) Current(
	s *Editor,
) bool {
	return d != nil && d.source.Current(s)
}
func (d *PaletteDraft) Section() PaletteSection { return d.section }
func (d *PaletteDraft) Image(section PaletteSection) (*image.Paletted, error) {
	return paletteChart(d.content, section)
}

// PalettePreview separates unreadable graphics or layouts from valid colors.
type PalettePreview struct {
	Map, Tileset       Preview
	MapErr, TilesetErr error
}

func (p *Palette) preview(data []byte, section PaletteSection) PalettePreview {
	if section != NormalPalette && section != FogPalette {
		return PalettePreview{MapErr: edit.ErrValue, TilesetErr: edit.ErrValue}
	}
	if p.resourcesErr != nil {
		return PalettePreview{
			MapErr:     p.resourcesErr,
			TilesetErr: p.resourcesErr,
		}
	}
	resources := p.resources
	resources.palette = data
	if section == FogPalette {
		resources.palette = data[160:PaletteSize]
	}
	result := PalettePreview{MapErr: p.layoutErr}
	result.Tileset, result.TilesetErr = resources.tileset()
	if result.MapErr == nil {
		result.Map, result.MapErr = resources.preview(resources.layout)
	}
	return result
}

func (p *Palette) Preview(section PaletteSection) PalettePreview {
	return p.preview(p.original, section)
}

func (d *PaletteDraft) Preview(section PaletteSection) PalettePreview {
	return d.source.preview(d.content, section)
}

type PaletteChange struct {
	Index         int
	Before, After color.RGBA
}

type PalettePlan struct {
	commit          *edit.Commit
	plan            space.WritePlan
	source, pointer int
	changes         []PaletteChange
	uses            []Use
}

func (p *PalettePlan) Current(
	s *Editor,
) bool {
	return p != nil && p.commit.Current(s.context)
}
func (p *PalettePlan) Source() int  { return p.source }
func (p *PalettePlan) Pointer() int { return p.pointer }

func (p *PalettePlan) Destination() int { return p.plan.Destination.Start }

func (p *PalettePlan) Length() int { return p.plan.Destination.End - p.plan.Destination.Start }

func (p *PalettePlan) Changes() []PaletteChange { return slices.Clone(p.changes) }
func (p *PalettePlan) Uses() []Use              { return slices.Clone(p.uses) }

func (s *Editor) PlanPalette(
	draft *PaletteDraft,
) (result *PalettePlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map_palette", fault.Context{Resource: PaletteField}) }()
	s.Refresh()
	if draft == nil || draft.source == nil {
		return nil, rom.ErrStale
	}
	if err := draft.source.binding.Check(s.context); err != nil {
		return nil, err
	}
	if bytes.Equal(draft.source.original, draft.content) {
		return nil, edit.ErrValue
	}
	uses, err := s.mapResourceUses(
		s.format,
		PaletteField,
		draft.source.pointer,
		draft.source.address,
	)
	if err != nil {
		return nil, err
	}
	found := slices.ContainsFunc(
		uses,
		func(use Use) bool { return use.Updated && use.Chapter == draft.source.chapter },
	)
	if !found {
		return nil, rom.ErrStale
	}
	var changes []PaletteChange
	for index := range PaletteSize / 2 {
		before, after := mapPaletteColor(
			draft.source.original,
			index,
		), mapPaletteColor(
			draft.content,
			index,
		)
		if before != after {
			changes = append(
				changes,
				PaletteChange{Index: index, Before: before, After: after},
			)
		}
	}
	builder := space.NewBuilder(
		s.image.Data,
		space.Protection{
			Pointer: func(_ []byte, _ space.Owner, site int) error { return s.mapWriteSite(s.format, site, PaletteField) },
		},
	)
	if _, err := builder.Replace(
		draft.content,
		space.PointerSite{
			Owner:        resourceOwner(PaletteField, draft.source.pointer),
			Address:      draft.source.pointer,
			ExpectedWord: gba.Uint32(s.image.Data, draft.source.pointer),
		},
	); err != nil {
		return nil, err
	}
	plan := builder.Plan()
	return &PalettePlan{
		commit:  edit.NewCommit(draft.source.binding, PaletteField, plan.Edit),
		plan:    plan,
		source:  draft.source.address,
		pointer: draft.source.pointer,
		changes: changes,
		uses:    uses,
	}, nil
}

func (s *Editor) ApplyPalette(plan *PalettePlan) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map_palette", fault.Context{Resource: PaletteField}) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err := plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
