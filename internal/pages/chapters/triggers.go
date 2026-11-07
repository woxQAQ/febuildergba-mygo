package chapters

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type triggerState struct {
	table           event.Table
	index, selected int
	list            ui.ListState
	newType         string
}

type triggerRow struct {
	table     event.Table
	index     int
	condition event.Condition
}

func triggerCaption(row triggerRow) (kind, condition string) {
	if row.table == event.TurnEvents {
		if row.condition.Kind != 2 {
			return locales.DeployTurnEvent(), fmt.Sprintf(
				"0x%02X",
				row.condition.Kind,
			)
		}
		return locales.TurnEvents(), turnInterval(
			row.condition.Start,
			row.condition.End,
		) + " / " + phaseLabel(
			row.condition.Phase,
		)
	}
	if row.condition.Kind == 0x0B {
		return locales.AreaEvents(), fmt.Sprintf(
			"%d,%d → %d,%d",
			row.condition.X1,
			row.condition.Y1,
			row.condition.X2,
			row.condition.Y2,
		)
	}
	return locales.ChapterAlwaysConditions(), objectiveConditionCaption(
		row.condition,
	)
}

func (v *Page) triggerEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	width float32,
	jump func(records.Kind, int),
	openText func(uint16),
) {
	selection := v.triggers[row.ID]
	if selection == nil {
		selection = &triggerState{}
		v.triggers[row.ID] = selection
	}
	entries := make(map[event.Table]*eventState, 2)
	var rows []triggerRow
	for _, table := range []event.Table{event.TurnEvents, event.AlwaysEvents} {
		entry, err := v.eventDraft(state, eventKey{row.ID, table})
		if err != nil {
			ui.Text(c, v.labelsFor(table).title+": "+components.ErrorText(err)).
				TextColor(c.Theme().Danger).Tooltip(err.Error())
			continue
		}
		entries[table] = entry
		for index, condition := range entry.data.Events() {
			rows = append(rows, triggerRow{table, index, condition})
		}
	}
	selection.selected = min(selection.selected, max(0, len(rows)-1))
	for index, item := range rows {
		if item.table == selection.table && item.index == selection.index {
			selection.selected = index
			break
		}
	}
	if len(rows) > 0 {
		item := rows[selection.selected]
		selection.table, selection.index = item.table, item.index
		entries[item.table].selected = item.index
	}
	ui.Column(c.Key(selection)).Gap(12).Children(func() {
		if entry := entries[event.AlwaysEvents]; entry != nil {
			v.objectiveSummary(c, state, row, entry, openText)
		}
		mapWorkspace(c, width, func(c *ui.Context) {
			if len(rows) == 0 {
				v.renderMap(c, state, row, mapview.Overlay{})
				return
			}
			entry := entries[selection.table]
			v.eventScene(c, state, row, entry, func() {
				selection.index = entry.selected
			})
		}, func(c *ui.Context) {
			ui.Text(c, locales.ChapterTriggers()).Bold()
			if len(rows) > 0 {
				selection.list.Selected = &selection.selected
				selection.list.Key = func(index int) any {
					return struct {
						table event.Table
						index int
					}{rows[index].table, rows[index].index}
				}
				ui.Table(c.Key("triggers"), &selection.list, []ui.TableColumn{
					{
						ID:    "type",
						Title: locales.ChapterTriggerType(),
						Width: 110,
						Fixed: true,
					},
					{ID: "condition", Title: locales.ChapterTriggerCondition()},
					{
						ID:    "flag",
						Title: locales.Label(event.EventFlag),
						Width: 76,
						Fixed: true,
					},
				}, len(rows), func(index, column int) {
					kind, condition := triggerCaption(rows[index])
					switch column {
					case 0:
						ui.Text(c, kind).SingleLine()
					case 1:
						ui.Text(c, condition).SingleLine().Tooltip(condition)
					case 2:
						ui.Textf(c, "0x%04X", rows[index].condition.Flag).
							FontFeatures("tnum")
					}
				}).Height(min(220, c.Theme().Space(9)*float32(len(rows)+1))).Label(locales.ChapterTriggers())
				item := rows[selection.selected]
				selection.table, selection.index = item.table, item.index
				entries[item.table].selected = item.index
			}
			v.triggerActions(c, state, selection, entries, len(rows) > 0)
			if entry := entries[selection.table]; entry != nil &&
				len(entry.data.Events()) > 0 {
				ui.Column(c.Key(eventKey{row.ID, selection.table})).
					Gap(8).
					Children(func() {
						if !entry.data.Current(state.Session.Events) {
							ui.Text(c, locales.DeployStale()).
								TextColor(c.Theme().Danger)
						}
						v.eventDetails(c, state, row, entry, jump, openText)
					})
			}
			for _, table := range []event.Table{event.TurnEvents, event.AlwaysEvents} {
				entry := entries[table]
				if entry == nil ||
					!entry.data.Modified() && !entry.invalid() &&
						entry.data.Current(state.Session.Events) {
					continue
				}
				ui.Column(c.Key(table)).Gap(8).Children(func() {
					ui.Text(c, v.labelsFor(table).title).Bold()
					v.eventActions(c, state, row, entry)
				})
			}
		})
	})
}

