package maps

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

const (
	GraphicsAltField = "map_object_alt_id"
	MaxGraphicsPNG   = 1 << 20
)

// GraphicsSlot identifies a native loading position, not a separate PLIST.
// The same graphics entry can be consumed at either or both positions.
type GraphicsSlot int

const (
	FirstGraphics GraphicsSlot = iota
	SecondGraphics
)

type graphicsConsumer struct {
	field                string
	offset, first, limit int
	optional             bool
}

var graphicsConsumers = [...]graphicsConsumer{
	{field: GraphicsField, offset: 4, first: 0, limit: 0x8000},
	{
		field:    GraphicsAltField,
		offset:   5,
		first:    512,
		limit:    0x4000,
		optional: true,
	},
}

type graphicsInput struct {
	pointer, address int
	data             []byte
	err              error
}

// Read the consumers independently so an unrelated broken resource does not
// prevent editing a valid stream. A zero second ID is an absent consumer.
func (f Format) graphicsInputs(data []byte, address int) [2]graphicsInput {
	var inputs [2]graphicsInput
	for index, consumer := range graphicsConsumers {
		input := &inputs[index]
		if !gba.Within(data, address, 6) {
			input.err = edit.ErrLayout
			continue
		}
		id := data[address+consumer.offset]
		if consumer.optional && id == 0 {
			continue
		}
		list, ok := gba.Pointer(data, f.Graphics)
		if !ok || list < 0xC0 || list%4 != 0 {
			input.err = fault.New(
				ErrMap,
				"game.map_pointer",
				fault.Context{
					Resource:   consumer.field,
					ResourceID: new(int(id)),
					Range: &fault.Range{
						Offset: f.Graphics,
						Length: 4,
						Size:   len(data),
					},
				},
				nil,
			)
			continue
		}
		input.pointer = list + int(id)*4
		input.address, input.err = mapResource(
			data,
			f.Graphics,
			id,
			consumer.field,
		)
		if input.err != nil {
			continue
		}
		if input.address < 0xC0 || input.address%4 != 0 {
			input.err = fault.New(
				ErrMap,
				"game.map_pointer",
				fault.Context{
					Resource:   consumer.field,
					ResourceID: new(int(id)),
				},
				nil,
			)
			continue
		}
		input.data, input.err = mapCompressed(
			data,
			f.Graphics,
			id,
			consumer.field,
			consumer.limit,
		)
		if input.err == nil &&
			(len(input.data) == 0 || len(input.data)%32 != 0) {
			input.err = fault.New(
				ErrMap,
				"game.map_graphics",
				fault.Context{
					Resource:   consumer.field,
					ResourceID: new(int(id)),
				},
				nil,
			)
		}
	}
	return inputs
}

// Each later consumer overwrites only its loaded range. Unloaded gaps remain
// unavailable. Replacement matches a concrete entry, including both roles.
func combineMapGraphics(
	inputs [2]graphicsInput,
	pointer int,
	replacement []byte,
) ([]byte, [1024]bool, error) {
	var present [1024]bool
	var graphics []byte
	for index, input := range inputs {
		if input.err != nil {
			return nil, present, input.err
		}
		if input.pointer == 0 {
			continue
		}
		pixels := input.data
		if pointer != 0 && input.pointer == pointer {
			pixels = replacement
		}
		consumer := graphicsConsumers[index]
		if len(pixels) == 0 || len(pixels)%32 != 0 ||
			len(pixels) > consumer.limit {
			return nil, present, ErrGraphicsReference
		}
		start, end := consumer.first*32, consumer.first*32+len(pixels)
		graphics = append(graphics, make([]byte, max(0, end-len(graphics)))...)
		copy(graphics[start:end], pixels)
		for tile := range len(pixels) / 32 {
			present[consumer.first+tile] = true
		}
	}
	return graphics, present, nil
}

