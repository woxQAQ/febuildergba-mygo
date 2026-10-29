package chapterview

import (
	"fmt"
	"image"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type mapPainter struct {
	image, tileset game.MapPreview
	bitmap, brush  *ui.Bitmap
	view           mapview.View
	scroll         ui.ScrollState
	tool           int
	tile           game.MapCell
}

type mapState struct {
	mapPainter
	data                                    *game.MapDraft
	preview                                 *game.MapEdit
	revision                                uint64
	address                                 int
	err                                     error
	resizing, details                       bool
	editFocus, reviewFocus                  ui.Handle
	resizeWidth, resizeHeight, anchorChoice string
	anchor                                  game.MapAnchor
	impactList                              ui.ListState
	impactSelected                          int
}

func (v *View) mapDraft(state *components.State, row game.Row) *mapState {
	entry := v.mapDrafts[row.ID]
	if entry != nil {
		obsolete := entry.revision != state.Session.Document.State().Revision ||
			entry.address != row.Address
		if entry.data != nil {
			obsolete = !entry.data.Current(state.Session)
		}
		if obsolete && (entry.data == nil || !entry.data.Modified()) {
			delete(v.mapDrafts, row.ID)
			entry = nil
		}
	}
	if entry != nil {
		return entry
	}
	entry = &mapState{
		revision: state.Session.Document.State().Revision,
		address:  row.Address,
	}
	entry.data, entry.err = state.Session.NewMapDraft(row.ID)
	if entry.err == nil {
		entry.tileset, entry.err = entry.data.Tileset()
	}
	if entry.err == nil {
		entry.bitmap = ui.NewBitmap(entry.tileset.Image).Pixelated()
		entry.choose(entry.tileset, image.Point{})
		entry.refresh()
	}
	v.mapDrafts[row.ID] = entry
	return entry
}

func (entry *mapState) refresh() {
	entry.preview = nil
	entry.image, entry.err = entry.data.Preview()
	entry.resizeWidth, entry.resizeHeight = strconv.Itoa(
		entry.data.Width(),
	), strconv.Itoa(
		entry.data.Height(),
	)
	entry.impactSelected = -1
	entry.impactList = ui.ListState{}
}

func (entry *mapPainter) choose(preview game.MapPreview, point image.Point) {
	cell, valid := preview.Cell(point.X, point.Y)
	if !valid {
		return
	}
	entry.tile = cell
	bounds := image.Rect(point.X*16, point.Y*16, (point.X+1)*16, (point.Y+1)*16)
	entry.brush = ui.NewBitmap(preview.Image.SubImage(bounds)).Pixelated()
}

func (v *View) mapEditor(
	c *ui.Context,
	state *components.State,
	row game.Row,
	jump func(game.Kind, int),
	openHex func(int, int),
) {
	entry := v.mapDraft(state, row)
	if entry.err != nil {
		ui.Text(c, components.ErrorText(entry.err)).
			Tooltip(entry.err.Error()).
			TextColor(c.Theme().Danger)
		return
	}
	stale := !entry.data.Current(state.Session)
	disabled := state.Busy || stale
	ui.Column(c.Key("map-editor")).Gap(12).Children(func() {
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		v.mapActions(c, state, row, entry, disabled)
		if entry.preview != nil {
			v.mapPreview(c, state, row, entry, jump, openHex)
			return
		}
		if entry.resizing {
			entry.resizeControls(c, state, disabled)
		}
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Tabs(c.Key("map-tools"), &entry.tool, locales.MapBrush(), locales.MapRectangle(), locales.MapEyedropper(), locales.MapPan()).
				Label(locales.MapTools()).
				Tooltip(locales.MapEditHelp()).
				Disabled(disabled)
			ui.Image(c, entry.brush).Size(32, 32).Shrink(0)
			ui.Textf(c, "%s 0x%04X · %s 0x%02X", locales.MapTile(), entry.tile.Tile, locales.MapTerrain(), entry.tile.Terrain).
				FontFeatures("tnum")
		})
		ui.Row(c.Key("map-workspace")).
			Wrap().
			Gap(16).
			AlignItems(ui.Start).
			Children(func() {
				ui.Column(c.Key("map-drawing")).
					Width(360).
					Grow(1).
					MaxWidthPercent(100).
					Children(func() {
						entry.drawing(c, state, disabled)
					})
				ui.Column(c.Key("map-palette")).
					Width(514).
					MaxWidthPercent(100).
					Gap(6).
					Children(func() {
						entry.palette(c, disabled)
					})
			})
	})
}

