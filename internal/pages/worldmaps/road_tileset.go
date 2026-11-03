package worldmaps

import (
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type roadTilesetState struct {
	draft           *worldmap.RoadTilesetDraft
	plan            *worldmap.RoadTilesetPlan
	image           *image.RGBA
	preview         bool
	err             error
	route           *worldmap.RouteDraft
	road, roadInput *image.RGBA
}

func (p *Page) SetRoadTilesetActions(
	importPNG func(*worldmap.RoadTileset, func(*worldmap.RoadTilesetDraft, error)),
	exportPNG func(*worldmap.RoadTileset),
) {
	p.importRoadTileset, p.exportRoadTileset = importPNG, exportPNG
}

func (p *Page) previewRoadTileset() *worldmap.RoadTilesetDraft {
	s := p.roadTileset
	if s.preview && s.draft.Current(p.state.Session.WorldMap) {
		return s.draft
	}
	return nil
}

func (p *Page) roadTilesetFiles(c *ui.Context) {
	s, e := p.roadTileset, p.state.Session.WorldMap
	ui.Column(c.Key("road-tileset-files")).Gap(4).Children(func() {
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Button(c.Key("export-road-tileset"), locales.WorldmapExportRoadTileset()).
				Disabled(p.state.Busy || p.exportRoadTileset == nil).
				OnClick(func() {
					source, err := e.RoadTileset()
					s.err = err
					if err == nil {
						p.exportRoadTileset(source)
					}
				})
			ui.Button(c.Key("import-road-tileset"), locales.WorldmapImportRoadTileset()).
				Disabled(p.state.Busy || p.importRoadTileset == nil).
				OnClick(func() {
					source, err := e.RoadTileset()
					s.err = err
					if err != nil {
						return
					}
					session := p.state.Session
					p.importRoadTileset(
						source,
						func(draft *worldmap.RoadTilesetDraft, err error) {
							if p.state.Session != session ||
								p.roadTileset != s ||
								!source.Current(e) {
								return
							}
							s.err = err
							if err != nil || draft == nil {
								return
							}
							s.draft, s.image, s.preview = draft, draft.Image(), true
							s.route, s.road, s.roadInput = nil, nil, nil
							s.plan, s.err = e.PlanRoadTileset(draft)
							p.imagesOpen = false
							p.imageFiles.Focus()
							if p.routes.active != nil {
								p.routes.active.clearRoadGesture()
							}
						},
					)
				})
		})
		ui.Text(c, locales.WorldmapRoadTilesetFormat()).
			FontSize(components.FontCaption).
			TextColor(c.Theme().TextMuted)
	})
}

func (p *Page) roadTilesetPreview(c *ui.Context) {
	s, e := p.roadTileset, p.state.Session.WorldMap
	if s.err == nil && s.draft == nil {
		return
	}
	ui.Column(c.Key("road-tileset-preview")).Gap(6).Children(func() {
		if s.err != nil {
			ui.Text(c, components.ErrorText(s.err)).TextColor(c.Theme().Danger)
		}
		if s.draft == nil {
			return
		}
		stale := !s.draft.Current(e)
		if stale {
			ui.Text(c, locales.WorldmapStale()).TextColor(c.Theme().Danger)
		}
		ui.Text(c, locales.WorldmapRoadTilesetScope()).
			FontSize(components.FontCaption)
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c.Key("preview-road-tileset"), &s.preview, locales.WorldmapPreviewRoadTileset()).
				Disabled(p.state.Busy || stale)
			ui.PrimaryButton(c.Key("apply-road-tileset"), locales.WorldmapApplyRoadTileset()).
				Disabled(p.state.Busy || stale || !s.plan.Current(e)).
				OnClick(func() {
					s.err = e.ApplyRoadTileset(s.plan)
					if s.err == nil {
						p.state.Session.Refresh()
						p.state.HistoryEpoch++
						p.roadTileset = &roadTilesetState{}
						p.imageFiles.Focus(c)
					}
				})
			ui.Button(c.Key("cancel-road-tileset"), locales.Cancel()).
				Disabled(p.state.Busy).
				OnClick(func() { p.roadTileset = &roadTilesetState{}; p.imageFiles.Focus(c) })
			if s.plan != nil {
				added := s.plan.Appended()
				ui.Text(c, fmt.Sprintf("+%d B", added.End-added.Start)).
					FontSize(components.FontCaption).
					Font("monospace").
					TextColor(c.Theme().TextMuted)
			}
		})
	})
}
