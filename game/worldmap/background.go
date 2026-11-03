package worldmap

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const MaxBackgroundPNG = 1 << 20

// BackgroundTone selects the independent daytime or dark palette. Both use
// the same pixel indices and palette map.
type BackgroundTone int

const (
	Daytime BackgroundTone = iota
	Darkened
)

func (f SceneFormat) paletteSite(tone BackgroundTone) int {
	if tone == Darkened {
		return f.DarkPalette
	}
	if tone == Daytime {
		return f.Palette
	}
	return 0
}

var (
	ErrDarkBackgroundPixels = fault.Code{
		ID:   "game.worldmap_dark_background_pixels",
		Kind: fault.InvalidInput,
	}
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

func (f SceneFormat) readBackground(
	data []byte,
	tone BackgroundTone,
) (backgroundData, error) {
	pixels, _, err := fixedAsset(data, f.Image, MapWidth*MapHeight/2)
	if err != nil {
		return backgroundData{}, err
	}
	palette, _, err := fixedAsset(data, f.paletteSite(tone), 128)
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
	palette := nativePalette(b.palette, false)
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

// Background is an immutable copy of the selected native background resources. PNG
// encoding and import use only this copy and may run away from the UI thread.
type Background struct {
	stamp        edit.Stamp
	original     backgroundData
	format       SceneFormat
	paletteSites []int
	tone         BackgroundTone
}

func (e *Editor) Background(tone BackgroundTone) (*Background, error) {
	image := e.context.Capture()
	if err := e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	for _, site := range []int{e.format.Scene.Image, e.format.Scene.paletteSite(tone), e.format.Scene.PaletteMap} {
		if site < 0xC0 || site%4 != 0 || seen[site] {
			return nil, ErrScene
		}
		seen[site] = true
	}
	original, err := e.format.Scene.readBackground(image.Data, tone)
	if err != nil {
		return nil, err
	}
	palette, _ := gba.Pointer(image.Data, e.format.Scene.paletteSite(tone))
	sites := []int{e.format.Scene.paletteSite(tone)}
	var consumers []int
	if tone == Daytime {
		consumers = []int{
			e.format.Scene.PaletteReload,
			e.format.Scene.PaletteFade,
		}
	}
	for _, site := range consumers {
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
		tone:         tone,
		paletteSites: sites,
		stamp:        image.Stamp(),
		original:     original,
		format:       e.format.Scene,
	}, nil
}

func (b *Background) Current(
	e *Editor,
) bool {
	return b != nil && b.stamp.Current(e.context)
}

func (b *Background) Tone() BackgroundTone { return b.tone }

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
	if err := putPalette(after.palette, indexed.Palette, false); err != nil {
		return nil, fault.New(
			ErrBackgroundColors,
			"worldmap.background_palette",
			fault.Context{},
			err,
		)
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
	if b.tone == Darkened &&
		(!bytes.Equal(after.pixels, b.original.pixels) || !bytes.Equal(after.banks, b.original.banks)) {
		return nil, ErrDarkBackgroundPixels
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
	commit   *edit.Commit
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
	image := e.context.Capture()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	builder := space.NewBuilder(image.Data, space.Protection{
		Pointer: func(data []byte, owner space.Owner, site int) error {
			if err := e.source.ProtectPointer(data, owner, site); err != nil {
				return fault.New(
					ErrReference,
					"worldmap.protect_background",
					fault.Context{},
					err,
				)
			}
			return nil
		},
	})
	for _, part := range []struct {
		site          int
		before, after []byte
		compressed    bool
	}{
		{e.format.Scene.Image, d.source.original.pixels, d.after.pixels, false},
		{e.format.Scene.paletteSite(d.source.tone), d.source.original.palette, d.after.palette, false},
		{e.format.Scene.PaletteMap, d.source.original.banks, d.after.banks, true},
	} {
		if bytes.Equal(part.before, part.after) {
			continue
		}
		sites := []int{part.site}
		if part.site == e.format.Scene.paletteSite(d.source.tone) {
			sites = d.source.paletteSites
		}
		pointers := make([]space.PointerSite, len(sites))
		for i, site := range sites {
			pointers[i] = space.PointerSite{
				Owner:   space.Owner{Resource: "worldmap.scene", ID: site},
				Address: site, ExpectedWord: gba.Uint32(image.Data, site),
			}
		}
		payload := part.after
		if part.compressed {
			payload, err = gba.CompressLZ77(payload)
			if err != nil {
				return nil, err
			}
		}
		if _, err = builder.Replace(payload, pointers...); err != nil {
			return nil, err
		}
	}
	allocation := builder.Plan()
	return &BackgroundPlan{
		commit: edit.NewCommit(
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
