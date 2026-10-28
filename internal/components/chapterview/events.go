package chapterview

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type eventKey struct {
	chapter int
	table   game.EventTable
}

type eventState struct {
	data                         *game.EventDraft
	preview                      *game.EventEdit
	selected, lastSelected, view int
	list                         ui.ListState
	inputs                       map[inputKey]*numberDraft
	units                        map[*game.DeploymentDraft]*entryDraft
	generations                  map[*game.DeploymentDraft]uint64
	textChoices                  []dialogueChoice
	textReference                string
}

type eventLabels struct{ title, add, remove, scope, preview, reload, apply, previewScope, help string }

func (v *View) labelsFor(table game.EventTable) eventLabels {
	if v.mode == 5 {
		return eventLabels{
			title:        locales.Objectives(),
			add:          locales.ObjectiveAddBoss(),
			remove:       locales.TalkRemove(),
			scope:        locales.ObjectiveScope(),
			preview:      locales.ObjectivePreview(),
			reload:       locales.ObjectiveReload(),
			apply:        locales.ObjectiveApply(),
			previewScope: locales.ObjectivePreviewScope(),
			help:         locales.ObjectiveFlagHelp(),
		}
	}
	if table == game.TalkEvents {
		return eventLabels{
			title:        locales.TalkEvents(),
			add:          locales.TalkAdd(),
			remove:       locales.TalkRemove(),
			scope:        locales.TalkScope(),
			preview:      locales.TalkPreview(),
			reload:       locales.TalkReload(),
			apply:        locales.TalkApply(),
			previewScope: locales.TalkPreviewScope(),
			help:         locales.TalkConditionHelp(),
		}
	}
	if table == game.AlwaysEvents {
		return eventLabels{
			title:        locales.AreaEvents(),
			add:          locales.AreaAdd(),
			remove:       locales.AreaRemove(),
			scope:        locales.AreaScope(),
			preview:      locales.AreaPreview(),
			reload:       locales.AreaReload(),
			apply:        locales.AreaApply(),
			previewScope: locales.AreaPreviewScope(),
			help:         locales.AreaConditionHelp(),
		}
	}
	return eventLabels{
		title:        locales.TurnEvents(),
		add:          locales.TurnAdd(),
		remove:       locales.TurnRemove(),
		scope:        locales.TurnScope(),
		preview:      locales.TurnPreview(),
		reload:       locales.TurnReload(),
		apply:        locales.TurnApply(),
		previewScope: locales.TurnPreviewScope(),
		help:         locales.TurnConditionHelp(),
	}
}

func (v *View) eventDraft(
	state *components.State,
	key eventKey,
) (*eventState, error) {
	entry := v.events[key]
	if entry != nil && !entry.data.Modified() && !entry.invalid() &&
		!entry.data.Current(state.Session) {
		delete(v.events, key)
		entry = nil
	}
	if entry == nil {
		data, err := state.Session.NewEventDraft(key.chapter, key.table)
		if err != nil {
			return nil, err
		}
		entry = &eventState{
			data:         data,
			lastSelected: -1,
			units:        make(map[*game.DeploymentDraft]*entryDraft),
		}
		for i := range data.Events() {
			if len(data.Fields(i)) > 0 &&
				(key.table != game.AlwaysEvents || data.Events()[i].Kind == 11) {
				entry.selected = i
				break
			}
		}
		v.events[key] = entry
	}
	for units, generation := range entry.generations {
		if units.Generation() != generation {
			entry.preview = nil
			break
		}
	}
	return entry, nil
}

func (e *eventState) invalid() bool {
	for i := range e.data.Events() {
		if id, created := e.data.Dialog(i); created && id == 0 {
			return true
		}
	}
	for _, input := range e.inputs {
		if input.invalid {
			return true
		}
	}
	for _, unit := range e.units {
		if unit.invalid() {
			return true
		}
	}
	return false
}

func phaseLabel(value int) string {
	switch value {
	case 0:
		return locales.TurnPhasePlayer()
	case 0x40:
		return locales.TurnPhaseAlly()
	case 0x80:
		return locales.TurnPhaseEnemy()
	default:
		return fmt.Sprintf("0x%04X", value)
	}
}

