// Package quotes owns drafts for native battle and defeat dialog tables.
package quotes

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	fieldKey struct {
		row   uint64
		field string
	}
	valueInput struct {
		reference components.TextReferenceState
		text      string
		invalid   bool
	}
	tableState struct {
		draft    *quote.Draft
		preview  *quote.Plan
		err      error
		inputs   map[fieldKey]*valueInput
		rows     []quote.Entry
		filtered []int
		selected int
		list     ui.ListState
	}
)

func (entry *tableState) invalid() bool {
	for _, row := range entry.rows {
		for key, input := range entry.inputs {
			if key.row == row.ID && input.invalid {
				return true
			}
		}
	}
	return false
}

func (entry *tableState) index() int {
	if entry.selected < 0 || entry.selected >= len(entry.filtered) {
		return -1
	}
	return entry.filtered[entry.selected]
}

type Page struct {
	state          *editor.State
	openRecord     func(records.Kind, int)
	openText       func(uint16)
	openHex        func(int, int)
	sidebar        components.Sidebar
	category       int
	categoryChoice string
	query          string
	advanced       bool
	showHelp       bool
	defeat         defeatState
	tables         []quote.Table
	drafts         map[quote.Kind]*tableState
	unit           int
	extraKind      quote.Kind
	thumbnails     components.Thumbnails
	battleScope    int
	scopeChoice    string
}

func New(
	state *editor.State,
	openRecord func(records.Kind, int),
	openText func(uint16),
	openHex func(int, int),
) *Page {
	return &Page{
		state:      state,
		unit:       -1,
		openRecord: openRecord,
		openText:   openText,
		openHex:    openHex,
		drafts:     make(map[quote.Kind]*tableState),
	}
}

func (p *Page) Reset() {
	state, record, text, hex, sidebar, unit := p.state, p.openRecord, p.openText, p.openHex, p.sidebar, p.unit
	*p = *New(state, record, text, hex)
	p.sidebar, p.unit = sidebar, unit
}

func (p *Page) Open(kind quote.Kind, index int) {
	p.battleScope, p.scopeChoice = 0, ""
	if p.unit < 0 && defeatTable(kind) {
		p.extraKind = kind
	}
	p.refreshTables()
	for i, table := range p.tables {
		if table.Kind == kind {
			p.category = i
			p.categoryChoice = ""
			p.query = ""
			entry := p.draft(table)
			entry.selected = index
			entry.filtered = nil
			if entry.draft != nil {
				for position, row := range entry.draft.Entries() {
					if index >= 0 && row.ID == uint64(index)+1 {
						entry.selected = position
						break
					}
				}
			}
			physical := entry.selected
			p.selectIndex(entry, physical)
			return
		}
	}
}

func (p *Page) draft(table quote.Table) *tableState {
	entry := p.drafts[table.Kind]
	var selected uint64
	if entry != nil && entry.draft != nil &&
		!entry.draft.Current(p.state.Session.Quotes) &&
		!entry.draft.Modified() &&
		!entry.invalid() {
		if index := entry.index(); index >= 0 {
			selected = entry.rows[index].ID
		}
		entry = nil
	}
	if entry == nil || entry.draft == nil && table.Err == nil {
		entry = &tableState{selected: 0, inputs: make(map[fieldKey]*valueInput)}
		entry.draft, entry.err = p.state.Session.Quotes.NewDraft(table.Kind)
		p.drafts[table.Kind] = entry
		if selected != 0 && entry.draft != nil {
			for index, row := range entry.draft.Entries() {
				if row.ID == selected {
					p.selectIndex(entry, index)
					break
				}
			}
		}
	}
	return entry
}

func (p *Page) name(kind records.Kind, id uint32) string {
	if table := p.state.Session.Records.Table(
		kind,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == int(id) {
				return row.Name
			}
		}
	}
	return ""
}

func (p *Page) caption(entry *tableState, index int) string {
	id, _ := entry.draft.Value(index, quote.Unit)
	caption := p.unitName(id)
	if other, err := entry.draft.Value(index, quote.Opponent); err == nil {
		caption += " ↔ " + p.unitName(other)
	}
	if p.unit >= 0 {
		return caption + " " + p.conditions(entry, index)
	}
	return caption
}

