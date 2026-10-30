package chapters

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *Page) eventDetails(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *eventState,
	advanced bool,
	jump func(records.Kind, int),
	openHex func(int, int),
	openText func(uint16),
) {
	events := entry.data.Events()
	condition := events[entry.selected]
	ui.Column(c.Key(fmt.Sprintf("condition-%d", entry.selected))).
		Gap(8).
		Children(func() {
			fields := entry.data.Fields(entry.selected)
			if len(fields) == 0 {
				ui.Text(c, locales.TurnOpaque()).TextColor(c.Theme().TextMuted)
			} else {
				ui.Row(c).Wrap().Gap(16).Children(func() {
					for _, field := range fields {
						width := float32(260)
						if entry.data.Table() == event.TalkEvents {
							width = 300
						}
						ui.Row(c.Key(field.ID)).
							Width(width).
							MaxWidthPercent(100).
							Gap(8).
							AlignItems(ui.Center).
							Children(func() {
								ui.Text(c, locales.Label(field.ID)).
									Grow(1).
									Tooltip(v.conditionHelp(entry))
								v.eventNumber(c, state, entry, field)
							})
					}
				})
			}
			switch {
			case v.mode == 5:
				v.objectiveDetails(
					c,
					state,
					entry,
					condition,
					openHex,
					openText,
				)
			case entry.data.Table() == event.LocationEvents:
				v.locationDetails(c, state, row, entry, condition, openText)
			case entry.data.Table() == event.TalkEvents:
				v.talkDetails(c, state, entry, condition, jump, openText)
			default:
				v.eventScene(c, state, row, entry, events, advanced, jump)
			}
			if v.mode != 5 && v.openScript != nil && condition.Address > 0 &&
				condition.Script >= 0xC0 {
				ui.Button(c, locales.ScriptEditor()).
					Disabled(state.Busy || (entry.data.Table() == event.LocationEvents && entry.data.Modified())).
					OnClick(func() { v.openScript(row.ID, condition.Address+4) })
			}
			if advanced && condition.Address > 0 {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					ui.Button(c, locales.EditBytes()).
						Disabled(state.Busy).
						OnClick(func() { openHex(condition.Address, len(condition.Raw)) })
					ui.Textf(c, "0x%08X → 0x%08X · % X", condition.Address, condition.Operand, condition.Raw).
						Selectable().
						Font("monospace")
				})
			}
		})
}

func (v *Page) eventScene(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *eventState,
	events []event.Condition,
	advanced bool,
	jump func(records.Kind, int),
) {
	condition := events[entry.selected]
	overlay := areaOverlay(entry, events)
	var unit *entryDraft
	if units := entry.data.Deployment(entry.selected); units != nil {
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
		v.eventLinks(c, state, condition)
		v.renderMap(c, state, row, overlay)
	}
	if entry.data.Table() == event.AlwaysEvents && condition.Kind == 0x0B {
		v.areaControls(c, state, entry, condition)
	}
	if unit != nil {
		v.unitActions(c, state, unit)
		v.fields(c, state, unit, advanced, jump)
	}
}

func eventTexts(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	event event.Condition,
	openText func(uint16),
) {
	if len(event.Texts) == 0 {
		return
	}
	choices := make([]string, len(event.Texts))
	for i, id := range event.Texts {
		choices[i] = fmt.Sprintf("%04X", id)
	}
	if !slices.Contains(choices, entry.textReference) {
		entry.textReference = choices[0]
	}
	ui.Row(c.Key("text-reference")).
		Wrap().
		Gap(8).
		AlignItems(ui.Center).
		Children(func() {
			if len(choices) > 1 {
				ui.Text(c, locales.EventTextRefs())
				ui.Select(c.Key("reference-id"), &entry.textReference, choices).
					Width(100).
					Label(locales.TextEditor() + " / " + locales.EventTextRefs()).
					Disabled(state.Busy)
			}
			index := max(0, slices.Index(choices, entry.textReference))
			id := event.Texts[index]
			ui.Button(c, fmt.Sprintf("%s · %04X", locales.TextEditor(), id)).
				Tooltip(locales.TalkTextHelp()).Disabled(state.Busy).
				OnClick(func() { openText(id) })
		})
	index := max(0, slices.Index(choices, entry.textReference))
	value, err := state.Session.Texts.Read(event.Texts[index])
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
	} else {
		ui.Scroll(c.Key("text-preview")).Height(112).Children(func() {
			ui.Text(c, value.Content).Selectable().FontSize(13)
		})
	}
}
