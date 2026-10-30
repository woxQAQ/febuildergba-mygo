package chapters

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type inputKey struct {
	index int
	field string
}

type numberDraft struct {
	text    string
	value   int64
	invalid bool
}

func (v *Page) fields(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	advanced bool,
	jump func(records.Kind, int),
) {
	if entry.data.Count() == 0 || entry.selected >= entry.data.Count() {
		return
	}
	ui.Column(c.Key(fmt.Sprintf("unit-%d", entry.selected))).
		Gap(12).
		Children(func() {
			unit := entry.data.Units()[entry.selected]
			ui.Textf(c, "%02X · %s", entry.selected, nameOf(state.Session, records.Units, unit.Character)).
				Bold().
				Tooltip(locales.DeployPositionHelp())
			for _, group := range []string{"general", "deploy_position", "items", "deploy_ai", "advanced"} {
				if group == "advanced" && !advanced {
					continue
				}
				ui.Column(c.Key(group)).Gap(4).Children(func() {
					ui.Text(c, locales.Label(group)).Bold()
					ui.Row(c).Wrap().Gap(16).Children(func() {
						for _, field := range entry.data.Fields() {
							if field.Group != group {
								continue
							}
							width := float32(380)
							if group == records.GroupGeneral {
								width = 260
							}
							ui.Row(c.Key(field.ID)).
								Width(width).
								MaxWidthPercent(100).
								Gap(6).
								AlignItems(ui.Center).
								Children(func() {
									ui.Text(c, locales.Label(field.ID)).
										Grow(1).
										FontSize(13).
										SingleLine().
										Tooltip(locales.DeployPositionHelp())
									v.number(c, state, entry, field)
									kind := records.Kind("")
									switch field.ID {
									case "deploy_character", "deploy_leader":
										kind = records.Units
									case "deploy_class":
										kind = records.Classes
									case "deploy_item_1",
										"deploy_item_2",
										"deploy_item_3",
										"deploy_item_4":
										kind = records.Items
									}
									if kind != "" {
										value, _ := entry.data.Value(
											entry.selected,
											field.ID,
										)
										ui.Button(c, locales.Jump()).
											Height(26).
											Padding(3, 6).
											Disabled(state.Busy || value == 0).
											OnClick(func() { jump(kind, int(value)) })
									} else {
										ui.Box(c).Width(42)
									}
								})
						}
					})
				})
			}
		})
}

func (v *Page) number(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	field deployment.Field,
) {
	unit := entry.selected
	value, _ := entry.data.Value(unit, field.ID)
	deploymentNumber(
		c,
		state,
		entry,
		field,
		inputKey{unit, field.ID},
		value,
		func(value int64) error { return entry.data.Set(unit, field.ID, value) },
	)
}

func deploymentNumber(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	field deployment.Field,
	key inputKey,
	value int64,
	set func(int64) error,
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	draft := entry.inputs[key]
	if draft == nil {
		draft = &numberDraft{text: strconv.FormatInt(value, 10), value: value}
		entry.inputs[key] = draft
	}
	if draft.value != value {
		draft.text = strconv.FormatInt(value, 10)
		draft.value, draft.invalid = value, false
	}
	control := components.TextDraft(
		state,
		ui.TextInput(c.Key("value"), &draft.text).
			Width(58).Height(26).Padding(3, 6).FontSize(13).
			FontFeatures("tnum").TextAlign(ui.End).
			Label(locales.Deployments()+" / "+locales.Label(field.ID)).
			Disabled(state.Busy || !entry.data.Current(state.Session.Deployments)).
			OnChange(func() {
				value, err := input.ParseInteger(draft.text)
				if err != nil || value < field.Min || value > field.Max {
					draft.invalid = true
					entry.preview = nil
					return
				}
				if err := set(value); err != nil {
					state.Err = err
					draft.invalid = true
					return
				}
				draft.value, draft.invalid = value, false
				entry.preview = nil
			}),
	)
	if draft.invalid {
		message := fmt.Sprintf(
			"%s: %d…%d",
			locales.AllowedRange(),
			field.Min,
			field.Max,
		)
		control.Error(message).Tooltip(message)
	}
}