var (
	ErrGraphicsAbsent = fault.Code{
		ID:   "game.map_graphics_absent",
		Kind: fault.InvalidInput,
	}
	ErrGraphicsPNG = fault.Code{
		ID:   "game.map_graphics_png",
		Kind: fault.InvalidInput,
	}
	ErrGraphicsSize = fault.Code{
		ID:   "game.map_graphics_size",
		Kind: fault.InvalidInput,
	}
	ErrGraphicsColors = fault.Code{
		ID:   "game.map_graphics_colors",
		Kind: fault.InvalidInput,
	}
	ErrGraphicsPadding = fault.Code{
		ID:   "game.map_graphics_padding",
		Kind: fault.InvalidInput,
	}
	ErrGraphicsReference = fault.Code{
		ID:   "game.map_graphics_reference",
		Kind: fault.Conflict,
	}
)

// Graphics owns an immutable stream and optional independent preview resources.
// PNG workers consume this value without accessing an editor or its document.
type Graphics struct {
	binding                 edit.Stamp
	chapter                 int
	slot                    GraphicsSlot
	input                   graphicsInput
	inputs                  [2]graphicsInput
	palette                 []byte
	paletteSource           *Palette
	paletteErr              error
	resources               mapResources
	resourcesErr, layoutErr error
}

func (s *Editor) Graphics(
	chapter int,
	slot GraphicsSlot,
) (result *Graphics, err error) {
	defer func() { err = fault.Wrap(err, "game.read_map_graphics", fault.Context{Resource: GraphicsField}) }()
	if slot < FirstGraphics || int(slot) >= len(graphicsConsumers) {
		return nil, edit.ErrValue
	}
	s.Refresh()
	if err := s.context.Validate(s.image.Data); err != nil {
		return nil, err
	}
	_, row, _, err := s.records.Resolve(
		records.Chapters,
		chapter,
		graphicsConsumers[slot].field,
	)
	if err != nil {
		return nil, err
	}
	inputs := s.format.graphicsInputs(s.image.Data, row.Address)
	input := inputs[slot]
	if input.err != nil {
		return nil, input.err
	}
	if input.pointer == 0 {
		return nil, ErrGraphicsAbsent
	}
	g := &Graphics{
		binding: s.image.Stamp(),
		chapter: chapter,
		slot:    slot,
		input:   input,
		inputs:  inputs,
	}
	g.paletteSource, g.paletteErr = s.Palette(chapter)
	if g.paletteErr == nil {
		g.palette = g.paletteSource.original
	}
	g.resources, g.resourcesErr = s.format.tilesetResources(
		s.image.Data,
		row.Address,
	)
	g.resources.palette = g.palette
	if g.resourcesErr == nil {
		g.resources.layout, g.layoutErr = mapCompressed(
			s.image.Data,
			s.format.Layout,
			s.image.Data[row.Address+8],
			LayoutField,
			maxMapBytes,
		)
		g.resources.layoutID = int(s.image.Data[row.Address+8])
	}
	return g, nil
}

func (g *Graphics) Current(
	s *Editor,
) bool {
	return g != nil && g.binding.Current(s.context)
}
func (g *Graphics) Count() int         { return len(g.input.data) / 32 }
func (g *Graphics) PaletteCount() int  { return len(g.palette) / 32 }
func (g *Graphics) Slot() GraphicsSlot { return g.slot }

func (g *Graphics) image(
	pixels, palette []byte,
	bank int,
) (*image.Paletted, error) {
	if g.paletteErr != nil {
		return nil, g.paletteErr
	}
	if bank < 0 || bank >= g.PaletteCount() {
		return nil, edit.ErrValue
	}
	colors := make(color.Palette, 16)
	for index := range colors {
		colors[index] = mapPaletteColor(palette, bank*16+index)
	}
	out := image.NewPaletted(image.Rect(0, 0, 128, (g.Count()+15)/16*8), colors)
	for tile := range g.Count() {
		for y := range 8 {
			for x := range 8 {
				value := pixels[tile*32+y*4+x/2] >> (x % 2 * 4) & 15
				out.SetColorIndex(tile%16*8+x, tile/16*8+y, value)
			}
		}
	}
	return out, nil
}

