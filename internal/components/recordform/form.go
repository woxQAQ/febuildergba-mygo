// Package recordform renders a record's fields, asset preview and active-field
// inspector.
package recordform

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/chapterview"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type Form struct {
	state                 *components.State
	drafts                map[fieldKey]*fieldDraft
	texts                 map[fieldKey]*textDraft
	activeField           fieldKey
	preview               assetPreview
	chapter               chapterview.View
	largePreview, showRaw bool
	sidebar               float32
	openHex               func(int, int)
	openText              func(uint16)
	jump                  func(game.Kind, int)
	assets                AssetActions
}

func New(
	state *components.State,
	openHex func(int, int),
	openText func(uint16),
	jump func(game.Kind, int),
	assets AssetActions,
) *Form {
	return &Form{
		state:    state,
		openHex:  openHex,
		openText: openText,
		jump:     jump,
		assets:   assets,
	}
}

func (a *Form) Reset() {
	a.ClearDrafts()
	a.preview = assetPreview{}
	a.chapter = chapterview.View{}
}

func (a *Form) ClearDrafts() {
	a.drafts = nil
	a.texts = nil
	a.activeField = fieldKey{}
}

func (a *Form) View(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
	sidebar float32,
) {
	a.sidebar = sidebar
	preview := a.previewFor(kind, row.ID)
	ui.Column(c.Key(fmt.Sprintf("%s-%d", kind, row.ID))).
		Fill().
		Children(func() {
			ui.Scroll(c.Key("record")).
				Grow(1).
				Padding(16).
				Gap(16).
				Children(func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						if preview != nil && preview.bitmap != nil {
							scale := float32(1)
							bounds := preview.resource.Image.Bounds()
							if a.largePreview {
								scale *= 2
							}
							ui.Image(c, preview.bitmap).
								Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).
								Label(row.Name + " " + locales.ImagePreview())
						}
						ui.Textf(c, "%02X", row.ID).
							FontSize(20).
							TextColor(c.Theme().TextMuted)
						if kind == game.Chapters {
							ui.Text(c, row.Name).FontSize(20).Grow(1)
						} else {
							a.nameEditor(c, kind, row).Grow(1).MinWidth(0)
						}
						if preview != nil && preview.bitmap != nil {
							ui.Checkbox(c, &a.largePreview, "2×").
								Label(locales.PreviewZoom())
						}
						ui.Checkbox(c, &a.showRaw, locales.Advanced())
					})
					a.assetEditor(c, preview)
					if preview != nil && preview.err != nil {
						ui.Text(c, components.ErrorText(preview.err)).
							TextColor(c.Theme().TextMuted)
					}
					if a.showRaw && preview != nil &&
						len(preview.resource.Regions) > 0 {
						ui.Row(c).Gap(8).Children(func() {
							for _, region := range preview.resource.Regions {
								ui.Button(c, fmt.Sprintf("%s · 0x%08X", locales.Label(region.Label), region.Address)).
									Disabled(a.state.Busy).
									OnClick(func() { a.openHex(region.Address, region.Length) })
							}
						})
					}
					if row.TextError != nil {
						ui.Text(c, locales.TextError()).
							TextColor(c.Theme().TextMuted)
					}
					if len(a.state.Session.Patches) > 0 {
						ui.Text(c, locales.Patches()+": "+strings.Join(a.state.Session.Patches, ", ")).
							TextColor(c.Theme().TextMuted)
					}
					if kind == game.Chapters {
						a.chapter.Render(
							c,
							a.state,
							row,
							a.showRaw,
							a.jump,
							a.openHex,
						)
					} else {
						a.descriptionEditors(c, kind, row, table)
					}
					if kind != game.Chapters || a.chapter.Configuration() {
						a.recordFields(c, kind, row, table)
					}
					if a.showRaw &&
						(kind != game.Chapters || a.chapter.Configuration()) {
						data, err := a.state.Session.Raw(
							row.Address,
							min(table.Stride, 256),
						)
						if err == nil {
							ui.Textf(c, "0x%08X\n% X", row.Address, data).
								Selectable()
							ui.Button(c, locales.EditBytes()).
								Disabled(a.state.Busy).
								OnClick(func() { a.openHex(row.Address, min(table.Stride, 256)) })
						}
					}
				})
			if kind != game.Chapters || a.chapter.Configuration() {
				a.fieldInspector(c, kind, row, table)
			}
		})
}
