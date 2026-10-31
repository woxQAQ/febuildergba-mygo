package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages/chapters"
)

type form struct {
	state                 *editor.State
	drafts                map[fieldKey]*fieldDraft
	texts                 map[fieldKey]*textDraft
	preview               assetPreview
	chapter               chapters.Page
	profiles              map[fieldKey]*profileState
	largePreview, showRaw bool
	sidebar               float32
	openHex               func(int, int)
	openText              func(uint16)
	openScript            func(int, int)
	openPortrait          func(int)
	openSupport           func(int)
	imageEditor           components.AssetEditor
	jump                  func(records.Kind, int)
	assets                components.AssetActions
}

func newForm(
	state *editor.State,
	openHex func(int, int),
	openText func(uint16),
	jump func(records.Kind, int),
	assets components.AssetActions,
) *form {
	return &form{
		state:    state,
		openHex:  openHex,
		openText: openText,
		jump:     jump,
		assets:   assets,
	}
}

func (a *form) SetPortraitOpener(open func(int)) { a.openPortrait = open }

func (a *form) SetScriptOpener(open func(int, int)) { a.openScript = open }

func (a *form) OpenMapChange(chapter, id int) {
	a.chapter.OpenMapChange(a.state, chapter, id)
}

func (a *form) OpenShop(chapter, location int) {
	a.chapter.OpenShop(a.state, chapter, location)
}

func (a *form) OpenDeployment(
	chapter, site int,
) {
	a.chapter.OpenDeployment(a.state, chapter, site)
}

func (a *form) Reset() {
	a.ClearDrafts()
	a.preview = assetPreview{}
	a.imageEditor = components.AssetEditor{}
	a.chapter = chapters.Page{}
}

func (a *form) ClearDrafts() {
	a.drafts = nil
	a.texts = nil
	a.profiles = nil
}

func (a *form) View(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	sidebar float32,
) {
	a.sidebar = sidebar
	if kind == records.Units || kind == records.Classes {
		a.profile(c, kind, row, table)
		return
	}
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
						if kind == records.Chapters {
							ui.Text(c, row.Name).
								FontSize(20).
								Grow(1).
								MinWidth(0).
								SingleLine()
						} else {
							a.nameEditor(c, kind, row).Grow(1).MinWidth(0)
						}
						if preview != nil && preview.bitmap != nil {
							ui.Checkbox(c, &a.largePreview, "2×").
								Label(locales.PreviewZoom())
						}
						if kind == records.Chapters && a.openScript != nil {
							ui.Button(c, locales.ChapterScripts()).
								Tooltip(locales.ScriptChapterOpen()).
								Disabled(a.state.Busy).
								OnClick(func() { a.openScript(row.ID, 0) })
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
					if len(a.state.Session.Records.Patches()) > 0 {
						ui.Text(c, locales.Patches()+": "+strings.Join(a.state.Session.Records.Patches(), ", ")).
							TextColor(c.Theme().TextMuted)
					}
					if kind == records.Chapters {
						width, _ := c.Size()
						a.chapter.Render(
							c,
							a.state,
							row,
							max(0, width-sidebar-40),
							a.showRaw,
							a.jump,
							a.openHex,
							a.openText,
							a.openScript,
						)
					} else {
						a.descriptionEditors(c, kind, row, table)
					}
					if kind != records.Chapters || a.chapter.Configuration() {
						a.recordFields(c, kind, row, table)
					}
					if a.showRaw &&
						(kind != records.Chapters || a.chapter.Configuration()) {
						data, err := a.state.Session.Bytes.Slice(
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
		})
}
