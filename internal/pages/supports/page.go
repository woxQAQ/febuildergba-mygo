// Package supports owns the native support dialog panel.
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
	dialogueState struct {
		draft    *support.TalkDraft
		original []support.Talk
		plan     *support.Plan
		err      error
		inputs   map[string]*valueInput
	}
)

func (s *dialogueState) invalid() bool {
	for _, value := range s.inputs {
		if value.invalid {
			return true
		}
	}
	return false
}

type Page struct {
	state         *editor.State
	openSupport   func(int)
	openText      func(uint16)
	openHex       func(int, int)
	sidebar       components.Sidebar
	query         string
	unit, partner int
	selected      int
	list          ui.ListState
	rows          []support.Talk
	entry         *dialogueState
	advanced      bool
	textRevision  uint64
	textChoices   []string
	textIDs       map[string]uint32
}

func New(
	state *editor.State,
	character func(int),
	text func(uint16),
	hex func(int, int),
) *Page {
	return &Page{
		state:       state,
		openSupport: character,
		openText:    text,
		openHex:     hex,
	}
}

func (p *Page) Reset() {
	state, character, text, hex, sidebar := p.state, p.openSupport, p.openText, p.openHex, p.sidebar
	*p = *New(state, character, text, hex)
	p.sidebar = sidebar
}

func (p *Page) OpenPair(unit, partner int) {
	p.unit, p.partner, p.query = unit, partner, ""
	p.selected = 0
	p.rows = nil
	p.list = ui.ListState{}
}

func (p *Page) OpenTalk(index int) {
	p.OpenPair(0, 0)
	p.query = fmt.Sprintf("#%03X", index)
}

func (p *Page) draft() *dialogueState {
	e := p.entry
	if e != nil && e.draft != nil &&
		!e.draft.Current(p.state.Session.Supports) &&
		!e.draft.Modified() &&
		!e.invalid() {
		e = nil
	}
	if e == nil {
		e = &dialogueState{inputs: make(map[string]*valueInput)}
		e.draft, e.err = p.state.Session.Supports.NewTalkDraft()
		if e.draft != nil {
			e.original = e.draft.Talks()
		}
		p.entry = e
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
	entry := p.draft()
	if entry.draft == nil {
		ui.Column(c).Padding(16).Gap(8).Children(func() {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
			ui.Button(c, locales.QuoteReload()).
				Disabled(p.state.Busy).
				OnClick(func() { p.entry = nil })
		})
		return
	}
	previous := -1
	if p.selected >= 0 && p.selected < len(p.rows) {
		previous = p.rows[p.selected].Index
	}
	p.rows = nil
	query := strings.ToLower(strings.TrimSpace(p.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 16)
	index, indexErr := strconv.ParseUint(strings.TrimPrefix(query, "#"), 16, 16)
	for _, row := range entry.draft.Talks() {
		if p.unit != 0 && int(row.Unit) != p.unit &&
			int(row.Partner) != p.unit {
			continue
		}
		if p.partner != 0 && int(row.Unit) != p.partner &&
			int(row.Partner) != p.partner {
			continue
		}
		matches := query == "" ||
			strings.Contains(
				strings.ToLower(
					p.name(int(row.Unit))+" "+p.name(int(row.Partner)),
				),
				query,
			) ||
			idErr == nil &&
				(id == uint64(row.Unit) || id == uint64(row.Partner))
		if strings.HasPrefix(query, "#") {
			matches = indexErr == nil && int(index) == row.Index
		}
		if matches {
			p.rows = append(p.rows, row)
		}
	}
	p.selected = min(max(0, p.selected), len(p.rows)-1)
	for i, row := range p.rows {
		if row.Index == previous {
			p.selected = i
		}
	}
	stale := !entry.draft.Current(p.state.Session.Supports)
	p.sidebar.Render(c, 280, func() {
		ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
			components.TextDraft(
				p.state,
				ui.TextInput(c, &p.query).
					Label(locales.SupportSearch()).
					Placeholder(locales.SupportSearch()),
			)
			if p.unit != 0 {
				caption := fmt.Sprintf("%02X %s", p.unit, p.name(p.unit))
				if p.partner != 0 {
					caption += fmt.Sprintf(
						" / %02X %s",
						p.partner,
						p.name(p.partner),
					)
				}
				ui.Text(c, caption).FontSize(12)
				ui.Button(c, locales.SupportAllDialogues()).
					OnClick(func() { p.OpenPair(0, 0) })
			}
			p.list.Selected = &p.selected
			p.list.Key = func(i int) any { return p.rows[i].Index }
			ui.Table(c, &p.list, []ui.TableColumn{{ID: "pair", Title: locales.SupportDialogues()}}, len(p.rows), func(row, _ int) {
				r := p.rows[row]
				caption := p.talkCaption(r)
				if r.Texts != entry.original[r.Index].Texts ||
					p.rowInvalid(entry, r.Index) {
					caption += " *"
				}
				ui.Text(c, caption).SingleLine()
			}).
				Grow(1).
				Disabled(p.state.Busy)
		})
	}, func() {
		ui.Scroll(c.Key(entry)).Fill().Padding(16).Gap(12).Children(func() {
			if stale {
				ui.Text(c, locales.SupportStale()).TextColor(c.Theme().Danger)
			}
			if p.selected < 0 || p.selected >= len(p.rows) {
				ui.Text(c, locales.SupportNoDialogue())
			} else {
				row := p.rows[p.selected]
				ui.Column(c.Key(row.Index)).Gap(12).Children(func() {
					p.detail(c, entry, row, stale)
				})
			}
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			p.actions(c, entry, stale)
		})
	})
}

