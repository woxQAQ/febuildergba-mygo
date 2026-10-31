// Package supports owns character support drafts and navigation.
package supports

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	valueInput struct {
		text    string
		invalid bool
	}
	unitState struct {
		draft       *support.Draft
		plan        *support.Plan
		err         error
		inputs      map[string]*valueInput
		synchronize bool
		talk        string
		generation  int
	}
)

func (s *unitState) invalid() bool {
	for _, value := range s.inputs {
		if value.invalid {
			return true
		}
	}
	return false
}

type Page struct {
	state        *editor.State
	openRecord   func(records.Kind, int)
	openText     func(uint16)
	openHex      func(int, int)
	sidebar      components.Sidebar
	query        string
	selected     int
	list         ui.ListState
	rows         []support.Record
	drafts       map[int]*unitState
	advanced     bool
	textRevision uint64
	textChoices  []string
	textIDs      map[string]uint32
}

func New(
	state *editor.State,
	record func(records.Kind, int),
	text func(uint16),
	hex func(int, int),
) *Page {
	return &Page{
		state:      state,
		openRecord: record,
		openText:   text,
		openHex:    hex,
		drafts:     make(map[int]*unitState),
	}
}

func (p *Page) Reset() {
	state, record, text, hex, sidebar := p.state, p.openRecord, p.openText, p.openHex, p.sidebar
	*p = *New(state, record, text, hex)
	p.sidebar = sidebar
}

func (p *Page) Open(id int) {
	p.query = fmt.Sprintf("0x%02X", id)
	p.selected = 0
	p.list = ui.ListState{}
}

func (p *Page) OpenTalk(index int) {
	rows, _ := p.state.Session.Supports.Talks()
	if index < 0 || index >= len(rows) {
		return
	}
	owner := int(rows[index].Unit)
	if owner < 1 || owner > 255 {
		owner = int(rows[index].Partner)
	}
	p.Open(owner)
	entry := p.draft(owner)
	entry.talk = p.talkCaption(rows[index])
}

func (p *Page) draft(id int) *unitState {
	e := p.drafts[id]
	if e != nil && e.draft != nil &&
		!e.draft.Current(p.state.Session.Supports) &&
		!e.draft.Modified() &&
		!e.invalid() {
		e = nil
	}
	if e == nil {
		e = &unitState{inputs: make(map[string]*valueInput)}
		e.draft, e.err = p.state.Session.Supports.NewDraft(id)
		p.drafts[id] = e
	}
	return e
}

func (p *Page) name(id int) string {
	if table := p.state.Session.Records.Table(
		records.Units,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == id {
				return row.Name
			}
		}
	}
	return ""
}

func (p *Page) talkCaption(row support.Talk) string {
	return fmt.Sprintf(
		"%03X · %02X %s / %02X %s",
		row.Index,
		row.Unit,
		p.name(int(row.Unit)),
		row.Partner,
		p.name(int(row.Partner)),
	)
}

func (p *Page) View(c *ui.Context) {
	rows, err := p.state.Session.Supports.Records()
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		return
	}
	previous := 0
	if p.selected >= 0 && p.selected < len(p.rows) {
		previous = p.rows[p.selected].ID
	}
	p.rows = nil
	query := strings.ToLower(strings.TrimSpace(p.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 8)
	for _, row := range rows {
		if query == "" || strings.Contains(strings.ToLower(row.Name), query) ||
			idErr == nil && int(id) == row.ID {
			p.rows = append(p.rows, row)
		}
	}
	p.selected = min(max(0, p.selected), len(p.rows)-1)
	for i, row := range p.rows {
		if row.ID == previous {
			p.selected = i
		}
	}
	p.sidebar.Render(c, 280, func() {
		ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
			components.TextDraft(
				p.state,
				ui.TextInput(c, &p.query).
					Label(locales.SupportSearch()).
					Placeholder(locales.SupportSearch()).
					Disabled(p.state.Busy),
			)
			p.list.Selected = &p.selected
			p.list.Key = func(i int) any { return p.rows[i].ID }
			ui.Table(c, &p.list, []ui.TableColumn{{ID: "unit", Title: locales.SupportEditor()}}, len(p.rows), func(row, _ int) {
				r := p.rows[row]
				caption := fmt.Sprintf("%02X · %s", r.ID, r.Name)
				if d := p.drafts[r.ID]; d != nil &&
					(d.invalid() || d.draft != nil && d.draft.Modified()) {
					caption += " *"
				}
				ui.Text(c, caption).SingleLine()
			}).
				Grow(1).
				Disabled(p.state.Busy)
		})
	}, func() {
		if p.selected < 0 || p.selected >= len(p.rows) {
			ui.Text(c, locales.QuoteNoMatches())
			return
		}
		row := p.rows[p.selected]
		p.detail(c.Key(row.ID), row, p.draft(row.ID))
	})
}

