package game

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Native base-map limits include the engine's row and cell buffers.
const (
	MaxMapWidth  = 60
	MaxMapHeight = 63
	MaxMapCells  = 1536
	maxMapBytes  = 2 + MaxMapCells*2

	chapterMapField  = "map_id"
	mapGraphicsField = "map_object_id"
)

var ErrMap = fault.Code{ID: "game.invalid_map", Kind: fault.InvalidData}

type MapCell struct {
	Tile    uint16
	Terrain byte
}

// MapPreview is the static base map, before units, animations or event changes.
// Tile stores the native map word; Terrain is its raw terrain ID.
type MapPreview struct {
	Image         *image.RGBA
	Width, Height int
	Cells         []MapCell
}

func (m MapPreview) Cell(x, y int) (MapCell, bool) {
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height ||
		y*m.Width+x >= len(m.Cells) {
		return MapCell{}, false
	}
	return m.Cells[y*m.Width+x], true
}

// ChapterMap follows the current chapter layout, including manual and relocated
// tables, and reads resources from the same snapshot as the chapter fields.
func (s *Session) ChapterMap(id int) (preview MapPreview, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_map",
			fault.Context{Table: string(Chapters), RecordID: new(id)},
		)
	}()
	if !s.Adapter.Match(s.image.Data) {
		return MapPreview{}, ErrVersion
	}
	_, row, _, err := s.locate(Chapters, id, chapterMapField)
	if err != nil {
		return MapPreview{}, err
	}
	return s.Adapter.ChapterMap(s.image.Data, row.Address)
}

type mapFormat struct {
	graphics, palette, config, layout           int
	animation, animation2, changes, worldEvents int
}

func (g gameFormat) ChapterMap(data []byte, address int) (MapPreview, error) {
	return g.maps.read(data, address)
}

// Resource kinds can have separate pointer lists after the legacy PLIST split.
// Resolve each kind through its own live pointer, without assuming a shared
// list.
func mapResource(data []byte, root int, id byte, label string) (int, error) {
	base, ok := pointer(data, root)
	if !ok {
		return 0, fault.New(
			ErrMap,
			"game.map_pointer",
			fault.Context{
				Resource:   label,
				ResourceID: new(int(id)),
				Range: &fault.Range{
					Offset: root,
					Length: 4,
					Size:   len(data),
				},
			},
			nil,
		)
	}
	at, ok := pointer(data, base+int(id)*4)
	if !ok {
		return 0, fault.New(
			ErrMap,
			"game.map_pointer",
			fault.Context{
				Resource:   label,
				ResourceID: new(int(id)),
				Range: &fault.Range{
					Offset: base + int(id)*4,
					Length: 4,
					Size:   len(data),
				},
			},
			nil,
		)
	}
	return at, nil
}

func mapCompressed(
	data []byte,
	root int,
	id byte,
	label string,
	limit int,
) ([]byte, error) {
	at, err := mapResource(data, root, id, label)
	if err != nil {
		return nil, err
	}
	decoded, err := gba.DecompressLZ77(data[at:], limit)
	if err != nil {
		return nil, fault.New(
			ErrMap,
			"game.decompress_map",
			fault.Context{
				Resource:   label,
				ResourceID: new(int(id)),
				Range: &fault.Range{
					Offset: at,
					Length: limit,
					Size:   len(data),
				},
			},
			err,
		)
	}
	return decoded, nil
}

type mapResources struct {
	graphics, palette, config, layout []byte
	objectID, configID, layoutID      int
}

func (f mapFormat) read(data []byte, address int) (MapPreview, error) {
	resources, err := f.resources(data, address)
	if err != nil {
		return MapPreview{}, err
	}
	return resources.preview(resources.layout)
}

