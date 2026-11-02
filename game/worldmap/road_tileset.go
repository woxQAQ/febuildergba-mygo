package worldmap

import (
	"bytes"
	"image"
	"image/png"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const MaxRoadTilesetPNG = 1 << 20

var (
	ErrRoadTilesetPNG = fault.Code{
		ID:   "game.worldmap_road_tileset_png",
		Kind: fault.InvalidInput,
	}
	ErrRoadTilesetSize = fault.Code{
		ID:   "game.worldmap_road_tileset_size",
		Kind: fault.InvalidInput,
	}
	ErrRoadTilesetColors = fault.Code{
		ID:   "game.worldmap_road_tileset_colors",
		Kind: fault.InvalidInput,
	}
	ErrRoadTilesetPadding = fault.Code{
		ID:   "game.worldmap_road_tileset_padding",
		Kind: fault.InvalidInput,
	}
)

type roadTilesData struct{ pixels, palette []byte }

func (f SceneFormat) readRoadTiles(data []byte) (roadTilesData, error) {
	pixels, _, _, err := compressedAsset(data, f.RoadTiles, 0x3000)
	if err != nil {
		return roadTilesData{}, err
	}
	if len(pixels) == 0 || len(pixels)%32 != 0 {
		return roadTilesData{}, ErrScene
	}
	palette, _, err := fixedAsset(data, f.RoadPalette, 32)
	if err != nil {
		return roadTilesData{}, err
	}
	return roadTilesData{pixels, bytes.Clone(palette)}, nil
}

func (r roadTilesData) tiles() ([]*image.RGBA, error) {
	tiles := make([]*image.RGBA, len(r.pixels)/32)
	for i := range tiles {
		var err error
		tiles[i], err = gba.Tiles4BPP(r.pixels[i*32:i*32+32], r.palette, 8, 8)
		if err != nil {
			return nil, err
		}
	}
	return tiles, nil
}

func (r roadTilesData) indexed() *image.Paletted {
	count := len(r.pixels) / 32
	out := image.NewPaletted(
		image.Rect(0, 0, 128, ((count+15)/16)*8),
		nativePalette(r.palette, true),
	)
	for i := range count {
		for y := range 8 {
			for x := range 8 {
				index := r.pixels[i*32+y*4+x/2] >> ((x % 2) * 4) & 15
				out.SetColorIndex(i%16*8+x, i/16*8+y, index)
			}
		}
	}
	return out
}

// RoadTileset is an immutable snapshot of the shared native road graphics.
// Its PNG sheet has sixteen columns and preserves every existing tile ID.
type RoadTileset struct {
	stamp     edit.Stamp
	format    SceneFormat
	original  roadTilesData
	tileSites []int
}

func (e *Editor) RoadTileset() (*RoadTileset, error) {
	snapshot := e.context.Document().Snapshot()
	if err := e.context.Validate(snapshot.Data); err != nil {
		return nil, err
	}
	f := e.format.Scene
	if f.RoadTiles < 0xC0 || f.RoadPalette < 0xC0 || f.RoadTiles%4 != 0 ||
		f.RoadPalette%4 != 0 ||
		f.RoadTiles == f.RoadPalette {
		return nil, ErrScene
	}
	original, err := f.readRoadTiles(snapshot.Data)
	if err != nil {
		return nil, err
	}
	sites := []int{f.RoadTiles}
	if f.RoadTilesReload != 0 {
		if f.RoadTilesReload < 0xC0 || f.RoadTilesReload%4 != 0 ||
			f.RoadTilesReload == f.RoadTiles ||
			f.RoadTilesReload == f.RoadPalette {
			return nil, ErrScene
		}
		primary, _ := gba.Pointer(snapshot.Data, f.RoadTiles)
		if address, ok := gba.Pointer(
			snapshot.Data,
			f.RoadTilesReload,
		); ok &&
			address == primary {
			sites = append(sites, f.RoadTilesReload)
		}
	}
	return &RoadTileset{
		stamp:     e.context.Bind(snapshot.Revision),
		format:    f,
		original:  original,
		tileSites: sites,
	}, nil
}

func (r *RoadTileset) Current(
	e *Editor,
) bool {
	return r != nil && r.stamp.Current(e.context)
}

func (r *RoadTileset) TileCount() int { return len(r.original.pixels) / 32 }
func (r *RoadTileset) PNG() ([]byte, error) {
	var out bytes.Buffer
	if err := png.Encode(&out, r.original.indexed()); err != nil {
		return nil, fault.New(
			ErrRoadTilesetPNG,
			"worldmap.export_road_tileset",
			fault.Context{},
			err,
		)
	}
	return out.Bytes(), nil
}

type RoadTilesetDraft struct {
	source *RoadTileset
	after  roadTilesData
	tiles  []*image.RGBA
	image  *image.RGBA
}

func (r *RoadTileset) ImportPNG(content []byte) (*RoadTilesetDraft, error) {
	if len(content) == 0 || len(content) > MaxRoadTilesetPNG {
		return nil, ErrRoadTilesetSize
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrRoadTilesetPNG,
			"worldmap.road_png_header",
			fault.Context{},
			err,
		)
	}
	height := ((r.TileCount() + 15) / 16) * 8
	if config.Width != 128 || config.Height != height {
		return nil, ErrRoadTilesetSize
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrRoadTilesetPNG,
			"worldmap.decode_road_png",
			fault.Context{},
			err,
		)
	}
	indexed, ok := decoded.(*image.Paletted)
	if !ok {
		return nil, ErrRoadTilesetPNG
	}
	after := roadTilesData{
		make([]byte, len(r.original.pixels)),
		bytes.Clone(r.original.palette),
	}
	if err = putPalette(after.palette, indexed.Palette, true); err != nil {
		return nil, fault.New(
			ErrRoadTilesetColors,
			"worldmap.road_palette",
			fault.Context{},
			err,
		)
	}
	for y := range height {
		for x := range 128 {
			index := indexed.ColorIndexAt(x, y)
			if index >= 16 {
				return nil, ErrRoadTilesetColors
			}
			tile := y/8*16 + x/8
			if tile >= r.TileCount() {
				if index != 0 {
					return nil, ErrRoadTilesetPadding
				}
				continue
			}
			at := tile*32 + y%8*4 + x%8/2
			after.pixels[at] |= index << ((x % 2) * 4)
		}
	}
	if bytes.Equal(after.pixels, r.original.pixels) &&
		bytes.Equal(after.palette, r.original.palette) {
		return nil, edit.ErrValue
	}
	tiles, err := after.tiles()
	if err != nil {
		return nil, err
	}
	return &RoadTilesetDraft{
		source: r,
		after:  after,
		tiles:  tiles,
		image:  roadTilesImage(tiles),
	}, nil
}

