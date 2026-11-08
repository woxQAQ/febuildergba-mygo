package records

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type itemFieldChoice struct {
	choiceDraft

	custom bool
	focus  bool
}

func (a *form) itemOptionField(
	c *ui.Context,
	row records.Row,
	field records.Field,
) ui.Element {
	value, err := a.state.Session.Records.Value(records.Items, row.ID, field.ID)
	if err != nil {
		return ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
	}
	options, err := a.state.Session.Records.ItemChoices(field.ID)
	if err != nil {
		return ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
	}
	key := fieldKey{records.Items, row.ID, field.ID}
	if a.itemChoices == nil {
		a.itemChoices = make(map[fieldKey]*itemFieldChoice)
	}
	draft := a.itemChoices[key]
	if draft == nil {
		draft = &itemFieldChoice{}
		a.itemChoices[key] = draft
	}
	effect := field.ID == "use_effect" || field.ID == "weapon_effect"
	caption := locales.ItemCustomValue(fmt.Sprintf("0x%02X", value))
	var help string
	labels := make([]string, 0, len(options)+2)
	values := make(map[string]int64)
	for _, option := range options {
		label := locales.Label(option.Name)
		if effect {
			label = fmt.Sprintf("%02X · %s", option.Value, label)
		}
		labels = append(labels, label)
		values[label] = option.Value
		if option.Value == value {
			caption, help = label, option.Help
		}
	}
	if _, known := values[caption]; !known {
		labels = append(labels, caption)
	}
	labels = append(labels, locales.ItemCustomCode())
	if !draft.initialized || draft.value != value || draft.text == draft.label {
		draft.text, draft.label, draft.value, draft.initialized = caption, caption, value, true
	}
	label := locales.Label(field.ID)
	if field.ID == "combat.range" {
		label = locales.ItemRange()
	}
	return ui.Column(c.Key(field.ID)).
		Width(164).
		MaxWidthPercent(100).
		Gap(4).
		Children(func() {
			ui.Text(c, label).FontSize(components.FontCaption)
			if draft.custom {
				ui.Row(c).Gap(4).Children(func() {
					if field.ID == "combat.range" {
						field.Hex = true
					}
					// The custom value commits immediately like any
					// number field; undo belongs to the document
					// history, so no TextDraft claim here.
					a.numberField(c, records.Items, row, field, value).
						Grow(1).
						MinWidth(0)
					ui.Button(c, "▾").Width(26).Padding(0).
						Label(locales.ItemUsePresets()).
						Tooltip(locales.ItemUsePresets()).
						Disabled(a.state.Busy).
						OnClick(func() { draft.custom, draft.focus = false, true })
				})
				if draft.focus {
					a.textReferenceDraft(key).input.Focus(c)
					draft.focus = false
				}
				return
			}
			chooser := ui.Select
			if effect {
				chooser = ui.Combobox
			}
			control := chooser(c.Key("choice"), &draft.text, labels).
				WidthPercent(100).Height(26).
				Label(a.fieldLabel(records.Items, field)).
				Tooltip(fmt.Sprintf("%s\n0x%02X", label, value)).
				Bind(&a.textReferenceDraft(key).input).
				Disabled(a.state.Busy).
				OnChange(func() {
					if draft.text == locales.ItemCustomCode() {
						draft.custom, draft.focus = true, true
						draft.text = draft.label
						return
					}
					if next, exists := values[draft.text]; exists {
						a.state.SetErr(a.state.Edit(func() error {
							return a.state.Session.Records.Set(
								records.Items,
								[]int{row.ID},
								field.ID,
								next,
							)
						}))
					}
				})
			if help != "" {
				ui.Text(c, locales.Label(help)).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
			}
			if effect {
				components.TextDraft(a.state, control)
			}
			if draft.focus {
				a.textReferenceDraft(key).input.Focus(c)
				draft.focus = false
			}
		})
}

func (a *form) itemNumberGrid(
	c *ui.Context,
	row records.Row,
	table *records.Table,
	ids []string,
	columns int,
) {
	var fields []records.Field
	for _, id := range ids {
		if field, exists := profileField(table, id); exists {
			fields = append(fields, field)
		}
	}
	if len(fields) == 0 {
		return
	}
	ui.Grid(c).
		Columns(min(columns, len(fields))).
		GapX(12).
		GapY(8).
		Children(func() {
			for _, field := range fields {
				ui.Column(c.Key(field.ID)).MinWidth(0).Gap(4).Children(func() {
					label := locales.Label(field.ID)
					caption := ui.Text(c, label).
						Label(a.fieldLabel(records.Items, field)).
						SingleLine().
						FontSize(components.FontCaption).
						Tooltip(label)
					value, err := a.state.Session.Records.Value(
						records.Items,
						row.ID,
						field.ID,
					)
					if err != nil {
						ui.Text(c, components.ErrorText(err)).
							TextColor(c.Theme().Danger)
						return
					}
					// Item numbers commit immediately; undo belongs to the
					// document history, so no TextDraft claim here.
					a.numberField(c, records.Items, row, field, value).
						LabelledBy(caption).
						WidthPercent(100).
						MinWidth(0)
				})
			}
		})
}