func (v *Page) triggerActions(
	c *ui.Context,
	state *editor.State,
	selection *triggerState,
	entries map[event.Table]*eventState,
	hasSelection bool,
) {
	templates := []struct {
		label, help string
		table       event.Table
		objective   bool
		template    event.ObjectiveTemplate
	}{
		{locales.TurnAdd(), locales.TurnNewHelp(), event.TurnEvents, false, 0},
		{locales.AreaAdd(), locales.AreaScope(), event.AlwaysEvents, false, 0},
		{
			locales.ObjectiveAddBoss(),
			locales.ObjectiveBossHelp(),
			event.AlwaysEvents,
			true,
			event.BossVictory,
		},
		{
			locales.ObjectiveAddRout(),
			locales.ObjectiveRoutHelp(),
			event.AlwaysEvents,
			true,
			event.RoutVictory,
		},
		{
			locales.ObjectiveAddFailure(),
			locales.ObjectiveFailureHelp(),
			event.AlwaysEvents,
			true,
			event.FlagFailure,
		},
	}
	labels := make([]string, len(templates))
	choice := 0
	for index, template := range templates {
		labels[index] = template.label
		if selection.newType == template.label {
			choice = index
		}
	}
	selection.newType = labels[choice]
	item := templates[choice]
	entry := entries[item.table]
	available := entry != nil && entry.data.Current(state.Session.Events)
	if available && item.objective {
		available = false
		kind := event.ChapterEndingScript
		if item.template == event.FlagFailure {
			kind = event.GameOverScript
		}
		for _, script := range entry.data.Scripts() {
			available = available || script.Kind == kind
		}
	}
	ui.Row(c.Key("trigger-actions")).Wrap().Gap(6).Children(func() {
		ui.Select(c.Key("new-type"), &selection.newType, labels).
			Label(locales.ChapterTriggerNewType()).
			Width(210).MaxWidthPercent(100).Disabled(state.Busy)
		components.ActionButton(c, "+", item.label).Tooltip(item.help).
			Disabled(state.Busy || !available).
			OnClick(func() {
				var err error
				if item.objective {
					err = entry.data.AddObjective(
						state.Session.Events,
						item.template,
					)
				} else {
					err = entry.data.Add(state.Session.Events)
				}
				if state.SetErr(err); err == nil {
					entry.selected = len(entry.data.Events()) - 1
					entry.preview = nil
					selection.table, selection.index = item.table, entry.selected
				}
			})
		selected := entries[selection.table]
		components.ActionButton(c, "−", locales.AreaRemove()).
			Disabled(state.Busy || !hasSelection || selected == nil || !selected.data.Current(state.Session.Events)).
			OnClick(func() {
				if state.SetErr(selected.removeSelected()); state.Err == nil {
					selection.index = min(
						selection.index,
						max(0, len(selected.data.Events())-1),
					)
				}
			})
	})
}