func (p *Page) unitName(id uint32) string {
	if id == 0 {
		return locales.QuoteUnspecifiedSide()
	}
	if name := p.name(records.Units, id); name != "" {
		return name
	}
	return locales.QuoteUnknownUnit()
}

func (p *Page) filter(entry *tableState) {
	if entry.draft == nil {
		return
	}
	var selected uint64
	if index := entry.index(); index >= 0 && index < len(entry.rows) {
		selected = entry.rows[index].ID
	}
	entry.rows = entry.draft.Entries()
	entry.filtered = nil
	query := strings.ToLower(strings.TrimSpace(p.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 32)
	for i := range entry.rows {
		if p.unit >= 0 {
			unit, err := entry.draft.Value(i, quote.Unit)
			if err != nil || int(unit) != p.unit {
				continue
			}
		}
		matches := query == "" ||
			strings.Contains(strings.ToLower(p.caption(entry, i)), query) ||
			idErr == nil && id == uint64(i)
		for _, field := range []string{quote.Unit, quote.Opponent, quote.Text, quote.FinalText} {
			value, err := entry.draft.Value(i, field)
			matches = matches ||
				idErr == nil && err == nil && uint64(value) == id
		}
		if matches && p.matchesBattle(entry, i) {
			entry.filtered = append(entry.filtered, i)
		}
	}
	entry.selected = min(max(0, entry.selected), len(entry.filtered)-1)
	for i, index := range entry.filtered {
		if entry.rows[index].ID == selected {
			entry.selected = i
			break
		}
	}
}

func (p *Page) View(c *ui.Context) {
	p.refreshTables()
	if len(p.tables) == 0 {
		return
	}
	p.category = max(0, min(p.category, len(p.tables)-1))
	table := p.tables[p.category]
	entry := p.draft(table)
	p.filter(entry)
	p.sidebar.Render(
		c,
		340,
		func() { p.navigation(c, entry) },
		func() { p.detail(c, table, entry) },
	)
}

func (p *Page) navigation(c *ui.Context, entry *tableState) {
	ui.Column(c.Key("navigation")).Fill().Padding(12).Gap(8).Children(func() {
		labels := make([]string, len(p.tables))
		for i, table := range p.tables {
			labels[i] = p.tableLabel(table.Kind)
		}
		if !slices.Contains(labels, p.categoryChoice) {
			p.categoryChoice = labels[p.category]
		}
		if len(p.tables) > 1 {
			ui.Select(c, &p.categoryChoice, labels).
				Label(locales.QuoteTable()).
				Disabled(p.state.Busy).
				OnChange(func() {
					if index := slices.Index(
						labels,
						p.categoryChoice,
					); index >= 0 {
						p.category = index
						p.query = ""
					}
				})
		}
		p.battleFilter(c, entry)
		components.TextDraft(
			p.state,
			ui.TextInput(c, &p.query).
				Label(locales.QuoteSearch()).
				Placeholder(locales.QuoteSearch()).
				Tooltip(locales.QuoteSearchHelp()).
				Disabled(p.state.Busy),
		)
		entry.list.Selected = &entry.selected
		entry.list.Key = func(index int) any { return entry.rows[entry.filtered[index]].ID }
		list := ui.Table(c.Key(entry), &entry.list, []ui.TableColumn{{ID: "quote", Title: locales.QuoteRecords()}}, len(entry.filtered), func(row, _ int) {
			p.listRow(c, entry, entry.filtered[row])
		}).
			Label(locales.QuoteRecords()).
			Disabled(p.state.Busy)
		list.Grow(1)
		// Keyed lists can clear selection when a deleted row disappears.
		entry.selected = min(max(0, entry.selected), len(entry.filtered)-1)
		if len(entry.filtered) == 0 {
			ui.Text(c, locales.QuoteNoMatches()).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
	})
}

func (p *Page) detail(c *ui.Context, table quote.Table, entry *tableState) {
	state, s := p.state, p.state.Session
	stale := entry.draft == nil || !entry.draft.Current(s.Quotes)
	ui.Scroll(c.Key(entry)).Fill().Children(func() {
		ui.Column(c).Padding(16).Gap(12).Children(func() {
			ui.Text(c, p.tableLabel(table.Kind)).
				FontSize(components.FontSectionTitle).
				Bold()
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger).
					Tooltip(entry.err.Error())
			}
			if stale && entry.draft != nil {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Checkbox(c, &p.advanced, locales.Advanced())
				ui.Checkbox(c, &p.showHelp, locales.QuoteMatchingRules())
				if p.advanced {
					for _, site := range table.Pointers {
						ui.Button(c.Key(site), fmt.Sprintf("0x%08X", site)).
							Label(locales.DeployReference()).
							Disabled(state.Busy).
							OnClick(func() { p.openHex(site, 4) })
					}
				}
			})
			if p.showHelp {
				ui.Text(c, p.tableHelp(table.Kind)).
					FontSize(components.FontCaption)
			}
			if entry.draft == nil {
				p.actions(c, entry, stale)
				return
			}
			p.structure(c, entry, stale || state.Busy)
			index := entry.index()
			if index >= 0 {
				row := entry.rows[index]
				ui.Column(c.Key(row.ID)).Gap(8).Children(func() {
					for _, field := range entry.draft.Fields() {
						p.field(
							c.Key(field.ID),
							entry,
							index,
							field,
							stale || state.Busy,
							360,
						)
					}
					if p.advanced {
						p.rawRecord(c, row)
					}
				})
			}
			p.actions(c, entry, stale)
		})
	})
}

