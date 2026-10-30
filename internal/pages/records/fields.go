package records

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type fieldKey struct {
	Kind   records.Kind
	Record int
	Field  string
}

type fieldDraft struct {
	text    string
	value   int64
	epoch   uint64
	invalid bool
	input   ui.Handle
}

func formatField(f records.Field, value int64) string {
	if f.Hex {
		return fmt.Sprintf("0x%08X", value)
	}
	return strconv.FormatInt(value, 10)
}

func (a *form) fieldLabel(kind records.Kind, f records.Field) string {
	group := locales.Label(f.Group)
	if f.Group == records.GroupBase && !strings.HasPrefix(f.ID, "base.") {
		group = locales.General()
	} else if kind == records.Units && strings.HasPrefix(f.ID, "base.") {
		group = locales.PersonalBase()
	}
	return group + " / " + locales.Label(f.ID)
}

// Field drafts retain incomplete input, while every valid value belongs to ROM
// history.
func (a *form) numberField(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	f records.Field,
	value int64,
) ui.Element {
	if a.drafts == nil {
		a.drafts = make(map[fieldKey]*fieldDraft)
	}
	key := fieldKey{kind, row.ID, f.ID}
	draft := a.drafts[key]
	if draft == nil {
		draft = &fieldDraft{
			text:  formatField(f, value),
			value: value,
			epoch: a.state.HistoryEpoch,
		}
		a.drafts[key] = draft
	}
	if draft.value != value || draft.epoch != a.state.HistoryEpoch {
		draft.text = formatField(f, value)
		draft.value = value
		draft.epoch = a.state.HistoryEpoch
		draft.invalid = false
	}
	low, high := f.Bounds()
	commit := func(next int64) {
		if err := a.state.Session.Records.Set(
			kind,
			[]int{row.ID},
			f.ID,
			next,
		); err != nil {
			a.state.Err = err
			return
		}
		draft.value = next
		draft.invalid = false
	}
	input := ui.TextInput(c.Key(f.ID), &draft.text).Height(26).Padding(3, 6).
		FontSize(13).
		FontFeatures("tnum").TextAlign(ui.End).Disabled(a.state.Busy).
		Bind(&draft.input).
		Label(a.fieldLabel(kind, f)).OnTextKey(func(k ui.TextKey) bool {
		switch {
		case k.Key == ui.KeyZ && k.Mods == ui.Cmd:
			a.state.Undo()
		case k.Key == ui.KeyZ && k.Mods == ui.Cmd|ui.Shift:
			a.state.Redo()
		case k.Key == ui.KeyUp && k.Mods == 0:
			if draft.value < high {
				commit(draft.value + 1)
			}
		case k.Key == ui.KeyDown && k.Mods == 0:
			if draft.value > low {
				commit(draft.value - 1)
			}
		default:
			return false
		}
		current, err := a.state.Session.Records.Value(kind, row.ID, f.ID)
		if err == nil {
			draft.value = current
			draft.epoch = a.state.HistoryEpoch
			draft.invalid = false
			k.SetText(formatField(f, current))
		}
		return true
	}).OnChange(func() {
		parsed, err := input.ParseInteger(draft.text)
		if err != nil || parsed < low || parsed > high {
			draft.invalid = true
			return
		}
		commit(parsed)
	})
	if draft.input.Focused(c) {
		a.activeField = key
	}
	if draft.invalid {
		message := fmt.Sprintf("%s: %d…%d", locales.AllowedRange(), low, high)
		input.Error(message).Tooltip(message)
	}
	return input
}

func (a *form) focusField(key fieldKey) {
	if draft := a.drafts[key]; draft != nil {
		draft.input.Focus()
	}
}

func (a *form) nudgeField(key fieldKey, f records.Field, delta int64) {
	if a.state.Busy || a.state.Session == nil {
		return
	}
	value, err := a.state.Session.Records.Value(key.Kind, key.Record, key.Field)
	low, high := f.Bounds()
	if err == nil && (value+delta < low || value+delta > high) {
		return
	}
	if err == nil {
		err = a.state.Session.Records.Set(
			key.Kind,
			[]int{key.Record},
			key.Field,
			value+delta,
		)
	}
	if err != nil {
		a.state.Err = err
		return
	}
	if draft := a.drafts[key]; draft != nil {
		draft.value = value + delta
		draft.text = formatField(f, draft.value)
		draft.invalid = false
		draft.input.Focus()
	}
}

