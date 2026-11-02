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

func (p *Page) backgroundControls(c *ui.Context) {
	b, e := p.background, p.state.Session.WorldMap
	ui.Column(c.Key("background-actions")).Gap(6).Children(func() {
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Button(c.Key("export-background"), locales.WorldmapExportBackground()).
				Disabled(p.state.Busy || p.exportBackground == nil).
				OnClick(func() {
					source, err := e.Background()
					b.err = err
					if err == nil {
						p.exportBackground(source)
					}
				})
			ui.Button(c.Key("import-background"), locales.WorldmapImportBackground()).
				Tooltip(locales.WorldmapBackgroundFormat()).
				Disabled(p.state.Busy || p.importBackground == nil).
				OnClick(func() {
					source, err := e.Background()
					b.err = err
					if err != nil {
						return
					}
					session := p.state.Session
					p.importBackground(
						source,
						func(draft *worldmap.BackgroundDraft, err error) {
							if p.state.Session != session ||
								p.background != b ||
								!source.Current(e) {
								return
							}
							b.err = err
							if err != nil || draft == nil {
								return
							}
							b.draft, b.image, b.preview = draft, draft.Image(), true
							b.plan, b.err = e.PlanBackground(draft)
						},
					)
				})
		})
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
		ui.Text(c, locales.WorldmapBackgroundScope()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.WorldmapBackgroundFormat()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		if b.plan != nil {
			appended := b.plan.Appended()
			ui.Text(c, fmt.Sprintf("+%d B", appended.End-appended.Start)).
				FontSize(12).
				Font("monospace")
		}
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c.Key("preview-background"), &b.preview, locales.WorldmapPreviewBackground()).
				Disabled(p.state.Busy || stale)
			ui.PrimaryButton(c.Key("apply-background"), locales.WorldmapApplyBackground()).
				Disabled(p.state.Busy || stale || !b.plan.Current(e)).
				OnClick(func() {
					b.err = e.ApplyBackground(b.plan)
					if b.err == nil {
						p.state.Session.Refresh()
						p.state.HistoryEpoch++
						p.background = &backgroundState{}
					}
				})
			ui.Button(c.Key("cancel-background"), locales.Cancel()).
				Disabled(p.state.Busy).
				OnClick(func() { p.background = &backgroundState{} })
		})
	})
}
