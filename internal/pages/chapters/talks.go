package chapters

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func characterCaption(session *game.Session, id int, anyInitiator bool) string {
	if id == 0 && anyInitiator {
		return "00 · " + locales.TalkAnyone()
	}
	return fmt.Sprintf("%02X · %s", id, nameOf(session, records.Units, id))
}

func (v *Page) talkCharacter(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	field records.Field,
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	value, _ := entry.data.Value(entry.selected, field.ID)
	anyInitiator := field.ID == event.TalkInitiator &&
		len(entry.data.Fields(entry.selected)) > 3
	var choices []string
	choices = append(choices, characterCaption(state.Session, 0, anyInitiator))
	known := value == 0
	if table := state.Session.Records.Table(
		records.Units,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID <= 0 || row.ID > 255 {
				continue
			}
			choices = append(
				choices,
				characterCaption(state.Session, row.ID, false),
			)
			known = known || int64(row.ID) == value
		}
	}
	caption := characterCaption(state.Session, int(value), anyInitiator)
	if !known {
		choices = append(choices, caption)
	}
	key := inputKey{entry.selected, field.ID}
	input := entry.inputs[key]
	if input == nil {
		input = &numberDraft{}
		entry.inputs[key] = input
	}
	if input.value != value || !slices.Contains(choices, input.text) {
		input.text, input.value = caption, value
	}
	components.TextDraft(
		state,
		ui.Combobox(c.Key("value"), &input.text, choices).
			Width(180).
			MaxWidthPercent(100).
			Label(locales.TalkEvents()+" / "+locales.Label(field.ID)).
			Disabled(state.Busy || !entry.data.Current(state.Session.Events)).
			OnChange(func() {
				encoded, _, _ := strings.Cut(input.text, " · ")
				id, err := strconv.ParseInt(encoded, 16, 16)
				if err == nil {
					state.Err = entry.data.Set(entry.selected, field.ID, id)
				}
				entry.preview = nil
			}),
	)
}

func (v *Page) talkDetails(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
	jump func(records.Kind, int),
	openText func(uint16),
) {
	fields := entry.data.Fields(entry.selected)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, field := range fields {
			if field.ID != event.TalkInitiator &&
				field.ID != event.TalkTarget {
				continue
			}
			value, _ := entry.data.Value(entry.selected, field.ID)
			ui.Button(c, locales.Jump()+" · "+characterCaption(state.Session, int(value), false)).
				Disabled(state.Busy || value == 0).
				OnClick(func() { jump(records.Units, int(value)) })
		}
	})
	if len(fields) > 0 && condition.Flag == 0 {
		ui.Text(c, locales.TalkRepeat()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
	if len(fields) == 5 && fields[3].Width == 2 &&
		(condition.TalkMode == 1 || condition.TalkMode == 2) {
		ui.Text(c, locales.TalkDisabledMode()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
	if id, created := entry.data.Dialog(entry.selected); created {
		v.dialoguePicker(c, state, entry, id)
	} else {
		v.eventLinks(c, state, condition)
	}
	eventTexts(c, state, entry, condition, openText)
}

type dialogueChoice struct {
	id              uint16
	caption, search string
}

func (v *Page) dialoguePicker(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	id uint16,
) {
	if entry.textChoices == nil {
		entries, err := state.Session.Texts.Entries()
		if err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
			return
		}
		entry.textChoices = make([]dialogueChoice, 0, len(entries))
		for _, text := range entries {
			if text.Err != nil {
				continue
			}
			content := strings.Join(strings.Fields(text.Content), " ")
			caption := []rune(content)
			if len(caption) > 36 {
				caption = append(caption[:36], '…')
			}
			entry.textChoices = append(entry.textChoices, dialogueChoice{
				id:      text.ID,
				caption: fmt.Sprintf("%04X · %s", text.ID, string(caption)),
				search: strings.ToLower(
					fmt.Sprintf("%04X %s", text.ID, content),
				),
			})
		}
	}
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	key := inputKey{entry.selected, "dialog-text"}
	input := entry.inputs[key]
	if input == nil {
		input = &numberDraft{}
		entry.inputs[key] = input
	}
	caption := ""
	for _, choice := range entry.textChoices {
		if choice.id == id {
			caption = choice.caption
			break
		}
	}
	picker := ui.ComboboxBase(c.Key("dialog-text"), &input.text)
	components.TextDraft(state, picker.Input).
		Width(580).MaxWidthPercent(100).Height(28).Padding(4, 8).
		Background(c.Theme().Surface).
		Border(1, c.Theme().Border).Radius(c.Theme().Radius).
		Label(locales.TalkText()).
		Placeholder(locales.ChooseTextId()).Tooltip(locales.TalkTextHelp()).
		Disabled(state.Busy || !entry.data.Current(state.Session.Events))
	if input.value != int64(id) || (!picker.Input.Focused() && !picker.Open()) {
		input.text, input.value = caption, int64(id)
	}
	query := strings.TrimPrefix(
		strings.ToLower(strings.TrimSpace(input.text)),
		"0x",
	)
	picker.Popup(func(panel ui.Element) {
		panel.Background(c.Theme().Surface).
			Border(1, c.Theme().Border).
			Radius(c.Theme().Radius).
			Padding(4)
		ui.Scroll(c).MaxHeight(240).Children(func() {
			for _, choice := range entry.textChoices {
				if picker.Filtering() &&
					!strings.Contains(choice.search, query) {
					continue
				}
				item := picker.Item(choice.caption).
					Padding(5, 8).
					Radius(c.Theme().Radius)
				if item.Highlighted() {
					item.Background(c.Theme().Accent).
						TextColor(c.Theme().AccentText)
				}
				item.Children(
					func() { ui.Text(c, choice.caption).SingleLine() },
				)
			}
		})
	})
	if chosen, ok := picker.Chosen(); ok {
		for _, choice := range entry.textChoices {
			if choice.caption == chosen {
				state.Err = entry.data.SetDialogue(entry.selected, choice.id)
				input.text, input.value = choice.caption, int64(choice.id)
				entry.preview = nil
				break
			}
		}
	}
	if _, err := state.Session.Events.DialogueText(id); err != nil {
		message := components.ErrorText(err)
		if id == 0 {
			message = locales.ChooseTextId()
		}
		ui.Text(c, message).FontSize(12).TextColor(c.Theme().TextMuted)
	}
}