func (p *Page) detail(c *ui.Context, row support.Record, entry *unitState) {
	stale := entry.draft == nil ||
		!entry.draft.Current(p.state.Session.Supports)
	disabled := stale || p.state.Busy
	ui.Scroll(c.Key(entry)).Fill().Children(func() {
		ui.Column(c).Padding(16).Gap(12).Children(func() {
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Textf(c, "%02X · %s", row.ID, row.Name).FontSize(18).Bold()
				ui.Button(c, locales.SupportOpenUnit()).
					Disabled(p.state.Busy).
					OnClick(func() { p.openRecord(records.Units, row.ID) })
				ui.Checkbox(c, &p.advanced, locales.Advanced())
			})
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger).
					Tooltip(entry.err.Error())
			}
			if stale && entry.draft != nil {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			if entry.draft != nil {
				ui.Text(c, locales.SupportRelationships()).Bold()
				ui.Text(c, locales.SupportRelationshipHelp()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
				if row.Empty {
					ui.Text(c, locales.SupportInitialize()).FontSize(12)
				}
				p.relationships(c.Key(entry.generation), entry, disabled)
				ui.Checkbox(c, &entry.synchronize, locales.SupportSynchronize()).
					Disabled(disabled).
					OnChange(func() { entry.plan = nil })
				ui.Text(c, locales.SupportSynchronizeHelp()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
				ui.Text(c, locales.SupportDialogues()).Bold()
				p.dialogs(c, entry, disabled)
				if p.advanced {
					ui.Textf(c, "% X", entry.draft.Raw()).
						Font("monospace").
						Selectable()
					ui.Button(c, locales.EditBytes()).
						Disabled(p.state.Busy || entry.draft.Address() == 0).
						OnClick(func() { p.openHex(entry.draft.Address(), len(entry.draft.Raw())) })
				}
			}
			p.actions(c, row.ID, entry, stale)
		})
	})
}

func (p *Page) relationships(c *ui.Context, entry *unitState, disabled bool) {
	choices := []string{}
	values := map[string]uint32{}
	if table := p.state.Session.Records.Table(
		records.Units,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID < 1 || row.ID > 255 || row.ID == entry.draft.Unit().ID {
				continue
			}
			label := fmt.Sprintf("%02X · %s", row.ID, row.Name)
			choices = append(choices, label)
			values[label] = uint32(row.ID)
		}
	}
	for i, relation := range entry.draft.Relationships() {
		ui.Row(c.Key(i)).Wrap().AlignItems(ui.End).Gap(8).Children(func() {
			base := fmt.Sprintf("relation-%d-", i)
			p.value(
				c,
				entry,
				base+"unit",
				locales.SupportPartner(),
				uint32(relation.Unit),
				255,
				choices,
				values,
				disabled,
				func(value uint32) error {
					r := entry.draft.Relationships()[i]
					r.Unit = uint8(value)
					return entry.draft.Set(i, r)
				},
			)
			p.value(
				c,
				entry,
				base+"initial",
				locales.SupportInitial(),
				uint32(relation.Initial),
				255,
				nil,
				nil,
				disabled,
				func(value uint32) error {
					r := entry.draft.Relationships()[i]
					r.Initial = uint8(value)
					return entry.draft.Set(i, r)
				},
			)
			p.value(
				c,
				entry,
				base+"growth",
				locales.SupportGrowth(),
				uint32(relation.Growth),
				255,
				nil,
				nil,
				disabled,
				func(value uint32) error {
					r := entry.draft.Relationships()[i]
					r.Growth = uint8(value)
					return entry.draft.Set(i, r)
				},
			)
			ui.Button(c, locales.QuoteDelete()).
				Disabled(disabled || entry.invalid()).
				OnClick(func() {
					p.state.Err = entry.draft.Remove(i)
					p.resetRelations(entry)
				})
		})
	}
	ui.Button(c, locales.SupportAdd()).
		Disabled(disabled || entry.invalid() || len(entry.draft.Relationships()) >= entry.draft.Capacity() || len(choices) == 0).
		OnClick(func() {
			used := map[uint8]bool{}
			for _, r := range entry.draft.Relationships() {
				used[r.Unit] = true
			}
			for _, label := range choices {
				id := uint8(values[label])
				if !used[id] {
					p.state.Err = entry.draft.Add(
						support.Relationship{Unit: id},
					)
					p.resetRelations(entry)
					break
				}
			}
		})
}

