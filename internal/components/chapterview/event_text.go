package chapterview

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func eventTexts(
	c *ui.Context,
	state *components.State,
	entry *eventState,
	event game.EventCondition,
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
	value, err := state.Session.TextByID(event.Texts[index])
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
	} else {
		ui.Scroll(c.Key("text-preview")).Height(112).Children(func() {
			ui.Text(c, value.Content).Selectable().FontSize(13)
		})
	}
}
