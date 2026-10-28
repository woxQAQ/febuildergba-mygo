package chapterview

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func objectiveConditionCaption(event game.EventCondition) string {
	if event.Rout {
		return locales.ObjectiveRout()
	}
	if event.Kind == 1 {
		return fmt.Sprintf(
			"%s · 0x%X",
			locales.Label(game.EventTrigger),
			event.TriggerFlag,
		)
	}
	if event.Kind == 11 {
		return locales.AreaRange()
	}
	return fmt.Sprintf("0x%02X", event.Kind)
}

func scriptCaption(ref game.EventScriptRef) string {
	label := locales.ObjectiveExistingScript()
	switch ref.Kind {
	case game.ChapterEndingScript:
		label = locales.ObjectiveEnding()
	case game.GameOverScript:
		label = locales.ObjectiveGameOver()
	case game.FlagOnlyEvent:
		return locales.ObjectiveFlagOnly()
	}
	return fmt.Sprintf("%s · 0x%08X", label, ref.Address)
}

func (v *View) conditionHelp(entry *eventState) string {
	events := entry.data.Events()
	if entry.data.Table() == game.AlwaysEvents && entry.selected < len(events) {
		event := events[entry.selected]
		if event.Kind == 1 {
			return locales.ObjectiveFlagHelp()
		}
		if event.Rout {
			return locales.ObjectiveRoutHelp()
		}
	}
	return v.labelsFor(entry.data.Table()).help
}

func (v *View) objectiveSummary(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *eventState,
	openHex func(int, int),
	openText func(uint16),
) {
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.ObjectiveDisplay()).
			Tooltip(locales.ObjectiveDisplayHelp())
		for _, field := range []string{"objective_text_id", "objective_detail_text_id"} {
			value, err := state.Session.Value(game.Chapters, row.ID, field)
			if err != nil || value <= 0 || value >= 0x7FFF {
				continue
			}
			id := uint16(value) // #nosec G115 -- The text ID was bounded above.
			ui.Button(c.Key(field), fmt.Sprintf("%s · %04X", locales.Label(field), value)).
				Disabled(state.Busy).
				OnClick(func() { openText(id) })

			// #nosec G115 -- Text IDs are bounded above.
		}
		for _, ref := range entry.data.Scripts() {
			if ref.Kind != game.ChapterEndingScript {
				continue
			}
			ui.Button(c, scriptCaption(ref)).
				Tooltip(locales.ObjectiveScope()).
				Disabled(state.Busy).
				OnClick(func() { openHex(ref.Address, 4) })
		}
	})
}

func (v *View) objectiveAdd(
	c *ui.Context,
	state *components.State,
	entry *eventState,
) {
	for _, item := range []struct {
		template    game.ObjectiveTemplate
		label, help string
		kind        game.EventScriptKind
	}{
		{game.BossVictory, locales.ObjectiveAddBoss(), locales.ObjectiveBossHelp(), game.ChapterEndingScript},
		{game.RoutVictory, locales.ObjectiveAddRout(), locales.ObjectiveRoutHelp(), game.ChapterEndingScript},
		{game.FlagFailure, locales.ObjectiveAddFailure(), locales.ObjectiveFailureHelp(), game.GameOverScript},
	} {
		available := slices.ContainsFunc(
			entry.data.Scripts(),
			func(ref game.EventScriptRef) bool { return ref.Kind == item.kind },
		)
		ui.Button(c.Key(item.template), item.label).Tooltip(item.help).
			Disabled(state.Busy || !entry.data.Current(state.Session) || !available).
			OnClick(func() {
				state.Err = entry.data.AddObjective(
					state.Session,
					item.template,
				)
				if state.Err == nil {
					entry.selected = len(entry.data.Events()) - 1
				}
				entry.preview = nil
			})
	}
}

func (v *View) objectiveDetails(
	c *ui.Context,
	state *components.State,
	entry *eventState,
	event game.EventCondition,
	openHex func(int, int),
	openText func(uint16),
) {
	if event.Kind == 1 || event.Rout {
		v.objectiveScript(c, state, entry, event)
		if event.Kind == 1 && event.TriggerFlag == 0 {
			message := locales.ObjectiveZeroNever()
			for _, field := range entry.data.Fields(entry.selected) {
				if field.ID == game.EventTrigger && field.Width == 4 {
					message = locales.ObjectiveZeroAlways()
				}
			}
			ui.Text(c, message).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if event.Rout {
			ui.Text(c, locales.ObjectiveRoutHelp()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		if event.Script >= 0xC0 {
			ui.Button(c, locales.ObjectiveOpenScript()).Disabled(state.Busy).
				OnClick(func() { openHex(event.Script, 4) })
		}
		if event.Script == 1 {
			ui.Text(c, locales.ObjectiveFlagOnly()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
	})
	v.eventLinks(c, state, event)
	eventTexts(c, state, entry, event, openText)
}

func (v *View) objectiveScript(
	c *ui.Context,
	state *components.State,
	entry *eventState,
	event game.EventCondition,
) {
	refs := entry.data.Scripts()
	captions := make([]string, len(refs))
	selected := ""
	for i, ref := range refs {
		captions[i] = scriptCaption(ref)
		if ref.Address == event.Script {
			selected = captions[i]
		}
	}
	if selected == "" {
		selected = fmt.Sprintf("0x%08X", event.Script)
		captions = append(captions, selected)
	}
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	key := inputKey{entry.selected, "objective-script"}
	input := entry.inputs[key]
	if input == nil {
		input = &numberDraft{}
		entry.inputs[key] = input
	}
	if input.value != int64(event.Script) ||
		!slices.Contains(captions, input.text) {
		input.text, input.value = selected, int64(event.Script)
	}
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.ObjectiveEvent())
		components.TextDraft(
			state,
			ui.Combobox(c.Key("objective-script"), &input.text, captions).
				Width(400).
				MaxWidthPercent(100).
				Label(locales.Objectives()+" / "+locales.ObjectiveEvent()).
				Tooltip(locales.ObjectiveScriptHelp()).
				Disabled(state.Busy || !entry.data.Current(state.Session)).
				OnChange(func() {
					if index := slices.Index(
						captions,
						input.text,
					); index >= 0 &&
						index < len(refs) {
						state.Err = entry.data.SetEventScript(
							state.Session,
							entry.selected,
							refs[index].Address,
						)
						entry.preview = nil
					}
				}),
		)
	})
}