func (d *RoadTilesetDraft) Current(
	e *Editor,
) bool {
	return d != nil && d.source.Current(e)
}

func (d *RoadTilesetDraft) Image() *image.RGBA {
	out := image.NewRGBA(d.image.Bounds())
	copy(out.Pix, d.image.Pix)
	return out
}

func (d *RoadTilesetDraft) Brush(
	cell RoadCell,
) (*image.RGBA, error) {
	return roadBrush(d.tiles, cell)
}

// Preview renders the current route draft, including its pending road edits,
// against the replacement graphics without changing either draft.
func (d *RoadTilesetDraft) Preview(route *RouteDraft) (*image.RGBA, error) {
	if route == nil || route.stamp != d.source.stamp {
		return nil, rom.ErrStale
	}
	road, err := route.Road()
	if err != nil {
		return nil, err
	}
	return roadImage(road.cells, d.tiles), nil
}

type RoadTilesetPlan struct {
	commit   *edit.Plan
	appended space.Range
}

func (p *RoadTilesetPlan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}
func (p *RoadTilesetPlan) Appended() space.Range { return p.appended }

func (e *Editor) PlanRoadTileset(
	d *RoadTilesetDraft,
) (plan *RoadTilesetPlan, err error) {
	defer func() { err = fault.Wrap(err, "worldmap.plan_road_tileset", fault.Context{}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.source.stamp.Check(e.context); err != nil {
		return nil, err
	}
	if e.source.ProtectPointer == nil || d.source.format != e.format.Scene {
		return nil, edit.ErrLayout
	}
	snapshot := e.context.Document().Snapshot()
	if err = e.context.Validate(snapshot.Data); err != nil {
		return nil, err
	}
	appender := space.NewAppender(len(snapshot.Data))
	for _, part := range []struct {
		site          int
		before, after []byte
		compressed    bool
	}{
		{e.format.Scene.RoadTiles, d.source.original.pixels, d.after.pixels, true},
		{e.format.Scene.RoadPalette, d.source.original.palette, d.after.palette, false},
	} {
		if bytes.Equal(part.before, part.after) {
			continue
		}
		sites := []int{part.site}
		if part.site == e.format.Scene.RoadTiles {
			sites = d.source.tileSites
		}
		for _, site := range sites {
			if err = e.source.ProtectPointer(
				snapshot.Data,
				space.Owner{Resource: "worldmap.scene", ID: site},
				site,
			); err != nil {
				return nil, fault.New(
					ErrReference,
					"worldmap.protect_road_tileset",
					fault.Context{},
					err,
				)
			}
		}
		payload := part.after
		if part.compressed {
			payload, err = gba.CompressLZ77(payload)
			if err != nil {
				return nil, err
			}
		}
		if _, err = appender.Add(payload, sites...); err != nil {
			return nil, err
		}
	}
	allocation := appender.Plan()
	return &RoadTilesetPlan{
		commit: edit.NewPlan(
			d.source.stamp,
			"worldmap.road_tileset",
			allocation.Edit,
		),
		appended: allocation.Destination,
	}, nil
}

func (e *Editor) ApplyRoadTileset(plan *RoadTilesetPlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"worldmap.apply_road_tileset",
		fault.Context{},
	)
}
