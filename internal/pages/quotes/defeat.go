package quotes

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type defeatState struct {
	choice                          string
	kind                            quote.Kind
	row                             uint64
	edit                            *defeatEdit
	openText                        uint16
	actions, text, scope, flag, end ui.Handle
}

type defeatVariant struct {
	category int
	entry    *tableState
	index    int
	label    string
}

type defeatEdit struct {
	entry            *tableState
	row              uint64
	title            string
	fields           []quote.Field
	values           map[string]uint32
	inputs           map[string]*valueInput
	creating, scoped bool
	contentChosen    bool
	clearText        bool
	open             bool
	message          string
	returnTo         *ui.Handle
}

// ViewDefeat presents the text first. Native rows remain separate, ordered
// variants; each uncommon operation has its own editing task.
func (p *Page) ViewDefeat(c *ui.Context, available float32, raw bool) {
	p.advanced = raw
	p.refreshTables()
	if len(p.tables) == 0 {
		return
	}
	p.query = ""
	p.category = max(0, min(p.category, len(p.tables)-1))
	variants := p.defeatVariants()
	entry := p.draft(p.tables[p.category])
	p.filter(entry)
	if entry.index() < 0 && entry.err == nil && entry.draft != nil &&
		!entry.draft.Modified() && !entry.invalid() && len(variants) > 0 {
		p.category = variants[0].category
		entry = variants[0].entry
		p.selectIndex(entry, variants[0].index)
	}
	stale := entry.draft == nil || !entry.draft.Current(p.state.Session.Quotes)
	disabled := stale || p.state.Busy
	width := max(float32(0), min(available, 880))
	ui.Column(c.Key(p)).
		Width(width).
		MaxWidthPercent(100).
		Gap(10).
		Children(func() {
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if stale && entry.draft != nil {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			p.defeatVariantChoice(c, variants, entry)
			index := entry.index()
			if index >= 0 {
				ui.Column(c.Key(struct {
					table *tableState
					row   uint64
				}{entry, entry.rows[index].ID})).Gap(8).Children(func() {
					p.defeatBody(c, entry, index, disabled)
					if raw {
						p.rawRecord(c, entry.rows[index])
					}
				})
			} else {
				ui.Text(c, locales.QuoteNoDefeat()).
					TextColor(c.Theme().TextMuted)
				ui.Row(c).Wrap().Gap(8).Children(func() {
					ui.Button(c, locales.DefeatAddText()).Bind(&p.defeat.text).
						Disabled(disabled || !p.canAddDefeat(entry)).
						OnClick(func() {
							p.startDefeatAddition(
								entry,
								-1,
								false,
								&p.defeat.text,
							)
						})
					p.defeatCommands(c, entry, index, disabled)
				})
			}
			p.actions(c, entry, stale)
		})
	p.defeatTask(c)
}

func (p *Page) defeatVariants() []defeatVariant {
	var variants []defeatVariant
	for category, table := range p.tables {
		entry := p.draft(table)
		p.filter(entry)
		if entry.draft == nil {
			variants = append(
				variants,
				defeatVariant{category, entry, -1, p.tableLabel(table.Kind)},
			)
			continue
		}
		if len(entry.filtered) == 0 &&
			(entry.draft.Modified() || entry.invalid()) {
			variants = append(
				variants,
				defeatVariant{
					category,
					entry,
					-1,
					p.tableLabel(table.Kind) + " · " + locales.DefeatPending(),
				},
			)
		}
		for _, index := range entry.filtered {
			label := p.defeatScope(entry, index)
			if label == "" {
				label = locales.DefeatDefault()
			}
			if table.Kind != quote.Death {
				label = p.tableLabel(table.Kind) + " · " + label
			}
			variants = append(
				variants,
				defeatVariant{category, entry, index, label},
			)
		}
	}
	counts := make(map[string]int)
	for _, variant := range variants {
		counts[variant.label]++
	}
	ordinals := make(map[string]int)
	for index := range variants {
		label := variants[index].label
		if counts[label] > 1 {
			ordinals[label]++
			variants[index].label = fmt.Sprintf(
				"%s · %d",
				label,
				ordinals[label],
			)
		}
	}
	return variants
}

func (p *Page) defeatVariantChoice(
	c *ui.Context,
	variants []defeatVariant,
	entry *tableState,
) {
	index := entry.index()
	if len(variants) == 1 {
		if index >= 0 &&
			(p.defeatScope(entry, index) != "" || entry.draft.Kind() != quote.Death) {
			ui.Button(c, variants[0].label).Bind(&p.defeat.scope).
				Disabled(p.state.Busy || !entry.draft.Current(p.state.Session.Quotes)).
				OnClick(func() {
					p.startDefeatEdit(
						entry,
						index,
						locales.DefeatEditScope(),
						[]string{quote.Chapter, quote.Route},
						&p.defeat.scope,
					)
				})
		}
		return
	}
	if len(variants) < 2 {
		return
	}
	labels := make([]string, len(variants))
	caption := ""
	for position, variant := range variants {
		labels[position] = variant.label
		if variant.entry == entry && variant.index == index {
			caption = variant.label
		}
	}
	var row uint64
	if index >= 0 {
		row = entry.rows[index].ID
	}
	if p.defeat.kind != p.tables[p.category].Kind || p.defeat.row != row ||
		!slices.Contains(labels, p.defeat.choice) {
		p.defeat.choice = caption
		p.defeat.kind, p.defeat.row = p.tables[p.category].Kind, row
	}
	ui.Select(c.Key("defeat-variant"), &p.defeat.choice, labels).
		Label(locales.DefeatVariants()).
		MaxWidthPercent(100).Disabled(p.state.Busy).
		OnChange(func() {
			for _, variant := range variants {
				if variant.label == p.defeat.choice {
					p.defeat.openText = 0
					p.category = variant.category
					p.selectIndex(variant.entry, variant.index)
					return
				}
			}
		})
}

func (p *Page) defeatBody(
	c *ui.Context,
	entry *tableState,
	index int,
	disabled bool,
) {
	textID, textErr := entry.draft.Value(index, quote.Text)
	script, _ := entry.draft.Value(index, quote.Script)
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		if textID != 0 && textErr == nil {
			label := locales.DefeatEditText()
			if entry.draft.Modified() {
				label = locales.DefeatApplyThenEdit()
			}
			ui.PrimaryButton(c, label).
				Disabled(disabled || textID > 65535 || p.openText == nil).
				OnClick(func() {
					chosenID := textID
					if chosenID == 0 || chosenID > 65535 || p.state.Busy ||
						!entry.draft.Current(p.state.Session.Quotes) {
						return
					}
					id := uint16(chosenID)
					if entry.draft.Modified() {
						entry.preview, entry.err = p.state.Session.Quotes.Plan(
							entry.draft,
						)
						p.state.Report("quote.preview", entry.err)
						if entry.err == nil {
							p.defeat.openText = id
						}
					} else {
						p.openText(id)
					}
				})
		} else if script != 0 {
			ui.Button(c, locales.QuoteScriptBytes()).
				Disabled(disabled || p.openHex == nil || script < 0x080000C0 || script >= 0x0A000000).
				OnClick(func() { p.openHex(int(script)-0x08000000, 16) })
		}
		if textErr == nil {
			label := locales.DefeatChangeText()
			if textID == 0 && script != 0 {
				label = locales.DefeatUseText()
			}
			ui.Button(c, label).Bind(&p.defeat.text).
				Disabled(disabled).OnClick(func() {
				p.startDefeatEdit(
					entry,
					index,
					label,
					[]string{quote.Text},
					&p.defeat.text,
				)
			})
		}
		p.defeatCommands(c, entry, index, disabled)
		ui.Box(c).Grow(1)
		if flag, err := entry.draft.Value(
			index,
			quote.Flag,
		); err == nil &&
			flag != 0 {
			control := ui.Button(c, fmt.Sprintf("%s 0x%X", locales.QuoteDefeatFlag(), flag)).
				Bind(&p.defeat.flag).
				Disabled(disabled).
				OnClick(func() {
					p.startDefeatEdit(
						entry,
						index,
						locales.QuoteDefeatFlag(),
						[]string{quote.Flag},
						&p.defeat.flag,
					)
				})
			if flag == 0x65 {
				control.Tooltip(locales.QuoteDeathHelp())
			}
		}
		if final, err := entry.draft.Value(
			index,
			quote.FinalText,
		); err == nil &&
			final != 0 {
			ui.Button(c, locales.Label(quote.FinalText)).
				Bind(&p.defeat.end).
				Disabled(disabled).
				OnClick(func() {
					p.startDefeatEdit(
						entry,
						index,
						locales.Label(quote.FinalText),
						[]string{quote.FinalText},
						&p.defeat.end,
					)
				})
		}
	})
	switch {
	case textErr == nil && textID != 0:
		components.TextPreview(c, p.state.Session.Texts, textID, p.advanced)
	case script != 0:
		ui.Textf(c, "%s · 0x%08X", locales.Label(quote.Script), script).
			Selectable()
	default:
		label := locales.QuoteNoText()
		if p.state.Session.Quotes.SilentDeath() {
			label = locales.DefeatSilent()
		}
		ui.Text(c, label)
	}
}

