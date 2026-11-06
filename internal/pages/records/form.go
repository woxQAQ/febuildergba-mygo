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
	state           *editor.State
	drafts          map[fieldKey]*fieldDraft
	texts           map[fieldKey]*textDraft
	preview         assetPreview
	portraitChoice  *portraitChoice
	chapter         chapters.Page
	profiles        map[fieldKey]*profileState
	largePreview    bool
	sidebar         float32
	openHex         func(int, int)
	openText        func(uint16)
	openScript      func(int, int)
	openPortrait    func(int)
	openSupportTalk func(int, int)
	imageEditor     components.AssetEditor
	thumbnails      components.Thumbnails
	defeatView      func(*ui.Context, int, float32)
	jump            func(records.Kind, int)
	assets          components.AssetActions
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
	a.profiles = nil
	a.preview = assetPreview{}
	a.imageEditor = components.AssetEditor{}
	a.thumbnails = components.Thumbnails{}
	a.chapter.Reset()
}

func (a *form) ClearDrafts() {
	a.portraitChoice = nil
	a.drafts = nil
	a.texts = nil
	profiles := a.profiles
	a.profiles = nil
	// Layout changes invalidate resource drafts without discarding pending input.
	for key, entry := range profiles {
		if entry.terrain.draft != nil &&
			(entry.terrain.draft.Modified() || entry.terrain.invalid()) ||
			entry.support.draft != nil &&
				(entry.support.draft.Modified() || entry.support.invalid()) {
			retained := a.profileFor(key.Kind, key.Record)
			retained.section = entry.section
			retained.terrain = entry.terrain
			retained.support = entry.support
		}
	}
}

func (a *form) View(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	sidebar float32,
) {
	a.sidebar = sidebar
	if choice := a.portraitChoice; choice != nil &&
		(choice.kind != kind || choice.record != row.ID) {
		a.portraitChoice = nil
	}
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
							FontSize(components.FontTitle).
							TextColor(c.Theme().TextMuted)
						if kind == records.Chapters {
							ui.Text(c, row.Name).
								FontSize(components.FontTitle).
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
					})
					a.assetEditor(c, preview)
					if preview != nil && preview.err != nil {
						ui.Text(c, components.ErrorText(preview.err)).
							TextColor(c.Theme().TextMuted)
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
							a.jump,
							a.openHex,
							a.openText,
							a.openScript,
							func(c *ui.Context) { a.chapterFields(c, row, table) },
						)
					} else {
						a.descriptionEditors(c, kind, row, table)
					}
					if kind != records.Chapters {
						a.recordFields(c, kind, row, table)
					}
				})
		})
}
