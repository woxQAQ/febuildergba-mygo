package chapters

import (
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
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
		{ID: "index", Title: "#", Width: 36, Fixed: true},
		{ID: "name", Title: locales.Name()},
		{ID: "level", Title: locales.DeployLevel(), Width: 44, Fixed: true},
		{
			ID:    "position",
			Title: locales.DeployPosition(),
			Width: 76,
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
			ui.Textf(c, "%d", unit.Level).FontFeatures("tnum")
		case 3:
			point := unit.Start
			if v.positionMode == 1 {
				point = unit.End
			}
			ui.Textf(c, "%d, %d", point.X, point.Y).FontFeatures("tnum")
		}
	}).
		Height(230).
		Label(locales.Deployments() + " · " + locales.Name()).
		Tooltip(locales.DeployPickHelp())
	ui.Text(c, locales.RecordSummary(len(units))).
		FontSize(12).
		TextColor(c.Theme().TextMuted)
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
		state.Err = err
		return
	}
	state.Err = entry.data.Set(entry.selected, y, int64(point.Y))
	entry.preview, entry.inputs = nil, nil
}

func (v *Page) unitScene(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *entryDraft,
	regions []mapview.Region,
) {
	units := entry.data.Units()
	if entry.selected >= len(units) {
		entry.selected = max(0, len(units)-1)
	}
	mapOverlay := v.overlay(entry, units)
	mapOverlay.Regions = regions
	if entry.selected != entry.lastSelected && entry.selected < len(units) {
		entry.lastSelected = entry.selected
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
	ui.Row(c.Key("map-and-units")).
		Wrap().
		Gap(16).
		AlignItems(ui.Start).
		Children(func() {
			ui.Column(c.Key("map")).
				Width(420).
				MaxWidthPercent(100).
				MinWidth(0).
				Children(func() {
					v.renderMap(c, state, row, mapOverlay)
				})
			ui.Column(c.Key("units")).
				MinWidth(280).
				Grow(1).
				Gap(8).
				Children(func() {
					v.unitSidebar(c, state, entry, units)
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
		ui.Button(c, locales.DeployAdd()).Disabled(disabled).OnClick(func() {
			state.Err = entry.data.Add()
			entry.selected = max(0, entry.data.Count()-1)
			entry.paths = nil
			entry.preview, entry.inputs = nil, nil
		})
		ui.Button(c, locales.DeployDuplicate()).
			Disabled(disabled || entry.data.Count() == 0).
			OnClick(func() {
				state.Err = entry.data.Duplicate(entry.selected)
				entry.selected = max(0, entry.data.Count()-1)
				entry.paths = nil
				entry.preview, entry.inputs = nil, nil
			})
		ui.Button(c, locales.DeployRemove()).
			Disabled(disabled || entry.data.Count() == 0).
			OnClick(func() {
				state.Err = entry.data.Remove(entry.selected)
				entry.selected = min(
					entry.selected,
					max(0, entry.data.Count()-1),
				)
				entry.paths = nil
				entry.preview, entry.inputs = nil, nil
			})
		point, selected := v.mapView.Selection()
		ui.Button(c, locales.DeployPlaceStart()).
			Disabled(disabled || !selected || entry.data.Count() == 0).
			OnClick(func() { v.place(state, entry, point, false) })
		ui.Button(c, locales.DeployPlaceEnd()).
			Disabled(disabled || !selected || entry.data.Count() == 0).
			OnClick(func() { v.place(state, entry, point, true) })
	})
}