func (p *Page) canAddDefeat(entry *tableState) bool {
	return entry.draft != nil && len(entry.rows) < quote.MaxEntries &&
		p.unit > 0 &&
		p.unit <= 255 &&
		!entry.invalid()
}

func (p *Page) defeatCommands(
	c *ui.Context,
	entry *tableState,
	index int,
	disabled bool,
) {
	ui.PopoverMenuButton(c.Key("defeat-actions"), locales.DefeatActions(), func(menu *ui.Menu) {
		if menu.Item(locales.DefeatAddSpecial()).
			Disabled(disabled || index < 0 || !p.canAddDefeat(entry)).
			Chosen() {
			p.startDefeatAddition(entry, index, true, &p.defeat.actions)
		}
		for _, table := range p.tables {
			target := p.draft(table)
			if len(target.filtered) > 0 {
				continue
			}
			label := locales.DefeatAddTable(p.tableLabel(table.Kind))
			if menu.Item(label).
				Disabled(p.state.Busy || target.draft == nil || !target.draft.Current(p.state.Session.Quotes) || !p.canAddDefeat(target)).
				Chosen() {
				p.startDefeatAddition(target, -1, false, &p.defeat.actions)
			}
		}
		menu.Separator()
		if menu.Item(locales.DefeatEditScope()).
			Disabled(disabled || index < 0).
			Chosen() {
			p.startDefeatEdit(
				entry,
				index,
				locales.DefeatEditScope(),
				[]string{quote.Chapter, quote.Route},
				&p.defeat.actions,
			)
		}
		if entry.draft != nil {
			for _, field := range entry.draft.Fields() {
				if field.ID != quote.Flag && field.ID != quote.Script &&
					field.ID != quote.FinalText {
					continue
				}
				label := fieldLabel(entry.draft.Kind(), field.ID)
				if field.ID == quote.Script {
					label = locales.DefeatChangeScript()
					if textID, _ := entry.draft.Value(
						index,
						quote.Text,
					); textID != 0 {
						label = locales.DefeatFallbackScript()
						if menu.Item(locales.DefeatUseScript()).
							Disabled(disabled || index < 0).
							Chosen() {
							task := p.startDefeatEdit(
								entry,
								index,
								locales.DefeatUseScript(),
								[]string{quote.Script},
								&p.defeat.actions,
							)
							task.clearText = true
							task.values[quote.Text] = 0
						}
					}
				}
				if menu.Item(label + "…").
					Disabled(disabled || index < 0).
					Chosen() {
					p.startDefeatEdit(
						entry,
						index,
						label,
						[]string{field.ID},
						&p.defeat.actions,
					)
				}
			}
		}
		menu.Separator()
		for _, direction := range []int{-1, 1} {
			label := locales.MoveUp()
			if direction > 0 {
				label = locales.MoveDown()
			}
			position := entry.selected + direction
			if menu.Item(label).
				Disabled(disabled || index < 0 || position < 0 || position >= len(entry.filtered)).
				Chosen() {
				p.state.SetErr(
					entry.draft.Move(index, entry.filtered[position]),
				)
				entry.preview = nil
			}
		}
		if menu.Item(locales.QuoteCopy()).
			Disabled(disabled || index < 0 || !p.canAddDefeat(entry)).
			Chosen() {
			p.state.SetErr(entry.draft.Duplicate(index))
			if p.state.Err == nil {
				entry.preview = nil
				p.selectIndex(entry, index+1)
			}
		}
		if menu.Item(locales.DefeatRemove()).
			Disabled(disabled || index < 0).
			Chosen() {
			p.state.SetErr(entry.draft.Remove(index))
			entry.preview, entry.filtered = nil, nil
			entry.rows = entry.draft.Entries()
		}
		menu.Separator()
		if menu.Item(locales.QuoteMatchingRules()).Chosen() {
			p.startDefeatEdit(
				entry,
				index,
				locales.QuoteMatchingRules(),
				nil,
				&p.defeat.actions,
			)
		}
		if menu.Item(locales.QuoteRaw()).
			Disabled(p.state.Busy || index < 0 || entry.rows[index].Address < 0 || p.openHex == nil).
			Chosen() {
			row := entry.rows[index]
			p.openHex(row.Address, len(row.Raw))
		}
	}).
		Bind(&p.defeat.actions)
}

