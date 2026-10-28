package recordform

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func (a *Form) profilePreview(kind game.Kind, id int) *assetPreview {
	revision := a.state.Session.Document.State().Revision
	p := &a.preview
	if p.loaded && p.revision == revision && p.kind == kind && p.id == id &&
		p.source.Current(a.state.Session) {
		return p
	}
	*p = assetPreview{loaded: true, revision: revision, kind: kind, id: id}
	source, err := a.state.Session.RecordPortrait(kind, id)
	p.source, p.ref, p.resource, p.err = &source, source.Ref, source.Main, err
	if err == nil {
		p.err = source.MainErr
	}
	if source.Main.Image != nil {
		p.bitmap = ui.NewBitmap(source.Main.Image)
		p.bitmap.Pixelated()
	}
	if source.Mini.Image != nil {
		p.mini = ui.NewBitmap(source.Mini.Image)
		p.mini.Pixelated()
	}
	return p
}

type assetPreview struct {
	loaded   bool
	revision uint64
	ref      game.AssetRef
	resource game.Preview
	bitmap   *ui.Bitmap
	err      error
	source   *game.RecordPortrait
	mini     *ui.Bitmap
	kind     game.Kind
	id       int
}

func (a *Form) previewFor(kind game.Kind, id int) *assetPreview {
	if kind == game.Units || kind == game.Classes {
		return a.profilePreview(kind, id)
	}
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
