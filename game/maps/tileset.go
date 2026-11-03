package maps

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const tilesetTerrainOffset = 0x2000

var (
	ErrTilesetReference = fault.Code{
		ID:   "game.map_tileset_reference",
		Kind: fault.Conflict,
	}
	ErrTilesetConfigSize = fault.Code{
		ID:   "game.map_tileset_config_size",
		Kind: fault.InvalidInput,
	}
	ErrTilesetConfigData = fault.Code{
		ID:   "game.map_tileset_config_data",
		Kind: fault.InvalidInput,
	}
)

// TilePart describes one 8x8 background tile in a 16x16 map definition.
// The native halfword uses ten tile bits, two flips and four palette bits.
type TilePart struct {
	Tile, Palette int
	FlipX, FlipY  bool
}

func readTilePart(word uint16) TilePart {
	return TilePart{
		Tile:    int(word & 0x3FF),
		Palette: int(word >> 12),
		FlipX:   word&0x400 != 0,
		FlipY:   word&0x800 != 0,
	}
}

func (p TilePart) word() uint16 {
	word := uint16(p.Tile | p.Palette<<12)
	if p.FlipX {
		word |= 0x400
	}
	if p.FlipY {
		word |= 0x800
	}
	return word
}

// TileDefinition stores parts in row order: top left, top right, bottom left,
// bottom right. Terrain is a raw byte ID, independent of class movement tables.
type TileDefinition struct {
	Parts   [4]TilePart
	Terrain int
}

// TilesetDraft edits the existing configuration without editing map layouts,
// graphics or palettes. A damaged base map only affects its optional preview.
type TilesetDraft struct {
	binding                   edit.Stamp
	chapter, pointer, address int
	resources                 mapResources
	content                   []byte
	layoutErr                 error
}

func (s *Editor) NewTilesetDraft(chapter int) (*TilesetDraft, error) {
	s.Refresh()
	if err := s.context.Validate(s.image.Data); err != nil {
		return nil, err
	}
	_, row, _, err := s.records.Resolve(records.Chapters, chapter, ConfigField)
	if err != nil {
		return nil, err
	}
	resources, err := s.format.tilesetResources(s.image.Data, row.Address)
	if err != nil {
		return nil, err
	}
	if len(resources.config) <= tilesetTerrainOffset {
		return nil, ErrMap
	}
	list, ok := gba.Pointer(s.image.Data, s.format.Config)
	if !ok || list < 0xC0 || list%4 != 0 {
		return nil, ErrMap
	}
	site := list + resources.configID*4
	address, ok := gba.Pointer(s.image.Data, site)
	if !ok || address < 0xC0 || address%4 != 0 {
		return nil, ErrMap
	}
	resources.palette = bytes.Clone(resources.palette)
	resources.layout, err = mapCompressed(
		s.image.Data,
		s.format.Layout,
		s.image.Data[row.Address+8],
		LayoutField,
		maxMapBytes,
	)
	return &TilesetDraft{
		binding:   s.image.Stamp(),
		chapter:   chapter,
		pointer:   site,
		address:   address,
		resources: resources,
		content:   bytes.Clone(resources.config),
		layoutErr: err,
	}, nil
}

// TilesetConfig is an immutable .mapchip_config file snapshot. Its private
// draft shares only immutable source resources; file workers never read the
// live editing draft or document.
type TilesetConfig struct {
	source TilesetDraft
}

func (d *TilesetDraft) ConfigSize() int { return len(d.content) }

func (d *TilesetDraft) Config() *TilesetConfig {
	source := *d
	source.content = bytes.Clone(d.content)
	return &TilesetConfig{source: source}
}

func (f *TilesetConfig) Current(
	s *Editor,
) bool {
	return f != nil && f.source.Current(s)
}

func (f *TilesetConfig) Size() int { return len(f.source.content) }

// Bytes exports the complete uncompressed stream, including unchanged words
// whose terrain entries are absent from a short native configuration.
func (f *TilesetConfig) Bytes() []byte { return bytes.Clone(f.source.content) }

