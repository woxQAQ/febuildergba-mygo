package chapterview

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *View) unitScene(
	c *ui.Context,
	state *components.State,
	row game.Row,
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
		point := units[entry.selected].Start
		if v.positionMode == 1 && !units[entry.selected].RelativeEnd {
			point = units[entry.selected].End
		}
		mapOverlay.Focus = new(point)
	}
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
					v.mapView.Render(c, state.Session, row, mapOverlay)
				})
			ui.Column(c.Key("units")).
				MinWidth(280).
				Grow(1).
				Gap(8).
				Children(func() {
					v.unitList(c, state, entry, units)
				})
		})
}

func (v *View) unitActions(
	c *ui.Context,
	state *components.State,
	entry *entryDraft,
) {
	disabled := state.Busy || !entry.data.Current(state.Session)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.DeployAdd()).Disabled(disabled).OnClick(func() {
			state.Err = entry.data.Add()
			entry.selected = max(0, entry.data.Count()-1)
			entry.preview, entry.inputs = nil, nil
		})
		ui.Button(c, locales.DeployDuplicate()).
			Disabled(disabled || entry.data.Count() == 0).
			OnClick(func() {
				state.Err = entry.data.Duplicate(entry.selected)
				entry.selected = max(0, entry.data.Count()-1)
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
				entry.preview, entry.inputs = nil, nil
			})
		point, selected := v.mapView.Selection()
		ui.Button(c, locales.DeployPlaceStart()).
			Disabled(disabled || !selected || entry.data.Count() == 0).
			OnClick(func() { v.place(state, entry, point, false) })
		units := entry.data.Units()
		relative := entry.selected < len(units) &&
			units[entry.selected].RelativeEnd
		ui.Button(c, locales.DeployPlaceEnd()).
			Disabled(disabled || !selected || entry.data.Count() == 0 || relative).
			OnClick(func() { v.place(state, entry, point, true) })
	})
}
