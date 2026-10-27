package chapterview

import (
	"image"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func areaOverlay(
	entry *eventState,
	events []game.EventCondition,
) mapview.Overlay {
	if entry.data.Table() != game.AlwaysEvents {
		return mapview.Overlay{}
	}
	overlay := mapview.Overlay{}
	for i, event := range events {
		if event.Kind != 0x0B {
			continue
		}
		overlay.Regions = append(
			overlay.Regions,
			mapview.Region{
				Bounds:   event.Area(),
				Index:    i,
				Selected: i == entry.selected,
			},
		)
	}
	if entry.lastSelected != entry.selected {
		entry.lastSelected = entry.selected
		if bounds := events[entry.selected].Area(); !bounds.Empty() {
			overlay.Focus = new(
				image.Pt(max(0, bounds.Min.X), max(0, bounds.Min.Y)),
			)
		}
	}
	overlay.Choose = func(point image.Point) {
		var matches []int
		for _, region := range overlay.Regions {
			if point.In(region.Bounds) {
				matches = append(matches, region.Index)
			}
		}
		if len(matches) == 0 {
			return
		}
		next := matches[0]
		for i, index := range matches {
			if index == entry.selected {
				next = matches[(i+1)%len(matches)]
				break
			}
		}
		entry.selected = next
	}
	return overlay
}

func (v *View) areaControls(
	c *ui.Context,
	state *components.State,
	entry *eventState,
	event game.EventCondition,
) {
	point, selected := v.mapView.Selection()
	disabled := state.Busy || !selected || !entry.data.Current(state.Session)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.AreaPlaceFirst()).
			Disabled(disabled).
			OnClick(func() { v.placeArea(state, entry, point, false) })
		ui.Button(c, locales.AreaPlaceLast()).
			Disabled(disabled).
			OnClick(func() { v.placeArea(state, entry, point, true) })
	})
	if event.Area().Empty() {
		ui.Text(c, locales.AreaEmpty()).TextColor(c.Theme().TextMuted)
	}
	if event.Flag == 0 {
		ui.Text(c, locales.AreaRepeats()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
}

func (v *View) placeArea(
	state *components.State,
	entry *eventState,
	point image.Point,
	last bool,
) {
	x, y := game.AreaX1, game.AreaY1
	if last {
		x, y = game.AreaX2, game.AreaY2
	}
	for field, value := range map[string]int64{x: int64(point.X), y: int64(point.Y)} {
		if state.Err = entry.data.Set(
			entry.selected,
			field,
			value,
		); state.Err != nil {
			return
		}
		delete(entry.inputs, inputKey{entry.selected, field})
	}
	entry.preview = nil
}
