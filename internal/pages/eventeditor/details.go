package eventeditor

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (page *Page) commands(c *ui.Context, draft *draftState) {
	ops := page.state.Session.Events.ScriptCommands()
	labels := make([]string, len(ops))
	for index, op := range ops {
		labels[index] = locales.Label(string(op))
	}
	if !slices.Contains(labels, draft.command) {
		index := max(0, slices.Index(ops, event.ScriptWait))
		draft.command = labels[index]
	}
	disabled := page.state.Busy ||
		!draft.data.Current(page.state.Session.Events) ||
		!draft.data.Relocatable()
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Select(c.Key("new-command"), &draft.command, labels).
			Width(190).
			Label(locales.ScriptNewCommand()).
			Disabled(disabled)
		ui.Button(c, locales.ScriptInsert()).Disabled(disabled).OnClick(func() {
			if index := slices.Index(labels, draft.command); index >= 0 {
				page.state.SetErr(draft.data.Insert(draft.selected, ops[index]))
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
					page.state.SetErr(action.run())
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
	row event.ScriptInstruction,
) {
	fields := draft.data.Fields(draft.selected)
	if row.Help != "" {
		ui.Text(c, locales.Label(row.Help)).
			FontSize(components.FontCaption).
			TextColor(c.Theme().TextMuted)
	}
	if !row.Editable {
		ui.Text(c, locales.ScriptReadonlyInstruction()).
			FontSize(components.FontCaption).
			TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, field := range fields {
			if field.Target == event.ScriptTextTarget {
				continue
			}
			ui.Row(c.Key(field.ID)).
				Width(350).
				MaxWidthPercent(100).
				Gap(8).
				AlignItems(ui.Center).
				Children(func() {
					tooltip := locales.ScriptOperandHelp()
					if field.Help != "" {
						tooltip = locales.Label(field.Help)
					}
					ui.Text(c, locales.Label(field.ID)).
						Grow(1).
						Tooltip(tooltip)
					page.number(c, draft, row, field)
				})
		}
	})
	for _, field := range fields {
		if field.Target == event.ScriptTextTarget {
			page.textOperand(c, draft, row, field)
		}
		if field.Target == event.ScriptMovementTarget {
			page.movement(c, draft, row, field)
		}
	}
	var references []event.ScriptField
	for _, field := range fields {
		if field.Target == event.ScriptEventTarget ||
			field.Target == event.ScriptDeploymentTarget ||
			field.Target == event.ScriptWorldMapNodeTarget || field.Target == event.ScriptWorldMapRouteTarget ||
			field.Target == event.ScriptPortraitTarget ||
			field.Reference != "" {
			references = append(references, field)
		}
	}
	for _, field := range references {
		page.referenceChoice(c, draft, row, field, len(references) > 1)
		if field.Target == event.ScriptPortraitTarget {
			page.portraitOperand(c, draft, field)
		}
	}
	page.operandLinks(c, draft, row, fields)
	page.mapChanges(c, draft, row)
	if page.showRaw || !row.Editable {
		ui.Textf(c, "% X", row.Raw).Font("monospace").Selectable()
	}
}

func (page *Page) textOperand(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
	field event.ScriptField,
) {
	key := operandKey{row.ID, field.ID}
	current := draft.inputs[key]
	if current == nil {
		current = &numberDraft{}
		draft.inputs[key] = current
	}
	index := draft.selected
	value, _ := draft.data.Value(index, field.ID)
	ui.Column(c.Key(key)).Gap(4).Children(func() {
		label := locales.Label(field.ID)
		ui.Text(c, label).Tooltip(locales.Label(field.Help))
		if value < 0 || value > 0xFFFFFFFF {
			ui.Text(c, locales.GameInvalidValue()).TextColor(c.Theme().Danger)
			return
		}
		components.TextReference(
			c,
			page.state,
			&current.reference,
			uint32(value),
			locales.ScriptEditor()+" / "+label,
			page.showRaw,
			!row.Editable ||
				!draft.data.Current(page.state.Session.Events),
			func(id uint16) {
				draft.preview = nil
				page.state.SetErr(draft.data.Set(index, field.ID, int64(id)))
				current.invalid = page.state.Err != nil
			},
			page.openText,
		)
	})
}

func (page *Page) operandLinks(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
	fields []event.ScriptField,
) {
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, field := range fields {
			value, _ := draft.data.Value(draft.selected, field.ID)
			switch {
			case field.Target == event.ScriptWorldMapNodeTarget || field.Target == event.ScriptWorldMapRouteTarget:
				page.worldMapLink(c, draft, field, value)
			case field.Target == event.ScriptEventTarget || field.Target == event.ScriptDeploymentTarget:
				reference, err := draft.data.Reference(draft.selected, field.ID)
				if err == nil && reference.Indirect {
					ui.Textf(c, locales.ScriptIndirectUnitsReference(), 0x08000000+reference.Address, reference.Pointer).
						FontSize(components.FontCaption).
						TextColor(c.Theme().TextMuted)
				}
				label := locales.ScriptOpenCall()
				if field.Target == event.ScriptDeploymentTarget {
					label = locales.TurnOpenUnits()
				}
				ui.Button(c.Key(field.ID), label).
					Tooltip(locales.ScriptReferenceHelp()).
					Disabled(page.state.Busy || err != nil || draft.data.Modified() || !draft.data.Current(page.state.Session.Events)).
					OnClick(func() {
						if field.Target == event.ScriptEventTarget {
							page.Open(reference.Chapter, reference.Pointer)
						} else {
							page.openDeployment(
								reference.Chapter,
								reference.Pointer,
							)
						}
					})
			case field.ID == event.ScriptLabelID:
				if row.Op == event.ScriptLabel {
					continue
				}
				ui.Button(c, locales.ScriptFindLabel()).
					Disabled(page.state.Busy).
					OnClick(func() {
						for index, instruction := range draft.data.Instructions() {
							if instruction.Op != event.ScriptLabel {
								continue
							}
							if label, _ := draft.data.Value(
								index,
								event.ScriptLabelID,
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
}

func (page *Page) number(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
	field event.ScriptField,
) {
	key := operandKey{row.ID, field.ID}
	value, _ := draft.data.Value(draft.selected, field.ID)
	numberInput := draft.inputs[key]
	format := func(value int64) string {
		if field.Hex && value >= 0 {
			return fmt.Sprintf("0x%X", value)
		}
		return strconv.FormatInt(value, 10)
	}
	if numberInput == nil {
		numberInput = &numberDraft{text: format(value), value: value}
		draft.inputs[key] = numberInput
	}
	if numberInput.value != value {
		numberInput.text, numberInput.value, numberInput.invalid = format(
			value,
		), value, false
	}
	index := draft.selected
	control := components.TextDraft(
		page.state,
		ui.TextInput(c.Key("operand"), &numberInput.text).
			Width(130).
			Height(28).
			Padding(4, 8).
			FontFeatures("tnum").
			TextAlign(ui.End).
			Label(locales.ScriptEditor()+" / "+locales.Label(field.ID)).
			Disabled(page.state.Busy || !draft.data.Current(page.state.Session.Events)).
			OnChange(func() {
				draft.preview = nil
				parsed, err := input.ParseInteger(numberInput.text)
				if err != nil || parsed < field.Min || parsed > field.Max {
					numberInput.invalid = true
					return
				}
				page.state.SetErr(draft.data.Set(index, field.ID, parsed))
				numberInput.invalid = page.state.Err != nil
				if !numberInput.invalid {
					numberInput.value = parsed
				}
			}),
	)
	if numberInput.invalid {
		control.Error(
			fmt.Sprintf(
				"%s: %d…%d",
				locales.AllowedRange(),
				field.Min,
				field.Max,
			),
		)
	}
}

func (page *Page) referenceOptions(
	field event.ScriptField,
) ([]int64, []string) {
	if field.Target == event.ScriptWorldMapNodeTarget ||
		field.Target == event.ScriptWorldMapRouteTarget {
		return page.worldMapOptions(field)
	}
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
	if field.Target == event.ScriptPortraitTarget {
		return page.portraitOptions(field)
	}
	if field.Reference != "" {
		table := page.state.Session.Records.Table(field.Reference)
		if table == nil || table.Err != nil {
			return nil, nil
		}
		for _, record := range table.Rows {
			if record.ID <= 0 || int64(record.ID) > field.Max {
				continue
			}
			values = append(values, int64(record.ID))
			captions = append(
				captions,
				fmt.Sprintf("%02X · %s", record.ID, record.Name),
			)
		}
		return values, captions
	}
	if field.Target == event.ScriptEventTarget {
		for _, entry := range page.entries {
			if entry.Err == nil {
				add(entry.Address, entryCaption(entry))
			}
		}
		return values, captions
	}
	groups, err := page.state.Session.Events.Deployments(page.chapter)
	if err != nil {
		return nil, nil
	}
	for _, group := range groups.Groups {
		if group.Err == nil {
			add(group.Address, locales.Label(group.Label))
		}
	}
	return values, captions
}

func (page *Page) referenceChoice(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
	field event.ScriptField,
	named bool,
) {
	values, captions := page.referenceOptions(field)
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
	label := locales.ScriptChooseReference()
	if named {
		label += " · " + locales.Label(field.ID)
	}
	ui.Select(c.Key(key), &reference.choice, captions).
		Width(440).
		MaxWidthPercent(100).
		Label(label).
		Placeholder(locales.ScriptChooseReference()).
		Disabled(page.state.Busy || !draft.data.Current(page.state.Session.Events)).
		OnChange(func() {
			if index := slices.Index(captions, reference.choice); index >= 0 {
				page.state.SetErr(draft.data.Set(
					draft.selected,
					field.ID,
					values[index],
				))
				draft.preview = nil
			}
		})
}

func (page *Page) actions(c *ui.Context, draft *draftState) {
	stale := !draft.data.Current(page.state.Session.Events)
	if !draft.data.Modified() && !draft.invalid() && !stale {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.ScriptPreview()).
			Disabled(page.state.Busy || stale || draft.invalid() || !draft.data.Modified()).
			OnClick(func() {
				preview, err := page.state.Session.Events.PlanScriptEdit(
					draft.data,
				)
				draft.preview = preview
				page.state.SetErr(err)
			})
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
	if preview.MovementCount() > 0 {
		ui.Textf(c, locales.Label("script_movement_preview"), preview.MovementCount()).
			FontSize(components.FontCaption)
	}
	if preview.Relocated() {
		scope = locales.ScriptRelocateScope()
	}
	ui.Text(c, scope).
		FontSize(components.FontCaption).
		TextColor(c.Theme().TextMuted)
	ui.Scroll(c).MaxHeight(120).Children(func() {
		for _, use := range preview.Uses() {
			caption := entryCaption(
				event.ScriptEntry{
					Source:   use.Source,
					WorldMap: use.WorldMap,
					EventID:  use.EventID,
				},
			)
			if use.Chapter >= 0 {
				caption = fmt.Sprintf("%02X · %s", use.Chapter, caption)
			}
			ui.Textf(c, "%s · 0x%08X", caption, use.Pointer).
				FontSize(components.FontCaption).
				FontFeatures("tnum")
		}
	})
	ui.Button(c, locales.ScriptApply()).
		Disabled(page.state.Busy || stale || draft.invalid()).
		OnClick(func() {
			page.state.SetErr(
				page.state.Edit(
					func() error { return page.state.Session.Events.ApplyScriptEdit(preview) },
				),
			)
			if page.state.Err == nil {
				delete(page.drafts, draft.data.Entry().Pointer)
				page.state.HistoryEpoch++
			}
		})
}