func (g *Graphics) Image(
	bank int,
) (*image.Paletted, error) {
	return g.image(g.input.data, g.palette, bank)
}

func (g *Graphics) PNG(bank int) ([]byte, error) {
	bitmap, err := g.Image(bank)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := png.Encode(&output, bitmap); err != nil {
		return nil, fault.New(
			ErrGraphicsPNG,
			"game.encode_map_graphics",
			fault.Context{},
			err,
		)
	}
	return output.Bytes(), nil
}

// GraphicsImport selects whether PNG colors replace one native palette bank.
// Bank is ignored when WithPalette is false; indices always retain their order.
type GraphicsImport struct {
	WithPalette bool
	Bank        int
}

type GraphicsDraft struct {
	source  *Graphics
	content []byte
	encoded []byte
	palette []byte
	options GraphicsImport
}

// ImportPNG retains the resource's tile count and numbering. A joint import
// requires all sixteen opaque colors, including slots unused by these pixels.
func (g *Graphics) ImportPNG(
	content []byte,
	options GraphicsImport,
) (*GraphicsDraft, error) {
	if options.WithPalette {
		if g.paletteErr != nil {
			return nil, g.paletteErr
		}
		if options.Bank < 0 || options.Bank >= g.PaletteCount() {
			return nil, edit.ErrValue
		}
	}
	if len(content) == 0 || len(content) > MaxGraphicsPNG {
		return nil, ErrGraphicsPNG
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrGraphicsPNG,
			"game.decode_map_graphics",
			fault.Context{},
			err,
		)
	}
	if config.Width != 128 || config.Height != (g.Count()+15)/16*8 {
		return nil, ErrGraphicsSize
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrGraphicsPNG,
			"game.decode_map_graphics",
			fault.Context{},
			err,
		)
	}
	bitmap, ok := decoded.(*image.Paletted)
	if !ok {
		return nil, ErrGraphicsColors
	}
	for _, value := range bitmap.Palette[:min(16, len(bitmap.Palette))] {
		_, _, _, alpha := value.RGBA()
		if alpha != 0xFFFF {
			return nil, ErrGraphicsColors
		}
	}
	for y := range config.Height {
		for x := range config.Width {
			index := bitmap.ColorIndexAt(x, y)
			if index > 15 || int(index) >= len(bitmap.Palette) {
				return nil, ErrGraphicsColors
			}
			if y/8*16+x/8 >= g.Count() && index != 0 {
				return nil, ErrGraphicsPadding
			}
		}
	}
	pixels := make([]byte, len(g.input.data))
	for tile := range g.Count() {
		for y := range 8 {
			for x := 0; x < 8; x += 2 {
				left, top := tile%16*8+x, tile/16*8+y
				pixels[tile*32+y*4+x/2] = bitmap.ColorIndexAt(
					left,
					top,
				) | bitmap.ColorIndexAt(
					left+1,
					top,
				)<<4
			}
		}
	}
	encoded, err := gba.CompressLZ77(pixels)
	if err != nil {
		return nil, err
	}
	palette := g.palette
	if options.WithPalette {
		if len(bitmap.Palette) < 16 {
			return nil, ErrGraphicsColors
		}
		palette, err = replacePaletteColors(
			g.palette,
			options.Bank*16,
			bitmap.Palette[:16],
		)
		if err != nil {
			return nil, err
		}
	}
	return &GraphicsDraft{
		source:  g,
		content: pixels,
		encoded: encoded,
		palette: palette,
		options: options,
	}, nil
}

func (d *GraphicsDraft) Current(
	s *Editor,
) bool {
	return d != nil && d.source.Current(s)
}

