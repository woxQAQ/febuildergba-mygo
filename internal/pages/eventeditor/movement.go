package eventeditor

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type movementDraft struct {
	selected, command, value string
	signature                string
	invalid                  bool
}

var movementLabels = [...]string{
	"script_move_left",
	"script_move_right",
	"script_move_down",
	"script_move_up",
	"script_move_halt",
	"script_face_left",
	"script_face_right",
	"script_face_down",
	"script_face_up",
	"script_move_sleep",
	"script_move_bump",
	"",
	"script_move_speed",
	"script_move_camera_on",
	"script_move_camera_off",
}

func (page *Page) movement(
	c *ui.Context,
	draft *draftState,
	row game.ScriptInstruction,
	field game.ScriptField,
) {
	if draft.movements == nil {
		draft.movements = make(map[operandKey]*movementDraft)
	}
	key := operandKey{row.ID, field.ID}
	state := draft.movements[key]
	if state == nil {
		state = &movementDraft{}
		draft.movements[key] = state
	}
	disabled := page.state.Busy || !draft.data.Current(page.state.Session)
	steps, err := draft.data.Movement(draft.selected, field.ID)
	ui.Column(c.Key(key)).Gap(8).Children(func() {
		ui.Text(c, locales.Label(field.ID))
		ui.Text(c, locales.Label("script_movement_editor_help")).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		if err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.Label("script_movement_create")).
				Disabled(disabled).
				OnClick(func() {
					page.state.Err = draft.data.SetMovement(
						draft.selected,
						field.ID,
						[]game.ScriptMovementStep{{Command: 4}},
					)
					draft.preview = nil
				})
			return
		}
		var captions []string
		for index, step := range steps {
			label := locales.Label(movementLabels[step.Command])
			captions = append(captions, fmt.Sprintf("%d · %s", index+1, label))
		}
		index := slices.Index(captions, state.selected)
		if index < 0 {
			index = 0
			state.selected = captions[0]
		}
		signature := fmt.Sprintf("%v/%d", steps, index)
		if signature != state.signature {
			state.signature = signature
			state.value = strconv.Itoa(steps[index].Value)
			state.invalid = false
		}
		update := func(next []game.ScriptMovementStep, selected int) {
			page.state.Err = draft.data.SetMovement(
				draft.selected,
				field.ID,
				next,
			)
			draft.preview = nil
			if page.state.Err == nil {
				state.selected = ""
				if selected >= 0 && selected < len(captions) {
					state.selected = captions[selected]
				}
			}
		}
		ui.Text(c, strings.Join(captions, "  →  ")).FontSize(12)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Select(c.Key("step"), &state.selected, captions).
				Width(230).
				Label(locales.Label("script_movement_step")).
				Disabled(disabled)
			if steps[index].Command == 9 || steps[index].Command == 12 {
				minimum := 1
				if steps[index].Command == 12 {
					minimum = 16
				}
				control := components.TextDraft(
					page.state,
					ui.TextInput(c.Key("value"), &state.value).
						Width(100).
						Label(locales.Label("script_movement_value")).
						Disabled(disabled).
						OnChange(func() {
							draft.preview = nil
							value, err := strconv.Atoi(state.value)
							state.invalid = err != nil || value < minimum ||
								value > 127
							if state.invalid {
								return
							}
							next := slices.Clone(steps)
							next[index].Value = value
							page.state.Err = draft.data.SetMovement(
								draft.selected,
								field.ID,
								next,
							)
							state.invalid = page.state.Err != nil
						}),
				)
				if state.invalid {
					control.Error(fmt.Sprintf("%d…127", minimum))
				}
			}
			ui.Button(c, locales.ScriptDelete()).
				Disabled(disabled || index == len(steps)-1).
				OnClick(func() { update(slices.Delete(slices.Clone(steps), index, index+1), -1) })
			ui.Button(c, locales.MoveUp()).
				Disabled(disabled || index == 0 || index == len(steps)-1).
				OnClick(func() {
					next := slices.Clone(steps)
					next[index-1], next[index] = next[index], next[index-1]
					update(next, -1)
				})
			ui.Button(c, locales.MoveDown()).
				Disabled(disabled || index >= len(steps)-2).
				OnClick(func() {
					next := slices.Clone(steps)
					next[index+1], next[index] = next[index], next[index+1]
					update(next, -1)
				})
		})
		var commands []string
		var codes []byte
		for code, label := range movementLabels {
			if label == "" || code == 4 {
				continue
			}
			commands = append(commands, locales.Label(label))
			codes = append(
				codes,
				byte(code),
			) // #nosec G115 -- fixed native command list.
		}
		if !slices.Contains(commands, state.command) {
			state.command = commands[1]
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Select(c.Key("command"), &state.command, commands).
				Width(230).
				Label(locales.ScriptNewCommand()).
				Disabled(disabled)
			ui.Button(c, locales.Label("script_movement_insert")).
				Disabled(disabled).
				OnClick(func() {
					choice := slices.Index(commands, state.command)
					if choice < 0 {
						return
					}
					step := game.ScriptMovementStep{Command: codes[choice]}
					if step.Command == 9 {
						step.Value = 1
					}
					if step.Command == 12 {
						step.Value = 16
					}
					update(slices.Insert(slices.Clone(steps), index, step), -1)
				})
		})
	})
}
