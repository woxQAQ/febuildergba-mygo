package chapters

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *Page) eventNumber(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	field records.Field,
) {
	if field.ID == event.TalkInitiator || field.ID == event.TalkTarget {
		v.talkCharacter(c, state, entry, field)
		return
	}
	if field.ID == event.TurnPhase || field.ID == event.TurnRoute ||
		field.ID == event.TalkMode {
		v.eventChoice(c, state, entry, field)
		return
	}
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	key := inputKey{entry.selected, field.ID}
	value, _ := entry.data.Value(entry.selected, field.ID)
	draft := entry.inputs[key]
	if draft == nil {
		text := strconv.FormatInt(value, 10)
		if field.Hex {
			text = fmt.Sprintf("0x%X", value)
		}
		draft = &numberDraft{text: text, value: value}
		entry.inputs[key] = draft
	}
	low, high := field.Bounds()
	control := components.TextDraft(
		state,
		ui.TextInput(c.Key("value"), &draft.text).
			Width(92).
			Height(26).
			Padding(3, 6).
			FontSize(13).
			FontFeatures("tnum").
			TextAlign(ui.End).
			Label(v.labelsFor(entry.data.Table()).title+" / "+locales.Label(field.ID)).
			Disabled(state.Busy || !entry.data.Current(state.Session.Events)).
			OnChange(func() {
				parsed, err := input.ParseInteger(draft.text)
				entry.preview = nil
				if err != nil || parsed < low || parsed > high {
					draft.invalid = true
					return
				}
				if state.Err = entry.data.Set(
					entry.selected,
					field.ID,
					parsed,
				); state.Err != nil {
					draft.invalid = true
					return
				}
				draft.value, draft.invalid = parsed, false
			}),
	)
	if draft.invalid {
		control.Error(
			fmt.Sprintf("%s: %d…%d", locales.AllowedRange(), low, high),
		)
	}
}

func (v *Page) eventChoice(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	field records.Field,
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	values := []int64{0, 0x40, 0x80}
	labels := []string{
		locales.TurnPhasePlayer(),
		locales.TurnPhaseAlly(),
		locales.TurnPhaseEnemy(),
	}
	if field.ID == event.TalkMode {
		values = []int64{0, 3}
		labels = []string{locales.TalkModeNone(), locales.TalkModeFlag()}
		if field.Width == 1 {
			values = []int64{0, 1, 2, 3}
			labels = []string{
				locales.TalkModeNone(),
				locales.TurnRouteEliwood(),
				locales.TurnRouteHector(),
				locales.TalkModeFlag(),
			}
		}
	}
	if field.ID == event.TurnRoute {
		values = []int64{0, 1, 2, 3, 4, 5}
		labels = []string{
			locales.TurnRouteAny(),
			locales.TurnRouteEliwood(),
			locales.TurnRouteHector(),
			locales.TurnRouteEliwoodHard(),
			locales.TurnRouteHectorHard(),
			locales.TurnRouteHard(),
		}
	}
	value, _ := entry.data.Value(entry.selected, field.ID)
	selected := slices.Index(values, value)
	if selected < 0 {
		selected = len(values)
		values = append(values, value)
		labels = append(labels, fmt.Sprintf("0x%X", value))
	}
	key := inputKey{entry.selected, field.ID}
	draft := entry.inputs[key]
	if draft == nil {
		draft = &numberDraft{}
		entry.inputs[key] = draft
	}
	// Resolve captions each frame so locale switches preserve the encoded value.
	if draft.value != value || !slices.Contains(labels, draft.text) {
		draft.text, draft.value = labels[selected], value
	}
	ui.Select(c.Key("value"), &draft.text, labels).
		Width(180).
		MaxWidthPercent(100).
		Label(v.labelsFor(entry.data.Table()).title + " / " + locales.Label(field.ID)).
		Disabled(state.Busy || !entry.data.Current(state.Session.Events)).
		OnChange(func() {
			if index := slices.Index(labels, draft.text); index >= 0 {
				state.Err = entry.data.Set(
					entry.selected,
					field.ID,
					values[index],
				)
				entry.preview = nil
			}
		})
}
