// Package texteditor owns text-ID navigation and revision-bound text drafts.
package texteditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type draft struct {
	text, original string
	revision       uint64
	preview        *game.TextEdit
	err            error
}

type Page struct {
	state      *components.State
	openRecord func(game.Kind, int)
	sidebar    float32
	query      string
	lastQuery  string
	selected   int
	list       ui.ListState
	entries    []game.TextEntry
	rows       []game.TextEntry
	revision   uint64
	indexed    bool
	indexErr   error
	drafts     map[uint16]*draft
	showRaw    bool
	showHelp   bool
}

func New(state *components.State, openRecord func(game.Kind, int)) *Page {
	return &Page{
		state:      state,
		openRecord: openRecord,
		sidebar:    330,
		drafts:     make(map[uint16]*draft),
	}
}

func (p *Page) Reset() {
	state, openRecord, sidebar := p.state, p.openRecord, p.sidebar
	*p = *New(state, openRecord)
	p.sidebar = sidebar
}

func (p *Page) refresh() {
	revision := p.state.Session.Document.State().Revision
	changed := !p.indexed || p.revision != revision
	var selectedID uint16
	if p.selected >= 0 && p.selected < len(p.rows) {
		selectedID = p.rows[p.selected].ID
	}
	if changed {
		p.entries, p.indexErr = p.state.Session.TextEntries()
		p.revision, p.indexed = revision, true
	}
	if !changed && p.query == p.lastQuery {
		return
	}
	query := strings.ToLower(strings.TrimSpace(p.query))
	textID, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 16)
	p.rows = nil
	for _, entry := range p.entries {
		if query == "" ||
			strings.Contains(strings.ToLower(entry.Content), query) ||
			(idErr == nil && uint64(entry.ID) == textID) {
			p.rows = append(p.rows, entry)
		}
	}
	p.selected = 0
	if p.query == p.lastQuery {
		for i, entry := range p.rows {
			if entry.ID == selectedID {
				p.selected = i
				break
			}
		}
	} else {
		p.list = ui.ListState{}
	}
	p.lastQuery = p.query
}

func (p *Page) Open(id uint16) {
	p.query = fmt.Sprintf("0x%04X", id)
	p.refresh()
	for i, entry := range p.rows {
		if entry.ID == id {
			p.selected = i
			p.list.ScrollTo(i, ui.Center)
			return
		}
	}
	p.selected = -1
	p.state.Err = game.ErrText
}

func (p *Page) View(c *ui.Context) {
	p.refresh()
	ui.Split(c, &p.sidebar, func() { p.navigation(c) }, func() { p.editor(c) }).
		Grow(1)
}

func (p *Page) navigation(c *ui.Context) {
	ui.Column(c.Key("text-list")).Fill().Gap(8).Padding(12).Children(func() {
		components.TextDraft(
			p.state,
			ui.TextInput(c, &p.query).
				Placeholder(locales.SearchText()).
				Label(locales.SearchText()).
				Disabled(p.state.Busy).
				OnChange(p.refresh),
		)
		if p.indexErr != nil {
			ui.Text(c, components.ErrorText(p.indexErr)).
				TextColor(c.Theme().Danger)
			return
		}
		p.list.Selected = &p.selected
		p.list.Key = func(i int) any { return p.rows[i].ID }
		columns := []ui.TableColumn{
			{ID: "id", Title: "ID", Width: 60, Fixed: true},
			{ID: "text", Title: locales.TextContent()},
		}
		ui.Table(c, &p.list, columns, len(p.rows), func(row, col int) {
			entry := p.rows[row]
			switch {
			case col == 0:
				ui.Textf(c, "%04X", entry.ID).FontFeatures("tnum")
			case entry.Err != nil:
				ui.Text(c, components.ErrorText(entry.Err)).
					SingleLine().
					TextColor(c.Theme().Danger)
			default:
				ui.Text(c, strings.ReplaceAll(entry.Content, "\n", " ")).
					SingleLine()
			}
		}).Grow(1).Label(locales.TextEditor())
		ui.Text(c, locales.RecordSummary(len(p.rows))).
			TextColor(c.Theme().TextMuted)
	})
}