func (p *Page) rowInvalid(entry *dialogueState, index int) bool {
	for rank := range 3 {
		if v := entry.inputs[fmt.Sprintf("%d/%d", index, rank)]; v != nil &&
			v.invalid {
			return true
		}
	}
	return false
}

func (p *Page) detail(
	c *ui.Context,
	entry *dialogueState,
	row support.Talk,
	stale bool,
) {
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "%03X", row.Index).
			FontSize(13).
			TextColor(c.Theme().TextMuted)
		for _, id := range []uint16{row.Unit, row.Partner} {
			ui.Button(c, fmt.Sprintf("%02X %s", id, p.name(int(id)))).
				Tooltip(locales.SupportOpenUnit()).
				Disabled(p.state.Busy || id == 0 || id > 255).
				OnClick(func() { p.openSupport(int(id)) })
		}
		ui.Checkbox(c, &p.advanced, locales.Advanced())
	})
	ui.Text(c, locales.SupportDialogueHelp()).
		FontSize(12).
		TextColor(c.Theme().TextMuted)
	choices, values := p.textOptions()
	for rank, id := range row.Texts {
		ui.Column(c.Key(rank)).Gap(4).Children(func() {
			label := []string{locales.SupportC(), locales.SupportB(), locales.SupportA()}[rank]
			key := fmt.Sprintf("%d/%d", row.Index, rank)
			current := entry.inputs[key]
			if current == nil {
				current = &valueInput{text: fmt.Sprintf("0x%04X", id)}
				for _, choice := range choices {
					if values[choice] == id {
						current.text = choice
						break
					}
				}
				entry.inputs[key] = current
			}
			ui.Text(c, label).FontSize(13).Bold()
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				control := components.TextDraft(
					p.state,
					ui.Autocomplete(c.Key(key), &current.text, choices).
						Width(320).
						MaxWidthPercent(100).
						Label(locales.SupportDialogues()+" / "+label).
						Disabled(stale || p.state.Busy).OnChange(func() {
						entry.plan = nil
						number, chosen := values[current.text]
						if !chosen {
							value, err := input.ParseInteger(current.text)
							if err != nil || value < 0 || value > 65535 {
								current.invalid = true
								return
							}
							number = uint32(value)
						}
						entry.err = entry.draft.SetText(
							row.Index,
							rank,
							uint16(number),
						)
						current.invalid = entry.err != nil
					}),
				)
				if current.invalid {
					control.Error(locales.GameInvalidValue())
				}
				ui.Button(c, locales.QuoteOpenText()).
					Disabled(p.state.Busy || id == 0 || id > 65535 || current.invalid).
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
	if p.advanced {
		ui.Textf(c, "0x%08X · % X", row.Address, row.Raw).
			Font("monospace").
			FontSize(12).
			Selectable()
		ui.Button(c, locales.EditBytes()).
			Disabled(p.state.Busy || stale).
			OnClick(func() { p.openHex(row.Address, len(row.Raw)) })
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

func (p *Page) actions(c *ui.Context, entry *dialogueState, stale bool) {
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.SupportPreview()).
			Disabled(p.state.Busy || stale || !entry.draft.Modified() || entry.invalid()).
			OnClick(func() { entry.plan, entry.err = p.state.Session.Supports.PlanTalks(entry.draft) })
		label := locales.QuoteReload()
		if entry.draft.Modified() || entry.invalid() {
			label = locales.DeployReload()
		}
		ui.Button(c, label).
			Disabled(p.state.Busy).
			OnClick(func() { p.entry = nil })
	})
	if entry.invalid() {
		for _, row := range entry.draft.Talks() {
			if !p.rowInvalid(entry, row.Index) ||
				p.selected >= 0 && p.selected < len(p.rows) &&
					p.rows[p.selected].Index == row.Index {
				continue
			}
			ui.Button(c, fmt.Sprintf("%s · %03X", locales.SupportInvalidInput(), row.Index)).
				AlignSelf(ui.Start).
				Disabled(p.state.Busy).
				OnClick(func() { p.OpenTalk(row.Index) })
			break
		}
	}
	if entry.plan == nil {
		return
	}
	ui.Column(c.Key("preview")).
		Gap(8).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			for _, row := range entry.draft.Talks() {
				if row.Texts == entry.original[row.Index].Texts {
					continue
				}
				ui.Text(c, p.talkCaption(row)).FontSize(13).Bold()
				for rank, id := range row.Texts {
					before := entry.original[row.Index].Texts[rank]
					if before != id {
						ui.Textf(c, "%s: %04X → %04X", []string{locales.SupportC(), locales.SupportB(), locales.SupportA()}[rank], before, id).
							FontSize(12)
					}
				}
			}
			ui.Text(c, locales.SupportTalkApplyHelp()).FontSize(12)
			if p.advanced {
				for _, move := range entry.plan.Moves() {
					ui.Textf(c, "0x%08X → 0x%08X", move.Before, move.After).
						Font("monospace").
						FontSize(12)
				}
			}
			ui.Button(c, locales.SupportApply()).
				AlignSelf(ui.Start).
				Disabled(p.state.Busy || stale || entry.invalid() || !entry.plan.Current(p.state.Session.Supports)).
				OnClick(func() {
					entry.err = p.state.Session.Supports.Apply(entry.plan)
					if entry.err == nil {
						p.entry = nil
						p.state.HistoryEpoch++
						p.state.Session.Refresh()
					}
				})
		})
}