func (p *Page) startDefeatEdit(
	entry *tableState,
	index int,
	title string,
	fields []string,
	returnTo *ui.Handle,
) *defeatEdit {
	task := &defeatEdit{
		entry: entry, title: title, values: make(map[string]uint32),
		inputs: make(map[string]*valueInput), open: true, returnTo: returnTo,
	}
	if entry.draft != nil {
		for _, field := range entry.draft.Fields() {
			value := field.Min
			if index >= 0 {
				value, _ = entry.draft.Value(index, field.ID)
			} else {
				switch field.ID {
				case quote.Chapter:
					value = p.state.Session.Quotes.AnyChapter()
				case quote.Route:
					value = 0xFF
				}
			}
			task.values[field.ID] = value
		}
		for _, id := range fields {
			for _, field := range entry.draft.Fields() {
				if field.ID == id {
					task.fields = append(task.fields, field)
					break
				}
			}
		}
	}
	if index >= 0 {
		task.row = entry.rows[index].ID
	}
	p.defeat.edit = task
	return task
}

func (p *Page) startDefeatAddition(
	entry *tableState,
	index int,
	scoped bool,
	returnTo *ui.Handle,
) {
	title := locales.DefeatAddText()
	fields := []string{quote.Script}
	for _, field := range entry.draft.Fields() {
		if field.ID == quote.Text {
			fields = []string{quote.Text}
			break
		}
	}
	if scoped {
		title = locales.DefeatAddSpecial()
		fields = append([]string{quote.Chapter, quote.Route}, fields...)
	} else if entry.draft.Kind() != quote.Death {
		title = locales.DefeatAddTable(p.tableLabel(entry.draft.Kind()))
		fields = append([]string{quote.Chapter}, fields...)
	}
	task := p.startDefeatEdit(entry, index, title, fields, returnTo)
	task.creating, task.scoped = true, scoped
	if _, exists := task.values[quote.Text]; exists {
		task.values[quote.Text] = 0
	}
	if _, exists := task.values[quote.Script]; exists {
		task.values[quote.Script] = 0
	}
	if scoped {
		task.values[quote.Chapter] = p.state.Session.Quotes.AnyChapter()
		if _, exists := task.values[quote.Route]; exists {
			task.values[quote.Route] = 0xFF
		}
	}
}

