package chapters

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
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
	if entry.data.Table() == event.LocationEvents {
		location := entry.data.Events()[entry.selected].Location
		if field.ID == event.ChestItem {
			low, high = 1, int64(location.ItemMaximum)
		}
		if field.ID == event.ChestMoney && location.Item == location.GoldItem {
			low = int64(location.GoldMinimum)
		}
	}
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
