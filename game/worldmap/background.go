package worldmap

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const MaxBackgroundPNG = 1 << 20

var (
	ErrBackgroundPNG = fault.Code{
		ID:   "game.worldmap_background_png",
		Kind: fault.InvalidInput,
	}
	ErrBackgroundSize = fault.Code{
		ID:   "game.worldmap_background_size",
		Kind: fault.InvalidInput,
	}
	ErrBackgroundColors = fault.Code{
		ID:   "game.worldmap_background_colors",
		Kind: fault.InvalidInput,
	}
)

// backgroundData retains the unused palette high bits and row padding. All
// colors, including the first color of each bank, are opaque background colors.
type backgroundData struct {
	pixels, palette, banks []byte
}

func (f SceneFormat) readBackground(data []byte) (backgroundData, error) {
	pixels, _, err := fixedAsset(data, f.Image, MapWidth*MapHeight/2)
	if err != nil {
		return backgroundData{}, err
	}
	palette, _, err := fixedAsset(data, f.Palette, 128)
	if err != nil {
		return backgroundData{}, err
	}
	banks, _, _, err := compressedAsset(data, f.PaletteMap, 1280)
	if err != nil {
		return backgroundData{}, err
	}
	if len(banks) != 1280 {
		return backgroundData{}, ErrScene
	}
	for y := range 40 {
		for x := range 60 {
			if banks[y*32+x/2]>>((x%2)*4)&15 >= 4 {
				return backgroundData{}, ErrScene
			}
		}
	}
	return backgroundData{bytes.Clone(pixels), bytes.Clone(palette), banks}, nil
}

func (b backgroundData) indexed() *image.Paletted {
	palette := make(color.Palette, 64)
	for i := range palette {
		v := binary.LittleEndian.Uint16(b.palette[i*2:])
		expand := func(v uint16) uint8 { return uint8((v<<3 | v>>2) & 255) }
		palette[i] = color.RGBA{
			R: expand(v & 31),
			G: expand(v >> 5 & 31),
			B: expand(v >> 10 & 31),
			A: 255,
		}
	}
	out := image.NewPaletted(image.Rect(0, 0, MapWidth, MapHeight), palette)
	for y := range MapHeight {
		for x := range MapWidth {
			tileX, tileY := x/8, y/8
			bank := b.banks[tileY*32+tileX/2] >> ((tileX % 2) * 4) & 15
			at := (tileY*60+tileX)*32 + y%8*4 + x%8/2
			index := b.pixels[at] >> ((x % 2) * 4) & 15
			out.SetColorIndex(x, y, bank*16+index)
		}
	}
	return out
}

func (b backgroundData) image() *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, MapWidth, MapHeight))
	draw.Draw(out, out.Bounds(), b.indexed(), image.Point{}, draw.Src)
	return out
}

// Background is an immutable copy of the three native daytime resources. PNG
// encoding and import use only this copy and may run away from the UI thread.
type Background struct {
	stamp        edit.Stamp
	original     backgroundData
	format       SceneFormat
	paletteSites []int
}

func (e *Editor) Background() (*Background, error) {
	image := e.context.Document().Snapshot()
	if err := e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	for _, site := range []int{e.format.Scene.Image, e.format.Scene.Palette, e.format.Scene.PaletteMap} {
		if site < 0xC0 || site%4 != 0 || seen[site] {
			return nil, ErrScene
		}
		seen[site] = true
	}
	original, err := e.format.Scene.readBackground(image.Data)
	if err != nil {
		return nil, err
	}
	palette, _ := gba.Pointer(image.Data, e.format.Scene.Palette)
	sites := []int{e.format.Scene.Palette}
	for _, site := range []int{e.format.Scene.PaletteReload, e.format.Scene.PaletteFade} {
		if site == 0 {
			continue
		}
		if site < 0xC0 || site%4 != 0 || seen[site] {
			return nil, ErrScene
		}
		seen[site] = true
		if address, ok := gba.Pointer(
			image.Data,
			site,
		); ok &&
			address == palette {
			sites = append(sites, site)
		}
	}
	return &Background{
		paletteSites: sites,
		stamp:        e.context.Bind(image.Revision),
		original:     original,
		format:       e.format.Scene,
	}, nil
}

func (b *Background) Current(
	e *Editor,
) bool {
	return b != nil && b.stamp.Current(e.context)
}

func (b *Background) PNG() ([]byte, error) {
	var out bytes.Buffer
	if err := png.Encode(&out, b.original.indexed()); err != nil {
		return nil, fault.New(
			ErrBackgroundPNG,
			"worldmap.export_background",
			fault.Context{},
			err,
		)
	}
	return out.Bytes(), nil
}

// BackgroundDraft holds an imported image without changing the document. Bank
// selection is explicit in indexed PNGs: indices 0..15, 16..31, 32..47, 48..63.
// True-color images cannot retain that selection or duplicate palette entries.
type BackgroundDraft struct {
	source *Background
	after  backgroundData
	image  *image.RGBA
}