func (p *Page) closeDefeatTask() {
	if task := p.defeat.edit; task != nil {
		task.open = false
		if task.returnTo != nil {
			target := task.returnTo
			task.returnTo = nil
			target.Focus()
		}
	}
}

func (p *Page) defeatTask(c *ui.Context) {
	task := p.defeat.edit
	if task == nil {
		return
	}
	if !task.open {
		p.closeDefeatTask()
		return
	}
	width, height := c.Size()
	modal := ui.Modal(c.Key(task), &task.open, func() {
		ui.Column(c).
			Width(min(float32(560), max(float32(0), width-80))).
			Gap(12).
			Children(func() {
				ui.Text(c, task.title).
					FontSize(components.FontPanelTitle).
					Bold()
				stale := task.entry.draft == nil ||
					!task.entry.draft.Current(p.state.Session.Quotes)
				if stale {
					ui.Text(c, locales.DeployStale()).
						TextColor(c.Theme().Danger)
				}
				invalid := false
				ui.Scroll(c).
					MaxHeight(max(float32(100), height-220)).
					Gap(12).
					Children(func() {
						if len(task.fields) == 0 {
							ui.Text(c, p.tableHelp(p.tables[p.category].Kind))
						} else {
							for _, field := range task.fields {
								p.defeatTaskField(
									c.Key(field.ID),
									task,
									field,
									stale || p.state.Busy,
								)
							}
						}
						for _, current := range task.inputs {
							invalid = invalid || current.invalid
						}
						if task.message != "" {
							ui.Text(c, task.message).TextColor(c.Theme().Danger)
						}
					})
				ui.Row(c).Gap(8).Children(func() {
					if len(task.fields) > 0 {
						ui.PrimaryButton(c, locales.DefeatConfirm()).
							Disabled(stale || p.state.Busy || invalid).
							OnClick(func() { p.commitDefeatTask(task) })
					}
					label := locales.Cancel()
					if len(task.fields) == 0 {
						label = locales.Dismiss()
					}
					ui.Button(c, label).OnClick(p.closeDefeatTask)
				})
			})
	})
	if modal.Dismissed() {
		p.closeDefeatTask()
	}
}

