package game

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type AssetKind string

const (
	PortraitAsset AssetKind = "portrait"
	ItemIconAsset AssetKind = "item_icon"
)

type AssetRef struct {
	Kind AssetKind
	ID   int
}

type AssetRegion struct {
	Label           string
	Address, Length int
}

type Preview struct {
	Image   *image.RGBA
	Regions []AssetRegion
}

// Preview reads the same document snapshot as the fields that reference the
// asset.
func (s *Session) Preview(ref AssetRef) (result Preview, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.preview_asset",
			fault.Context{Resource: string(ref.Kind), ResourceID: new(ref.ID)},
		)
	}()
	if !s.Adapter.Match(s.image.Data) {
		return Preview{}, ErrVersion
	}
	return s.Adapter.Preview(s.image.Data, ref)
}

type iconFormat struct {
	pointer, palette  int
	original, maximum int
}

func (f iconFormat) read(data []byte, id int) (Preview, error) {
	base, ok := pointer(data, f.pointer)
	if !ok || id < 0 || id >= 0xFF || (base == f.original && id > f.maximum) {
		return Preview{}, ErrImage
	}
	palette, ok := pointer(data, f.palette)
	if !ok || !within(data, palette, 32) || !within(data, base+id*128, 128) {
		return Preview{}, ErrImage
	}
	at := base + id*128
	bitmap, err := gba.Tiles4BPP(
		data[at:at+128],
		data[palette:palette+32],
		16,
		16,
	)
	if err != nil {
		return Preview{}, fault.New(
			ErrImage,
			"game.render_icon",
			fault.Context{
				Range: &fault.Range{Offset: at, Length: 128, Size: len(data)},
			},
			err,
		)
	}
	return Preview{Image: bitmap, Regions: []AssetRegion{
		{Label: "image_tiles", Address: at, Length: 128},
		{Label: "image_palette", Address: palette, Length: 32},
	}}, nil
}