func (p *Page) rawRecord(c *ui.Context, row quote.Entry) {
	ui.Textf(c, "% X", row.Raw).
		Selectable().
		Font("monospace").
		FontSize(components.FontCaption)
	ui.Row(c).Children(func() {
		ui.Button(c, locales.QuoteRaw()).
			Disabled(p.state.Busy || row.Address < 0).
			OnClick(func() { p.openHex(row.Address, len(row.Raw)) })
	})
}

func (p *Page) addButton(
	c *ui.Context,
	entry *tableState,
	disabled bool,
) ui.Element {
	return ui.Button(c, locales.QuoteAdd()).
		Disabled(disabled || len(entry.rows) >= quote.MaxEntries || p.unit == 0 || p.unit > 255).
		OnClick(func() {
			p.state.SetErr(p.addRecord(entry))
			if p.state.Err == nil && p.unit >= 0 {
				p.state.SetErr(entry.draft.Set(
					len(entry.draft.Entries())-1,
					quote.Unit,
					uint32(p.unit),
				)) // #nosec G115 -- Character IDs are bounded by the enabled action.
			}
			if p.state.Err == nil {
				entry.preview = nil
				p.query = ""
				p.selectIndex(entry, len(entry.draft.Entries())-1)
			}
		})
}

func (p *Page) structure(c *ui.Context, entry *tableState, disabled bool) {
	index := entry.index()
	ui.Row(c).Wrap().Gap(6).Children(func() {
		p.addButton(c, entry, disabled)
		ui.Button(c, locales.QuoteCopy()).
			Disabled(disabled || index < 0 || entry.invalid() || len(entry.rows) >= quote.MaxEntries).
			OnClick(func() {
				p.state.SetErr(entry.draft.Duplicate(index))
				if p.state.Err == nil {
					entry.preview = nil
					p.query = ""
					p.selectIndex(entry, index+1)
				}
			})
		ui.Button(c, locales.QuoteDelete()).
			Disabled(disabled || index < 0).
			OnClick(func() {
				p.state.SetErr(entry.draft.Remove(index))
				if p.state.Err == nil {
					entry.preview = nil
					entry.filtered = nil
					entry.rows = entry.draft.Entries()
					entry.list = ui.ListState{}
				}
			})
		if p.unit >= 0 && len(entry.filtered) < 2 {
			return
		}
		for _, direction := range []int{-1, 1} {
			label, caption := "↑", locales.MoveUp()
			if direction > 0 {
				label, caption = "↓", locales.MoveDown()
			}
			destination := -1
			if position := entry.selected + direction; position >= 0 &&
				position < len(entry.filtered) {
				destination = entry.filtered[position]
			}
			ui.Button(c.Key(direction), label).
				Label(caption).
				Tooltip(caption + "\n" + locales.QuoteOrderHelp()).
				Disabled(disabled || index < 0 || destination < 0).
				OnClick(func() {
					p.state.SetErr(entry.draft.Move(index, destination))
					if p.state.Err == nil {
						entry.preview = nil
					}
				})
		}
	})
}

