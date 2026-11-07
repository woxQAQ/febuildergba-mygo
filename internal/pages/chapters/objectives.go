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

func objectiveConditionCaption(condition event.Condition) string {
	if condition.Rout {
		return locales.ObjectiveRout()
	}
	if condition.Kind == 1 {
		return fmt.Sprintf(
			"%s · 0x%X",
			locales.Label(event.EventTrigger),
			condition.TriggerFlag,
		)
	}
	if condition.Kind == 11 {
		return locales.AreaRange()
	}
	return fmt.Sprintf("0x%02X", condition.Kind)
}

func scriptCaption(ref event.EventScriptRef) string {
	label := locales.ObjectiveExistingScript()
	switch ref.Kind {
	case event.ChapterEndingScript:
		label = locales.ObjectiveEnding()
	case event.GameOverScript:
		label = locales.ObjectiveGameOver()
	case event.FlagOnlyEvent:
		return locales.ObjectiveFlagOnly()
	}
	return fmt.Sprintf("%s · 0x%08X", label, ref.Address)
}

func (v *Page) conditionHelp(entry *eventState) string {
	events := entry.data.Events()
	if entry.data.Table() == event.AlwaysEvents &&
		entry.selected < len(events) {
		condition := events[entry.selected]
		if condition.Kind == 1 {
			return locales.ObjectiveFlagHelp()
		}
		if condition.Rout {
			return locales.ObjectiveRoutHelp()
		}
	}
	return v.labelsFor(entry.data.Table()).help
}

func (v *Page) objectiveSummary(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	entry *eventState,
	openText func(uint16),
) {
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.ObjectiveDisplay()).
			Tooltip(locales.ObjectiveDisplayHelp())
		for _, field := range []string{"objective_text_id", "objective_detail_text_id"} {
			value, err := state.Session.Records.Value(
				records.Chapters,
				row.ID,
				field,
			)
			if err != nil || value <= 0 || value >= 0x7FFF {
				continue
			}
			id := uint16(value)
			ui.Button(c.Key(field), fmt.Sprintf("%s · %04X", locales.Label(field), value)).
				Disabled(state.Busy).
				OnClick(func() { openText(id) })

		}
		for _, ref := range entry.data.Scripts() {
			if ref.Kind != event.ChapterEndingScript {
				continue
			}
			pointer := 0
			if sources, err := state.Session.Events.ScriptEntries(
				row.ID,
			); err == nil {
				for _, source := range sources {
					if source.Address == ref.Address {
						pointer = source.Pointer
						break
					}
				}
			}
			ui.Button(c, scriptCaption(ref)).Tooltip(locales.ObjectiveScope()).
				Disabled(state.Busy || pointer == 0 || v.openScript == nil).
				OnClick(func() { v.openScript(row.ID, pointer) })
		}
	})
}

func (v *Page) objectiveDetails(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
	openText func(uint16),
) {
	if condition.Kind == 1 || condition.Rout {
		v.conditionScript(c, state, entry, condition)
		if condition.Kind == 1 && condition.TriggerFlag == 0 {
			message := locales.ObjectiveZeroNever()
			for _, field := range entry.data.Fields(entry.selected) {
				if field.ID == event.EventTrigger && field.Width == 4 {
					message = locales.ObjectiveZeroAlways()
				}
			}
			ui.Text(c, message).FontSize(components.FontCaption)
		}
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		if condition.Script >= 0xC0 {
			ui.Button(c, locales.ObjectiveOpenScript()).
				Disabled(state.Busy || entry.data.Modified() || entry.invalid() || condition.Address == 0 || v.openScript == nil).
				OnClick(func() { v.openScript(v.chapter, condition.Address+4) })
		}
		if condition.Script == 1 {
			ui.Text(c, locales.ObjectiveFlagOnly()).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
	})
	v.eventLinks(c, state, condition)
	eventTexts(c, state, entry, condition, openText)
}

func (v *Page) conditionScript(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
) {
	label, help := locales.ObjectiveEvent(), locales.ObjectiveScriptHelp()
	if entry.data.Table() == event.LocationEvents {
		label, help = locales.AreaScript(), locales.LocationScriptHelp()
	}
	refs := entry.data.Scripts()
	captions := make([]string, len(refs))
	selected := ""
	for i, ref := range refs {
		captions[i] = scriptCaption(ref)
		if entry.data.Table() == event.LocationEvents &&
			ref.Kind == event.FlagOnlyEvent {
			captions[i] = locales.LocationNativeAction()
		}
		if ref.Address == condition.Script {
			selected = captions[i]
		}
	}
	if selected == "" {
		selected = fmt.Sprintf("0x%08X", condition.Script)
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
	if input.value != int64(condition.Script) ||
		!slices.Contains(captions, input.text) {
		input.text, input.value = selected, int64(condition.Script)
	}
	ui.Column(c).Width(400).MaxWidthPercent(100).Gap(4).Children(func() {
		ui.Text(c, label)
		components.TextDraft(
			state,
			ui.Combobox(c.Key("objective-script"), &input.text, captions).
				WidthPercent(100).MinWidth(0).
				Label(v.labelsFor(entry.data.Table()).title+" / "+label).
				Tooltip(help).
				Disabled(state.Busy || !entry.data.Current(state.Session.Events)).
				OnChange(func() {
					if index := slices.Index(
						captions,
						input.text,
					); index >= 0 &&
						index < len(refs) {
						state.SetErr(entry.data.SetEventScript(
							state.Session.Events,
							entry.selected,
							refs[index].Address,
						))
						entry.preview = nil
					}
				}),
		)
	})
}
