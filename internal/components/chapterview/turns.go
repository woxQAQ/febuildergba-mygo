package chapterview

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type turnState struct {
	data        *game.TurnDraft
	preview     *game.TurnEdit
	selected    int
	list        ui.ListState
	inputs      map[inputKey]*numberDraft
	units       map[*game.DeploymentDraft]*entryDraft
	generations map[*game.DeploymentDraft]uint64
}

func (v *View) turnDraft(
	state *components.State,
	chapter int,
) (*turnState, error) {
	entry := v.turns[chapter]
	if entry != nil && !entry.data.Modified() &&
		!entry.data.Current(state.Session) {
		delete(v.turns, chapter)
		entry = nil
	}
	if entry == nil {
		data, err := state.Session.NewTurnDraft(chapter)
		if err != nil {
			return nil, err
		}
		entry = &turnState{
			data:  data,
			units: make(map[*game.DeploymentDraft]*entryDraft),
		}
		v.turns[chapter] = entry
	}
	for units, generation := range entry.generations {
		if units.Generation() != generation {
			entry.preview = nil
			break
		}
	}
	return entry, nil
}

func (e *turnState) invalid() bool {
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

func (v *View) turnEditor(
	c *ui.Context,
	state *components.State,
	row game.Row,
	advanced bool,
	jump func(game.Kind, int),
	openHex func(int, int),
) {
	entry, err := v.turnDraft(state, row.ID)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			TextColor(c.Theme().Danger).
			Tooltip(err.Error())
		v.mapView.Render(c, state.Session, row, mapview.Overlay{})
		return
	}
	stale := !entry.data.Current(state.Session)
	disabled := state.Busy || stale
	events := entry.data.Events()
	entry.selected = min(entry.selected, max(0, len(events)-1))
	ui.Column(c.Key("turn-events")).Gap(12).Children(func() {
		turnTable(c, entry, events)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Button(c, locales.TurnAdd()).
				Tooltip(locales.TurnScope()).
				Disabled(disabled).
				OnClick(func() {
					state.Err = entry.data.Add(state.Session)
					entry.selected = len(entry.data.Events()) - 1
					entry.preview = nil
				})
			ui.Button(c, locales.TurnRemove()).
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
				})
		})
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		if len(events) > 0 {
			v.turnDetails(c, state, row, entry, advanced, jump, openHex)
		} else {
			v.mapView.Render(c, state.Session, row, mapview.Overlay{})
		}
		v.turnActions(c, state, row, entry)
	})
}

func turnTable(c *ui.Context, entry *turnState, events []game.TurnEvent) {
	if len(events) == 0 {
		return
	}
	entry.list.Selected = &entry.selected
	entry.list.Key = func(index int) any { return index }
	columns := []ui.TableColumn{
		{ID: "index", Title: "#", Width: 44, Fixed: true},
		{ID: "turn", Title: locales.TurnInterval()},
		{ID: "phase", Title: locales.TurnPhase()},
		{ID: "flag", Title: locales.TurnFlag(), Width: 100, Fixed: true},
	}
	ui.Table(c.Key("conditions"), &entry.list, columns, len(events), func(index, column int) {
		event := events[index]
		switch column {
		case 0:
			ui.Textf(c, "%02X", index).FontFeatures("tnum")
		case 1:
			if event.Kind != 2 {
				ui.Textf(c, "0x%02X · 0x%08X", event.Kind, event.Script)
				return
			}
			ui.Text(c, turnInterval(event.Start, event.End)).
				FontFeatures("tnum").
				Tooltip(locales.TurnConditionHelp())
		case 2:
			ui.Text(c, phaseLabel(event.Phase))
		case 3:
			ui.Textf(c, "0x%04X", event.Flag).FontFeatures("tnum")
		}
	}).
		Height(min(160, c.Theme().Space(9)*float32(len(events)+1))).
		Label(locales.TurnEvents())
}

func (v *View) turnLinks(
	c *ui.Context,
	state *components.State,
	event game.TurnEvent,
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

func (v *View) turnActions(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *turnState,
) {
	stale := !entry.data.Current(state.Session)
	if !entry.data.Modified() && !stale && !entry.invalid() {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.TurnPreview()).
			Disabled(state.Busy || stale || entry.invalid() || !entry.data.Modified()).
			OnClick(func() {
				entry.preview, state.Err = state.Session.PlanTurnEdit(
					entry.data,
				)
				entry.generations = make(map[*game.DeploymentDraft]uint64)
				for data := range entry.units {
					entry.generations[data] = data.Generation()
				}
			})
		ui.Button(c, locales.TurnReload()).
			Disabled(state.Busy).
			OnClick(func() { delete(v.turns, row.ID) })
	})
	if entry.preview == nil {
		return
	}
	preview := entry.preview
	ui.Column(c.Key("turn-preview")).Gap(8).Children(func() {
		ui.Textf(c, "%s: %d → %d · 0x%08X · %d B", locales.TurnEvents(), preview.Before(), preview.After(), preview.Destination(), preview.Length()).
			Selectable().
			FontFeatures("tnum")
		ui.Text(c, locales.TurnPreviewScope()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		for _, chapter := range preview.Uses() {
			ui.Textf(c, "%02X %s", chapter, nameOf(state.Session, game.Chapters, chapter)).
				FontSize(12)
		}
		ui.Row(c).Children(func() {
			ui.Button(c, locales.TurnApply()).
				Disabled(state.Busy || stale || entry.invalid()).
				OnClick(func() {
					if state.Err = state.Session.ApplyTurnEdit(
						preview,
					); state.Err == nil {
						fresh, err := state.Session.NewTurnDraft(row.ID)
						state.Err = err
						if err == nil {
							v.turns[row.ID] = &turnState{
								data:     fresh,
								selected: entry.selected,
								units: make(
									map[*game.DeploymentDraft]*entryDraft,
								),
							}
						}
						state.HistoryEpoch++
					}
				})
		})
	})
}