func (p *Page) choices(field quote.Field) ([]string, map[string]uint32) {
	var labels []string
	values := make(map[string]uint32)
	add := func(id uint32, label string) {
		caption := fmt.Sprintf("%02X · %s", id, label)
		labels = append(labels, caption)
		values[caption] = id
	}
	switch field.ID {
	case quote.Unit, quote.Opponent:
		for id := field.Min; id <= 255; id++ {
			name := p.name(records.Units, id)
			if id == 0 {
				name = locales.QuoteAnyUnit()
			}
			add(id, name)
		}
	case quote.Chapter:
		if table := p.state.Session.Records.Table(
			records.Chapters,
		); table != nil &&
			table.Err == nil {
			for _, row := range table.Rows {
				if row.ID < 0 || row.ID > 255 ||
					uint32(row.ID) == p.state.Session.Quotes.AnyChapter() {
					continue
				}
				add(uint32(row.ID), row.Name)
			}
		}
		add(p.state.Session.Quotes.AnyChapter(), locales.QuoteAnyChapter())
		if triangle, available := p.state.Session.Quotes.TriangleChapter(
			p.tables[p.category].Kind,
		); available {
			add(triangle, locales.QuoteTriangle())
		}
	case quote.Route:
		add(0, locales.QuoteRouteCommon())
		add(1, locales.QuoteRouteEirika())
		add(2, locales.QuoteRouteEphraim())
		add(0xFF, locales.QuoteAnyRoute())

	}
	return labels, values
}

func defeatTable(kind quote.Kind) bool {
	return kind == quote.Death || kind == quote.DeathLyn ||
		kind == quote.DeathOther
}

func (p *Page) tableHelp(kind quote.Kind) string {
	// Help wording follows the format's declared capabilities, not a version
	// identity.
	if defeatTable(kind) {
		switch {
		case p.state.Session.Quotes.SilentDeath():
			return locales.QuoteFe8DeathHelp()
		case p.state.Session.Quotes.FallbackDeaths():
			return locales.QuoteFe7DeathHelp()
		default:
			return locales.QuoteFe6DeathHelp()
		}
	}
	switch {
	case p.state.Session.Quotes.BattleConditions():
		return locales.QuoteFe8Help()
	case p.state.Session.Quotes.FlagBlocksFallback():
		return locales.QuoteFe7Help()
	default:
		return locales.QuoteFe6Help()
	}
}

func fieldLabel(kind quote.Kind, field string) string {
	if field == quote.Unit {
		if kind == quote.Battle {
			return locales.QuotePairUnit()
		}
		if defeatTable(kind) {
			return locales.QuoteDefeatedUnit()
		}
	}
	if field == quote.Flag && defeatTable(kind) {
		return locales.QuoteDefeatFlag()
	}
	return locales.Label(field)
}

func (p *Page) fieldHelp(kind quote.Kind, field string) string {
	switch field {
	case quote.Unit, quote.Opponent:
		if kind == quote.Battle && p.state.Session.Quotes.BattleConditions() {
			return locales.QuoteFe8ViewsHelp() + "\n" + locales.QuoteReferenceHelp()
		}
		return locales.QuoteReferenceHelp()
	case quote.Text, quote.FinalText, quote.Route:
		return locales.QuoteReferenceHelp()
	case quote.Chapter:
		help := fmt.Sprintf(
			locales.QuoteChapterHelp(),
			p.state.Session.Quotes.AnyChapter(),
		)
		if value, available := p.state.Session.Quotes.TriangleChapter(
			kind,
		); available {
			help += "\n" + fmt.Sprintf(
				locales.QuoteTriangleChapterHelp(),
				value,
			)
		}
		return help + "\n" + locales.QuoteReferenceHelp()
	case quote.Script:
		return locales.QuoteScriptHelp()
	default:
		return ""
	}
}