func (p *Page) draftFor(id uint16, value game.TextValue) *draft {
	d := p.drafts[id]
	revision := p.state.Session.Document.State().Revision
	if d == nil || (d.text == d.original && d.original != value.Content) {
		d = &draft{
			text:     value.Content,
			original: value.Content,
			revision: revision,
		}
		p.drafts[id] = d
	}
	if d.revision != revision {
		d.preview = nil
		d.revision = revision
	}
	return d
}

func (p *Page) editor(c *ui.Context) {
	if len(p.rows) == 0 || p.selected < 0 || p.selected >= len(p.rows) {
		ui.Text(c, locales.NoResults()).Padding(24)
		return
	}
	id := p.rows[p.selected].ID
	value, err := p.state.Session.TextByID(id)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).Padding(24)
		return
	}
	d := p.draftFor(id, value)
	stale := d.original != value.Content
	unavailable := p.state.Busy || !value.Editable || stale
	ui.Column(c.Key(id)).Fill().Padding(16).Gap(10).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%s · %04X", locales.TextEditor(), id).
				FontSize(20).
				Bold().
				Grow(1)
			ui.Checkbox(c, &p.showRaw, locales.RawText())
			ui.Checkbox(c, &p.showHelp, locales.TextControlReference())
		})
		if p.showHelp {
			ui.Scroll(c.Key("command-help")).
				Height(120).
				Gap(6).
				Children(func() {
					ui.Text(c, locales.TextControlHelp()).
						FontSize(12).
						TextColor(c.Theme().TextMuted).
						Selectable()
					ui.Text(c, strings.Join(p.state.Session.Adapter.TextCommands(), " ")).
						FontSize(12).
						Selectable()
				})
		}
		components.TextDraft(
			p.state,
			ui.TextArea(c.Key(d), &d.text).
				Grow(1).
				Label(locales.TextContent()).
				Tooltip(locales.DescriptionEditScope()).
				ReadOnly(!value.Editable).
				Disabled(p.state.Busy).
				OnChange(func() { d.preview, d.err = nil, nil }),
		)
		if p.showRaw {
			raw := value.Raw
			ui.TextArea(c.Key("raw-text"), &raw).
				ReadOnly(true).
				Height(100).
				Font("monospace").
				Label(locales.RawText())
		}
		if stale {
			ui.Text(c, locales.TextDraftStale()).TextColor(c.Theme().Danger)
		} else if !value.Editable {
			ui.Text(c, components.ErrorText(value.ReadOnlyReason)).
				TextColor(c.Theme().TextMuted)
		}
		if d.text != value.Content || stale {
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, locales.PreviewText()).
					Disabled(unavailable).
					OnClick(func() {
						d.preview, d.err = p.state.Session.PlanTextIDEdit(
							id,
							d.text,
						)
					})
				ui.Button(c, locales.ReloadText()).
					Disabled(p.state.Busy).
					OnClick(func() { delete(p.drafts, id) })
			})
		}
		if d.err != nil {
			ui.Text(c, components.ErrorText(d.err)).TextColor(c.Theme().Danger)
		}
		p.preview(c, id, d, unavailable)
	})
}

func (p *Page) preview(c *ui.Context, id uint16, d *draft, unavailable bool) {
	if d.preview == nil {
		return
	}
	edit := d.preview
	ui.Text(c, locales.TextEditSummary(fmt.Sprintf("0x%04X", id), len(edit.Uses()))).
		FontSize(12)
	ui.Scroll(c.Key("text-uses")).MaxHeight(140).Gap(4).Children(func() {
		for _, use := range edit.Uses() {
			ui.Button(c.Key(use), fmt.Sprintf("%s %02X · %s", locales.Label(string(use.Kind)), use.ID, locales.Label(use.Field))).
				Disabled(p.state.Busy).
				OnClick(func() { p.openRecord(use.Kind, use.ID) })
		}
	})
	ui.Button(c, locales.ApplyText()).Disabled(unavailable).OnClick(func() {
		if err := p.state.Session.ApplyTextEdit(edit); err != nil {
			d.err, d.preview = err, nil
			return
		}
		delete(p.drafts, id)
	})
}
