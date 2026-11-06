package chapters

import (
	"fmt"

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
	jump func(records.Kind, int),
	openText func(uint16),
) {
	events := entry.data.Events()
	condition := events[entry.selected]
	ui.Column(c.Key(fmt.Sprintf("condition-%d", entry.selected))).
		Gap(8).
		Children(func() {
			v.eventFields(c, state, entry, jump)
			switch {
			case v.mode == 5:
				v.objectiveDetails(
					c,
					state,
					entry,
					condition,
					openText,
				)
			case entry.data.Table() == event.TalkEvents:
				v.talkDetails(c, state, entry, condition, openText)
			default:
				v.eventLinks(c, state, condition)
				if unit := v.eventUnit(entry); unit != nil {
					ui.Text(c, locales.TurnNewUnits()).
						Bold().
						Tooltip(locales.TurnNewHelp())
					v.unitInspector(c, state, unit, jump)
				}
			}
			if v.mode != 5 && v.openScript != nil && condition.Address > 0 &&
				condition.Script >= 0xC0 {
				ui.Row(c).Children(func() {
					ui.Button(c, locales.ScriptEditor()).Disabled(state.Busy).
						OnClick(func() { v.openScript(row.ID, condition.Address+4) })
				})
			}
		})
}

func (v *Page) eventUnit(entry *eventState) *entryDraft {
	data := entry.data.Deployment(entry.selected)
	if data == nil {
		return nil
	}
	unit := entry.units[data]
	if unit == nil {
		unit = &entryDraft{data: data, lastSelected: -1}
		entry.units[data] = unit
	}
	return unit
}

func (v *Page) eventScene(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *eventState,
) {
	events := entry.data.Events()
	condition := events[entry.selected]
	overlay := areaOverlay(entry, events)
	if unit := v.eventUnit(entry); unit != nil {
		v.renderMap(c, state, row, v.unitMap(unit, overlay.Regions))
		v.placeActions(c, state, unit)
	} else {
		v.renderMap(c, state, row, overlay)
	}
	if entry.data.Table() == event.AlwaysEvents && condition.Kind == 0x0B {
		v.areaControls(c, state, entry, condition)
	}
}

func (v *Page) eventFields(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	jump func(records.Kind, int),
) {
	fields := entry.data.Fields(entry.selected)
	if len(fields) == 0 {
		ui.Text(c, locales.TurnOpaque()).TextColor(c.Theme().TextMuted)
		return
	}
	ui.Row(c.Key("condition-fields")).
		Wrap().
		Gap(12).
		AlignItems(ui.Start).
		Children(func() {
			for _, field := range fields {
				width := float32(132)
				switch field.ID {
				case event.TalkInitiator, event.TalkTarget:
					width = 260
				case event.TurnPhase, event.TurnRoute, event.TalkMode:
					width = 180
				}
				ui.Column(c.Key(field.ID)).
					Width(width).
					MaxWidthPercent(100).
					Gap(4).
					Children(func() {
						ui.Text(c, locales.Label(field.ID)).
							Tooltip(v.conditionHelp(entry))
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							v.eventNumber(c, state, entry, field)
							if field.ID == event.TalkInitiator ||
								field.ID == event.TalkTarget {
								value, _ := entry.data.Value(
									entry.selected,
									field.ID,
								)
								_, target, _, err := state.Session.Records.Resolve(
									records.Units,
									int(value),
									"id",
								)
								components.ReferenceLink(
									c,
									locales.OpenReference(
										characterCaption(
											state.Session,
											int(value),
											false,
										),
									),
									state.Busy || value == 0 || err != nil ||
										jump == nil,
									func() { jump(records.Units, target.ID) },
								)
							}
						})
					})
			}
		})
}

func eventTexts(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	event event.Condition,
	openText func(uint16),
) {
	components.TextReferences(
		c.Key("event-texts"),
		state,
		&entry.textReference,
		event.Texts,
		openText,
	)
}
