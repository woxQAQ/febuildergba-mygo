// Package records owns record navigation, table settings and batch editing.
package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type pane struct {
	query       string
	sidebar     components.Sidebar
	selected    int
	initialized bool
	list        ui.ListState
	chosen      ui.Selection[int]
	layout      layoutDraft
	batch       batchDraft
}

type Page struct {
	state      *editor.State
	panes      []pane
	tab        int
	form       *form
	selectKind func(records.Kind)
}

func New(
	state *editor.State,
	openHex func(int, int),
	openText func(uint16),
	selectKind func(records.Kind),
	assets components.AssetActions,
) *Page {
	a := &Page{
		state:      state,
		selectKind: selectKind,
		panes:      make([]pane, len(records.Kinds)),
	}
	a.form = newForm(state, openHex, openText, a.Open, assets)
	return a
}

func (a *Page) Reset() {
	a.panes = make([]pane, len(records.Kinds))
	a.tab = 0
	a.form.Reset()
}

func (a *Page) Deactivate() { a.form.chapter.Deactivate() }

func (a *Page) View(c *ui.Context, tab int) {
	a.tab = tab
	if records.Kinds[tab] != records.Chapters {
		a.Deactivate()
	}
	pane := &a.panes[tab]
	initial := float32(330)
	if records.Kinds[tab] == records.Units ||
		records.Kinds[tab] == records.Classes {
		initial = 260
	}
	pane.sidebar.Render(
		c,
		initial,
		func() { a.records(c) },
		func() { a.details(c) },
	)
}

func (a *Page) details(c *ui.Context) {
	kind := records.Kinds[a.tab]
	table := a.state.Session.Records.Table(kind)
	if table == nil {
		return
	}
	if table.Err != nil {
		ui.Text(c, components.ErrorText(table.Err)).Padding(24)
		return
	}
	rows := a.rows()
	ids := a.panes[a.tab].selectedIDs(rows)
	if len(ids) > 1 {
		a.batchEditor(c, table, ids)
		return
	}
	selected := a.panes[a.tab].selected
	if len(rows) == 0 || selected < 0 || selected >= len(rows) {
		ui.Text(c, locales.NoResults()).Padding(24)
		return
	}
	a.form.View(c, kind, rows[selected], table, a.panes[a.tab].sidebar.Width())
}

func (a *Page) rows() []records.Row {
	if a.state.Session == nil || a.tab >= len(a.panes) {
		return nil
	}
	table := a.state.Session.Records.Table(records.Kinds[a.tab])
	if table == nil || table.Err != nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(a.panes[a.tab].query))
	if query == "" {
		return table.Rows
	}
	var rows []records.Row
	for _, row := range table.Rows {
		if strings.Contains(strings.ToLower(row.Name), query) ||
			strings.EqualFold(fmt.Sprintf("%02X", row.ID), query) ||
			strings.EqualFold(fmt.Sprintf("0x%02X", row.ID), query) {
			rows = append(rows, row)
		}
	}
	return rows
}

func (a *Page) records(c *ui.Context) {
	pane := &a.panes[a.tab]
	ui.Column(c.Key(a.tab)).Fill().Gap(8).Padding(12).Children(func() {
		if components.TextDraft(a.state, ui.TextInput(c.Key("search"), &pane.query).Placeholder(locales.Search()).Label(locales.Search())).
			Changed() {
			pane.selected = 0
			pane.chosen.Clear()
			pane.initialized = false
			pane.list = ui.ListState{}
		}
		a.tableLayout(c)
		rows := a.rows()
		if pane.selected >= len(rows) {
			pane.selected = 0
		}
		if !pane.initialized && len(rows) > 0 {
			pane.chosen.Add(rows[pane.selected].ID)
			pane.initialized = true
		}
		pane.list.Selected = &pane.selected
		pane.list.Selection = &pane.chosen
		pane.list.Key = func(i int) any { return rows[i].ID }
		cols := []ui.TableColumn{
			{ID: "id", Title: "ID", Width: 52, Fixed: true},
			{ID: "name", Title: locales.Name()},
		}
		ui.Table(c, &pane.list, cols, len(rows), func(row, col int) {
			if col == 0 {
				ui.Textf(c, "%02X", rows[row].ID).FontFeatures("tnum")
			} else {
				ui.Text(c, rows[row].Name).SingleLine()
			}
		}).Grow(1).Label(locales.Label(string(records.Kinds[a.tab])))
		ui.Text(c, locales.RecordSummary(len(rows))).
			TextColor(c.Theme().TextMuted)
	})
}

func (a *Page) SetPortraitOpener(
	open func(int),
) {
	a.form.SetPortraitOpener(open)
}

func (a *Page) SetScriptOpener(
	open func(int, int),
) {
	a.form.SetScriptOpener(open)
}

func (a *Page) OpenMapChange(chapter, id int) {
	a.Open(records.Chapters, chapter)
	a.form.OpenMapChange(chapter, id)
}

func (a *Page) OpenShop(chapter, location int) {
	a.Open(records.Chapters, chapter)
	a.form.OpenShop(chapter, location)
}

func (a *Page) OpenDeployment(chapter, site int) {
	a.Open(records.Chapters, chapter)
	a.form.OpenDeployment(chapter, site)
}

func (a *Page) Open(kind records.Kind, id int) {
	for i, k := range records.Kinds {
		if k != kind {
			continue
		}
		table := a.state.Session.Records.Table(kind)
		if table == nil || table.Err != nil {
			a.state.Err = edit.ErrLayout
			return
		}
		for index, row := range table.Rows {
			if row.ID == id {
				a.tab = i
				a.selectKind(kind)
				a.panes[i].query = ""
				a.panes[i].selected = index
				a.panes[i].chosen.Clear()
				a.panes[i].chosen.Add(id)
				a.panes[i].list.ScrollTo(index, ui.Center)
				return
			}
		}
	}
	a.state.Err = edit.ErrField
}