func (entry *mapState) drawing(
	c *ui.Context,
	state *components.State,
	disabled bool,
) {
	entry.view.RenderPreview(
		c.Key("drawing"),
		entry.image,
		mapview.Overlay{Drawing: &mapview.Drawing{
			Tool:    mapview.Tool(entry.tool),
			Enabled: !disabled,
			Brush:   entry.brush,
			Pick: func(point image.Point) {
				entry.choose(entry.image, point)
				entry.tool = int(mapview.Brush)
			},
			Commit: func(points []image.Point) {
				if state.Busy || !entry.data.Current(state.Session) {
					return
				}
				var err error
				if mapview.Tool(entry.tool) == mapview.Rectangle {
					a, b := points[0], points[len(points)-1]
					err = entry.data.Fill(
						image.Rect(
							min(a.X, b.X),
							min(a.Y, b.Y),
							max(a.X, b.X)+1,
							max(a.Y, b.Y)+1,
						),
						entry.tile.Tile,
					)
				} else {
					err = entry.data.Stroke(points, entry.tile.Tile)
				}
				state.Err = err
				if err == nil {
					entry.refresh()
				}
			},
		}},
	)
}

func (entry *mapPainter) palette(c *ui.Context, disabled bool) {
	ui.Text(c, locales.MapTileset()).Bold()
	ui.ScrollBoth(c.Key("map-tileset-scroll")).
		Width(514).
		MaxWidthPercent(100).
		Height(194).
		Shrink(0).
		TrackScroll(&entry.scroll).
		BorderWidth(1).
		BorderColor(c.Theme().Border).
		Children(func() {
			canvas := ui.Image(c, entry.bitmap).
				Size(float32(entry.tileset.Width*32), float32(entry.tileset.Height*32)).
				Shrink(0).
				Label(locales.MapTilesetCanvas()).
				Cursor(ui.CursorCrosshair).
				Focusable().
				Disabled(disabled)
			if canvas.Clicked() {
				if x, y, over := canvas.PointerPosition(); over {
					entry.choose(entry.tileset, image.Pt(int(x/32), int(y/32)))
				}
			}
			accent := c.Theme().Accent
			canvas.DrawOver(func(p *ui.Painter, r ui.Rect) {
				for i, cell := range entry.tileset.Cells {
					if cell.Tile == entry.tile.Tile {
						box := ui.Rect{
							X: r.X + float32(i%entry.tileset.Width)*32,
							Y: r.Y + float32(i/entry.tileset.Width)*32,
							W: 32,
							H: 32,
						}
						p.Stroke(box, ui.RGB(255, 255, 255), 0, 3)
						p.Stroke(box, accent, 0, 2)
						break
					}
				}
			})
		})
}

func (v *View) mapActions(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *mapState,
	disabled bool,
) {
	undo := func() {
		if entry.data.Undo() {
			entry.refresh()
			entry.editFocus.Focus(c)
		}
	}
	redo := func() {
		if entry.data.Redo() {
			entry.refresh()
			entry.editFocus.Focus(c)
		}
	}
	if !disabled {
		if entry.data.CanUndo() {
			c.OnShortcut(ui.Cmd, ui.KeyZ, undo)
		}
		if entry.data.CanRedo() {
			c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, redo)
		}
	}
	if entry.preview != nil {
		return
	}
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Button(c, locales.MapUndo()).
			Disabled(disabled || !entry.data.CanUndo()).
			OnClick(undo)
		ui.Button(c, locales.MapRedo()).
			Disabled(disabled || !entry.data.CanRedo()).
			OnClick(redo)
		ui.Button(c, locales.MapPreviewEdit()).
			Disabled(disabled || !entry.data.Modified() || entry.resizePending()).
			OnClick(func() {
				entry.preview, state.Err = state.Session.PlanMapEdit(entry.data)
				entry.details = false
				if state.Err == nil {
					entry.reviewFocus.Focus(c)
				}
			})
		ui.Checkbox(c, &entry.resizing, locales.MapResizeOptions()).
			Bind(&entry.editFocus).
			Disabled(disabled || entry.resizePending())
		ui.Button(c, locales.MapReload()).
			Disabled(state.Busy).
			OnClick(func() { delete(v.mapDrafts, row.ID) })
		if entry.data.Modified() {
			ui.Text(c, locales.MapDraft()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
	})
}

