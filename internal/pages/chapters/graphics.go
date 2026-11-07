package chapters

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type graphicsRenderKey struct {
	source     *maps.Graphics
	draft      *maps.GraphicsDraft
	bank, tone int
}

type graphicsEntry struct {
	mapPainter

	source             *maps.Graphics
	draft              *maps.GraphicsDraft
	plan               *maps.GraphicsPlan
	err                error
	revision           uint64
	address            int
	bank, importBank   string
	withPalette        bool
	tone               int
	preview, unchanged bool
	filesFocus         ui.Handle
	key                graphicsRenderKey
	loaded             bool
	sheet              *ui.Bitmap
	sheetHeight        int
	sheetErr           error
	render             maps.GraphicsPreview
	colorChanges       ui.ListState
}

type graphicsState struct {
	slot    int
	entries [2]*graphicsEntry
}

func (v *Page) SetGraphicsActions(
	importPNG func(*maps.Graphics, maps.GraphicsImport, func(*maps.GraphicsDraft, error)),
	exportPNG func(*maps.Graphics, int),
) {
	v.importGraphics, v.exportGraphics = importPNG, exportPNG
}

func (v *Page) graphicsDraft(
	state *editor.State,
	row records.Row,
) (*graphicsState, *graphicsEntry) {
	group := v.graphicsDrafts[row.ID]
	if group == nil {
		group = &graphicsState{}
		v.graphicsDrafts[row.ID] = group
	}
	entry := group.entries[group.slot]
	if entry != nil {
		stale := entry.revision != state.Session.Document.State().Revision ||
			entry.address != row.Address
		if entry.source != nil {
			stale = !entry.source.Current(state.Session.Maps)
		}
		if stale && entry.draft == nil {
			entry = nil
		}
	}
	if entry == nil {
		entry = &graphicsEntry{
			bank:       "0",
			importBank: "0",
			revision:   state.Session.Document.State().Revision,
			address:    row.Address,
		}
		entry.source, entry.err = state.Session.Maps.Graphics(
			row.ID,
			maps.GraphicsSlot(group.slot),
		)
		group.entries[group.slot] = entry
	}
	return group, entry
}

func (entry *graphicsEntry) prepare() {
	bank, _ := strconv.Atoi(entry.bank)
	key := graphicsRenderKey{
		source: entry.source,
		bank:   bank,
		tone:   entry.tone,
	}
	if entry.preview {
		key.draft = entry.draft
	}
	if entry.loaded && entry.key == key {
		return
	}
	entry.loaded, entry.key = true, key
	sheet, err := entry.source.Image(bank)
	entry.render = entry.source.Preview(maps.PaletteSection(entry.tone))
	if key.draft != nil {
		sheet, err = key.draft.Image(bank)
		entry.render = key.draft.Preview(maps.PaletteSection(entry.tone))
	}
	entry.sheet, entry.sheetErr = nil, err
	if err == nil {
		entry.sheet = ui.NewBitmap(sheet).Pixelated()
		entry.sheetHeight = sheet.Bounds().Dy() * 2
	}
	entry.image, entry.tileset = entry.render.Map, entry.render.Tileset
	entry.bitmap = nil
	if entry.render.TilesetErr == nil {
		entry.bitmap = ui.NewBitmap(entry.tileset.Image).Pixelated()
	}
}

