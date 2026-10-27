package game

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/internal/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/internal/space"
)

var (
	ErrAssetFormat = errors.New("game.unsupported_asset_format")
	ErrAssetSize   = errors.New("game.invalid_asset_size")
	ErrAssetColors = errors.New("game.invalid_asset_colors")
	ErrAssetPixels = errors.New("game.unmapped_asset_pixels")
)

// MaxAssetPNG bounds both file reads and decoding; sheets are at most 160x160.
const MaxAssetPNG = 1 << 20

type AssetUse struct {
	Kind  Kind
	ID    int
	Field string
}

// AssetEdit binds a complete, immutable resource replacement to one document state.
// Other asset IDs retain their existing pointers, even when their old data aliases.
type AssetEdit struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	ref      AssetRef
	uses     []AssetUse
	plan     space.Plan
	image    *image.RGBA
}

func (e *AssetEdit) Ref() AssetRef           { return e.ref }
func (e *AssetEdit) Revision() uint64        { return e.revision }
func (e *AssetEdit) Uses() []AssetUse        { return slices.Clone(e.uses) }
func (e *AssetEdit) Bounds() image.Rectangle { return e.image.Bounds() }
func (e *AssetEdit) Image() *image.RGBA {
	out := image.NewRGBA(e.image.Bounds())
	copy(out.Pix, e.image.Pix)
	return out
}

func (s *Session) AssetPNG(ref AssetRef) ([]byte, error) {
	s.Refresh()
	bitmap, err := s.Adapter.AssetImage(s.image.Data, ref)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, bitmap); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (s *Session) PlanAssetEdit(ref AssetRef, content []byte) (*AssetEdit, error) {
	s.Refresh()
	if len(content) == 0 || len(content) > MaxAssetPNG {
		return nil, ErrAssetSize
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || config.Width < 8 || config.Height < 8 || config.Width > 160 || config.Height > 160 {
		return nil, ErrAssetSize
	}
	bitmap, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, ErrImage
	}
	plan, err := s.Adapter.PlanAssetEdit(s.image.Data, ref, bitmap)
	if err != nil {
		return nil, err
	}
	edit := &AssetEdit{document: s.Document, revision: s.image.Revision, layouts: maps.Clone(s.layouts), ref: ref, plan: plan}
	for _, table := range s.Tables {
		if table.Err != nil {
			continue
		}
		for _, change := range plan.Edit.Changes {
			if change.Offset < table.Address+table.Stride*table.Count && table.Address < change.Offset+len(change.Data) {
				return nil, ErrLayout
			}
		}
		for _, row := range table.Rows {
			for _, field := range table.Fields {
				if field.Asset == ref.Kind && field.Value(s.image.Data, row.Address) == int64(ref.ID) {
					edit.uses = append(edit.uses, AssetUse{Kind: table.Kind, ID: row.ID, Field: field.ID})
				}
			}
		}
	}
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, s.image.Data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	edit.image, err = s.Adapter.AssetImage(prospective, ref)
	if err != nil {
		return nil, err
	}
	expected := image.NewRGBA(bitmap.Bounds())
	if ref.Kind == PortraitAsset {
		bitmap = gba.Quantise4BPP(bitmap)
	}
	draw.Draw(expected, expected.Bounds(), bitmap, image.Point{}, draw.Src)
	if !bytes.Equal(expected.Pix, edit.image.Pix) {
		return nil, ErrImage
	}
	return edit, nil
}

func (s *Session) ApplyAssetEdit(edit *AssetEdit) error {
	if edit == nil || edit.document != s.Document || !maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(edit.revision, "asset."+string(edit.ref.Kind), edit.plan.Edit); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (g gameFormat) AssetImage(data []byte, ref AssetRef) (*image.RGBA, error) {
	if !g.Match(data) {
		return nil, ErrVersion
	}
	switch ref.Kind {
	case ItemIconAsset:
		preview, err := g.icons.read(data, ref.ID)
		return preview.Image, err
	case PortraitAsset:
		sheet, err := g.portraits.sheet(data, ref.ID)
		return sheet.image, err
	default:
		return nil, ErrImage
	}
}

func (g gameFormat) PlanAssetEdit(data []byte, ref AssetRef, bitmap image.Image) (space.Plan, error) {
	original, err := g.AssetImage(data, ref)
	if err != nil {
		return space.Plan{}, err
	}
	if bitmap.Bounds() != original.Bounds() {
		return space.Plan{}, ErrAssetSize
	}
	normalised := image.NewRGBA(bitmap.Bounds())
	draw.Draw(normalised, normalised.Bounds(), bitmap, bitmap.Bounds().Min, draw.Src)
	if bytes.Equal(normalised.Pix, original.Pix) {
		return space.Plan{}, ErrValue
	}
	var resources []assetResource
	switch ref.Kind {
	case ItemIconAsset:
		resources, err = g.icons.resources(data, ref.ID, bitmap)
	case PortraitAsset:
		var sheet assetSheet
		sheet, err = g.portraits.sheet(data, ref.ID)
		if err == nil {
			resources, err = sheet.encode(bitmap)
		}
	default:
		return space.Plan{}, ErrImage
	}
	if err != nil {
		return space.Plan{}, err
	}
	if !g.validAssetTargets(data, ref, resources) {
		return space.Plan{}, ErrLayout
	}
	return appendAssetResources(len(data), resources)
}

func (f iconFormat) resources(data []byte, id int, bitmap image.Image) ([]assetResource, error) {
	preview, err := f.read(data, id)
	if err != nil {
		return nil, err
	}
	palette := preview.Regions[1].Address
	tiles, err := gba.EncodeTiles4BPP(bitmap, data[palette:palette+32])
	if err != nil {
		return nil, ErrAssetColors
	}
	base, ok := pointer(data, f.pointer)
	// The byte-sized ID reserves FF. Preserve the full addressable window so
	// a relocated table retains all IDs, including those used by patched games.
	const capacity = 0xFF * 128
	if !ok || base < 0xC0 || base%4 != 0 || !within(data, base, capacity) {
		return nil, ErrImage
	}
	body := bytes.Clone(data[base : base+capacity])
	copy(body[id*128:], tiles)
	return []assetResource{{pointer: f.pointer, data: body}}, nil
}

func (g gameFormat) validAssetTargets(data []byte, ref AssetRef, resources []assetResource) bool {
	protected := []int{g.text.pointer, g.text.rootPointer, g.text.treePointer, g.portraits.pointer, g.icons.palette}
	if ref.Kind != ItemIconAsset {
		protected = append(protected, g.icons.pointer)
	}
	for _, rule := range g.tables {
		protected = append(protected, rule.pointers...)
	}
	seen := make(map[int]bool)
	for _, resource := range resources {
		at := resource.pointer
		if at < 0xC0 || at%4 != 0 || !within(data, at, 4) || seen[at] {
			return false
		}
		seen[at] = true
		for _, address := range protected {
			if at < address+4 && address < at+4 {
				return false
			}
		}
	}
	return true
}