func (a *form) fieldInspector(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	key := a.activeField
	var active *records.Field
	if key.Kind == kind && key.Record == row.ID {
		for i := range table.Fields {
			f := &table.Fields[i]
			if f.ID == key.Field &&
				(f.Group != records.GroupAdvanced || a.showRaw) {
				active = f
				break
			}
		}
	}
	ui.Column(c.Key("field-inspector")).Padding(6, 16).Gap(4).
		BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
		if active == nil {
			ui.Text(c, locales.FieldHint()).
				MinHeight(26).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
			return
		}
		f := *active
		value, err := a.state.Session.Records.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		low, high := f.Bounds()
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, a.fieldLabel(kind, f)).Grow(1).SingleLine().FontSize(13)
			ui.Text(c, formatField(f, value)).FontFeatures("tnum").FontSize(13)
			if (kind == records.Units || kind == records.Classes) &&
				f.Group == records.GroupWeapon {
				weapons, err := a.state.Session.Records.WeaponSkills(
					kind,
					row.ID,
				)
				if err == nil {
					for _, weapon := range weapons {
						if weapon.Field == f.ID {
							a.profileRank(
								c,
								kind,
								row,
								table,
								a.profileFor(kind, row.ID),
								weapon,
							)
							break
						}
					}
				}
			}
			ui.Button(c, "−").Size(28, 26).Label(locales.DecreaseValue()).
				Disabled(a.state.Busy || value <= low).
				OnClick(func() { a.nudgeField(key, f, -1) })
			ui.Button(c, "+").Size(28, 26).Label(locales.IncreaseValue()).
				Disabled(a.state.Busy || value >= high).
				OnClick(func() { a.nudgeField(key, f, 1) })
		})
		if draft := a.drafts[key]; draft != nil && draft.invalid {
			ui.Textf(c, "%s: %d…%d", locales.AllowedRange(), low, high).
				FontSize(12).
				TextColor(c.Theme().Danger)
		}
		if a.showRaw {
			ui.Row(c).Gap(8).Children(func() {
				ui.Textf(c, "0x%08X   0x%X", row.Address+f.Offset, value&((int64(1)<<(f.Width*8))-1)).
					Font("monospace").
					FontSize(12).
					Grow(1)
				ui.Button(c, locales.EditBytes()).
					Disabled(a.state.Busy).
					OnClick(func() { a.openHex(row.Address+f.Offset, f.Width) })
			})
		}
	})
}

func (a *form) recordFields(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	sections := make(map[string][]records.Field)
	var extra []string
	for _, field := range table.Fields {
		group := field.Group
		if group == records.GroupBase {
			group = records.GroupGeneral
		}
		if group != records.GroupGeneral && group != records.GroupCombat &&
			group != records.GroupWeapon &&
			group != records.GroupAdvanced &&
			sections[group] == nil {
			extra = append(extra, group)
		}
		sections[group] = append(sections[group], field)
	}
	groups := append(
		[]string{
			records.GroupCombat,
			records.GroupWeapon,
			records.GroupGeneral,
		},
		extra...)
	groups = append(groups, records.GroupAdvanced)
	width, _ := c.Size()
	ui.Column(c.Key("field-groups")).Gap(24).Children(func() {
		for _, group := range groups {
			if len(sections[group]) == 0 ||
				group == records.GroupAdvanced && !a.showRaw {
				continue
			}
			columns := 2
			if group == records.GroupWeapon {
				columns = 4
			}
			if width-a.sidebar-40 < 440 {
				columns = max(1, columns/2)
			}
			a.inlineSection(c, kind, row, group, sections[group], columns).
				MaxWidth(600)
		}
	})
}

func (a *form) inlineSection(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	group string,
	fields []records.Field,
	columns int,
) ui.Element {
	// Share input and action widths so each row keeps its values aligned.
	inputWidth, actionWidth := float32(56), float32(0)
	for _, f := range fields {
		if f.Width > 2 {
			inputWidth = 104
		} else if f.Width == 2 {
			inputWidth = max(inputWidth, 72)
		}
		if f.Reference != "" || f.Text != text.NoRole {
			actionWidth = 48
		}
	}
	return ui.Column(c.Key(group)).Gap(8).Children(func() {
		ui.Text(c, locales.Label(group)).Bold().FontSize(13)
		ui.Grid(c).Columns(columns).GapX(16).GapY(4).Children(func() {
			for _, f := range fields {
				a.inlineField(c, kind, row, f, inputWidth, actionWidth)
			}
		})
	})
}

func (a *form) inlineField(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	f records.Field,
	inputWidth, actionWidth float32,
) {
	ui.Row(c.Key(f.ID)).Gap(6).MinHeight(28).Children(func() {
		key := fieldKey{kind, row.ID, f.ID}
		label := locales.Label(f.ID)
		help := label
		if kind == records.Chapters &&
			(f.ID == "objective_text_id" || f.ID == "objective_detail_text_id") {
			help += "\n" + locales.ChapterObjectiveHelp()
		}
		ui.Text(c, label).
			FontSize(13).
			SingleLine().
			MaxWidth(140).
			MinWidth(0).
			Grow(1).
			Tooltip(help).
			OnClick(func() { a.focusField(key) })
		value, err := a.state.Session.Records.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		a.numberField(c, kind, row, f, value).Width(inputWidth).Shrink(0)
		switch {
		case f.Reference != "":
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy).
				OnClick(func() { a.jump(f.Reference, int(value)) })
		case f.Text != text.NoRole:
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy || value <= 0 || value >= 0x7FFF).
				OnClick(func() {
					if value > 0 && value < 0x7FFF {
						a.openText(uint16(value))
					}
				})
		case f.Asset == graphics.Portrait && a.openPortrait != nil:
			ui.Button(c, locales.Jump()).
				Height(26).
				Disabled(a.state.Busy).
				OnClick(func() { a.openPortrait(int(value)) })
		case actionWidth > 0:
			ui.Box(c).Width(actionWidth).Shrink(0)
		}
	})
}