func (v *Page) graphicsEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	width float32,
	jump func(records.Kind, int),
) {
	group, entry := v.graphicsDraft(state, row)
	ui.Column(c.Key(group)).Gap(12).Children(func() {
		ui.Tabs(c.Key("slot"), &group.slot, locales.MapGraphicsFirst(), locales.MapGraphicsSecond()).
			Label(locales.MapGraphicsSource()).
			Disabled(state.Busy).
			OnChange(func() { v.resourceEpoch++ })
		if group.entries[group.slot] != entry {
			return
		}
		ui.Column(c.Key(entry)).Gap(12).Children(func() {
			if entry.source == nil {
				if entry.err != nil {
					textColor := c.Theme().Danger
					if errors.Is(entry.err, maps.ErrGraphicsAbsent) {
						textColor = c.Theme().TextMuted
					}
					ui.Text(c, components.ErrorText(entry.err)).
						TextColor(textColor)
				}
				ui.Button(c, locales.MapGraphicsReload()).
					Bind(&entry.filesFocus).
					Disabled(state.Busy).
					OnClick(func() { v.reloadGraphics(c, state, row, group, entry) })
				return
			}
			e := state.Session.Maps
			stale := !entry.source.Current(e)
			if stale {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			ui.Row(c.Key("import-options")).
				Wrap().
				Gap(8).
				AlignItems(ui.Center).
				Children(func() {
					ui.Checkbox(c.Key("with-palette"), &entry.withPalette, locales.MapGraphicsWithPalette()).
						Disabled(state.Busy || stale || entry.source.PaletteCount() == 0 && !entry.withPalette).
						OnChange(func() { v.resourceEpoch++ })
					if entry.withPalette {
						choices := make([]string, entry.source.PaletteCount())
						for index := range choices {
							choices[index] = strconv.Itoa(index)
						}
						ui.Text(c, locales.MapGraphicsImportBank())
						ui.Select(c.Key("import-bank"), &entry.importBank, choices).
							Label(locales.MapGraphicsImportBank()).
							Width(100).
							Disabled(state.Busy || stale || entry.source.PaletteCount() == 0).
							OnChange(func() { v.resourceEpoch++ })
					}
				})
			ui.Row(c.Key("files")).Wrap().Gap(8).Children(func() {
				ui.Button(c.Key("export"), locales.MapGraphicsExport()).
					Tooltip(locales.MapGraphicsFormat()).
					Disabled(state.Busy || stale || entry.source.PaletteCount() == 0 || v.exportGraphics == nil).
					OnClick(func() {
						bank, _ := strconv.Atoi(entry.bank)
						v.exportGraphics(entry.source, bank)
					})
				ui.Button(c.Key("import"), locales.MapGraphicsImport()).
					Tooltip(locales.MapGraphicsFormat()).
					Bind(&entry.filesFocus).
					Disabled(state.Busy || stale || v.importGraphics == nil || entry.withPalette && entry.source.PaletteCount() == 0).
					OnClick(func() {
						source, session, slot, epoch := entry.source, state.Session, group.slot, v.resourceEpoch
						bank, _ := strconv.Atoi(entry.importBank)
						options := maps.GraphicsImport{
							WithPalette: entry.withPalette,
							Bank:        bank,
						}
						v.importGraphics(
							source,
							options,
							func(draft *maps.GraphicsDraft, err error) {
								if state.Session != session ||
									v.graphicsDrafts[row.ID] != group ||
									group.entries[slot] != entry ||
									group.slot != slot ||
									v.chapter != row.ID ||
									v.mode() != graphicsMode ||
									v.resourceEpoch != epoch ||
									!source.Current(e) {
									return
								}
								entry.err = err
								if err != nil || draft == nil {
									return
								}
								entry.draft, entry.preview, entry.unchanged = draft, true, false
								if options.WithPalette {
									entry.bank = strconv.Itoa(options.Bank)
									entry.tone = options.Bank / 5
								}
								entry.plan, entry.err = e.PlanGraphics(draft)
								state.Report("editor.operation", entry.err)
								if errors.Is(entry.err, edit.ErrValue) {
									entry.draft, entry.plan, entry.err = nil, nil, nil
									entry.unchanged = true
								}
								entry.filesFocus.Focus()
							},
						)
					})
				ui.Button(c.Key("reload"), locales.MapGraphicsReload()).
					Disabled(state.Busy).
					OnClick(func() { v.reloadGraphics(c, state, row, group, entry) })
			})
			ui.Textf(
				c,
				"%s: %d",
				locales.MapGraphicsCount(),
				entry.source.Count(),
			)
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if entry.unchanged {
				ui.Text(c, locales.MapGraphicsUnchanged()).
					FontSize(components.FontCaption)
			}
			if entry.draft != nil {
				ui.Text(c, locales.MapGraphicsDrafts()).
					FontSize(components.FontCaption)
				v.graphicsReview(c, state, row, group, entry, jump)
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
				if count := entry.source.PaletteCount(); count > 0 {
					choices := make([]string, count)
					for index := range choices {
						choices[index] = strconv.Itoa(index)
					}
					ui.Row(c.Key("bank-row")).
						Wrap().
						Gap(8).
						AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, locales.MapGraphicsDisplayBank())
							ui.Select(c.Key("display-bank"), &entry.bank, choices).
								Label(locales.MapGraphicsDisplayBank()).
								Width(100).
								Disabled(state.Busy)
						})
				}
				entry.prepare()
				if entry.sheetErr != nil {
					ui.Text(c, components.ErrorText(entry.sheetErr)).
						TextColor(c.Theme().Danger)
				} else {
					ui.Scroll(c.Key("sheet-scroll")).
						Width(258).
						MaxWidthPercent(100).
						Height(float32(min(360, entry.sheetHeight+2))).
						Children(func() {
							ui.Image(c.Key("sheet"), entry.sheet).
								Size(256, float32(entry.sheetHeight)).
								Shrink(0).
								Label(locales.MapGraphicsSheet())
						})
				}
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
	})
}

func (v *Page) reloadGraphics(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	group *graphicsState,
	entry *graphicsEntry,
) {
	group.entries[group.slot] = nil
	_, next := v.graphicsDraft(state, row)
	next.bank, next.tone = entry.bank, entry.tone
	next.withPalette, next.importBank = entry.withPalette, entry.importBank
	next.filesFocus.Focus(c)
}

func (v *Page) graphicsReview(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	group *graphicsState,
	entry *graphicsEntry,
	jump func(records.Kind, int),
) {
	ui.Column(c.Key("review")).Gap(8).Children(func() {
		ui.Row(c.Key("actions")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Checkbox(c.Key("preview"), &entry.preview, locales.MapGraphicsShowReplacement()).
					Disabled(state.Busy || !entry.draft.Current(state.Session.Maps))
				ui.PrimaryButton(c.Key("apply"), locales.MapGraphicsApply()).
					Disabled(state.Busy || !entry.plan.Current(state.Session.Maps)).
					OnClick(func() {
						entry.err = state.Edit(
							func() error { return state.Session.Maps.ApplyGraphics(entry.plan) },
						)
						if entry.err == nil {
							state.HistoryEpoch++
							v.reloadGraphics(c, state, row, group, entry)
						}
					})
				ui.Button(c.Key("cancel"), locales.Cancel()).
					Disabled(state.Busy).
					OnClick(func() { v.reloadGraphics(c, state, row, group, entry) })
			})
		if entry.plan == nil {
			return
		}
		ui.Textf(
			c,
			"%s: %d / %d · +%d B",
			locales.MapGraphicsChanges(),
			len(entry.plan.ChangedTiles()),
			entry.source.Count(),
			entry.plan.Length(),
		)
		uses := entry.plan.Uses()
		if len(uses) > 0 {
			ui.Text(c, locales.MapGraphicsSource()).Bold()
		}
		for _, updated := range []bool{true, false} {
			var selected []maps.GraphicsUse
			for _, use := range uses {
				if use.Updated == updated {
					selected = append(selected, use)
				}
			}
			if len(selected) == 0 {
				continue
			}
			ui.Column(c.Key(updated)).Gap(6).Children(func() {
				label := locales.MapUsesChanged()
				if !updated {
					label = locales.MapUsesRetained()
				}
				ui.Text(c, label).Bold()
				ui.Row(c).Wrap().Gap(8).Children(func() {
					for _, use := range selected {
						slot := locales.MapGraphicsFirst()
						if use.Slot == maps.SecondGraphics {
							slot = locales.MapGraphicsSecond()
						}
						ui.Button(c.Key(use), fmt.Sprintf("%02X %s · %s", use.Chapter, nameOf(state.Session, records.Chapters, use.Chapter), slot)).
							Disabled(state.Busy || jump == nil).
							OnClick(func() { jump(records.Chapters, use.Chapter) })
					}
				})
			})
		}
		if len(uses) > 0 {
			ui.Text(c, locales.MapGraphicsScope()).
				FontSize(components.FontCaption)
		}
		if options := entry.plan.ImportOptions(); options.WithPalette {
			ui.Column(c.Key("palette-review")).Gap(6).Children(func() {
				ui.Text(c, locales.MapPaletteEditor()).Bold()
				ui.Textf(
					c,
					"%s: %d · %s: %d",
					locales.MapGraphicsImportBank(),
					options.Bank,
					locales.MapPaletteChanges(),
					len(entry.plan.PaletteChanges()),
				)
				if changes := entry.plan.PaletteChanges(); len(changes) > 0 {
					paletteChangesTable(c, &entry.colorChanges, changes)
				}
				if uses := entry.plan.PaletteUses(); len(uses) > 0 {
					mapUses(
						c,
						state,
						uses,
						locales.MapGraphicsPaletteScope(),
						jump,
					)
					ui.Text(c, locales.MapGraphicsPaletteScope()).
						FontSize(components.FontCaption)
				}
			})
		}
	})
}
