package recordform

import (
	"github.com/woxQAQ/febuildergba-mygo/internal/game"

	"github.com/egoist/mygo/ui"
)

type assetPreview struct {
	loaded   bool
	revision uint64
	ref      game.AssetRef
	resource game.Preview
	bitmap   *ui.Bitmap
	err      error
	edit     *game.AssetEdit
	imported *ui.Bitmap
}

func (a *Form) previewFor(kind game.Kind, id int) *assetPreview {
	table := a.state.Session.Table(kind)
	if table == nil {
		return nil
	}
	for _, field := range table.Fields {
		if field.Asset == "" {
			continue
		}
		value, err := a.state.Session.Value(kind, id, field.ID)
		if err != nil {
			return nil
		}
		ref := game.AssetRef{Kind: field.Asset, ID: int(value)}
		revision := a.state.Session.Document.State().Revision
		p := &a.preview
		if !p.loaded || p.revision != revision || p.ref != ref {
			*p = assetPreview{loaded: true, revision: revision, ref: ref}
			p.resource, p.err = a.state.Session.Preview(ref)
			if p.resource.Image != nil {
				p.bitmap = ui.NewBitmap(p.resource.Image)
				p.bitmap.Pixelated()
			}
		}
		return p
	}
	return nil
}
