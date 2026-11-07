package records

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func (a *form) profilePreview(kind records.Kind, id int) *assetPreview {
	revision := a.state.Session.Document.State().Revision
	p := &a.preview
	if p.loaded && p.revision == revision && p.kind == kind && p.id == id &&
		p.source.Current(a.state.Session.Portraits) {
		return p
	}
	*p = assetPreview{loaded: true, revision: revision, kind: kind, id: id}
	selection, err := a.state.Session.Records.Portrait(kind, id)
	if err != nil {
		p.err = err
		return p
	}
	source, err := a.state.Session.Portraits.Preview(selection)
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
	ref      graphics.Ref
	resource graphics.Preview
	bitmap   *ui.Bitmap
	err      error
	source   *portrait.RecordPreview
	icons    itemicon.Table
	mini     *ui.Bitmap
	kind     records.Kind
	id       int
}

func (a *form) previewFor(kind records.Kind, id int) *assetPreview {
	if kind == records.Units || kind == records.Classes {
		return a.profilePreview(kind, id)
	}
	if kind != records.Items {
		return nil
	}
	value, err := a.state.Session.Records.Value(records.Items, id, "icon_id")
	if err != nil {
		return nil
	}
	ref := graphics.Ref{Kind: graphics.ItemIcon, ID: int(value)}
	preview := &a.preview
	if preview.loaded && preview.ref == ref &&
		preview.icons.Current(a.state.Session.ItemIcons) {
		return preview
	}
	*preview = assetPreview{
		loaded: true,
		ref:    ref,
		icons:  a.state.Session.ItemIcons.Table(),
	}
	preview.resource, preview.err = a.state.Session.ItemIcons.Preview(ref.ID)
	if preview.resource.Image != nil {
		preview.bitmap = ui.NewBitmap(preview.resource.Image)
		preview.bitmap.Pixelated()
	}
	return preview
}