func (p *Page) defeatTaskField(
	c *ui.Context,
	task *defeatEdit,
	field quote.Field,
	disabled bool,
) {
	value := task.values[field.ID]
	current := task.inputs[field.ID]
	labels, values := p.choices(field)
	if current == nil {
		current = &valueInput{text: fmt.Sprintf("0x%X", value)}
		for _, label := range labels {
			if values[label] == value {
				current.text = label
				break
			}
		}
		task.inputs[field.ID] = current
	}
	label := fieldLabel(task.entry.draft.Kind(), field.ID)
	inputLabel := task.title + " / " + label
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).Bold()
		switch {
		case field.ID == quote.Text || field.ID == quote.FinalText:
			components.TextReference(
				c,
				p.state,
				&current.reference,
				value,
				inputLabel,
				p.advanced,
				disabled,
				func(id uint16) {
					task.values[field.ID] = uint32(id)
					task.contentChosen = true
					task.message = ""
				},
				nil,
			)
			if field.ID == quote.Text && value == 0 &&
				task.values[quote.Script] != 0 {
				ui.Text(c, locales.DefeatUsingScript())
			}
		case len(labels) > 0:
			components.TextDraft(p.state, ui.Combobox(c, &current.text, labels).
				Label(inputLabel).Disabled(disabled).OnChange(func() {
				chosen, ok := values[current.text]
				current.invalid = !ok
				if ok {
					task.values[field.ID] = chosen
					task.message = ""
				}
			}))
		default:
			control := components.TextDraft(
				p.state,
				ui.TextInput(c, &current.text).
					Label(inputLabel).Disabled(disabled).OnChange(func() {
					value, err := input.ParseInteger(current.text)
					if field.ID == quote.Script {
						var address int
						address, err = input.ParseAddress(current.text)
						value = int64(address)
						if value != 0 {
							value += 0x08000000
						}
					}
					current.invalid = err != nil || value < int64(field.Min) ||
						value > int64(field.Max)
					if !current.invalid {
						task.values[field.ID] = uint32(
							value,
						) // #nosec G115 -- The native field bounds are checked above.
						if field.ID == quote.Script {
							task.contentChosen = true
						}
						task.message = ""
					}
				}),
			)
			if current.invalid {
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
		if current.invalid {
			ui.Text(c, locales.QuoteInvalidInput()).TextColor(c.Theme().Danger)
		}
		if field.ID == quote.Flag && value == 0x65 {
			ui.Text(c, locales.QuoteDeathHelp())
		}
	})
}

