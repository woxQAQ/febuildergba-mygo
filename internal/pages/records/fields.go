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
	text        string
	reference   components.TextReferenceState
	value       int64
	epoch       uint64
	initialized bool
	invalid     bool
	input       ui.Handle
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
		draft = &fieldDraft{}
		a.drafts[key] = draft
	}
	if !draft.initialized || draft.value != value ||
		draft.epoch != a.state.HistoryEpoch {
		draft.text = formatField(f, value)
		draft.value = value
		draft.epoch = a.state.HistoryEpoch
		draft.invalid = false
		draft.initialized = true
	}
	low, high := f.Bounds()
	help := fmt.Sprintf(
		"%s\n%s: %d…%d\n%s",
		a.fieldLabel(kind, f),
		locales.AllowedRange(),
		low,
		high,
		locales.FieldHint(),
	)
	raw := a.rawRecord(kind, row.ID)
	if raw {
		help += fmt.Sprintf(
			"\n0x%08X   0x%X",
			row.Address+f.Offset,
			value&((int64(1)<<(f.Width*8))-1),
		)
	}
	commit := func(next int64) {
		if err := a.state.Edit(func() error {
			return a.state.Session.Records.Set(
				kind,
				[]int{row.ID},
				f.ID,
				next,
			)
		}); err != nil {
			a.state.SetErr(err)
			return
		}
		draft.value = next
		draft.invalid = false
	}
	input := ui.TextInput(c.Key(f.ID), &draft.text).Height(26).Padding(3, 6).
		FontFeatures("tnum").TextAlign(ui.End).Disabled(a.state.Busy).
		Bind(&draft.input).
		Label(a.fieldLabel(kind, f)).
		Tooltip(help).OnTextKey(func(k ui.TextKey) bool {
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
	}).ContextMenu(func(menu *ui.Menu) {
		menu.EditItems()
		menu.Separator()
		if menu.Item(locales.IncreaseValue()).
			Disabled(a.state.Busy || draft.value >= high).Chosen() {
			a.nudgeField(key, f, 1)
		}
		if menu.Item(locales.DecreaseValue()).
			Disabled(a.state.Busy || draft.value <= low).Chosen() {
			a.nudgeField(key, f, -1)
		}
	})
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
		err = a.state.Edit(func() error {
			return a.state.Session.Records.Set(
				key.Kind,
				[]int{key.Record},
				key.Field,
				value+delta,
			)
		})
	}
	if err != nil {
		a.state.SetErr(err)
		return
	}
	if draft := a.drafts[key]; draft != nil {
		draft.value = value + delta
		draft.text = formatField(f, draft.value)
		draft.invalid = false
		draft.input.Focus()
	}
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
			if len(sections[group]) == 0 {
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
		if f.Reference != "" ||
			f.Text != text.NoRole && group != records.GroupAdvanced {
			actionWidth = 26
		}
	}
	width, _ := c.Size()
	if width-a.sidebar-32 < 680 {
		columns = 1
	}
	return ui.Column(c.Key(group)).Gap(8).Children(func() {
		if group != records.GroupAdvanced {
			ui.Text(c, locales.Label(group)).Bold()
		}
		ui.Grid(c).Columns(columns).GapX(16).GapY(4).Children(func() {
			for _, f := range fields {
				a.inlineField(
					c,
					kind,
					row,
					f,
					inputWidth,
					actionWidth,
					group == records.GroupAdvanced,
				)
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
	raw bool,
) {
	if f.Text != text.NoRole && !raw {
		a.textReferenceField(c, kind, row, f)
		return
	}
	ui.Row(c.Key(f.ID)).Gap(6).MinHeight(28).Children(func() {
		key := fieldKey{kind, row.ID, f.ID}
		label := locales.Label(f.ID)
		help := label
		if kind == records.Chapters &&
			(f.ID == "objective_text_id" || f.ID == "objective_detail_text_id") {
			help += "\n" + locales.ChapterObjectiveHelp()
		}
		caption := ui.Text(c, label).
			MaxWidth(140).
			MinWidth(0).
			Grow(1).
			Tooltip(help).
			OnClick(func() { a.focusField(key) })
		if kind != records.Chapters {
			caption.SingleLine()
		}
		value, err := a.state.Session.Records.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		a.numberField(c, kind, row, f, value).Width(inputWidth).Shrink(0)
		switch {
		case f.Reference != "":
			caption := fmt.Sprintf(
				"%s %02X",
				locales.Label(string(f.Reference)),
				value,
			)
			_, target, _, err := a.state.Session.Records.Resolve(
				f.Reference,
				int(value),
				"id",
			)
			if err == nil {
				caption = fmt.Sprintf("%02X · %s", target.ID, target.Name)
			}
			components.ReferenceLink(c, locales.OpenReference(caption),
				a.state.Busy || a.jump == nil || err != nil,
				func() { a.jump(f.Reference, int(value)) },
			)

		case f.Asset == graphics.Portrait && a.openPortrait != nil:
			components.ReferenceLink(
				c,
				locales.OpenReference(locales.Label("portrait_id")),
				a.state.Busy || value == 0,
				func() { a.openPortrait(int(value)) },
			)
		case actionWidth > 0:
			ui.Box(c).Width(actionWidth).Shrink(0)
		}
	})
}
