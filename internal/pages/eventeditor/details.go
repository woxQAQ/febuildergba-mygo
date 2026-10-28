package eventeditor

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	inputparse "github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (page *Page) commands(c *ui.Context, draft *draftState) {
	ops := page.state.Session.ScriptCommands()
	labels := make([]string, len(ops))
	for index, op := range ops {
		labels[index] = locales.Label(string(op))
	}
	if !slices.Contains(labels, draft.command) {
		index := max(0, slices.Index(ops, game.ScriptWait))
		draft.command = labels[index]
	}
	disabled := page.state.Busy || !draft.data.Current(page.state.Session) ||
		!draft.data.Relocatable()
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Select(c.Key("new-command"), &draft.command, labels).
			Width(190).
			Label(locales.ScriptNewCommand()).
			Disabled(disabled)
		ui.Button(c, locales.ScriptInsert()).Disabled(disabled).OnClick(func() {
			if index := slices.Index(labels, draft.command); index >= 0 {
				page.state.Err = draft.data.Insert(draft.selected, ops[index])
				if page.state.Err == nil {
					draft.structuralChanged(draft.selected)
				}
			}
		})
		for _, action := range []struct {
			label    string
			enabled  bool
			run      func() error
			selected int
		}{
			{locales.ScriptCopy(), true, func() error { return draft.data.Duplicate(draft.selected) }, draft.selected + 1},
			{locales.ScriptDelete(), true, func() error { return draft.data.Remove(draft.selected) }, max(0, draft.selected-1)},
			{locales.MoveUp(), draft.selected > 0, func() error { return draft.data.Move(draft.selected, draft.selected-1) }, draft.selected - 1},
			{locales.MoveDown(), draft.selected+1 < len(draft.data.Instructions()), func() error { return draft.data.Move(draft.selected, draft.selected+1) }, draft.selected + 1},
		} {
			ui.Button(c, action.label).
				Disabled(disabled || !action.enabled).
				OnClick(func() {
					page.state.Err = action.run()
					if page.state.Err == nil {
						draft.structuralChanged(action.selected)
					}
				})
		}
	})
}