func (p *Page) resetRelations(entry *unitState) {
	entry.plan = nil
	entry.generation++
	for key := range entry.inputs {
		if strings.HasPrefix(key, "relation-") {
			delete(entry.inputs, key)
		}
	}
}

func (p *Page) textOptions() ([]string, map[string]uint32) {
	revision := p.state.Session.Document.State().Revision
	if p.textChoices == nil || revision != p.textRevision {
		p.textRevision = revision
		label := "0000 · " + locales.QuoteNoText()
		p.textChoices = []string{label}
		p.textIDs = map[string]uint32{label: 0}
		rows, err := p.state.Session.Texts.Entries()
		if err == nil {
			for _, row := range rows {
				if row.Err != nil {
					continue
				}
				content := []rune(strings.ReplaceAll(row.Content, "\n", " "))
				if len(content) > 48 {
					content = append(content[:48], '…')
				}
				label := fmt.Sprintf("%04X · %s", row.ID, string(content))
				p.textChoices = append(p.textChoices, label)
				p.textIDs[label] = uint32(row.ID)
			}
		}
	}
	return p.textChoices, p.textIDs
}

func (p *Page) dialogs(c *ui.Context, entry *unitState, disabled bool) {
	if err := entry.draft.TalkError(); err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		return
	}
	rows := entry.draft.Talks()
	if len(rows) == 0 {
		ui.Text(c, locales.SupportNoDialogue()).FontSize(12)
		return
	}
	labels := make([]string, len(rows))
	selected := 0
	found := false
	for i, row := range rows {
		labels[i] = p.talkCaption(row)
		if labels[i] == entry.talk {
			selected = i
			found = true
		}
	}
	if !found {
		entry.talk = labels[0]
	}
	ui.Select(c, &entry.talk, labels).
		Label(locales.SupportDialogues()).
		Disabled(p.state.Busy)
	row := rows[selected]
	ui.Text(c, locales.SupportDialogueHelp()).
		FontSize(12).
		TextColor(c.Theme().TextMuted)
	choices, values := p.textOptions()
	for rank, id := range row.Texts {
		ui.Column(c.Key(fmt.Sprintf("talk-%d-%d", row.Index, rank))).
			Gap(4).
			Children(func() {
				ui.Row(c).Wrap().AlignItems(ui.End).Gap(8).Children(func() {
					p.value(
						c,
						entry,
						fmt.Sprintf("talk-%d-%d", row.Index, rank),
						[]string{locales.SupportC(), locales.SupportB(), locales.SupportA()}[rank],
						id,
						65535,
						choices,
						values,
						disabled,
						func(value uint32) error { return entry.draft.SetText(row.Index, rank, uint16(value)) },
					)
					ui.Button(c, locales.QuoteOpenText()).
						Disabled(p.state.Busy || id == 0 || id > 65535).
						OnClick(func() { p.openText(uint16(id)) })
				})
				if id != 0 && id <= 65535 {
					value, err := p.state.Session.Texts.Value(uint16(id))
					if err != nil {
						ui.Text(c, components.ErrorText(err)).
							TextColor(c.Theme().Danger)
					} else {
						ui.Scroll(c).
							Height(80).
							Children(func() { ui.Text(c, value.Content).FontSize(13).Selectable() })
					}
				}
			})
	}
}