func (d *GraphicsDraft) Image(bank int) (*image.Paletted, error) {
	return d.source.image(d.content, d.palette, bank)
}

// GraphicsPreview reports map and definition failures independently of the
// selected stream, which can remain valid and editable.
type GraphicsPreview struct {
	Map, Tileset       Preview
	MapErr, TilesetErr error
}

func (g *Graphics) preview(
	pixels, palette []byte,
	tone PaletteSection,
) GraphicsPreview {
	if tone != NormalPalette && tone != FogPalette {
		return GraphicsPreview{MapErr: edit.ErrValue, TilesetErr: edit.ErrValue}
	}
	if g.paletteErr != nil {
		return GraphicsPreview{MapErr: g.paletteErr, TilesetErr: g.paletteErr}
	}
	if g.resourcesErr != nil {
		return GraphicsPreview{
			MapErr:     g.resourcesErr,
			TilesetErr: g.resourcesErr,
		}
	}
	resources := g.resources
	var err error
	resources.graphics, resources.graphicsPresent, err = combineMapGraphics(
		g.inputs,
		g.input.pointer,
		pixels,
	)
	if err != nil {
		return GraphicsPreview{MapErr: err, TilesetErr: err}
	}
	resources.palette = palette
	if tone == FogPalette {
		resources.palette = palette[160:PaletteSize]
	}
	result := GraphicsPreview{MapErr: g.layoutErr}
	result.Tileset, result.TilesetErr = resources.tileset()
	if result.MapErr == nil {
		result.Map, result.MapErr = resources.preview(resources.layout)
	}
	return result
}

func (g *Graphics) Preview(
	tone PaletteSection,
) GraphicsPreview {
	return g.preview(g.input.data, g.palette, tone)
}

func (d *GraphicsDraft) Preview(tone PaletteSection) GraphicsPreview {
	return d.source.preview(d.content, d.palette, tone)
}

// GraphicsUse keeps the loading role distinct from the entry's identity.
// Updated is true for both roles sharing the selected concrete PLIST entry.
type GraphicsUse struct {
	Chapter, ResourceID, Pointer int
	Slot                         GraphicsSlot
	Updated                      bool
}

type GraphicsPlan struct {
	commit          *edit.Commit
	plan            romwrite.WritePlan
	source, pointer int
	changed         []int
	uses            []GraphicsUse
	paletteChanges  []PaletteChange
	paletteUses     []Use
	options         GraphicsImport
}

func (p *GraphicsPlan) Current(
	s *Editor,
) bool {
	return p != nil && p.commit.Current(s.context)
}
func (p *GraphicsPlan) Source() int  { return p.source }
func (p *GraphicsPlan) Pointer() int { return p.pointer }

func (p *GraphicsPlan) Destination() int { return p.plan.Destination.Start }

func (p *GraphicsPlan) Length() int { return p.plan.Destination.End - p.plan.Destination.Start }

func (p *GraphicsPlan) ChangedTiles() []int { return slices.Clone(p.changed) }

func (p *GraphicsPlan) Uses() []GraphicsUse { return slices.Clone(p.uses) }

func (p *GraphicsPlan) PaletteChanges() []PaletteChange { return slices.Clone(p.paletteChanges) }

func (p *GraphicsPlan) PaletteUses() []Use            { return slices.Clone(p.paletteUses) }
func (p *GraphicsPlan) ImportOptions() GraphicsImport { return p.options }

