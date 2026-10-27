package records

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type layoutDraft struct {
	show, initialized      bool
	address, stride, count string
	err                    error
	move                   *game.TableMove
}

func (a *Page) tableLayout(c *ui.Context) {
	p := &a.panes[a.tab]
	table := a.state.Session.Table(game.Kinds[a.tab])
	if table == nil {
		return
	}
	ui.Checkbox(c, &p.layout.show, locales.TableLayout())
	if !p.layout.show {
		return
	}
	if !p.layout.initialized {
		p.layout.address = fmt.Sprintf("%08X", table.Address)
		p.layout.stride = strconv.Itoa(table.Stride)
		p.layout.count = strconv.Itoa(table.Count)
		p.layout.initialized = true
	}
	mode := "automatic_layout"
	if table.Manual {
		mode = "manual_layout"
	}
	ui.Text(c, locales.Label(mode)).FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Text(c, locales.LayoutScope()).FontSize(12)
	ui.Field(c, locales.RomAddress(), func() {
		components.TextDraft(
			a.state,
			ui.TextInput(c, &p.layout.address).Disabled(a.state.Busy),
		)
	})
	ui.Grid(c).Columns(2).Gap(8).Children(func() {
		ui.Field(c, locales.RecordSize(), func() {
			components.TextDraft(
				a.state,
				ui.TextInput(c, &p.layout.stride).Disabled(a.state.Busy),
			)
		})
		ui.Field(c, locales.RecordCount(), func() {
			components.TextDraft(
				a.state,
				ui.TextInput(c, &p.layout.count).Disabled(a.state.Busy),
			)
		})
	})
	kind := table.Kind
	ui.Row(c).Gap(6).Children(func() {
		ui.Button(c, locales.UseLayout()).
			Disabled(a.state.Busy).
			OnClick(func() {
				address, err := components.ParseAddress(p.layout.address)
				if err != nil {
					p.layout.err = err
					return
				}
				stride, e1 := components.ParseInteger(p.layout.stride)
				count, e2 := components.ParseInteger(p.layout.count)
				if e1 != nil || e2 != nil || stride < 4 ||
					stride > rom.MaxSize ||
					count < 1 ||
					count > 256 {
					p.layout.err = game.ErrLayout
					return
				}
				p.layout.err = a.state.Session.UseLayout(
					kind,
					game.Layout{
						Address: address,
						Stride:  int(stride),
						Count:   int(count),
					},
				)
				if p.layout.err == nil {
					p.layout.move = nil
					p.chosen.Clear()
					p.selected = 0
					a.form.ClearDrafts()
					a.state.HistoryEpoch++
				}
			})
		ui.Button(c, locales.ResetLayout()).
			Disabled(a.state.Busy || !table.Manual).
			OnClick(func() {
				a.state.Session.UseAutomaticLayout(kind)
				p.layout.initialized = false
				p.layout.move = nil
				p.layout.err = nil
				p.selected = 0
				p.chosen.Clear()
				a.form.ClearDrafts()
				a.state.HistoryEpoch++
			})
	})
	if p.layout.err != nil {
		ui.Text(c, components.ErrorText(p.layout.err)).FontSize(12)
	}
	a.tableMove(c, table, p)
}

func (a *Page) tableMove(c *ui.Context, table *game.Table, p *pane) {
	ui.Text(c, locales.MoveTable()).FontSize(13)
	ui.Text(c, locales.MoveTableScope()).
		FontSize(12).
		TextColor(c.Theme().TextMuted)
	ui.Button(c, locales.PreviewMove()).
		Disabled(a.state.Busy || table.Manual || table.Err != nil).
		OnClick(func() {
			p.layout.move, p.layout.err = a.state.Session.PlanTableMove(
				table.Kind,
				nil,
			)
		})
	move := p.layout.move
	if move == nil {
		return
	}
	ui.Text(c, locales.MoveSummary(
		fmt.Sprintf(
			"0x%08X",
			move.Source(),
		),
		fmt.Sprintf("0x%08X", move.Destination()),
		move.Length(),
		move.ReferenceCount(),
		move.Size(),
	)).FontSize(12)
	stale := move.Revision() != a.state.Session.Document.State().Revision
	if stale {
		ui.Text(c, components.ErrorText(rom.ErrStale)).FontSize(12)
	}
	ui.Button(c, locales.ApplyMove()).
		Disabled(a.state.Busy || table.Manual || stale).
		OnClick(func() {
			p.layout.err = a.state.Session.ApplyTableMove(move)
			if p.layout.err == nil {
				p.layout.move = nil
				p.layout.initialized = false
				p.chosen.Clear()
				p.selected = 0
				a.form.ClearDrafts()
				a.state.HistoryEpoch++
			}
		})
}
