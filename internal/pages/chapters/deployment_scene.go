package chapters

import (
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *Page) overlay(
	entry *entryDraft,
	units []deployment.Unit,
) mapview.Overlay {
	var markers []mapview.Marker
	for i, unit := range units {
		point := unit.Start
		if v.positionMode == 1 {
			point = unit.End
		}
		markers = append(
			markers,
			mapview.Marker{
				Index:    i,
				Position: point,
				Faction:  unit.Faction,
				Selected: i == entry.selected,
			},
		)
	}
	slices.SortStableFunc(markers, func(a, b mapview.Marker) int {
		if a.Selected == b.Selected {
			return 0
		}
		if a.Selected {
			return 1
		}
		return -1
	})
	return mapview.Overlay{Markers: markers, Choose: func(point image.Point) {
		for offset := 1; offset <= len(units); offset++ {
			index := (entry.selected + offset) % len(units)
			unit := units[index]
			position := unit.Start
			if v.positionMode == 1 {
				position = unit.End
			}
			if position == point {
				entry.selected = index
				return
			}
		}
	}}
}

func (v *Page) unitList(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	units []deployment.Unit,
) {
	if len(units) == 0 {
		ui.Text(c, locales.DeployEmpty()).TextColor(c.Theme().TextMuted)
		return
	}
	entry.list.Selected = &entry.selected
	entry.list.Key = func(index int) any { return index }
	columns := []ui.TableColumn{
		{ID: "index", Title: "#", Width: 30, Fixed: true},
		{ID: "name", Title: locales.Name()},
		{
			ID:    "position",
			Title: locales.DeployPosition(),
			Width: 62,
			Fixed: true,
		},
	}
	ui.Table(c.Key("deployment-units"), &entry.list, columns, len(units), func(row, col int) {
		unit := units[row]
		switch col {
		case 0:
			ui.Textf(c, "%02X", row).FontFeatures("tnum")
		case 1:
			ui.Text(c, nameOf(state.Session, records.Units, unit.Character)).
				SingleLine()
		case 2:
			point := unit.Start
			if v.positionMode == 1 {
				point = unit.End
			}
			ui.Textf(c, "%d, %d", point.X, point.Y).FontFeatures("tnum")
		}
	}).
		Height(min(180, c.Theme().Space(9)*float32(len(units)+1))).
		Label(locales.Deployments() + " · " + locales.Name()).
		Tooltip(locales.DeployPickHelp())
}

func (v *Page) place(
	state *editor.State,
	entry *entryDraft,
	point image.Point,
	end bool,
) {
	x, y := "deploy_start_x", "deploy_start_y"
	if end {
		x, y = "deploy_end_x", "deploy_end_y"
	}
	if err := entry.data.Set(entry.selected, x, int64(point.X)); err != nil {
		state.SetErr(err)
		return
	}
	state.SetErr(entry.data.Set(entry.selected, y, int64(point.Y)))
	entry.preview, entry.inputs = nil, nil
}

func (v *Page) unitScene(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *entryDraft,
	regions []mapview.Region,
	width float32,
	jump func(records.Kind, int),
) {
	units := entry.data.Units()
	if entry.selected >= len(units) {
		entry.selected = max(0, len(units)-1)
	}
	mapOverlay := v.overlay(entry, units)
	mapOverlay.Regions = regions
	if entry.selected != entry.lastSelected && entry.selected < len(units) {
		entry.lastSelected = entry.selected
		entry.list.ScrollIntoView(entry.selected)
		if entry.pathMode == 1 {
			entry.pathState().lastSelected = -1
		}
		point := units[entry.selected].Start
		if v.positionMode == 1 {
			point = units[entry.selected].End
		}
		mapOverlay.Focus = new(point)
	}
	v.pathOverlay(entry, &mapOverlay, units)
	mapWidth := max(160, (width-16)*0.45)
	inspectorWidth := max(280, width-mapWidth-16)
	mapOverlay.FullHeight = true
	mapOverlay.Tools = func(c *ui.Context) {
		ui.Tabs(c.Key("position-mode"), &v.positionMode, locales.DeployStart(), locales.DeployEnd()).
			Label(locales.DeployPosition()).
			OnChange(func() { entry.lastSelected = -1 })
	}
	ui.ScrollHorizontal(c.Key("deployment-workspace")).
		Width(width).
		MaxWidthPercent(100).
		Children(func() {
			ui.Row(c.Key("map-and-units")).
				Width(mapWidth + inspectorWidth + 16).
				Gap(16).
				AlignItems(ui.Start).
				Children(func() {
					ui.Column(c.Key("map")).
						Width(mapWidth).
						Shrink(0).
						Gap(8).
						Children(func() {
							v.renderMap(c, state, row, mapOverlay)
							v.placeActions(c, state, entry)
						})
					ui.Column(c.Key("unit-inspector")).
						Width(inspectorWidth).
						Shrink(0).
						Gap(10).
						Children(func() {
							ui.Row(c.Key("unit-list-tools")).
								Wrap().
								Gap(6).
								AlignItems(ui.Center).
								Children(func() {
									ui.Text(c, locales.MoveUnits()).
										Bold().
										Grow(1)
									ui.Text(c, locales.RecordSummary(len(units))).
										FontSize(components.FontCaption).
										TextColor(c.Theme().TextMuted)
									v.unitActions(c, state, entry)
								})
							v.unitList(c, state, entry, units)
							v.unitSidebar(c, state, entry, units, jump)
						})
				})
		})
}

func (v *Page) unitActions(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
) {
	disabled := state.Busy || !entry.data.Current(state.Session.Deployments)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		components.ActionButton(c, "+", locales.DeployAdd()).
			Disabled(disabled).
			OnClick(func() {
				state.SetErr(entry.data.Add())
				entry.selected = max(0, entry.data.Count()-1)
				entry.paths = nil
				entry.preview, entry.inputs = nil, nil
			})
		components.ActionButton(c, "⧉", locales.DeployDuplicate()).
			Disabled(disabled || entry.data.Count() == 0).
			OnClick(func() {
				state.SetErr(entry.data.Duplicate(entry.selected))
				entry.selected = max(0, entry.data.Count()-1)
				entry.paths = nil
				entry.preview, entry.inputs = nil, nil
			})
		components.ActionButton(c, "−", locales.DeployRemove()).
			Disabled(disabled || entry.data.Count() == 0).
			OnClick(func() {
				state.SetErr(entry.data.Remove(entry.selected))
				entry.selected = min(
					entry.selected,
					max(0, entry.data.Count()-1),
				)
				entry.paths = nil
				entry.preview, entry.inputs = nil, nil
			})
	})
}

func (v *Page) placeActions(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
) {
	disabled := state.Busy || !entry.data.Current(state.Session.Deployments)
	ui.Row(c.Key("place-unit")).Wrap().Gap(8).Children(func() {
		point, selected := v.mapView.Selection()
		ui.Button(c, locales.DeployPlaceStart()).
			Disabled(disabled || !selected || entry.data.Count() == 0).
			OnClick(func() { v.place(state, entry, point, false) })
		ui.Button(c, locales.DeployPlaceEnd()).
			Disabled(disabled || !selected || entry.data.Count() == 0).
			OnClick(func() { v.place(state, entry, point, true) })
	})
}
