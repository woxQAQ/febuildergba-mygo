package chapters

import (
	"fmt"
	"image"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type mapChangesState struct {
	mapPainter
	data                              *maps.ChangesDraft
	preview                           *maps.ChangesEdit
	selected, lastSelected            int
	list, referenceList               ui.ListState
	referenceSelected                 int
	fields                            [5]string
	showBase, showReferences, details bool
	references                        maps.ChangeReferences
	editFocus, reviewFocus            ui.Handle
	err, imageErr                     error
	revision                          uint64
}

func (v *Page) changeDraft(
	state *editor.State,
	chapter int,
) *mapChangesState {
	entry := v.changeDrafts[chapter]
	if entry != nil {
		stale := entry.revision != state.Session.Document.State().Revision
		if entry.data != nil {
			stale = !entry.data.Current(state.Session.Maps)
		}
		if stale &&
			(entry.data == nil || (!entry.data.Modified() && !entry.pending())) {
			delete(v.changeDrafts, chapter)
			entry = nil
		}
	}
	if entry != nil {
		return entry
	}
	entry = &mapChangesState{
		selected:          0,
		lastSelected:      -1,
		referenceSelected: -1,
		revision:          state.Session.Document.State().Revision,
	}
	entry.data, entry.err = state.Session.Maps.NewChangesDraft(chapter)
	if entry.err == nil {
		entry.tileset, entry.err = entry.data.Tileset()
	}
	if entry.err == nil {
		entry.bitmap = ui.NewBitmap(entry.tileset.Image).Pixelated()
		entry.choose(entry.tileset, image.Point{})
		entry.refresh()
	}
	v.changeDrafts[chapter] = entry
	return entry
}

func (entry *mapChangesState) refresh() {
	entry.preview = nil
	entry.showReferences = false
	entry.references = maps.ChangeReferences{}
	entry.referenceSelected = -1
	entry.referenceList = ui.ListState{}
	rows := entry.data.Entries()
	entry.selected = max(0, min(entry.selected, len(rows)-1))
	entry.lastSelected = entry.selected
	if len(rows) > 0 {
		row := rows[entry.selected]
		entry.fields = [5]string{
			fmt.Sprintf("0x%02X", row.ID),
			strconv.Itoa(row.Bounds.Min.X),
			strconv.Itoa(row.Bounds.Min.Y),
			strconv.Itoa(row.Bounds.Dx()),
			strconv.Itoa(row.Bounds.Dy()),
		}
	}
	entry.image, entry.imageErr = entry.data.Preview(
		entry.selected,
		!entry.showBase,
	)
}

func (entry *mapChangesState) values() (int, image.Rectangle, bool) {
	var values [5]int64
	for index, text := range entry.fields {
		value, err := input.ParseInteger(text)
		if err != nil {
			return 0, image.Rectangle{}, false
		}
		values[index] = value
	}
	id, x, y, w, h := values[0], values[1], values[2], values[3], values[4]
	size := entry.data.Size()
	if id < 0 || id > 127 || x < 0 || x >= int64(size.X) || y < 0 ||
		y >= int64(size.Y) ||
		w < 1 ||
		w > int64(size.X)-x ||
		h < 1 ||
		h > int64(size.Y)-y {
		return 0, image.Rectangle{}, false
	}
	return int(id), image.Rect(int(x), int(y), int(x+w), int(y+h)), true
}

func (entry *mapChangesState) pending() bool {
	rows := entry.data.Entries()
	if entry.selected < 0 || entry.selected >= len(rows) {
		return false
	}
	row := rows[entry.selected]
	original := [5]int{
		row.ID,
		row.Bounds.Min.X,
		row.Bounds.Min.Y,
		row.Bounds.Dx(),
		row.Bounds.Dy(),
	}
	for index, text := range entry.fields {
		value, err := input.ParseInteger(text)
		if err != nil || value != int64(original[index]) {
			return true
		}
	}
	return false
}

func (v *Page) OpenMapChange(state *editor.State, chapter, id int) {
	table := state.Session.Records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		state.Err = edit.ErrLayout
		return
	}
	for _, row := range table.Rows {
		if row.ID == chapter {
			v.load(state, row)
			v.mode = 7
			entry := v.changeDraft(state, chapter)
			if entry.err != nil {
				state.Err = entry.err
				return
			}
			for index, change := range entry.data.Entries() {
				if change.ID == id {
					if entry.pending() {
						state.Err = edit.ErrValue
						return
					}
					entry.selected = index
					entry.refresh()
					return
				}
			}
			state.Err = edit.ErrField
			return
		}
	}
	state.Err = edit.ErrField
}

