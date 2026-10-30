package eventeditor

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (page *Page) mapChanges(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
) {
	if !draft.data.IsMapChange(draft.selected) {
		return
	}
	chapter, err := page.state.Session.Events.MapChapter(
		draft.data,
		draft.selected,
	)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
		return
	}
	for _, field := range draft.data.Fields(draft.selected) {
		if field.Target == event.ScriptMapChangeTarget {
			page.mapChangeChoice(c, draft, row, field, chapter)
		}
	}
	links, err := page.state.Session.Events.MapChanges(
		draft.data,
		draft.selected,
	)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for index, link := range links {
			ui.Button(c.Key(index), fmt.Sprintf("%s · %02X", locales.MapChangesEditor(), link.ID)).
				Disabled(page.state.Busy || draft.data.Modified() || draft.invalid() || !draft.data.Current(page.state.Session.Events) || page.openMapChange == nil).
				OnClick(func() { page.openMapChange(link.Chapter, link.ID) })
		}
	})
}

func (page *Page) mapChangeChoice(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
	field event.ScriptField,
	chapter int,
) {
	changes, err := page.state.Session.Maps.Changes(chapter)
	if err != nil || len(changes) == 0 {
		return
	}
	var values []int
	var captions []string
	for _, change := range changes {
		if slices.Contains(values, change.ID) {
			continue
		}
		values = append(values, change.ID)
		captions = append(
			captions,
			fmt.Sprintf(
				"%02X · %d,%d · %d × %d",
				change.ID,
				change.Bounds.Min.X,
				change.Bounds.Min.Y,
				change.Bounds.Dx(),
				change.Bounds.Dy(),
			),
		)
	}
	key := operandKey{row.ID, field.ID}
	value, _ := draft.data.Value(draft.selected, field.ID)
	reference := draft.references[key]
	if reference == nil {
		reference = &referenceDraft{value: -4}
		draft.references[key] = reference
	}
	if reference.value != value ||
		!slices.Contains(captions, reference.choice) {
		reference.value = value
		reference.choice = ""
		if index := slices.Index(values, int(value)); index >= 0 {
			reference.choice = captions[index]
		}
	}
	ui.Select(c.Key(key), &reference.choice, captions).
		Width(380).
		MaxWidthPercent(100).
		Label(locales.MapChangeChoose()).
		Placeholder(locales.MapChangeChoose()).
		Disabled(page.state.Busy || !draft.data.Current(page.state.Session.Events)).
		OnChange(func() {
			if index := slices.Index(captions, reference.choice); index >= 0 {
				page.state.Err = draft.data.Set(
					draft.selected,
					field.ID,
					int64(values[index]),
				)
				draft.preview = nil
				if page.state.Err == nil {
					delete(draft.inputs, key)
				}
			}
		})
}