// Import replaces existing definitions in a detached draft. The native count
// stays fixed, and bytes outside the known definitions must match the ROM.
// Validation compares with the ROM source, so unchanged custom words survive.
func (f *TilesetConfig) Import(
	content []byte,
) (result *TilesetDraft, err error) {
	defer func() { err = fault.Wrap(err, "game.import_map_tileset_config", fault.Context{Resource: ConfigField}) }()
	if len(content) != f.Size() {
		return nil, fault.New(
			ErrTilesetConfigSize,
			"game.map_tileset_config_size",
			fault.Context{
				Range: &fault.Range{Length: len(content), Size: f.Size()},
			},
			nil,
		)
	}
	draft := f.source
	draft.content = bytes.Clone(draft.resources.config)
	known := draft.Count() * 8
	if !bytes.Equal(
		content[known:tilesetTerrainOffset],
		draft.content[known:tilesetTerrainOffset],
	) {
		return nil, fault.New(
			ErrTilesetConfigData,
			"game.map_tileset_config_unknown",
			fault.Context{
				Range: &fault.Range{
					Offset: known,
					Length: tilesetTerrainOffset - known,
					Size:   len(content),
				},
			},
			nil,
		)
	}
	for index := range draft.Count() {
		definition := tileDefinition(content, index)
		for part, value := range definition.Parts {
			if err := draft.SetPart(index, part, value); err != nil {
				return nil, fault.New(
					ErrTilesetConfigData,
					"game.map_tileset_config_part",
					fault.Context{
						ResourceID: new(index),
						Range: &fault.Range{
							Offset: index*8 + part*2,
							Length: 2,
							Size:   len(content),
						},
					},
					err,
				)
			}
		}
		draft.content[tilesetTerrainOffset+index] = byte(definition.Terrain)
	}
	return &draft, nil
}

func (d *TilesetDraft) Current(
	s *Editor,
) bool {
	return d != nil && d.binding.Current(s.context)
}

func (d *TilesetDraft) Modified() bool { return !bytes.Equal(d.content, d.resources.config) }

func (d *TilesetDraft) Count() int { return min(1024, len(d.content)-tilesetTerrainOffset) }

func (d *TilesetDraft) GraphicsCount() int { return len(d.resources.graphics) / 32 }

// The native display adds five banks when drawing fogged cells.
func (d *TilesetDraft) PaletteCount() int { return 5 }

func tileDefinition(config []byte, index int) TileDefinition {
	definition := TileDefinition{
		Terrain: int(config[tilesetTerrainOffset+index]),
	}
	for part := range definition.Parts {
		definition.Parts[part] = readTilePart(
			gba.Uint16(config, index*8+part*2),
		)
	}
	return definition
}

func (d *TilesetDraft) Definition(index int) (TileDefinition, error) {
	if index < 0 || index >= d.Count() {
		return TileDefinition{}, edit.ErrValue
	}
	return tileDefinition(d.content, index), nil
}

func (d *TilesetDraft) SetPart(index, part int, value TilePart) error {
	if index < 0 || index >= d.Count() || part < 0 || part >= 4 ||
		value.Tile < 0 || value.Tile > 1023 || value.Palette < 0 || value.Palette > 15 {
		return edit.ErrValue
	}
	at := index*8 + part*2
	word := value.word()
	original := readTilePart(gba.Uint16(d.resources.config, at))
	if value.Palette != original.Palette && value.Palette >= d.PaletteCount() {
		return edit.ErrValue
	}
	// Existing invalid words can be restored; changing another field never
	// normalizes unused definitions. New parts must have readable graphics.
	if word != gba.Uint16(d.resources.config, at) &&
		!d.resources.hasGraphic(value.Tile) {
		return edit.ErrValue
	}
	binary.LittleEndian.PutUint16(d.content[at:], word)
	return nil
}

func (d *TilesetDraft) SetTerrain(index, terrain int) error {
	if index < 0 || index >= d.Count() || terrain < 0 || terrain > 255 {
		return edit.ErrValue
	}
	d.content[tilesetTerrainOffset+index] = byte(terrain)
	return nil
}

