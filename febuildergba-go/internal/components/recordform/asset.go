package recordform

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
)

// AssetActions delegates native file dialogs to the owning window.
// Import completes on the UI thread; bytes are planned before any ROM write.
type AssetActions struct {
	Import func(func([]byte, error))
	Export func(game.AssetRef)
}

func (a *Form) assetEditor(c *ui.Context, preview *assetPreview) {
	if preview == nil || preview.bitmap == nil {
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, locales.ExportPng()).Disabled(a.state.Busy || a.assets.Export == nil).OnClick(func() { a.assets.Export(preview.ref) })
		ui.Button(c, locales.ReplacePng()).Disabled(a.state.Busy || a.assets.Import == nil).OnClick(func() { a.importAsset(preview) })
	})
	if preview.edit == nil {
		return
	}
	ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
		bounds := preview.edit.Bounds()
		scale := float32(2)
		if preview.ref.Kind == game.ItemIconAsset {
			scale = 4
		}
		ui.Image(c, preview.imported).Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).Label(locales.ImportedImage())
		ui.Column(c).Gap(6).Grow(1).Children(func() {
			ui.Text(c, locales.ImportedImage()).Bold()
			if preview.ref.Kind == game.PortraitAsset {
				ui.Text(c, locales.PortraitColors()).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			a.assetUses(c, preview.edit)
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, locales.ApplyImage()).Disabled(a.state.Busy).OnClick(func() {
					if err := a.state.Session.ApplyAssetEdit(preview.edit); err != nil {
						a.state.Err = err
						return
					}
					a.state.Err = nil
					preview.edit, preview.imported = nil, nil
				})
				ui.Button(c, locales.Cancel()).Disabled(a.state.Busy).OnClick(func() { preview.edit, preview.imported = nil, nil })
			})
		})
	})
}

func (a *Form) importAsset(preview *assetPreview) {
	session, ref, revision := a.state.Session, preview.ref, preview.revision
	a.assets.Import(func(content []byte, err error) {
		if err != nil {
			a.state.Err = err
			return
		}
		if len(content) == 0 {
			return
		}
		if a.state.Session != session || session.Document.State().Revision != revision {
			a.state.Err = rom.ErrStale
			return
		}
		if a.preview.ref != ref || !a.preview.loaded {
			return
		}
		edit, err := session.PlanAssetEdit(ref, content)
		if errors.Is(err, game.ErrValue) {
			a.state.Err = nil
			preview.edit, preview.imported = nil, nil
			return
		}
		if err != nil {
			a.state.Err = err
			return
		}
		a.state.Err = nil
		preview.edit = edit
		preview.imported = ui.NewBitmap(edit.Image())
		preview.imported.Pixelated()
	})
}

func (a *Form) assetUses(c *ui.Context, edit *game.AssetEdit) {
	uses := edit.Uses()
	ui.Text(c, locales.AssetEditSummary(len(uses))).FontSize(12)
	for _, kind := range game.Kinds {
		var fields []string
		for _, use := range uses {
			if use.Kind == kind {
				fields = append(fields, fmt.Sprintf("%02X (%s)", use.ID, locales.Label(use.Field)))
			}
		}
		if len(fields) > 0 {
			ui.Text(c, locales.Label(string(kind))+": "+strings.Join(fields, ", ")).FontSize(12)
		}
	}
}