func (v *View) mapPreview(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *mapState,
	jump func(game.Kind, int),
	openHex func(int, int),
) {
	preview := entry.preview
	ui.Column(c.Key("map-edit-preview")).Gap(16).Children(func() {
		ui.Column(c.Key("summary")).Gap(6).Children(func() {
			ui.Text(c, locales.MapPreviewEdit()).FontSize(16).Bold()
			ui.Row(c).Wrap().Gap(12).Children(func() {
				if preview.Resized() {
					before, after := preview.BeforeSize(), preview.AfterSize()
					ui.Textf(c, "%d × %d → %d × %d", before.X, before.Y, after.X, after.Y).
						FontFeatures("tnum").
						Bold()
				}
				ui.Textf(c, "%s: %d", locales.MapChangedCells(), preview.ChangedCells()).
					FontFeatures("tnum")
			})
		})
		mapUses(c, state, preview.Uses(), locales.MapCommitScope(), jump)
		if preview.Resized() {
			ui.Column(c.Key("coordinate-impact")).Gap(6).Children(func() {
				v.mapImpacts(c, state, entry, jump, openHex)
			})
		}
		ui.Column(c.Key("details")).Gap(6).Children(func() {
			ui.Checkbox(c, &entry.details, locales.MapReviewDetails())
			if entry.details {
				ui.Textf(c, "0x%08X → 0x%08X · %d B", preview.Source(), preview.Destination(), preview.Length()).
					FontFeatures("tnum").
					Selectable()
				ui.Text(c, locales.MapCommitScope())
				if preview.Resized() {
					ui.Text(c, locales.MapImpactScope())
				}
			}
		})
		ui.Column(c.Key("apply")).Gap(6).Children(func() {
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Button(c, locales.MapApply()).
					Disabled(state.Busy || !entry.data.Current(state.Session) || entry.resizePending()).
					OnClick(func() {
						if state.Err = state.Session.ApplyMapEdit(
							preview,
						); state.Err == nil {
							delete(v.mapDrafts, row.ID)
							state.HistoryEpoch++
						}
					})
				ui.Button(c, locales.MapBackToEdit()).Disabled(state.Busy).
					Bind(&entry.reviewFocus).
					OnClick(func() {
						entry.preview = nil
						entry.editFocus.Focus(c)
					})
			})
			ui.Text(c, locales.MapEditScope()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		})
	})
}

func mapUses(
	c *ui.Context,
	state *components.State,
	uses []game.MapUse,
	scope string,
	jump func(game.Kind, int),
) {
	for _, updated := range []bool{true, false} {
		label := locales.MapUsesChanged()
		if !updated {
			label = locales.MapUsesRetained()
		}
		count := 0
		for _, use := range uses {
			if use.Updated == updated {
				count++
			}
		}
		if count == 0 {
			continue
		}
		ui.Column(c.Key(updated)).Gap(6).Children(func() {
			ui.Text(c, label).Bold().Tooltip(scope)
			ui.Row(c).Wrap().Gap(8).Children(func() {
				for _, use := range uses {
					if use.Updated != updated {
						continue
					}
					ui.Button(c, fmt.Sprintf("%02X %s", use.Chapter, nameOf(state.Session, game.Chapters, use.Chapter))).
						Disabled(state.Busy || jump == nil).
						OnClick(func() { jump(game.Chapters, use.Chapter) })
				}
			})
		})
	}
}