func (v *View) eventEditor(
	c *ui.Context,
	state *components.State,
	row game.Row,
	table game.EventTable,
	advanced bool,
	jump func(game.Kind, int),
	openHex func(int, int),
	openText func(uint16),
) {
	key := eventKey{row.ID, table}
	objective := v.mode == 5
	entry, err := v.eventDraft(state, key)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			TextColor(c.Theme().Danger).
			Tooltip(err.Error())
		if table != game.TalkEvents && !objective {
			v.mapView.Render(c, state.Session, row, mapview.Overlay{})
		}
		return
	}
	labels := v.labelsFor(table)
	stale := !entry.data.Current(state.Session)
	disabled := state.Busy || stale
	events := entry.data.Events()
	entry.selected = min(entry.selected, max(0, len(events)-1))
	if entry.view != v.mode {
		entry.view = v.mode
		for i, event := range events {
			if (objective && (event.Kind == 1 || event.Rout)) ||
				(!objective && table == game.AlwaysEvents && event.Kind == 11) {
				entry.selected = i
				break
			}
		}
	}
	ui.Column(c.Key(key)).Gap(12).Children(func() {
		if objective {
			v.objectiveSummary(c, state, row, entry, openHex, openText)
		}
		v.eventTable(c, entry, events, state.Session)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if objective {
				v.objectiveAdd(c, state, entry)
			} else {
				ui.Button(c, labels.add).
					Tooltip(labels.scope).
					Disabled(disabled).
					OnClick(func() {
						state.Err = entry.data.Add(state.Session)
						entry.selected = len(entry.data.Events()) - 1
						entry.preview = nil
					})
			}
			ui.Button(c, labels.remove).
				Disabled(disabled || len(events) == 0).
				OnClick(func() {
					if units := entry.data.Deployment(
						entry.selected,
					); units != nil {
						delete(entry.units, units)
						delete(entry.generations, units)
					}
					state.Err = entry.data.Remove(entry.selected)
					entry.preview, entry.inputs = nil, nil
					entry.lastSelected = -1
				})
		})
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		if len(events) > 0 {
			v.eventDetails(
				c,
				state,
				row,
				entry,
				advanced,
				jump,
				openHex,
				openText,
			)
		} else if table != game.TalkEvents && !objective {
			v.mapView.Render(c, state.Session, row, mapview.Overlay{})
		}
		v.eventActions(c, state, row, entry)
	})
}

func (v *View) eventTable(
	c *ui.Context,
	entry *eventState,
	events []game.EventCondition,
	session *game.Session,
) {
	if len(events) == 0 {
		return
	}
	entry.list.Selected = &entry.selected
	entry.list.Key = func(index int) any { return index }
	columns := []ui.TableColumn{
		{ID: "index", Title: "#", Width: 44, Fixed: true},
		{ID: "condition", Title: locales.TurnInterval()},
		{ID: "phase", Title: locales.TurnPhase()},
		{
			ID:    "flag",
			Title: locales.Label(game.EventFlag),
			Width: 100,
			Fixed: true,
		},
	}
	objective := v.mode == 5
	area := entry.data.Table() == game.AlwaysEvents
	talk := entry.data.Table() == game.TalkEvents
	if talk {
		columns[1].Title = locales.Label(game.TalkInitiator)
		columns[2].Title = locales.Label(game.TalkTarget)
	}
	if area {
		columns[1].Title = locales.AreaRange()
		columns[2].Title = locales.AreaScript()
	}
	if objective {
		columns[1].Title = locales.ObjectiveCondition()
		columns[2].Title = locales.ObjectiveEvent()
	}
	ui.Table(c.Key("conditions"), &entry.list, columns, len(events), func(index, column int) {
		event := events[index]
		switch column {
		case 0:
			ui.Textf(c, "%02X", index).FontFeatures("tnum")
		case 1:
			switch {
			case objective:
				ui.Text(c, objectiveConditionCaption(event)).SingleLine()
			case talk && len(entry.data.Fields(index)) > 0:
				ui.Text(c, characterCaption(session, event.Initiator, len(entry.data.Fields(index)) > 3)).
					SingleLine()
			case area && event.Kind == 0x0B:
				ui.Textf(c, "%d,%d → %d,%d", event.X1, event.Y1, event.X2, event.Y2).
					FontFeatures("tnum")
			case entry.data.Table() == game.TurnEvents && event.Kind == 2:
				ui.Text(c, turnInterval(event.Start, event.End)).
					FontFeatures("tnum")
			default:
				ui.Textf(c, "0x%02X", event.Kind)
			}
		case 2:
			switch {
			case talk && len(entry.data.Fields(index)) > 0:
				ui.Text(c, characterCaption(session, event.Target, false)).
					SingleLine()
			case area || talk:
				ui.Textf(c, "0x%08X", event.Script).FontFeatures("tnum")
			default:
				ui.Text(c, phaseLabel(event.Phase))
			}
		case 3:
			ui.Textf(c, "0x%04X", event.Flag).FontFeatures("tnum")
		}
	}).
		Height(min(160, c.Theme().Space(9)*float32(len(events)+1))).
		Label(v.labelsFor(entry.data.Table()).title)
}