func (p *Page) field(
	c *ui.Context,
	entry *tableState,
	index int,
	field quote.Field,
	disabled bool,
	width float32,
) {
	value, _ := entry.draft.Value(index, field.ID)
	extended := !field.Editable(entry.rows[index].Raw)
	disabled = disabled || extended
	key := fieldKey{entry.rows[index].ID, field.ID}
	current := entry.inputs[key]
	if field.ID == quote.Text || field.ID == quote.FinalText {
		if current == nil {
			current = &valueInput{}
			entry.inputs[key] = current
		}
		label := fieldLabel(entry.draft.Kind(), field.ID)
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, label).
				FontSize(components.FontCaption).
				Bold().
				Tooltip(p.fieldHelp(entry.draft.Kind(), field.ID))
			components.TextReference(
				c,
				p.state,
				&current.reference,
				value,
				locales.QuoteEditor()+" / "+label,
				p.advanced,
				disabled,
				func(id uint16) {
					entry.preview = nil
					p.state.SetErr(entry.draft.Set(index, field.ID, uint32(id)))
					current.invalid = p.state.Err != nil
				},
				p.openText,
			)
		})
		return
	}
	labels, values := p.choices(field)
	if current == nil {
		current = &valueInput{text: fmt.Sprintf("0x%X", value)}
		for _, label := range labels {
			if values[label] == value {
				current.text = label
				break
			}
		}
		entry.inputs[key] = current
	}
	label := fieldLabel(entry.draft.Kind(), field.ID)
	help := p.fieldHelp(entry.draft.Kind(), field.ID)
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(components.FontCaption).Bold().Tooltip(help)
		if extended {
			ui.Text(c, locales.QuoteExtendedUnit()).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if len(labels) > 0 {
				components.TextDraft(
					p.state,
					ui.Combobox(c, &current.text, labels).
						Width(max(100, width)).
						MaxWidthPercent(100).
						Label(locales.QuoteEditor()+" / "+label).
						Tooltip(help).
						Disabled(disabled).
						OnChange(func() {
							entry.preview = nil
							if chosen, ok := values[current.text]; ok {
								p.state.SetErr(entry.draft.Set(
									index,
									field.ID,
									chosen,
								))
								current.invalid = p.state.Err != nil
							} else {
								current.invalid = true
							}
						}),
				)
			} else {
				control := components.TextDraft(
					p.state,
					ui.TextInput(c, &current.text).
						Width(min(200, max(100, width))).
						MaxWidthPercent(100).
						Label(locales.QuoteEditor()+" / "+label).
						Tooltip(help).
						Disabled(disabled).
						OnChange(func() {
							entry.preview = nil
							number, err := input.ParseInteger(current.text)
							if field.ID == quote.Script {
								var address uint64
								address, err = strconv.ParseUint(
									strings.TrimPrefix(
										strings.ToLower(
											strings.TrimSpace(current.text),
										),
										"0x",
									),
									16,
									32,
								)
								if err == nil && address != 0 &&
									address < 0x08000000 {
									address += 0x08000000
								}
								number = int64(address)
							}
							current.invalid = err != nil || number < 0 ||
								number > int64(field.Max)
							if !current.invalid {
								p.state.SetErr(entry.draft.Set(
									index,
									field.ID,
									uint32(number),
								))
								current.invalid = p.state.Err != nil
							} // #nosec G115 -- Field bounds are checked above.
						}),
				)
				if current.invalid {
					control.Error(
						fmt.Sprintf(
							"%s: 0…%d",
							locales.AllowedRange(),
							field.Max,
						),
					)
				}
			}
			switch field.ID {
			case quote.Unit, quote.Opponent:
				ui.Button(c, locales.Jump()).
					Disabled(p.state.Busy || value == 0 || value > 255).
					OnClick(func() { p.openRecord(records.Units, int(value)) })
			case quote.Chapter:
				ui.Button(c, locales.Jump()).
					Disabled(p.state.Busy || p.state.Session.Quotes.SpecialChapter(entry.draft.Kind(), value) || value > 255).
					OnClick(func() { p.openRecord(records.Chapters, int(value)) })
			case quote.Script:
				ui.Button(c, locales.QuoteScriptBytes()).
					Disabled(p.state.Busy || value < 0x080000C0 || value >= 0x0A000000).
					OnClick(func() { p.openHex(int(value)-0x08000000, 16) })
			}
		})
		if current.invalid {
			ui.Text(c, locales.QuoteInvalidInput()).
				FontSize(components.FontCaption).
				TextColor(c.Theme().Danger)
		}
		if defeatTable(entry.draft.Kind()) {
			if field.ID == quote.Flag && value == 0x65 {
				ui.Text(c, locales.QuoteDeathHelp()).
					FontSize(components.FontCaption)
			}
			if field.ID == quote.Script && value == 0 &&
				p.state.Session.Quotes.SilentDeath() {
				if text, err := entry.draft.Value(
					index,
					quote.Text,
				); err == nil &&
					text == 0 {
					ui.Text(c, locales.QuoteSilentDeath()).
						FontSize(components.FontCaption)
				}
			}
		}
	})
}

