package chapters

import (
	"fmt"
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func locationCaption(condition event.Condition) string {
	if condition.Location.Kind != "" {
		return locales.Label(string(condition.Location.Kind))
	}
	return fmt.Sprintf(
		"0x%02X / 0x%04X",
		condition.Kind,
		condition.Location.Command,
	)
}

func (v *Page) locationAdd(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
) {
	kinds := []event.LocationKind{
		event.House,
		event.Village,
		event.Chest,
		event.Door,
		event.Seize,
	}
	labels := make([]string, len(kinds))
	for i, kind := range kinds {
		labels[i] = locales.Label(string(kind))
	}
	if !slices.Contains(labels, entry.locationTemplate) {
		entry.locationTemplate = labels[0]
	}
	disabled := state.Busy || !entry.data.Current(state.Session.Events)
	ui.Select(c.Key("location-template"), &entry.locationTemplate, labels).
		Width(180).Label(locales.LocationType()).Disabled(disabled)
	ui.Button(c, locales.LocationAdd()).
		Tooltip(locales.LocationScope()).
		Disabled(disabled).
		OnClick(func() {
			kind := kinds[max(0, slices.Index(labels, entry.locationTemplate))]
			state.Err = entry.data.AddLocation(state.Session.Events, kind)
			if state.Err == nil {
				entry.selected = len(entry.data.Events()) - 1
				entry.preview = nil
				entry.lastSelected = -1
			}
		})
	ui.Button(c, locales.LocationCopy()).
		Disabled(disabled || len(entry.data.Events()) == 0).
		OnClick(func() {
			state.Err = entry.data.CopyLocation(entry.selected)
			if state.Err == nil {
				entry.selected = len(entry.data.Events()) - 1
				entry.preview = nil
				entry.lastSelected = -1
			}
		})
}

func locationOverlay(entry *eventState) mapview.Overlay {
	overlay := mapview.Overlay{}
	events := entry.data.Events()
	for i, condition := range events {
		if condition.Location.Kind == "" {
			continue
		}
		point := condition.Location.Position
		overlay.Regions = append(overlay.Regions, mapview.Region{
			Bounds: image.Rect(point.X, point.Y, point.X+1, point.Y+1),
			Index:  i, Selected: i == entry.selected,
		})
	}
	if entry.lastSelected != entry.selected {
		entry.lastSelected = entry.selected
		if events[entry.selected].Location.Kind != "" {
			overlay.Focus = new(events[entry.selected].Location.Position)
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
		next := slices.Index(matches, entry.selected) + 1
		entry.selected = matches[next%len(matches)]
	}
	return overlay
}

func (v *Page) locationDetails(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *eventState,
	condition event.Condition,
	openText func(uint16),
) {
	disabled := state.Busy || !entry.data.Current(state.Session.Events)
	switch condition.OperandKind {
	case event.ConditionScript:
		v.conditionScript(c, state, entry, condition)
		if id, created := entry.data.Dialog(entry.selected); created {
			v.dialoguePicker(c, state, entry, id)
		}
		v.eventLinks(c, state, condition)
		eventTexts(c, state, entry, condition, openText)
	case event.RewardTarget:
		location := condition.Location
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if location.Item == location.GoldItem {
				ui.Textf(c, "%s: %d", locales.ChestGold(), location.Money)
			} else {
				ui.Textf(
					c,
					"%04X · %s",
					location.Item,
					nameOf(state.Session, records.Items, location.Item),
				)
			}
			ui.Button(c, locales.ChestUseGold()).Disabled(disabled).
				OnClick(func() {
					state.Err = entry.data.SetChestReward(
						entry.selected,
						location.GoldItem,
						max(location.GoldMinimum, location.Money),
					)
					if state.Err == nil {
						entry.preview = nil
						delete(
							entry.inputs,
							inputKey{entry.selected, event.ChestItem},
						)
						delete(
							entry.inputs,
							inputKey{entry.selected, event.ChestMoney},
						)
					}
				})
		})
		ui.Textf(c, "%s: %d–65535 · %s: 1–%d", locales.ChestGold(), location.GoldMinimum, locales.Label(event.ChestItem), location.ItemMaximum).
			FontSize(12)
		ui.Text(c, locales.ChestRewardHelp()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	case event.ShopTarget, event.RandomRewardTarget:
		ui.Text(c, locales.LocationResourceReadonly()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
	if condition.Flag == 0 && len(entry.data.Fields(entry.selected)) > 0 {
		ui.Text(c, locales.LocationRepeat()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
	v.renderMap(c, state, row, locationOverlay(entry))
	point, selected := v.mapView.Selection()
	ui.Button(c, locales.LocationPlace()).
		Disabled(disabled || !selected || len(entry.data.Fields(entry.selected)) == 0).
		OnClick(func() {
			state.Err = entry.data.SetLocationPosition(entry.selected, point)
			if state.Err == nil {
				delete(entry.inputs, inputKey{entry.selected, event.LocationX})
				delete(entry.inputs, inputKey{entry.selected, event.LocationY})
				entry.preview = nil
			}
		})
}
