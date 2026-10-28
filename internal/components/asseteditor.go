package components

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// AssetActions delegates native file dialogs to the owning window. Import
// publishes bytes on the UI thread; the editor previews before applying them.
type AssetActions struct {
	Import func(func([]byte, error))
	Export func(game.AssetRef)
}

// AssetEditor shares the PNG preview/apply workflow between resource pages.
// State contains only the bound source and persistent resource/widget data.
type AssetEditor struct {
	session  *game.Session
	ref      game.AssetRef
	revision uint64
	edit     *game.AssetEdit
	imported *ui.Bitmap
}

func (editor *AssetEditor) Render(
	c *ui.Context,
	state *State,
	ref game.AssetRef,
	actions AssetActions,
) {
	editor.RenderButtons(c, state, ref, actions)
	editor.RenderPreview(c, state)
}

func (editor *AssetEditor) RenderButtons(
	c *ui.Context,
	state *State,
	ref game.AssetRef,
	actions AssetActions,
) {
	revision := state.Session.Document.State().Revision
	if editor.session != state.Session || editor.ref != ref ||
		editor.revision != revision {
		*editor = AssetEditor{
			session:  state.Session,
			ref:      ref,
			revision: revision,
		}
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, locales.ExportPng()).
			Disabled(state.Busy || actions.Export == nil).
			OnClick(func() { actions.Export(ref) })
		ui.Button(c, locales.ReplacePng()).
			Disabled(state.Busy || actions.Import == nil).
			OnClick(func() { editor.importPNG(state, actions) })
	})
}

func (editor *AssetEditor) RenderPreview(c *ui.Context, state *State) {
	if editor.edit == nil {
		return
	}
	ref := editor.ref
	ui.Row(c).Wrap().Gap(12).AlignItems(ui.Start).Children(func() {
		bounds := editor.edit.Bounds()
		scale := float32(2)
		if ref.Kind == game.ItemIconAsset {
			scale = 4
		}
		ui.Image(c, editor.imported).
			Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).
			Label(locales.ImportedImage())
		ui.Column(c).Gap(6).Grow(1).Children(func() {
			ui.Text(c, locales.ImportedImage()).Bold()
			if ref.Kind == game.PortraitAsset {
				ui.Text(c, locales.PortraitColors()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
			assetUses(c, editor.edit)
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, locales.ApplyImage()).
					Disabled(state.Busy).
					OnClick(func() {
						if err := state.Session.ApplyAssetEdit(
							editor.edit,
						); err != nil {
							state.Err = err
							return
						}
						state.Err = nil
						editor.edit, editor.imported = nil, nil
					})
				ui.Button(c, locales.Cancel()).
					Disabled(state.Busy).
					OnClick(func() { editor.edit, editor.imported = nil, nil })
			})
		})
	})
}

func (editor *AssetEditor) importPNG(state *State, actions AssetActions) {
	session, ref, revision := editor.session, editor.ref, editor.revision
	actions.Import(func(content []byte, err error) {
		if err != nil {
			state.Err = err
			return
		}
		if len(content) == 0 {
			return
		}
		if state.Session != session ||
			session.Document.State().Revision != revision {
			state.Err = rom.ErrStale
			return
		}
		if editor.session != session || editor.ref != ref {
			return
		}
		edit, err := session.PlanAssetEdit(ref, content)
		if errors.Is(err, game.ErrValue) {
			state.Err = nil
			editor.edit, editor.imported = nil, nil
			return
		}
		if err != nil {
			state.Err = err
			return
		}
		state.Err = nil
		editor.edit = edit
		editor.imported = ui.NewBitmap(edit.Image())
		editor.imported.Pixelated()
	})
}

func assetUses(c *ui.Context, edit *game.AssetEdit) {
	uses := edit.Uses()
	ui.Text(c, locales.AssetEditSummary(len(uses))).FontSize(12)
	for _, kind := range game.Kinds {
		var fields []string
		for _, use := range uses {
			if use.Kind == kind {
				fields = append(
					fields,
					fmt.Sprintf("%02X (%s)", use.ID, locales.Label(use.Field)),
				)
			}
		}
		if len(fields) > 0 {
			ui.Text(c, locales.Label(string(kind))+": "+strings.Join(fields, ", ")).
				FontSize(12)
		}
	}
}
