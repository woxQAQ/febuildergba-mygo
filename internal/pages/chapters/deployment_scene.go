package chapters

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

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