func (v *Page) mapChangesEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	jump func(records.Kind, int),
	openHex func(int, int),
) {
	entry := v.changeDraft(state, row.ID)
	if entry.err != nil {
		ui.Text(c, components.ErrorText(entry.err)).
			TextColor(c.Theme().Danger).
			Tooltip(entry.err.Error())
		return
	}
	stale := !entry.data.Current(state.Session.Maps)
	disabled := state.Busy || stale
	ui.Column(c.Key("map-changes-editor")).Gap(12).Children(func() {
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
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
			v.changeReview(c, state, row, entry, jump)
			return
		}
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Button(c, locales.MapUndo()).
				Disabled(disabled || !entry.data.CanUndo()).
				OnClick(undo)
			ui.Button(c, locales.MapRedo()).
				Disabled(disabled || !entry.data.CanRedo()).
				OnClick(redo)
			ui.Button(c, locales.MapChangesReview()).
				Disabled(disabled || entry.pending() || !entry.data.Modified()).
				OnClick(func() {
					entry.preview, state.Err = state.Session.Maps.PlanChanges(
						entry.data,
					)
					entry.details = false
					if state.Err == nil {
						entry.reviewFocus.Focus(c)
					}
				})
			ui.Button(c, locales.MapChangesReload()).
				Disabled(state.Busy).
				OnClick(func() { delete(v.changeDrafts, row.ID) })
			if entry.data.Modified() {
				ui.Text(c, locales.MapChangesDraft()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
		})
		v.changeRows(c, state, entry, disabled, openHex)
		if len(entry.data.Entries()) > 0 {
			entry.properties(c, state, disabled)
		}
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Tabs(c.Key("change-tools"), &entry.tool, locales.MapBrush(), locales.MapRectangle(), locales.MapEyedropper(), locales.MapPan()).
				Label(locales.MapTools()).
				Tooltip(locales.MapChangePaintHelp()).
				Disabled(disabled || entry.pending())
			ui.Image(c, entry.brush).Size(32, 32).Shrink(0)
			ui.Textf(c, "%s 0x%04X", locales.MapTile(), entry.tile.Tile).
				FontFeatures("tnum")
			ui.Button(c, locales.MapChangeKeepCell()).
				Disabled(disabled).
				OnClick(func() { entry.tile = maps.Cell{}; entry.brush = nil })
			ui.Checkbox(c, &entry.showBase, locales.MapChangeShowBase()).
				Bind(&entry.editFocus).
				Disabled(disabled || entry.pending()).
				OnChange(entry.refresh)
		})
		if entry.tile.Tile == 0 {
			ui.Text(c, locales.MapChangeZero()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
		ui.Row(c.Key("change-workspace")).
			Wrap().
			Gap(16).
			AlignItems(ui.Start).
			Children(func() {
				ui.Column(c.Key("change-drawing")).
					Width(360).
					Grow(1).
					MaxWidthPercent(100).
					Children(func() { entry.drawing(c, state, disabled || entry.pending()) })
				ui.Column(c.Key("change-palette")).
					Width(514).
					MaxWidthPercent(100).
					Gap(6).
					Children(func() { entry.palette(c, disabled || entry.pending()) })
			})
		ui.Checkbox(c, &entry.showReferences, locales.MapChangesReferences()).
			Disabled(disabled || entry.pending()).
			OnChange(func() {
				if entry.showReferences {
					entry.references, state.Err = state.Session.Maps.References(
						entry.data,
					)
				}
			})
		if entry.showReferences {
			v.changeReferences(c, state, entry, entry.references)
		}
	})
}