func (p *Page) actions(c *ui.Context, entry *tableState, stale bool) {
	modified := entry.draft != nil && entry.draft.Modified()
	invalid := entry.invalid()
	if !modified && !invalid && !stale && entry.err == nil {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		if invalid {
			ui.Button(c, locales.QuoteInvalidInput()).
				Disabled(p.state.Busy).
				OnClick(func() {
					for index, row := range entry.rows {
						for key, input := range entry.inputs {
							if key.row == row.ID && input.invalid {
								p.query = ""
								p.battleScope, p.scopeChoice = 0, ""
								p.selectIndex(entry, index)
								if p.unit >= 0 {
									p.startDefeatEdit(
										entry,
										index,
										fieldLabel(
											entry.draft.Kind(),
											key.field,
										),
										[]string{key.field},
										&p.defeat.actions,
									)
								}
								return
							}
						}
					}
				})
		}
		if entry.draft != nil {
			ui.Button(c, locales.QuotePreview()).
				Disabled(p.state.Busy || stale || !modified || invalid).
				OnClick(func() {
					p.defeat.openText = 0
					preview, err := p.state.Session.Quotes.Plan(entry.draft)
					entry.preview, entry.err = preview, err
					p.state.SetErr(err)
				})
		}
		reload := locales.QuoteReload()
		if modified || invalid {
			reload = locales.DeployReload()
		}
		ui.Button(c, reload).
			Disabled(p.state.Busy).
			OnClick(func() {
				p.defeat.openText = 0
				delete(p.drafts, p.tables[p.category].Kind)
			})
	})
	if plan := entry.preview; plan != nil {
		ui.Column(c.Key("preview")).Gap(6).Children(func() {
			if p.unit < 0 {
				ui.Textf(c, "%s · %d · 0x%08X → 0x%08X", locales.QuoteRecords(), plan.Count(), plan.Before(), plan.After()).
					Selectable()
				ui.Text(c, locales.QuotePreviewHelp()).
					FontSize(components.FontCaption)
				for _, site := range plan.Pointers() {
					ui.Textf(c, "0x%08X → 0x%08X", site, 0x08000000+plan.After()).
						Selectable().
						FontSize(components.FontCaption)
				}
			}
			for _, kind := range plan.Shared() {
				label := ui.Textf(
					c,
					"%s · %s",
					locales.QuoteShared(),
					locales.Label(string(kind)),
				)
				if p.unit < 0 {
					label.FontSize(components.FontCaption)
				}
			}
			limit := ui.Text(c, locales.QuoteReferenceLimit())
			if p.unit < 0 {
				limit.FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
			}
			ui.Row(c).Children(func() {
				label := locales.QuoteApply()
				if p.unit >= 0 && p.defeat.openText != 0 {
					label = locales.DefeatApplyAndEdit()
				}
				ui.PrimaryButton(c, label).
					Disabled(p.state.Busy || stale || entry.invalid() || !plan.Current(p.state.Session.Quotes)).
					OnClick(func() {
						p.state.SetErr(
							p.state.Edit(
								func() error { return p.state.Session.Quotes.Apply(plan) },
							),
						)
						if p.state.Err == nil {
							selected := entry.index()
							delete(p.drafts, entry.draft.Kind())
							if p.unit >= 0 {
								fresh := p.draft(p.tables[p.category])
								p.selectIndex(fresh, selected)
							}
							p.state.HistoryEpoch++
							if id := p.defeat.openText; p.unit >= 0 &&
								id != 0 &&
								p.openText != nil {
								p.defeat.openText = 0
								p.openText(id)
							}
						}
					})
			})
		})
	}
}
