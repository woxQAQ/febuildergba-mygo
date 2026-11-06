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
	jump func(records.Kind, int),
) {
	for _, group := range []string{records.GroupGeneral, "deploy_position", "items", "deploy_ai"} {
		var fields []deployment.Field
		for _, field := range entry.data.Fields() {
			if field.Group == group ||
				group == "deploy_position" &&
					field.Group == records.GroupAdvanced {
				fields = append(fields, field)
			}
		}
		ui.Column(c.Key(group)).Gap(6).Children(func() {
			ui.Text(c, locales.Label(group)).Bold()
			if group == "deploy_position" {
				v.positionFields(c, state, entry, fields)
				return
			}
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				for _, field := range fields {
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
						ui.Row(c.Key(field.ID)).Width(420).MaxWidthPercent(100).
							Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, locales.Label(field.ID)).
								Width(76).
								Shrink(0)
							v.unitReference(c, state, entry, field, kind, jump)
						})
						continue
					}
					ui.Row(c.Key(field.ID)).
						Gap(6).
						AlignItems(ui.Center).
						Children(func() {
							if field.ID == "deploy_autolevel" {
								value, _ := entry.data.Value(
									entry.selected,
									field.ID,
								)
								enabled := value != 0
								ui.Checkbox(c.Key("value"), &enabled, locales.Label(field.ID)).
									Label(locales.Deployments() + " / " + locales.Label(field.ID)).
									Tooltip(locales.DeployPositionHelp()).
									Disabled(state.Busy || !entry.data.Current(state.Session.Deployments)).
									OnClick(func() {
										var value int64
										if enabled {
											value = 1
										}
										state.SetErr(
											entry.data.Set(
												entry.selected,
												field.ID,
												value,
											),
										)
										entry.preview = nil
									})
								return
							}
							ui.Text(c, locales.Label(field.ID)).
								Tooltip(locales.DeployPositionHelp())
							if field.ID == "deploy_faction" {
								v.unitFaction(c, state, entry, field)
							} else {
								v.number(c, state, entry, field)
							}
						})
				}
			})
		})
	}
}

func (v *Page) positionFields(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	fields []deployment.Field,
) {
	for _, pair := range []struct{ label, x, y string }{
		{locales.DeployStart(), "deploy_start_x", "deploy_start_y"},
		{locales.DeployEnd(), "deploy_end_x", "deploy_end_y"},
	} {
		ui.Row(c.Key(pair.x)).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, pair.label).
				Width(104).
				Shrink(0).
				Tooltip(locales.DeployPositionHelp())
			for _, field := range fields {
				if field.ID != pair.x && field.ID != pair.y {
					continue
				}
				ui.Row(c.Key(field.ID)).
					Gap(4).
					AlignItems(ui.Center).
					Children(func() {
						axis := "X"
						if field.ID == pair.y {
							axis = "Y"
						}
						ui.Text(c, axis)
						v.number(c, state, entry, field)
					})
			}
		})
	}
	for _, field := range fields {
		if field.Group != records.GroupAdvanced {
			continue
		}
		ui.Row(c.Key(field.ID)).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.Label(field.ID)).
				Width(104).
				Tooltip(locales.DeployPositionHelp())
			v.number(c, state, entry, field)
		})
	}
}

func (v *Page) unitReference(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	field deployment.Field,
	kind records.Kind,
	jump func(records.Kind, int),
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	unit := entry.selected
	value, _ := entry.data.Value(unit, field.ID)
	rows := make(map[int64]string)
	if table := state.Session.Records.Table(
		kind,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			rows[int64(row.ID)] = row.Name
		}
	}
	choices := make([]string, 0, field.Max-field.Min+1)
	for id := field.Min; id <= field.Max; id++ {
		name := rows[id]
		if id == 0 {
			name = locales.DeployNoReference()
			if field.ID == "deploy_class" {
				name = locales.DeployInheritClass()
			}
		}
		caption := fmt.Sprintf("%02X", id)
		if name != "" {
			caption += " · " + name
		}
		choices = append(choices, caption)
	}
	key := inputKey{unit, field.ID}
	draft := entry.inputs[key]
	if draft == nil {
		draft = &numberDraft{}
		entry.inputs[key] = draft
	}
	if draft.value != value || !slices.Contains(choices, draft.text) {
		draft.text, draft.value = choices[value-field.Min], value
	}
	components.TextDraft(state,
		ui.Combobox(c.Key("value"), &draft.text, choices).
			Grow(1).MinWidth(0).Height(26).
			Label(locales.Deployments()+" / "+locales.Label(field.ID)).
			Disabled(state.Busy || !entry.data.Current(state.Session.Deployments)).
			OnChange(func() {
				if index := slices.Index(choices, draft.text); index >= 0 {
					selected := int64(index) + field.Min
					state.SetErr(entry.data.Set(unit, field.ID, selected))
					draft.value = selected
					entry.preview = nil
				}
			}),
	)
	_, target, _, err := state.Session.Records.Resolve(kind, int(value), "id")
	components.ReferenceLink(
		c,
		locales.OpenReference(fmt.Sprintf("%02X · %s", value, target.Name)),
		state.Busy || value == 0 || err != nil || jump == nil,
		func() { jump(kind, int(value)) },
	)
}

func (v *Page) unitFaction(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	field deployment.Field,
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	choices := []string{
		locales.DeployFactionPlayer(), locales.DeployFactionAlly(),
		locales.DeployFactionEnemy(), locales.DeployFactionReserved(),
	}
	unit := entry.selected
	value, _ := entry.data.Value(unit, field.ID)
	key := inputKey{unit, field.ID}
	draft := entry.inputs[key]
	if draft == nil {
		draft = &numberDraft{}
		entry.inputs[key] = draft
	}
	if draft.value != value || !slices.Contains(choices, draft.text) {
		draft.text, draft.value = choices[value], value
	}
	ui.Select(c.Key("value"), &draft.text, choices).Width(110).
		Label(locales.Deployments() + " / " + locales.Label(field.ID)).
		Disabled(state.Busy || !entry.data.Current(state.Session.Deployments)).
		OnChange(func() {
			if index := slices.Index(choices, draft.text); index >= 0 {
				state.SetErr(entry.data.Set(unit, field.ID, int64(index)))
				draft.value = int64(index)
				entry.preview = nil
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
			Width(58).Height(26).Padding(3, 6).
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
					state.SetErr(err)
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
				if state.SetErr(entry.data.Set(
					entry.selected,
					field.ID,
					parsed,
				)); state.Err != nil {
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
				state.SetErr(entry.data.Set(
					entry.selected,
					field.ID,
					values[index],
				))
				entry.preview = nil
			}
		})
}