func (b *Background) ImportPNG(
	content []byte,
) (draft *BackgroundDraft, err error) {
	defer func() { err = fault.Wrap(err, "worldmap.import_background", fault.Context{}) }()
	if len(content) == 0 || len(content) > MaxBackgroundPNG {
		return nil, ErrBackgroundSize
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrBackgroundPNG,
			"worldmap.png_header",
			fault.Context{},
			err,
		)
	}
	if config.Width != MapWidth || config.Height != MapHeight {
		return nil, ErrBackgroundSize
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrBackgroundPNG,
			"worldmap.decode_png",
			fault.Context{},
			err,
		)
	}
	indexed, ok := decoded.(*image.Paletted)
	if !ok {
		return nil, ErrBackgroundPNG
	}
	after := backgroundData{
		make([]byte, len(b.original.pixels)),
		bytes.Clone(b.original.palette),
		bytes.Clone(b.original.banks),
	}
	for i := range min(64, len(indexed.Palette)) {
		pixel := color.NRGBAModel.Convert(indexed.Palette[i]).(color.NRGBA)
		if pixel.A != 255 {
			return nil, ErrBackgroundColors
		}
		quantize := func(v uint8) uint16 { return (uint16(v)*31 + 127) / 255 }
		value := quantize(
			pixel.R,
		) | quantize(
			pixel.G,
		)<<5 | quantize(
			pixel.B,
		)<<10
		value |= binary.LittleEndian.Uint16(after.palette[i*2:]) & 0x8000
		binary.LittleEndian.PutUint16(after.palette[i*2:], value)
	}
	for tileY := range 40 {
		for tileX := range 60 {
			bank := indexed.ColorIndexAt(tileX*8, tileY*8) / 16
			for y := range 8 {
				for x := range 8 {
					index := indexed.ColorIndexAt(tileX*8+x, tileY*8+y)
					if index >= 64 || index/16 != bank {
						return nil, fault.Wrap(
							ErrBackgroundColors,
							"worldmap.tile_palette",
							fault.Context{RecordID: new(tileY*60 + tileX)},
						)
					}
					at := (tileY*60+tileX)*32 + y*4 + x/2
					after.pixels[at] |= (index & 15) << ((x % 2) * 4)
				}
			}
			at, shift := tileY*32+tileX/2, (tileX%2)*4
			after.banks[at] = after.banks[at]&^(15<<shift) | bank<<shift
		}
	}
	if bytes.Equal(after.pixels, b.original.pixels) &&
		bytes.Equal(after.palette, b.original.palette) &&
		bytes.Equal(after.banks, b.original.banks) {
		return nil, edit.ErrValue
	}
	return &BackgroundDraft{source: b, after: after, image: after.image()}, nil
}

func (d *BackgroundDraft) Current(
	e *Editor,
) bool {
	return d != nil && d.source.Current(e)
}

// Image returns a copy so preview consumers cannot alter a later write.
func (d *BackgroundDraft) Image() *image.RGBA {
	out := image.NewRGBA(d.image.Bounds())
	copy(out.Pix, d.image.Pix)
	return out
}

type BackgroundPlan struct {
	commit   *edit.Plan
	appended space.Range
}

func (p *BackgroundPlan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}
func (p *BackgroundPlan) Appended() space.Range { return p.appended }

func (e *Editor) PlanBackground(
	d *BackgroundDraft,
) (plan *BackgroundPlan, err error) {
	defer func() { err = fault.Wrap(err, "worldmap.plan_background", fault.Context{}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.source.stamp.Check(e.context); err != nil {
		return nil, err
	}
	if e.source.ProtectPointer == nil || d.source.format != e.format.Scene {
		return nil, edit.ErrLayout
	}
	image := e.context.Document().Snapshot()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	appender := space.NewAppender(len(image.Data))
	for _, part := range []struct {
		site          int
		before, after []byte
		compressed    bool
	}{
		{e.format.Scene.Image, d.source.original.pixels, d.after.pixels, false},
		{e.format.Scene.Palette, d.source.original.palette, d.after.palette, false},
		{e.format.Scene.PaletteMap, d.source.original.banks, d.after.banks, true},
	} {
		if bytes.Equal(part.before, part.after) {
			continue
		}
		sites := []int{part.site}
		if part.site == e.format.Scene.Palette {
			sites = d.source.paletteSites
		}
		for _, site := range sites {
			owner := space.Owner{Resource: "worldmap.scene", ID: site}
			if err = e.source.ProtectPointer(
				image.Data,
				owner,
				site,
			); err != nil {
				return nil, fault.New(
					ErrReference,
					"worldmap.protect_background",
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
	return &BackgroundPlan{
		commit: edit.NewPlan(
			d.source.stamp,
			"worldmap.background",
			allocation.Edit,
		),
		appended: allocation.Destination,
	}, nil
}

func (e *Editor) ApplyBackground(plan *BackgroundPlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"worldmap.apply_background",
		fault.Context{},
	)
}