func (s *Editor) graphicsUses(g *Graphics) ([]GraphicsUse, error) {
	chapters := s.records.Table(records.Chapters)
	if chapters == nil || chapters.Err != nil {
		return nil, edit.ErrLayout
	}
	list, ok := gba.Pointer(s.image.Data, s.format.Graphics)
	if !ok {
		return nil, ErrMap
	}
	var uses []GraphicsUse
	found := false
	for _, row := range chapters.Rows {
		for slot, consumer := range graphicsConsumers {
			id, err := s.records.Value(records.Chapters, row.ID, consumer.field)
			if err != nil {
				return nil, err
			}
			if consumer.optional && id == 0 {
				continue
			}
			site := list + int(id)*4
			address, valid := gba.Pointer(s.image.Data, site)
			if !valid || address != g.input.address {
				continue
			}
			updated := site == g.input.pointer
			uses = append(
				uses,
				GraphicsUse{
					Chapter:    row.ID,
					ResourceID: int(id),
					Pointer:    site,
					Slot:       GraphicsSlot(slot),
					Updated:    updated,
				},
			)
			if !updated {
				continue
			}
			if len(g.input.data) > consumer.limit {
				return nil, ErrGraphicsReference
			}
			if row.ID == g.chapter && GraphicsSlot(slot) == g.slot {
				found = true
			}
		}
	}
	if !found {
		return nil, rom.ErrStale
	}
	return uses, nil
}

func (s *Editor) PlanGraphics(
	draft *GraphicsDraft,
) (result *GraphicsPlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map_graphics", fault.Context{Resource: GraphicsField}) }()
	s.Refresh()
	if draft == nil || draft.source == nil {
		return nil, rom.ErrStale
	}
	g := draft.source
	if err := g.binding.Check(s.context); err != nil {
		return nil, err
	}
	pixelsChanged := !bytes.Equal(g.input.data, draft.content)
	colorsChanged := !bytes.Equal(g.palette, draft.palette)
	if !pixelsChanged && !colorsChanged {
		return nil, edit.ErrValue
	}
	var uses []GraphicsUse
	if pixelsChanged {
		uses, err = s.graphicsUses(g)
		if err != nil {
			return nil, err
		}
	}
	var paletteUses []Use
	var changes []PaletteChange
	if colorsChanged {
		paletteUses, err = s.paletteUses(g.paletteSource)
		if err != nil {
			return nil, err
		}
		changes = paletteChanges(g.palette, draft.palette)
	}
	var changed []int
	for tile := range g.Count() {
		if !bytes.Equal(
			g.input.data[tile*32:(tile+1)*32],
			draft.content[tile*32:(tile+1)*32],
		) {
			changed = append(changed, tile)
		}
	}
	builder := romwrite.NewBuilder(
		s.image.Data,
		romwrite.Protection{
			Pointer: func(_ []byte, owner romwrite.Owner, site int) error {
				field := GraphicsField
				if owner == resourceOwner(PaletteField, site) {
					field = PaletteField
				}
				return s.mapWriteSite(s.format, site, field)
			},
		},
	)
	if pixelsChanged {
		if _, err := builder.Replace(
			draft.encoded,
			romwrite.PointerSite{
				Owner:        resourceOwner(GraphicsField, g.input.pointer),
				Address:      g.input.pointer,
				ExpectedWord: gba.Uint32(s.image.Data, g.input.pointer),
			},
		); err != nil {
			return nil, err
		}
	}
	if colorsChanged {
		p := g.paletteSource
		if _, err := builder.Replace(draft.palette, romwrite.PointerSite{
			Owner: resourceOwner(PaletteField, p.pointer), Address: p.pointer,
			ExpectedWord: gba.Uint32(s.image.Data, p.pointer),
		}); err != nil {
			return nil, err
		}
	}
	plan := builder.Plan()
	return &GraphicsPlan{
		commit:         edit.NewCommit(g.binding, GraphicsField, plan.Edit),
		plan:           plan,
		source:         g.input.address,
		pointer:        g.input.pointer,
		changed:        changed,
		uses:           uses,
		paletteChanges: changes,
		paletteUses:    paletteUses,
		options:        draft.options,
	}, nil
}

func (s *Editor) ApplyGraphics(plan *GraphicsPlan) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map_graphics", fault.Context{Resource: GraphicsField}) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err := plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