func (p *Page) value(
	c *ui.Context,
	entry *unitState,
	key, label string,
	value, maximum uint32,
	choices []string,
	values map[string]uint32,
	disabled bool,
	set func(uint32) error,
) {
	current := entry.inputs[key]
	if current == nil {
		current = &valueInput{text: fmt.Sprint(value)}
		for _, choice := range choices {
			if values[choice] == value {
				current.text = choice
				break
			}
		}
		entry.inputs[key] = current
	}
	update := func() {
		entry.plan = nil
		number, chosen := values[current.text]
		var err error
		if !chosen {
			var parsed int64
			parsed, err = input.ParseInteger(current.text)
			if err != nil || parsed < 0 || parsed > int64(maximum) {
				current.invalid = true
				return
			}
			number = uint32(parsed)
		}
		err = set(number)
		current.invalid = err != nil
		p.state.Err = err
	}
	ui.Column(c.Key(key)).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(12)
		var control ui.Element
		if len(choices) > 0 {
			control = ui.Autocomplete(c, &current.text, choices).
				Width(320).
				MaxWidthPercent(100)
		} else {
			control = ui.TextInput(c, &current.text).Width(85)
		}
		components.TextDraft(
			p.state,
			control.Label(locales.SupportEditor()+" / "+label).
				Disabled(disabled).
				OnChange(update),
		)
		if current.invalid {
			control.Error(locales.GameInvalidValue())
		}
	})
}

func (p *Page) actions(c *ui.Context, id int, entry *unitState, stale bool) {
	modified := entry.draft != nil && entry.draft.Modified()
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.SupportPreview()).
			Disabled(p.state.Busy || stale || !modified || entry.invalid()).
			OnClick(func() { entry.plan, p.state.Err = p.state.Session.Supports.Plan(entry.draft, entry.synchronize) })
		label := locales.QuoteReload()
		if modified || entry.invalid() {
			label = locales.DeployReload()
		}
		ui.Button(c, label).
			Disabled(p.state.Busy).
			OnClick(func() { delete(p.drafts, id) })
	})
	plan := entry.plan
	if plan == nil {
		return
	}
	ui.Text(c, locales.SupportPreviewHelp()).FontSize(12)
	for _, move := range plan.Moves() {
		label := locales.SupportDialogues()
		if move.Unit != 0 {
			label = fmt.Sprintf("%02X %s", move.Unit, p.name(move.Unit))
		}
		ui.Textf(c, "%s · 0x%08X → 0x%08X", label, move.Before, move.After).
			Selectable()
		for _, other := range move.Shared {
			ui.Textf(c, "%s · %02X %s", locales.SupportShared(), other.ID, other.Name).
				FontSize(12)
		}
		if p.advanced {
			for _, site := range move.Pointers {
				ui.Textf(c, "0x%08X → 0x%08X", site, move.After+0x08000000).
					FontSize(12).
					Selectable()
			}
		}
	}
	for _, unit := range plan.Missing() {
		ui.Textf(c, "%s · %02X %s", locales.SupportMissingReverse(), unit.ID, unit.Name).
			FontSize(12)
	}
	ui.PrimaryButton(c, locales.SupportApply()).
		Disabled(p.state.Busy || stale || entry.invalid() || !plan.Current(p.state.Session.Supports)).
		OnClick(func() {
			p.state.Err = p.state.Session.Supports.Apply(plan)
			if p.state.Err == nil {
				delete(p.drafts, id)
				p.state.HistoryEpoch++
			}
		})
}