func (v *Page) changeRows(
	c *ui.Context,
	state *editor.State,
	entry *mapChangesState,
	disabled bool,
	openHex func(int, int),
) {
	rows := entry.data.Entries()
	if len(rows) == 0 {
		ui.Text(c, locales.MapChangesEmpty()).TextColor(c.Theme().TextMuted)
	} else {
		entry.list.Selected = &entry.selected
		entry.list.Key = func(index int) any { return index }
		columns := []ui.TableColumn{
			{ID: "id", Title: locales.MapChangeId(), Width: 70, Fixed: true},
			{ID: "bounds", Title: locales.MapImpactBounds()},
			{ID: "address", Title: locales.RomAddress()},
		}
		ui.Table(c.Key("changes"), &entry.list, columns, len(rows), func(index, column int) {
			change := rows[index]
			text := ""
			switch column {
			case 0:
				text = fmt.Sprintf("%02X", change.ID)
			case 1:
				text = fmt.Sprintf(
					"%d,%d · %d × %d",
					change.Bounds.Min.X,
					change.Bounds.Min.Y,
					change.Bounds.Dx(),
					change.Bounds.Dy(),
				)
			case 2:
				if change.Address != 0 {
					text = fmt.Sprintf("0x%08X", change.Address)
				}
			}
			ui.Text(c, text).SingleLine().FontFeatures("tnum")
		}).
			Height(min(144, float32(len(rows)+1)*max(28, c.Theme().Space(8)))).
			Label(locales.MapChangeList()).
			Disabled(disabled || entry.pending())
		if entry.selected != entry.lastSelected {
			entry.refresh()
		}
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.MapChangeAdd()).
			Disabled(disabled || entry.pending()).
			OnClick(func() {
				id, err := entry.data.FreeID()
				if err == nil {
					err = entry.data.Add(id, image.Rect(0, 0, 1, 1))
				}
				state.Err = err
				if err == nil {
					entry.selected = len(entry.data.Entries()) - 1
					entry.refresh()
				}
			})
		ui.Button(c, locales.MapChangeCopy()).
			Disabled(disabled || entry.pending() || len(rows) == 0).
			OnClick(func() {
				id, err := entry.data.FreeID()
				if err == nil {
					err = entry.data.Copy(entry.selected, id)
				}
				state.Err = err
				if err == nil {
					entry.selected = len(entry.data.Entries()) - 1
					entry.refresh()
				}
			})
		ui.Button(c, locales.MapChangeDelete()).
			Disabled(disabled || entry.pending() || len(rows) == 0).
			OnClick(func() {
				state.Err = entry.data.Delete(entry.selected)
				if state.Err == nil {
					entry.refresh()
				}
			})
		if len(rows) > 0 && rows[entry.selected].Address != 0 {
			ui.Button(c, locales.EditBytes()).
				Disabled(state.Busy || openHex == nil).
				OnClick(func() { openHex(rows[entry.selected].Address, 12) })
		}
	})
}

func (entry *mapChangesState) properties(
	c *ui.Context,
	state *editor.State,
	disabled bool,
) {
	labels := []string{
		locales.MapChangeId(),
		locales.MapChangeX(),
		locales.MapChangeY(),
		locales.MapChangeWidth(),
		locales.MapChangeHeight(),
	}
	ui.Row(c).Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
		for index, label := range labels {
			ui.Row(c.Key(index)).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).Tooltip(locales.MapChangeGeometryHelp())
				components.TextDraft(
					state,
					ui.TextInput(c, &entry.fields[index]).
						Width(64).
						Label(locales.MapChangesEditor()+" / "+label).
						Disabled(disabled).
						OnChange(func() { entry.preview = nil; entry.showReferences = false }),
				)
			})
		}
		id, bounds, valid := entry.values()
		ui.Button(c, locales.MapChangeUpdate()).
			Disabled(disabled || !valid || !entry.pending()).
			OnClick(func() {
				state.Err = entry.data.Set(entry.selected, id, bounds)
				if state.Err == nil {
					entry.refresh()
					entry.editFocus.Focus(c)
				}
			})
		if entry.pending() {
			ui.Button(c, locales.MapChangeReset()).
				Disabled(disabled).
				OnClick(entry.refresh)
		}
	})
}

func (entry *mapChangesState) drawing(
	c *ui.Context,
	state *editor.State,
	disabled bool,
) {
	if entry.imageErr != nil {
		ui.Text(c, components.ErrorText(entry.imageErr)).
			TextColor(c.Theme().Danger)
		return
	}
	rows := entry.data.Entries()
	overlay := mapview.Overlay{
		Title: locales.MapChangePreview(),
		Help:  locales.MapChangePaintHelp(),
	}
	if len(rows) > 0 {
		overlay.Regions = []mapview.Region{
			{
				Bounds:   rows[entry.selected].Bounds,
				Index:    rows[entry.selected].ID,
				Selected: true,
			},
		}
	}
	overlay.Drawing = &mapview.Drawing{
		Tool: mapview.Tool(
			entry.tool,
		),
		Enabled: !disabled && !entry.showBase && len(rows) > 0,
		Brush:   entry.brush,
		Pick: func(point image.Point) {
			if tile, ok := entry.data.Cell(
				entry.selected,
				point.X,
				point.Y,
			); ok &&
				tile == 0 {
				entry.tile = maps.Cell{}
				entry.brush = nil
			} else {
				entry.choose(entry.image, point)
			}
			entry.tool = int(mapview.Brush)
		},
		Commit: func(points []image.Point) {
			if state.Busy || !entry.data.Current(state.Session.Maps) {
				return
			}
			if mapview.Tool(entry.tool) == mapview.Rectangle {
				a, b := points[0], points[len(points)-1]
				state.Err = entry.data.Fill(
					entry.selected,
					image.Rect(
						min(a.X, b.X),
						min(a.Y, b.Y),
						max(a.X, b.X)+1,
						max(a.Y, b.Y)+1,
					),
					entry.tile.Tile,
				)
			} else {
				state.Err = entry.data.Stroke(
					entry.selected,
					points,
					entry.tile.Tile,
				)
			}
			if state.Err == nil {
				entry.refresh()
			}
		},
	}
	entry.view.Render(c.Key("change-map"), entry.image, nil, overlay)
}