func (p *Page) commitDefeatTask(task *defeatEdit) {
	if p.state.Busy || task.entry.draft == nil ||
		!task.entry.draft.Current(p.state.Session.Quotes) {
		return
	}
	for _, current := range task.inputs {
		if current.invalid {
			return
		}
	}
	_, hasText := task.values[quote.Text]
	if (task.clearText || task.creating && !hasText) &&
		task.values[quote.Script] == 0 {
		task.message = locales.DefeatNeedsScript()
		return
	}
	if task.creating &&
		(!task.contentChosen || hasText && task.values[quote.Text] == 0 && !p.state.Session.Quotes.SilentDeath()) {
		task.message = locales.DefeatNeedsContent()
		return
	}
	if task.scoped &&
		task.values[quote.Chapter] == p.state.Session.Quotes.AnyChapter() {
		if route, exists := task.values[quote.Route]; !exists || route == 0xFF {
			task.message = locales.DefeatNeedsScope()
			return
		}
	}
	entry := task.entry
	rows := entry.draft.Entries()
	index := -1
	for position, row := range rows {
		if row.ID == task.row {
			index = position
			break
		}
	}
	if task.creating {
		unit := p.unit
		if unit <= 0 || unit > 255 || !p.canAddDefeat(entry) {
			return
		}
		var err error
		if index >= 0 {
			err = entry.draft.Duplicate(index)
			index++
		} else {
			err = entry.draft.Add()
			index = len(rows)
		}
		if err != nil {
			task.message = components.ErrorText(err)
			return
		}
		if err = entry.draft.Set(
			index,
			quote.Unit,
			uint32(unit),
		); err != nil {
			p.defeatTaskFailure(
				task,
				errors.Join(err, entry.draft.Remove(index)),
			)
			return
		}
	} else if index < 0 {
		return
	}
	fields := make([]string, len(task.fields))
	for index, field := range task.fields {
		fields[index] = field.ID
	}
	if task.clearText {
		fields = append(fields, quote.Text)
	}
	if task.creating {
		if _, exists := task.values[quote.Script]; exists &&
			!slices.Contains(fields, quote.Script) {
			fields = append(fields, quote.Script)
		}
	}
	before := make(map[string]uint32)
	for _, id := range fields {
		before[id], _ = entry.draft.Value(index, id)
	}
	for _, id := range fields {
		if err := entry.draft.Set(index, id, task.values[id]); err != nil {
			if task.creating {
				err = errors.Join(err, entry.draft.Remove(index))
			} else {
				for field, value := range before {
					err = errors.Join(err, entry.draft.Set(index, field, value))
				}
			}
			p.defeatTaskFailure(task, err)
			return
		}
	}
	if task.creating && task.scoped && len(entry.filtered) > 0 {
		// A new exception precedes existing matches without reordering them.
		destination := entry.filtered[0]
		if err := entry.draft.Move(index, destination); err != nil {
			p.defeatTaskFailure(
				task,
				errors.Join(err, entry.draft.Remove(index)),
			)
			return
		}
		index = destination
	}
	entry.preview, entry.err = nil, nil
	p.defeat.openText = 0
	if !task.creating {
		for _, id := range fields {
			delete(entry.inputs, fieldKey{task.row, id})
		}
	}
	for category, table := range p.tables {
		if table.Kind == entry.draft.Kind() {
			p.category = category
			break
		}
	}
	p.selectIndex(entry, index)
	p.closeDefeatTask()
}

func (p *Page) defeatTaskFailure(task *defeatEdit, err error) {
	task.message = components.ErrorText(err)
	p.state.Report("quote.defeat_edit", err)
}

func (p *Page) defeatScope(entry *tableState, index int) string {
	var parts []string
	for _, field := range entry.draft.Fields() {
		if field.ID != quote.Chapter && field.ID != quote.Route {
			continue
		}
		value, err := entry.draft.Value(index, field.ID)
		if err != nil ||
			field.ID == quote.Chapter &&
				value == p.state.Session.Quotes.AnyChapter() ||
			field.ID == quote.Route && value == 0xFF {
			continue
		}
		caption := fmt.Sprintf("%s %02X", locales.Label(field.ID), value)
		if field.ID == quote.Chapter {
			if name := p.name(records.Chapters, value); name != "" {
				caption = name
			}
		} else {
			labels, values := p.choices(field)
			for _, label := range labels {
				if values[label] == value {
					caption = label
					break
				}
			}
		}
		parts = append(parts, caption)
	}
	return strings.Join(parts, " / ")
}
