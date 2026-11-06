package chapters

import (
	"errors"
	"fmt"
	"image/color"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type paletteReplacement struct {
	draft     *maps.PaletteDraft
	plan      *maps.PalettePlan
	err       error
	preview   bool
	unchanged bool
	changes   ui.ListState
}

type paletteRenderKey struct {
	source        *maps.Palette
	draft         *maps.PaletteDraft
	section, tone maps.PaletteSection
}

type paletteState struct {
	mapPainter

	source        *maps.Palette
	err           error
	revision      uint64
	address       int
	section, tone int
	replacements  [3]paletteReplacement
	filesFocus    ui.Handle
	key           paletteRenderKey
	loaded        bool
	chart         *ui.Bitmap
	chartHeight   int
	render        maps.PalettePreview
}

func (v *Page) SetPaletteActions(
	importPNG func(*maps.Palette, maps.PaletteSection, func(*maps.PaletteDraft, error)),
	exportPNG func(*maps.Palette, maps.PaletteSection),
) {
	v.importPalette, v.exportPalette = importPNG, exportPNG
}

func (s *paletteState) pending() bool {
	for _, replacement := range s.replacements {
		if replacement.draft != nil {
			return true
		}
	}
	return false
}

func (v *Page) paletteDraft(
	state *editor.State,
	row records.Row,
) *paletteState {
	entry := v.paletteDrafts[row.ID]
	if entry != nil {
		stale := entry.revision != state.Session.Document.State().Revision ||
			entry.address != row.Address
		if entry.source != nil {
			stale = !entry.source.Current(state.Session.Maps)
		}
		if stale && !entry.pending() {
			entry = nil
		}
	}
	if entry == nil {
		entry = &paletteState{
			revision: state.Session.Document.State().Revision,
			address:  row.Address,
		}
		entry.source, entry.err = state.Session.Maps.Palette(row.ID)
		v.paletteDrafts[row.ID] = entry
	}
	return entry
}

func (s *paletteState) prepare() {
	section, tone := maps.PaletteSection(s.section), maps.PaletteSection(s.tone)
	replacement := &s.replacements[s.section]
	key := paletteRenderKey{source: s.source, section: section, tone: tone}
	if replacement.preview {
		key.draft = replacement.draft
	}
	if s.loaded && s.key == key {
		return
	}
	s.loaded, s.key = true, key
	chart, _ := s.source.Image(section)
	s.render = s.source.Preview(tone)
	if key.draft != nil {
		chart, _ = key.draft.Image(section)
		s.render = key.draft.Preview(tone)
	}
	s.chart = ui.NewBitmap(chart).Pixelated()
	s.chartHeight = chart.Bounds().Dy() * 2
	s.image, s.tileset = s.render.Map, s.render.Tileset
	s.bitmap = nil
	if s.render.TilesetErr == nil {
		s.bitmap = ui.NewBitmap(s.tileset.Image).Pixelated()
	}
}

func (v *Page) paletteEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	width float32,
	jump func(records.Kind, int),
) {
	entry := v.paletteDraft(state, row)
	ui.Column(c.Key(entry)).Gap(12).Children(func() {
		if entry.err != nil {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
		}
		if entry.source == nil {
			ui.Button(c, locales.MapPaletteReload()).
				Disabled(state.Busy).
				OnClick(func() { delete(v.paletteDrafts, row.ID) })
			return
		}
		e := state.Session.Maps
		stale := !entry.source.Current(e)
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		ui.Tabs(c.Key("section"), &entry.section, locales.MapPaletteNormal(), locales.MapPaletteFog(), locales.MapPaletteAll()).
			Label(locales.MapPaletteSection()).
			Disabled(state.Busy).
			OnChange(func() {
				if entry.section != int(maps.AllPalettes) {
					entry.tone = entry.section
				}
			})
		section := maps.PaletteSection(entry.section)
		replacement := &entry.replacements[entry.section]
		ui.Row(c.Key("files")).Wrap().Gap(8).Children(func() {
			ui.Button(c.Key("export"), locales.MapPaletteExport()).
				Tooltip(locales.MapPaletteFormat()).
				Bind(&entry.filesFocus).
				Disabled(state.Busy || stale || v.exportPalette == nil).
				OnClick(func() { v.exportPalette(entry.source, section) })
			ui.Button(c.Key("import"), locales.MapPaletteImport()).
				Tooltip(locales.MapPaletteFormat()).
				Disabled(state.Busy || stale || v.importPalette == nil).
				OnClick(func() {
					source, session := entry.source, state.Session
					v.importPalette(
						source,
						section,
						func(draft *maps.PaletteDraft, err error) {
							if state.Session != session ||
								v.paletteDrafts[row.ID] != entry ||
								v.chapter != row.ID ||
								v.mode != 10 ||
								entry.section != int(section) ||
								!source.Current(e) {
								return
							}
							replacement.err = err
							if err != nil || draft == nil {
								return
							}
							replacement.draft, replacement.preview, replacement.unchanged = draft, true, false
							replacement.plan, replacement.err = e.PlanPalette(
								draft,
							)
							state.Report("editor.operation", replacement.err)
							if errors.Is(replacement.err, edit.ErrValue) {
								replacement.draft, replacement.plan, replacement.err = nil, nil, nil
								replacement.unchanged = true
							}
							entry.filesFocus.Focus()
						},
					)
				})
			ui.Button(c.Key("reload"), locales.MapPaletteReload()).
				Disabled(state.Busy).
				OnClick(func() {
					delete(v.paletteDrafts, row.ID)
					next := v.paletteDraft(state, row)
					next.section, next.tone = entry.section, entry.tone
					next.filesFocus.Focus(c)
				})
		})
		if replacement.err != nil {
			ui.Text(c, components.ErrorText(replacement.err)).
				TextColor(c.Theme().Danger)
		}
		if replacement.unchanged {
			ui.Text(c, locales.MapPaletteUnchanged()).
				FontSize(components.FontCaption)
		}
		if entry.pending() {
			ui.Text(c, locales.MapPaletteDrafts()).
				FontSize(components.FontCaption)
		}
		if replacement.draft != nil {
			v.paletteReview(c, state, row, entry, replacement, jump)
		}
		entry.prepare()
		mapWorkspace(c, width, func(c *ui.Context) {
			if !stale {
				entry.view.Render(
					c,
					entry.image,
					entry.render.MapErr,
					mapview.Overlay{},
				)
			}
		}, func(c *ui.Context) {
			ui.Image(c.Key("palette-chart"), entry.chart).
				Size(256, float32(entry.chartHeight)).
				Shrink(0).Label(locales.MapPaletteChart())
			if stale {
				return
			}
			ui.Tabs(c.Key("preview-tone"), &entry.tone, locales.MapPaletteNormal(), locales.MapPaletteFog()).
				Label(locales.MapPalettePreviewTone()).
				Tooltip(locales.MapPalettePreviewTone()).
				Disabled(state.Busy)
			if entry.render.TilesetErr != nil {
				ui.Text(c, components.ErrorText(entry.render.TilesetErr)).
					TextColor(c.Theme().Danger)
			} else {
				entry.palette(c, state.Busy)
			}
		})
	})
}