func (v *View) eventLinks(
	c *ui.Context,
	state *components.State,
	event game.EventCondition,
) {
	if len(event.Warnings) > 0 {
		ui.Text(c, locales.GameIncompleteDeploymentScript()).
			FontSize(12).
			TextColor(c.Theme().TextMuted).
			Tooltip(event.Warnings[0].Error())
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, group := range event.Groups {
			ui.Button(c, fmt.Sprintf("%s · %d · 0x%08X", locales.TurnOpenUnits(), len(group.Units), group.Pointer)).
				Tooltip(locales.TurnExistingScript()).
				Disabled(state.Busy || group.Err != nil).
				OnClick(func() {
					found := false
					for _, known := range v.resource.Groups {
						if known.Pointer == group.Pointer {
							found = true
							break
						}
					}
					if !found {
						v.resource.Groups = append(v.resource.Groups, group)
					}
					v.selections[v.chapter] = &chapterSelection{
						pointer: group.Pointer,
					}
					v.mode = 1
				})
		}
	})
}

func (v *View) eventActions(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *eventState,
) {
	stale := !entry.data.Current(state.Session)
	if !entry.data.Modified() && !stale && !entry.invalid() {
		return
	}
	table := entry.data.Table()
	key := eventKey{row.ID, table}
	labels := v.labelsFor(table)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, labels.preview).
			Disabled(state.Busy || stale || entry.invalid() || !entry.data.Modified()).
			OnClick(func() {
				entry.preview, state.Err = state.Session.PlanEventEdit(
					entry.data,
				)
				entry.generations = make(map[*game.DeploymentDraft]uint64)
				for data := range entry.units {
					entry.generations[data] = data.Generation()
				}
			})
		ui.Button(c, labels.reload).
			Disabled(state.Busy).
			OnClick(func() { delete(v.events, key) })
	})
	if entry.preview == nil {
		return
	}
	preview := entry.preview
	ui.Column(c.Key("event-preview")).Gap(8).Children(func() {
		ui.Textf(c, "%s: %d → %d · 0x%08X · %d B", labels.title, preview.Before(), preview.After(), preview.Destination(), preview.Length()).
			Selectable().
			FontFeatures("tnum")
		ui.Text(c, labels.previewScope).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		for _, chapter := range preview.Uses() {
			ui.Textf(c, "%02X %s", chapter, nameOf(state.Session, game.Chapters, chapter)).
				FontSize(12)
		}
		ui.Button(c, labels.apply).
			Disabled(state.Busy || stale || entry.invalid()).
			OnClick(func() {
				if state.Err = state.Session.ApplyEventEdit(
					preview,
				); state.Err != nil {
					return
				}
				fresh, err := state.Session.NewEventDraft(row.ID, table)
				state.Err = err
				if err == nil {
					v.events[key] = &eventState{
						data:         fresh,
						view:         v.mode,
						selected:     entry.selected,
						lastSelected: -1,
						units: make(
							map[*game.DeploymentDraft]*entryDraft,
						),
					}
				}
				state.HistoryEpoch++
			})
	})
}