func (f mapFormat) resources(data []byte, address int) (mapResources, error) {
	if !within(data, address, 9) {
		return mapResources{}, ErrLayout
	}
	graphics, err := mapCompressed(
		data,
		f.graphics,
		data[address+4],
		mapGraphicsField,
		0x8000,
	)
	if err != nil {
		return mapResources{}, err
	}
	if len(graphics)%32 != 0 {
		return mapResources{}, fault.New(
			ErrMap,
			"game.map_graphics",
			fault.Context{
				Resource:   mapGraphicsField,
				ResourceID: new(int(data[address+4])),
			},
			nil,
		)
	}
	if data[address+5] != 0 {
		second, secondErr := mapCompressed(
			data,
			f.graphics,
			data[address+5],
			"map_object_alt_id",
			0x8000-len(graphics),
		)
		if secondErr != nil {
			return mapResources{}, secondErr
		}
		if len(second)%32 != 0 {
			return mapResources{}, fault.New(
				ErrMap,
				"game.map_graphics",
				fault.Context{
					Resource:   "map_object_alt_id",
					ResourceID: new(int(data[address+5])),
				},
				nil,
			)
		}
		graphics = append(graphics, second...)
	}
	paletteAt, err := mapResource(
		data,
		f.palette,
		data[address+6],
		"map_palette_id",
	)
	if err != nil {
		return mapResources{}, err
	}
	if !within(data, paletteAt, 512) {
		return mapResources{}, fault.New(
			ErrMap,
			"game.map_palette",
			fault.Context{
				Resource:   "map_palette_id",
				ResourceID: new(int(data[address+6])),
				Range: &fault.Range{
					Offset: paletteAt,
					Length: 512,
					Size:   len(data),
				},
			},
			nil,
		)
	}
	config, err := mapCompressed(
		data,
		f.config,
		data[address+7],
		"map_tileset_id",
		0x2400,
	)
	if err != nil {
		return mapResources{}, err
	}
	layout, err := mapCompressed(
		data,
		f.layout,
		data[address+8],
		chapterMapField,
		maxMapBytes,
	)
	if err != nil {
		return mapResources{}, err
	}
	return mapResources{
		graphics: graphics,
		palette:  data[paletteAt : paletteAt+512],
		config:   config,
		layout:   layout,
		objectID: int(data[address+4]),
		configID: int(data[address+7]),
		layoutID: int(data[address+8]),
	}, nil
}

func (r mapResources) preview(layout []byte) (MapPreview, error) {
	if len(layout) < 2 {
		return MapPreview{}, fault.New(
			ErrMap,
			"game.map_dimensions",
			fault.Context{
				Resource:   chapterMapField,
				ResourceID: new(r.layoutID),
			},
			nil,
		)
	}
	width, height := int(layout[0]), int(layout[1])
	if width < 1 || height < 1 || width > MaxMapWidth ||
		height > MaxMapHeight ||
		width*height > MaxMapCells ||
		len(layout) < 2+width*height*2 {
		return MapPreview{}, fault.New(
			ErrMap,
			"game.map_dimensions",
			fault.Context{
				Resource:   chapterMapField,
				ResourceID: new(r.layoutID),
				Range: &fault.Range{
					Length: 2 + width*height*2,
					Size:   len(layout),
				},
			},
			nil,
		)
	}
	result := MapPreview{
		Width:  width,
		Height: height,
		Cells:  make([]MapCell, width*height),
	}
	tiles := make([]uint16, width*height*4)
	for i := range result.Cells {
		tile := u16(layout, 2+i*2)
		at, terrain := int(tile)*2, 0x2000+int(tile>>2)
		if at+8 > min(len(r.config), 0x2000) || terrain >= len(r.config) {
			return MapPreview{}, fault.New(
				ErrMap,
				"game.map_tileset",
				fault.Context{
					Resource:   "map_tileset_id",
					ResourceID: new(r.configID),
					Range: &fault.Range{
						Offset: at,
						Length: 8,
						Size:   len(r.config),
					},
				},
				nil,
			)
		}
		result.Cells[i] = MapCell{Tile: tile, Terrain: r.config[terrain]}
		for part := range 4 {
			x, y := (i%width)*2+part%2, (i/width)*2+part/2
			tiles[y*width*2+x] = u16(r.config, at+part*2)
		}
	}
	var err error
	result.Image, err = gba.Background4BPP(
		r.graphics,
		r.palette,
		tiles,
		width*2,
		height*2,
	)
	if err != nil {
		return MapPreview{}, fault.New(
			ErrMap,
			"game.render_map",
			fault.Context{
				Resource:   mapGraphicsField,
				ResourceID: new(r.objectID),
			},
			err,
		)
	}
	return result, nil
}