func (d *TilesetDraft) Tileset() (Preview, error) {
	resources := d.resources
	resources.config = d.content
	return resources.tileset()
}

func (d *TilesetDraft) Preview() (Preview, error) {
	if d.layoutErr != nil {
		return Preview{}, d.layoutErr
	}
	resources := d.resources
	resources.config = d.content
	return resources.preview(resources.layout)
}

// TilesetChange and the plan's use list are immutable preview values.
type TilesetChange struct {
	Index         int
	Before, After TileDefinition
}

type TilesetPlan struct {
	commit          *edit.Commit
	plan            space.WritePlan
	source, pointer int
	changes         []TilesetChange
	uses            []Use
}

func (p *TilesetPlan) Current(
	s *Editor,
) bool {
	return p != nil && p.commit.Current(s.context)
}
func (p *TilesetPlan) Source() int  { return p.source }
func (p *TilesetPlan) Pointer() int { return p.pointer }

func (p *TilesetPlan) Destination() int { return p.plan.Destination.Start }

func (p *TilesetPlan) Length() int { return p.plan.Destination.End - p.plan.Destination.Start }

func (p *TilesetPlan) Changes() []TilesetChange { return slices.Clone(p.changes) }
func (p *TilesetPlan) Uses() []Use              { return slices.Clone(p.uses) }

func (s *Editor) PlanTileset(
	draft *TilesetDraft,
) (result *TilesetPlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map_tileset", fault.Context{Resource: ConfigField}) }()
	s.Refresh()
	if draft == nil {
		return nil, rom.ErrStale
	}
	if err := draft.binding.Check(s.context); err != nil {
		return nil, err
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
	}
	var changes []TilesetChange
	for index := range draft.Count() {
		before, after := tileDefinition(
			draft.resources.config,
			index,
		), tileDefinition(
			draft.content,
			index,
		)
		if before != after {
			changes = append(
				changes,
				TilesetChange{Index: index, Before: before, After: after},
			)
		}
	}
	uses, err := s.mapResourceUses(
		s.format,
		ConfigField,
		draft.pointer,
		draft.address,
	)
	if err != nil {
		return nil, err
	}
	found := false
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		found = found || use.Chapter == draft.chapter
		_, row, _, err := s.records.Resolve(
			records.Chapters,
			use.Chapter,
			ConfigField,
		)
		if err != nil {
			return nil, err
		}
		resources, err := s.format.tilesetResources(s.image.Data, row.Address)
		if err != nil {
			return nil, err
		}
		for _, change := range changes {
			for part, value := range change.After.Parts {
				if value != change.Before.Parts[part] &&
					!resources.hasGraphic(value.Tile) {
					return nil, fault.New(
						ErrTilesetReference,
						"game.shared_map_tileset",
						fault.Context{
							Table:      string(records.Chapters),
							RecordID:   new(use.Chapter),
							Resource:   ConfigField,
							ResourceID: new(change.Index),
						},
						nil,
					)
				}
			}
		}
	}
	if !found {
		return nil, rom.ErrStale
	}
	compressed, err := gba.CompressLZ77(draft.content)
	if err != nil {
		return nil, err
	}
	builder := space.NewBuilder(s.image.Data, space.Protection{
		Pointer: func(_ []byte, _ space.Owner, site int) error { return s.mapWriteSite(s.format, site, ConfigField) },
	})
	if _, err := builder.Replace(
		compressed,
		space.PointerSite{
			Owner:        resourceOwner(ConfigField, draft.pointer),
			Address:      draft.pointer,
			ExpectedWord: gba.Uint32(s.image.Data, draft.pointer),
		},
	); err != nil {
		return nil, err
	}
	plan := builder.Plan()
	return &TilesetPlan{
		commit: edit.NewCommit(
			draft.binding,
			ConfigField,
			plan.Edit,
		),
		plan:    plan,
		source:  draft.address,
		pointer: draft.pointer,
		changes: changes,
		uses:    uses,
	}, nil
}

func (s *Editor) ApplyTileset(plan *TilesetPlan) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map_tileset", fault.Context{Resource: ConfigField}) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err := plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
