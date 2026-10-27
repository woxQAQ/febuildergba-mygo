package chapterview

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *View) eventDetails(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *eventState,
	advanced bool,
	jump func(game.Kind, int),
	openHex func(int, int),
) {
	events := entry.data.Events()
	event := events[entry.selected]
	ui.Column(c.Key(fmt.Sprintf("condition-%d", entry.selected))).
		Gap(8).
		Children(func() {
			fields := entry.data.Fields(entry.selected)
			if len(fields) == 0 {
				ui.Text(c, locales.TurnOpaque()).TextColor(c.Theme().TextMuted)
			} else {
				ui.Row(c).Wrap().Gap(16).Children(func() {
					for _, field := range fields {
						ui.Row(c.Key(field.ID)).
							Width(260).
							MaxWidthPercent(100).
							Gap(8).
							AlignItems(ui.Center).
							Children(func() {
								ui.Text(c, locales.Label(field.ID)).
									Grow(1).
									Tooltip(labelsFor(entry.data.Table()).help)
								v.eventNumber(c, state, entry, field)
							})
					}
				})
			}
			overlay := areaOverlay(entry, events)
			var unit *entryDraft
			units := entry.data.Deployment(entry.selected)
			if units != nil {
				unit = entry.units[units]
				if unit == nil {
					unit = &entryDraft{data: units, lastSelected: -1}
					entry.units[units] = unit
				}
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, locales.TurnNewUnits()).
						Bold().
						Tooltip(locales.TurnNewHelp())
					ui.Tabs(c.Key("event-position-mode"), &v.positionMode, locales.DeployStart(), locales.DeployEnd()).
						Label(locales.DeployPosition()).
						OnChange(func() { unit.lastSelected = -1 })
				})
				v.unitScene(c, state, row, unit, overlay.Regions)
			} else {
				v.eventLinks(c, state, event)
				v.mapView.Render(c, state.Session, row, overlay)
			}
			if entry.data.Table() == game.AlwaysEvents && event.Kind == 0x0B {
				v.areaControls(c, state, entry, event)
			}
			if unit != nil {
				v.unitActions(c, state, unit)
				v.fields(c, state, unit, advanced, jump)
			}
			if advanced && event.Address > 0 {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					ui.Button(c, locales.EditBytes()).
						Disabled(state.Busy).
						OnClick(func() { openHex(event.Address, len(event.Raw)) })
					ui.Textf(c, "0x%08X → 0x%08X · % X", event.Address, event.Script, event.Raw).
						Selectable().
						Font("monospace")
				})
			}
		})
}
