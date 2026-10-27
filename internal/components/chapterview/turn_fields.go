package chapterview

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

func (v *View) turnNumber(
	c *ui.Context,
	state *components.State,
	entry *turnState,
	field game.Field,
) {
	if field.ID == game.TurnPhase || field.ID == game.TurnRoute {
		v.turnChoice(c, state, entry, field)
		return
	}
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	key := inputKey{entry.selected, field.ID}
	value, _ := entry.data.Value(entry.selected, field.ID)
	input := entry.inputs[key]
	if input == nil {
		text := strconv.FormatInt(value, 10)
		if field.Hex {
			text = fmt.Sprintf("0x%X", value)
		}
		input = &numberDraft{text: text, value: value}
		entry.inputs[key] = input
	}
	low, high := field.Bounds()
	control := components.TextDraft(
		state,
		ui.TextInput(c.Key("value"), &input.text).
			Width(92).
			Height(26).
			Padding(3, 6).
			FontSize(13).
			FontFeatures("tnum").
			TextAlign(ui.End).
			Label(locales.TurnEvents()+" / "+locales.Label(field.ID)).
			Disabled(state.Busy || !entry.data.Current(state.Session)).
			OnChange(func() {
				parsed, err := components.ParseInteger(input.text)
				entry.preview = nil
				if err != nil || parsed < low || parsed > high {
					input.invalid = true
					return
				}
				if state.Err = entry.data.Set(
					entry.selected,
					field.ID,
					parsed,
				); state.Err != nil {
					input.invalid = true
					return
				}
				input.value, input.invalid = parsed, false
			}),
	)
	if input.invalid {
		control.Error(
			fmt.Sprintf("%s: %d…%d", locales.AllowedRange(), low, high),
		)
	}
}

func (v *View) turnChoice(
	c *ui.Context,
	state *components.State,
	entry *turnState,
	field game.Field,
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
	if field.ID == game.TurnRoute {
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
	input := entry.inputs[key]
	if input == nil {
		input = &numberDraft{}
		entry.inputs[key] = input
	}
	// Resolve captions each frame so locale switches preserve the encoded value.
	if input.value != value || !slices.Contains(labels, input.text) {
		input.text, input.value = labels[selected], value
	}
	ui.Select(c.Key("value"), &input.text, labels).
		Width(180).
		MaxWidthPercent(100).
		Label(locales.TurnEvents() + " / " + locales.Label(field.ID)).
		Disabled(state.Busy || !entry.data.Current(state.Session)).
		OnChange(func() {
			if index := slices.Index(labels, input.text); index >= 0 {
				state.Err = entry.data.Set(
					entry.selected,
					field.ID,
					values[index],
				)
				entry.preview = nil
			}
		})
}
