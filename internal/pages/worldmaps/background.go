package worldmaps

import (
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type backgroundState struct {
	draft   *worldmap.BackgroundDraft
	plan    *worldmap.BackgroundPlan
	image   *image.RGBA
	preview bool
	err     error
}

func (p *Page) SetBackgroundActions(
	importPNG func(*worldmap.Background, func(*worldmap.BackgroundDraft, error)),
	exportPNG func(*worldmap.Background),
) {
	p.importBackground, p.exportBackground = importPNG, exportPNG
}

func (p *Page) backgroundFiles(c *ui.Context) {
	tone := worldmap.BackgroundTone(p.backgroundTone)
	b, e := p.backgrounds[p.backgroundTone], p.state.Session.WorldMap
	exportLabel, importLabel, format := locales.WorldmapExportBackground(), locales.WorldmapImportBackground(), locales.WorldmapBackgroundFormat()
	if tone == worldmap.Darkened {
		exportLabel, importLabel, format = locales.WorldmapExportDarkBackground(), locales.WorldmapImportDarkBackground(), locales.WorldmapDarkBackgroundFormat()
	}
	ui.Column(c.Key("background-files")).Gap(4).Children(func() {
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Button(c.Key("export-background"), exportLabel).
				Disabled(p.state.Busy || p.exportBackground == nil).
				OnClick(func() {
					source, err := e.Background(tone)
					b.err = err
					if err == nil {
						p.exportBackground(source)
					}
				})
			ui.Button(c.Key("import-background"), importLabel).
				Disabled(p.state.Busy || p.importBackground == nil).
				OnClick(func() {
					source, err := e.Background(tone)
					b.err = err
					if err != nil {
						return
					}
					session := p.state.Session
					p.importBackground(
						source,
						func(draft *worldmap.BackgroundDraft, err error) {
							if p.state.Session != session ||
								p.backgrounds[tone] != b ||
								!source.Current(e) {
								return
							}
							b.err = err
							if err != nil || draft == nil {
								return
							}
							b.draft, b.image, b.preview = draft, draft.Image(), true
							b.plan, b.err = e.PlanBackground(draft)
							p.imagesOpen = false
							p.imageFiles.Focus()
						},
					)
				})
		})
		ui.Text(c, format).
			FontSize(components.FontCaption).
			TextColor(c.Theme().TextMuted)
	})
}

func (p *Page) backgroundPreview(c *ui.Context) {
	tone := worldmap.BackgroundTone(p.backgroundTone)
	b, e := p.backgrounds[p.backgroundTone], p.state.Session.WorldMap
	if b.err == nil && b.draft == nil {
		return
	}
	ui.Column(c.Key("background-preview")).Gap(6).Children(func() {
		if b.err != nil {
			ui.Text(c, components.ErrorText(b.err)).TextColor(c.Theme().Danger)
		}
		if b.draft == nil {
			return
		}
		stale := !b.draft.Current(e)
		if stale {
			ui.Text(c, locales.WorldmapStale()).TextColor(c.Theme().Danger)
		}
		scope := locales.WorldmapBackgroundScope()
		if tone == worldmap.Darkened {
			scope = locales.WorldmapDarkBackgroundScope()
		}
		ui.Text(c, scope).FontSize(components.FontCaption)
		previewLabel, applyLabel := locales.WorldmapPreviewBackground(), locales.WorldmapApplyBackground()
		if tone == worldmap.Darkened {
			previewLabel, applyLabel = locales.WorldmapPreviewDarkBackground(), locales.WorldmapApplyDarkBackground()
		}
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c.Key("preview-background"), &b.preview, previewLabel).
				Disabled(p.state.Busy || stale)
			ui.PrimaryButton(c.Key("apply-background"), applyLabel).
				Disabled(p.state.Busy || stale || !b.plan.Current(e)).
				OnClick(func() {
					b.err = e.ApplyBackground(b.plan)
					if b.err == nil {
						p.state.Session.Refresh()
						p.state.HistoryEpoch++
						p.backgrounds[tone] = &backgroundState{}
						p.imageFiles.Focus(c)
					}
				})
			ui.Button(c.Key("cancel-background"), locales.Cancel()).
				Disabled(p.state.Busy).
				OnClick(func() { p.backgrounds[tone] = &backgroundState{}; p.imageFiles.Focus(c) })
			if b.plan != nil {
				added := b.plan.Appended()
				ui.Text(c, fmt.Sprintf("+%d B", added.End-added.Start)).
					FontSize(components.FontCaption).
					Font("monospace").
					TextColor(c.Theme().TextMuted)
			}
		})
	})
}
