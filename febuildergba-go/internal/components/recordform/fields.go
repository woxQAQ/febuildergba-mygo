package recordform

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/locales"

	"github.com/egoist/mygo/ui"
)

type fieldKey struct {
	Kind   game.Kind
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

func formatField(f game.Field, value int64) string {
	if f.Hex {
		return fmt.Sprintf("0x%08X", value)
	}
	return strconv.FormatInt(value, 10)
}

func (a *Form) fieldLabel(kind game.Kind, f game.Field) string {
	group := locales.Label(f.Group)
	if f.Group == game.GroupBase && !strings.HasPrefix(f.ID, "base.") {
		group = locales.General()
	} else if kind == game.Units && strings.HasPrefix(f.ID, "base.") {
		group = locales.PersonalBase()
	}
	return group + " / " + locales.Label(f.ID)
}

// Field drafts retain incomplete input, while every valid value belongs to ROM history.
func (a *Form) numberField(c *ui.Context, kind game.Kind, row game.Row, f game.Field, value int64) ui.Element {
	if a.drafts == nil {
		a.drafts = make(map[fieldKey]*fieldDraft)
	}
	key := fieldKey{kind, row.ID, f.ID}
	draft := a.drafts[key]
	if draft == nil {
		draft = &fieldDraft{text: formatField(f, value), value: value, epoch: a.state.HistoryEpoch}
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
		if err := a.state.Session.Set(kind, []int{row.ID}, f.ID, next); err != nil {
			a.state.Err = err
			return
		}
		draft.value = next
		draft.invalid = false
	}
	input := ui.TextInput(c.Key(f.ID), &draft.text).Height(26).Padding(3, 6).
		FontSize(13).FontFeatures("tnum").TextAlign(ui.End).Disabled(a.state.Busy).
		Bind(&draft.input).Label(a.fieldLabel(kind, f)).OnTextKey(func(k ui.TextKey) bool {
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
		current, err := a.state.Session.Value(kind, row.ID, f.ID)
		if err == nil {
			draft.value = current
			draft.epoch = a.state.HistoryEpoch
			draft.invalid = false
			k.SetText(formatField(f, current))
		}
		return true
	}).OnChange(func() {
		parsed, err := components.ParseInteger(draft.text)
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

func (a *Form) focusField(key fieldKey) {
	if draft := a.drafts[key]; draft != nil {
		draft.input.Focus()
	}
}

func (a *Form) nudgeField(key fieldKey, f game.Field, delta int64) {
	if a.state.Busy || a.state.Session == nil {
		return
	}
	value, err := a.state.Session.Value(key.Kind, key.Record, key.Field)
	low, high := f.Bounds()
	if err == nil && (value+delta < low || value+delta > high) {
		return
	}
	if err == nil {
		err = a.state.Session.Set(key.Kind, []int{key.Record}, key.Field, value+delta)
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

func (a *Form) fieldInspector(c *ui.Context, kind game.Kind, row game.Row, table *game.Table) {
	key := a.activeField
	var active *game.Field
	if key.Kind == kind && key.Record == row.ID {
		for i := range table.Fields {
			f := &table.Fields[i]
			if f.ID == key.Field && (f.Group != game.GroupAdvanced || a.showRaw) {
				active = f
				break
			}
		}
	}
	ui.Column(c.Key("field-inspector")).Padding(6, 16).Gap(4).
		BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
		if active == nil {
			ui.Text(c, locales.FieldHint()).MinHeight(26).FontSize(12).TextColor(c.Theme().TextMuted)
			return
		}
		f := *active
		value, err := a.state.Session.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		low, high := f.Bounds()
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, a.fieldLabel(kind, f)).Grow(1).SingleLine().FontSize(13)
			ui.Text(c, formatField(f, value)).FontFeatures("tnum").FontSize(13)
			ui.Button(c, "−").Size(28, 26).Label(locales.DecreaseValue()).
				Disabled(a.state.Busy || value <= low).OnClick(func() { a.nudgeField(key, f, -1) })
			ui.Button(c, "+").Size(28, 26).Label(locales.IncreaseValue()).
				Disabled(a.state.Busy || value >= high).OnClick(func() { a.nudgeField(key, f, 1) })
		})
		if draft := a.drafts[key]; draft != nil && draft.invalid {
			ui.Textf(c, "%s: %d…%d", locales.AllowedRange(), low, high).FontSize(12).TextColor(c.Theme().Danger)
		}
		if a.showRaw {
			ui.Row(c).Gap(8).Children(func() {
				ui.Textf(c, "0x%08X   0x%X", row.Address+f.Offset, value&((int64(1)<<(f.Width*8))-1)).Font("monospace").FontSize(12).Grow(1)
				ui.Button(c, locales.EditBytes()).Disabled(a.state.Busy).OnClick(func() { a.openHex(row.Address+f.Offset, f.Width) })
			})
		}
	})
}
