// Package records owns record navigation, table settings and batch editing.
package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/recordform"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type pane struct {
	query    string
	selected int
	list     ui.ListState
	chosen   ui.Selection[int]
	layout   layoutDraft
	batch    batchDraft
}

type Page struct {
	state      *editor.State
	panes      [3]pane
	tab        int
	sidebar    float32
	form       *recordform.Form
	selectKind func(game.Kind)
}

func New(state *editor.State, openHex func(int, int), selectKind func(game.Kind), assets recordform.AssetActions) *Page {
	a := &Page{state: state, sidebar: 330, selectKind: selectKind}
	a.form = recordform.New(state, openHex, a.Open, assets)
	return a
}

func (a *Page) Reset() {
	a.panes = [3]pane{}
	a.tab = 0
	a.form.Reset()
}

func (a *Page) View(c *ui.Context, tab int) {
	a.tab = tab
	ui.Split(c, &a.sidebar, func() { a.records(c) }, func() { a.details(c) }).Grow(1)
}

func (a *Page) details(c *ui.Context) {
	kind := game.Kinds[a.tab]
	table := a.state.Session.Table(kind)
	if table == nil {
		return
	}
	if table.Err != nil {
		ui.Text(c, locales.ErrorText(table.Err)).Padding(24)
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
	a.form.View(c, kind, rows[selected], table, a.sidebar)
}

func (a *Page) rows() []game.Row {
	if a.state.Session == nil || a.tab >= len(a.panes) {
		return nil
	}
	table := a.state.Session.Table(game.Kinds[a.tab])
	if table == nil || table.Err != nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(a.panes[a.tab].query))
	if query == "" {
		return table.Rows
	}
	var rows []game.Row
	for _, row := range table.Rows {
		if strings.Contains(strings.ToLower(row.Name), query) || strings.EqualFold(fmt.Sprintf("%02X", row.ID), query) || strings.EqualFold(fmt.Sprintf("0x%02X", row.ID), query) {
			rows = append(rows, row)
		}
	}
	return rows
}

func (a *Page) records(c *ui.Context) {
	pane := &a.panes[a.tab]
	ui.Column(c.Key(a.tab)).Fill().Gap(8).Padding(12).Children(func() {
		if components.TextDraft(a.state, ui.TextInput(c.Key("search"), &pane.query).Placeholder(locales.Search()).Label(locales.Search())).Changed() {
			pane.selected = 0
			pane.chosen.Clear()
			pane.list = ui.ListState{}
		}
		a.tableLayout(c)
		rows := a.rows()
		if pane.selected >= len(rows) {
			pane.selected = 0
		}
		pane.list.Selected = &pane.selected
		pane.list.Selection = &pane.chosen
		pane.list.Key = func(i int) any { return rows[i].ID }
		cols := []ui.TableColumn{{ID: "id", Title: "ID", Width: 52, Fixed: true}, {ID: "name", Title: locales.Name()}}
		ui.Table(c, &pane.list, cols, len(rows), func(row, col int) {
			if col == 0 {
				ui.Textf(c, "%02X", rows[row].ID).FontFeatures("tnum")
			} else {
				ui.Text(c, rows[row].Name).SingleLine()
			}
		}).Grow(1).Label(locales.Label(string(game.Kinds[a.tab])))
		ui.Text(c, locales.RecordSummary(len(rows))).TextColor(c.Theme().TextMuted)
	})
}

func (a *Page) Open(kind game.Kind, id int) {
	for i, k := range game.Kinds {
		if k != kind {
			continue
		}
		table := a.state.Session.Table(kind)
		if table == nil || table.Err != nil {
			a.state.Err = game.ErrLayout
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
	a.state.Err = game.ErrField
}
