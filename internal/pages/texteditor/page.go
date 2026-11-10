// Package texteditor owns text-ID navigation and revision-bound text drafts.
package texteditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type draft struct {
	text, original string
	revision       uint64
	preview        *text.Plan
	err            error
}

type Page struct {
	state          *editor.State
	openUse        func(text.Use)
	sidebar        components.Sidebar
	query          string
	lastQuery      string
	selected       int
	list           ui.ListState
	entries        []text.Entry
	rows           []text.Entry
	revision       uint64
	indexed        bool
	indexErr       error
	drafts         map[uint16]*draft
	showHelp       bool
	importFile     func(*text.File, func(*text.Import, string, error))
	exportFile     func(*text.File, []uint16)
	fileEpoch      uint64
	fileDraft      *text.Import
	filePlan       *text.ImportPlan
	fileErr        error
	fileRows       []text.FileEntry
	fileComparison *comparison
	fileSelection  int
	fileList       ui.ListState
	filesFocus     ui.Handle
	fileName       string
	reviewFocus    ui.Handle
}

func New(state *editor.State, openUse func(text.Use)) *Page {
	return &Page{
		state:   state,
		openUse: openUse,
		drafts:  make(map[uint16]*draft),
	}
}

func (p *Page) Reset() {
	state, openUse, sidebar := p.state, p.openUse, p.sidebar
	importFile, exportFile := p.importFile, p.exportFile
	epoch := p.fileEpoch + 1
	*p = *New(state, openUse)
	p.sidebar = sidebar
	p.importFile, p.exportFile, p.fileEpoch = importFile, exportFile, epoch
}

func (p *Page) refresh() {
	revision := p.state.Session.Document.State().Revision
	changed := !p.indexed || p.revision != revision
	var selectedID uint16
	if p.selected >= 0 && p.selected < len(p.rows) {
		selectedID = p.rows[p.selected].ID
	}
	if changed {
		p.entries, p.indexErr = p.state.Session.Texts.Entries()
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
	p.fileEpoch++
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
	p.state.SetErr(text.ErrText)
}

func (p *Page) View(c *ui.Context) {
	p.refresh()
	ui.Column(c.Key("text-page")).
		FillWidth().
		Grow(1).
		MinHeight(0).
		Children(func() {
			if p.fileDraft != nil {
				p.fileReview(c)
			} else {
				p.sidebar.Render(
					c.Key("text-browser"),
					330,
					func() { p.navigation(c) },
					func() { p.editor(c) },
				)
			}
		})
}

func (p *Page) navigation(c *ui.Context) {
	ui.Column(c.Key("text-list")).Fill().Gap(8).Padding(12).Children(func() {
		ui.SearchField(c, &p.query).
			Placeholder(locales.SearchText()).
			Label(locales.SearchText()).
			Disabled(p.state.Busy).
			OnChange(p.refresh)
		if p.indexErr != nil {
			ui.Text(c, components.ErrorText(p.indexErr)).
				TextColor(c.Theme().Danger)
			ui.Scroll(c.Key("file-actions")).
				Grow(1).
				Children(func() { p.fileActions(c) })
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
		ui.Scroll(c.Key("file-actions")).
			MaxHeight(220).
			Shrink(0).
			Children(func() { p.fileActions(c) })
	})
}

func (p *Page) draftFor(id uint16, value text.Value) *draft {
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
	value, err := p.state.Session.Texts.Read(id)
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
				FontSize(components.FontTitle).
				Bold().
				Grow(1)
			ui.Checkbox(c, &p.showHelp, locales.TextControlReference())
		})
		ui.Scroll(c.Key("command-help")).
			Hidden(!p.showHelp).
			Height(120).
			Gap(6).
			Children(func() {
				ui.Text(c, locales.TextControlHelp()).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted).
					Selectable()
				ui.Text(c, strings.Join(p.state.Session.Texts.Commands(), " ")).
					FontSize(components.FontCaption).
					Selectable()
			})
		ui.TextArea(c.Key(d), &d.text).
			Grow(1).
			Label(locales.TextContent()).
			Tooltip(locales.DescriptionEditScope()).
			ReadOnly(!value.Editable).
			Disabled(p.state.Busy).
			OnChange(func() { d.preview, d.err = nil, nil })
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
						d.preview, d.err = p.state.Session.Texts.PlanID(
							id,
							d.text,
						)
						p.state.Report("editor.operation", d.err)
					})
				ui.Button(c, locales.ReloadText()).
					Disabled(p.state.Busy).
					OnClick(func() { delete(p.drafts, id) })
			})
		}
		if d.err != nil {
			ui.Text(c, components.ErrorText(d.err)).TextColor(c.Theme().Danger)
		}
		if d.preview == nil {
			uses, complete := p.state.Session.Texts.Uses(id)
			components.TextUses(
				c.Key("current-uses"),
				p.state,
				uses,
				complete,
				false,
				p.openUse,
			)
		}
		p.preview(c, id, d, unavailable)
	})
}

func (p *Page) preview(c *ui.Context, id uint16, d *draft, unavailable bool) {
	if d.preview == nil {
		return
	}
	edit := d.preview
	if !edit.Current(p.state.Session.Texts) {
		unavailable = true
		ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
	}
	ui.Text(c, locales.TextEditSummary(fmt.Sprintf("0x%04X", id), len(edit.Uses()))).
		FontSize(components.FontCaption)
	components.TextUses(
		c.Key("preview-uses"),
		p.state,
		edit.Uses(),
		edit.UsesComplete(),
		unavailable,
		p.openUse,
	)
	ui.Button(c, locales.ApplyText()).Disabled(unavailable).OnClick(func() {
		if err := p.state.Edit(
			func() error { return p.state.Session.Texts.Apply(edit) },
		); err != nil {
			d.err, d.preview = err, nil
			return
		}
		delete(p.drafts, id)
	})
}