func paletteColorText(value color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", value.R, value.G, value.B)
}

func (v *Page) paletteReview(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *paletteState,
	replacement *paletteReplacement,
	jump func(records.Kind, int),
) {
	ui.Column(c.Key(replacement)).Gap(8).Children(func() {
		ui.Row(c.Key("actions")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Checkbox(c.Key("preview"), &replacement.preview, locales.MapPaletteShowReplacement()).
					Disabled(state.Busy || !replacement.draft.Current(state.Session.Maps))
				ui.PrimaryButton(c.Key("apply"), locales.MapPaletteApply()).
					Disabled(state.Busy || !replacement.plan.Current(state.Session.Maps)).
					OnClick(func() {
						replacement.err = state.Edit(func() error {
							return state.Session.Maps.ApplyPalette(
								replacement.plan,
							)
						})
						if replacement.err == nil {
							state.HistoryEpoch++
							*replacement = paletteReplacement{}
							if !entry.pending() {
								delete(v.paletteDrafts, row.ID)
								next := v.paletteDraft(state, row)
								next.section, next.tone = entry.section, entry.tone
								next.filesFocus.Focus(c)
							} else {
								entry.filesFocus.Focus(c)
							}
						}
					})
				ui.Button(c.Key("cancel"), locales.Cancel()).
					Disabled(state.Busy).
					OnClick(func() { *replacement = paletteReplacement{}; entry.filesFocus.Focus(c) })
			})
		if replacement.plan == nil {
			return
		}
		changes := replacement.plan.Changes()
		ui.Textf(
			c,
			"%s: %d · +%d B",
			locales.MapPaletteChanges(),
			len(changes),
			replacement.plan.Length(),
		)
		paletteChangesTable(c, &replacement.changes, changes)
		mapUses(
			c,
			state,
			replacement.plan.Uses(),
			locales.MapPaletteScope(),
			jump,
		)
		ui.Text(c, locales.MapPaletteScope()).FontSize(components.FontCaption)
	})
}

func paletteChangesTable(
	c *ui.Context,
	state *ui.ListState,
	changes []maps.PaletteChange,
) {
	ui.Table(c.Key("colors"), state, []ui.TableColumn{
		{ID: "slot", Title: locales.MapPaletteSlot(), Width: 80},
		{ID: "before", Title: locales.Before(), MinWidth: 140},
		{ID: "after", Title: locales.After(), MinWidth: 140},
	}, len(changes), func(index, column int) {
		change := changes[index]
		if column == 0 {
			ui.Textf(c, "%03d", change.Index).FontFeatures("tnum")
			return
		}
		value := change.Before
		if column == 2 {
			value = change.After
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).
				Size(14, 14).
				Background(ui.RGB(value.R, value.G, value.B))
			ui.Text(c, paletteColorText(value)).
				Font("monospace").
				FontSize(components.FontCaption)
		})
	}).Height(float32(min(240, max(70, (len(changes)+1)*28)))).MaxWidthPercent(100)
}