func (page *Page) operands(
	c *ui.Context,
	draft *draftState,
	row game.ScriptInstruction,
) {
	fields := draft.data.Fields(draft.selected)
	if !row.Editable {
		ui.Text(c, locales.ScriptReadonlyInstruction()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Wrap().Gap(16).Children(func() {
		for _, field := range fields {
			ui.Row(c.Key(field.ID)).
				Width(350).
				MaxWidthPercent(100).
				Gap(8).
				AlignItems(ui.Center).
				Children(func() {
					ui.Text(c, locales.Label(field.ID)).
						Grow(1).
						Tooltip(locales.ScriptOperandHelp())
					page.number(c, draft, row, field)
				})
		}
	})
	for _, field := range fields {
		switch field.ID {
		case game.ScriptPointer, game.ScriptUnitPointer:
			page.referenceChoice(c, draft, row, field)
		}
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, field := range fields {
			value, _ := draft.data.Value(draft.selected, field.ID)
			switch field.ID {
			case game.ScriptTextID:
				textID := uint16(
					value,
				) // #nosec G115 -- Text operands are bounded to 16 bits.
				ui.Button(c, fmt.Sprintf("%s · %04X", locales.TextEditor(), value)).
					Disabled(page.state.Busy || value <= 0 || value >= 0x7FFF).
					OnClick(func() { page.openText(textID) })
			// #nosec G115 -- The control is enabled only for bounded text IDs.
			case game.ScriptPointer, game.ScriptUnitPointer:
				reference, err := draft.data.Reference(draft.selected, field.ID)
				label := locales.ScriptOpenCall()
				if field.ID == game.ScriptUnitPointer {
					label = locales.TurnOpenUnits()
				}
				ui.Button(c, label).
					Tooltip(locales.ScriptReferenceHelp()).
					Disabled(page.state.Busy || err != nil || draft.data.Modified()).
					OnClick(func() {
						if field.ID == game.ScriptPointer {
							page.Open(reference.Chapter, reference.Pointer)
						} else {
							page.openDeployment(
								reference.Chapter,
								reference.Pointer,
							)
						}
					})
			case game.ScriptLabelID:
				if row.Op == game.ScriptLabel {
					continue
				}
				ui.Button(c, locales.ScriptFindLabel()).
					Disabled(page.state.Busy).
					OnClick(func() {
						for index, instruction := range draft.data.Instructions() {
							if instruction.Op != game.ScriptLabel {
								continue
							}
							if label, _ := draft.data.Value(
								index,
								game.ScriptLabelID,
							); label == value {
								draft.selected = index
								draft.list.ScrollIntoView(index)
								break
							}
						}
					})
			}
		}
		if row.Address >= 0xC0 {
			ui.Button(c, locales.EditBytes()).
				Disabled(page.state.Busy).
				OnClick(func() { page.openHex(row.Address, len(row.Raw)) })
		}
	})
	if page.showRaw || !row.Editable {
		ui.Textf(c, "% X", row.Raw).Font("monospace").Selectable()
	}
}

func (page *Page) number(
	c *ui.Context,
	draft *draftState,
	row game.ScriptInstruction,
	field game.ScriptField,
) {
	key := operandKey{row.ID, field.ID}
	value, _ := draft.data.Value(draft.selected, field.ID)
	input := draft.inputs[key]
	format := func(value int64) string {
		if field.Hex {
			return fmt.Sprintf("0x%X", value)
		}
		return strconv.FormatInt(value, 10)
	}
	if input == nil {
		input = &numberDraft{text: format(value), value: value}
		draft.inputs[key] = input
	}
	if input.value != value {
		input.text, input.value, input.invalid = format(value), value, false
	}
	index := draft.selected
	control := components.TextDraft(
		page.state,
		ui.TextInput(c.Key("operand"), &input.text).
			Width(130).
			Height(28).
			Padding(4, 8).
			FontFeatures("tnum").
			TextAlign(ui.End).
			Label(locales.ScriptEditor()+" / "+locales.Label(field.ID)).
			Disabled(page.state.Busy || !draft.data.Current(page.state.Session)).
			OnChange(func() {
				draft.preview = nil
				parsed, err := inputparse.ParseInteger(input.text)
				if err != nil || parsed < 0 || parsed > field.Max {
					input.invalid = true
					return
				}
				page.state.Err = draft.data.Set(index, field.ID, parsed)
				input.invalid = page.state.Err != nil
				if !input.invalid {
					input.value = parsed
				}
			}),
	)
	if input.invalid {
		control.Error(
			fmt.Sprintf("%s: 0…%d", locales.AllowedRange(), field.Max),
		)
	}
}

func (page *Page) referenceChoice(
	c *ui.Context,
	draft *draftState,
	row game.ScriptInstruction,
	field game.ScriptField,
) {
	var values []int64
	var captions []string
	add := func(address int, caption string) {
		if address < 0xC0 ||
			slices.Contains(values, int64(0x08000000+address)) {
			return
		}
		values = append(values, int64(0x08000000+address))
		captions = append(
			captions,
			fmt.Sprintf("0x%08X · %s", 0x08000000+address, caption),
		)
	}
	if field.ID == game.ScriptPointer {
		for _, entry := range page.entries {
			if entry.Err == nil {
				add(entry.Address, locales.Label(entry.Source))
			}
		}
	} else if groups, err := page.state.Session.ChapterDeployments(page.chapter); err == nil {
		for _, group := range groups.Groups {
			if group.Err == nil {
				add(group.Address, locales.Label(group.Label))
			}
		}
	}
	if len(captions) == 0 {
		return
	}
	key := operandKey{row.ID, field.ID}
	value, _ := draft.data.Value(draft.selected, field.ID)
	reference := draft.references[key]
	if reference == nil {
		reference = &referenceDraft{value: -1}
		draft.references[key] = reference
	}
	if reference.value != value {
		reference.value = value
		reference.choice = ""
		if index := slices.Index(values, value); index >= 0 {
			reference.choice = captions[index]
		}
	}
	ui.Select(c.Key(key), &reference.choice, captions).
		Width(440).
		MaxWidthPercent(100).
		Label(locales.ScriptChooseReference()).
		Placeholder(locales.ScriptChooseReference()).
		Disabled(page.state.Busy || !draft.data.Current(page.state.Session)).
		OnChange(func() {
			if index := slices.Index(captions, reference.choice); index >= 0 {
				page.state.Err = draft.data.Set(
					draft.selected,
					field.ID,
					values[index],
				)
				draft.preview = nil
			}
		})
}

func (page *Page) actions(c *ui.Context, draft *draftState) {
	stale := !draft.data.Current(page.state.Session)
	if !draft.data.Modified() && !draft.invalid() && !stale {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.ScriptPreview()).
			Disabled(page.state.Busy || stale || draft.invalid() || !draft.data.Modified()).
			OnClick(func() { draft.preview, page.state.Err = page.state.Session.PlanScriptEdit(draft.data) })
		ui.Button(c, locales.ScriptReload()).
			Disabled(page.state.Busy).
			OnClick(func() { delete(page.drafts, draft.data.Entry().Pointer) })
	})
	if draft.preview == nil {
		return
	}
	preview := draft.preview
	ui.Textf(c, "%d → %d · 0x%08X · %d B", preview.Before(), preview.After(), preview.Destination(), preview.Length()).
		Selectable().
		FontFeatures("tnum")
	scope := locales.ScriptPatchScope()
	if preview.Relocated() {
		scope = locales.ScriptRelocateScope()
	}
	ui.Text(c, scope).FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Scroll(c).MaxHeight(120).Children(func() {
		for _, use := range preview.Uses() {
			ui.Textf(c, "%02X · %s · 0x%08X", use.Chapter, locales.Label(use.Source), use.Pointer).
				FontSize(12).
				FontFeatures("tnum")
		}
	})
	ui.Button(c, locales.ScriptApply()).
		Disabled(page.state.Busy || stale || draft.invalid()).
		OnClick(func() {
			page.state.Err = page.state.Session.ApplyScriptEdit(preview)
			if page.state.Err == nil {
				delete(page.drafts, draft.data.Entry().Pointer)
				page.state.HistoryEpoch++
			}
		})
}
