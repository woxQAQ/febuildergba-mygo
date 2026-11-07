package records

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// tableOrigin keeps interpretation drafts and move reviews bound to their table.
type tableOrigin struct {
	session  *game.Session
	revision uint64
	layout   records.Layout
	manual   bool
}

func tableOriginFor(state *editor.State, table *records.Table) tableOrigin {
	return tableOrigin{
		state.Session,
		state.Session.Document.State().Revision,
		table.Layout,
		table.Manual,
	}
}

func (source tableOrigin) current(
	state *editor.State,
	table *records.Table,
) bool {
	return table != nil && source.session == state.Session &&
		source.revision == state.Session.Document.State().Revision &&
		source.layout == table.Layout &&
		source.manual == table.Manual
}

type layoutDraft struct {
	source                 tableOrigin
	address, stride, count string
	err                    error
	inputs                 [3]ui.Handle
	moveButton             ui.Handle
	move                   *tableMoveReview
}

type tableMoveReview struct {
	open   bool
	source tableOrigin
	plan   *records.MovePlan
	err    error
}

func (draft *layoutDraft) dirty() bool {
	return draft.address != fmt.Sprintf(
		"0x%08X",
		draft.source.layout.Address,
	) ||
		draft.stride != strconv.Itoa(draft.source.layout.Stride) ||
		draft.count != strconv.Itoa(draft.source.layout.Count)
}

func (draft *layoutDraft) read(state *editor.State, table *records.Table) {
	draft.source = tableOriginFor(state, table)
	draft.address = fmt.Sprintf("0x%08X", table.Address)
	draft.stride = strconv.Itoa(table.Stride)
	draft.count = strconv.Itoa(table.Count)
	draft.err = nil
}

func (draft *layoutDraft) parse() (records.Layout, error) {
	address, err := input.ParseAddress(draft.address)
	if err != nil {
		return records.Layout{}, err
	}
	stride, e1 := input.ParseInteger(draft.stride)
	count, e2 := input.ParseInteger(draft.count)
	if e1 != nil || e2 != nil || address%4 != 0 || stride < 4 ||
		stride > rom.MaxSize ||
		stride%4 != 0 ||
		count < 1 ||
		count > 256 {
		return records.Layout{}, edit.ErrLayout
	}
	return records.Layout{
		Address: address,
		Stride:  int(stride),
		Count:   int(count),
	}, nil
}

func (a *Page) tableLayout(c *ui.Context) {
	pane := &a.panes[a.tab]
	table := a.state.Session.Records.Table(records.Kinds[a.tab])
	if table == nil {
		return
	}
	draft := &pane.layout
	if draft.source.session == nil ||
		!draft.dirty() && !draft.source.current(a.state, table) {
		draft.read(a.state, table)
	}
	ui.Column(c.Key("table-layout")).
		MinWidth(0).
		Gap(6).
		Label(locales.TableLayout()).
		Children(func() {
			_, parseErr := draft.parse()
			current := draft.source.current(a.state, table)
			ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
				fields := []struct {
					label string
					value *string
					width float32
					help  string
				}{
					{
						locales.TableStartAddress(),
						&draft.address,
						112,
						locales.RomAddress(),
					},
					{
						locales.RecordSize(),
						&draft.stride,
						44,
						locales.LayoutScope(),
					},
					{
						locales.RecordCount(),
						&draft.count,
						44,
						locales.LayoutScope(),
					},
				}
				for index, field := range fields {
					ui.Row(c.Key(index)).
						Gap(6).
						AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, field.label).
								FontSize(components.FontCaption).
								TextColor(c.Theme().TextMuted).
								OnClick(func() { draft.inputs[index].Focus(c) })
							components.TextDraft(
								a.state,
								ui.TextInput(c, field.value).
									Width(field.width).
									Height(24).
									Padding(3, 6).Bind(&draft.inputs[index]).
									Label(field.label).
									Tooltip(field.help).
									FontFeatures("tnum").TextAlign(ui.End).
									Disabled(a.state.Busy).
									OnChange(func() { draft.err = nil }).
									OnSubmit(func() { a.applyTableLayout(c) }),
							)
						})
				}
				ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
					mode := locales.AutomaticLayout()
					if table.Manual {
						mode = locales.ManualLayout()
					}
					ui.Text(c, mode).
						FontSize(components.FontCaption).
						TextColor(c.Theme().TextMuted).
						Tooltip(locales.LayoutScope())
					if table.Manual {
						ui.Button(c, locales.ResetLayout()).
							Disabled(a.state.Busy).
							OnClick(func() {
								a.state.Session.Records.UseAutomaticLayout(
									table.Kind,
								)
								a.tableLayoutChanged()
								draft.read(
									a.state,
									a.state.Session.Records.Table(table.Kind),
								)
								draft.inputs[0].Focus(c)
							})
					}
					ui.Button(c, locales.PreviewMove()).Bind(&draft.moveButton).
						Disabled(a.state.Busy || table.Manual || table.Err != nil || draft.dirty()).
						Tooltip(locales.MoveTableScope()).
						OnClick(func() { a.previewTableMove() })
				})
				if draft.dirty() {
					ui.Row(c).Gap(8).Children(func() {
						ui.PrimaryButton(c, locales.UseLayout()).
							Disabled(a.state.Busy || !current || parseErr != nil).
							Tooltip(locales.LayoutPendingScope()).
							OnClick(func() { a.applyTableLayout(c) })
						ui.Button(c, locales.Cancel()).Disabled(a.state.Busy).
							OnClick(func() { draft.read(a.state, table); draft.inputs[0].Focus(c) })
					})
				}
			})
			if draft.dirty() {
				c.OnShortcut(
					0,
					ui.KeyEscape,
					func() { draft.read(a.state, table); draft.inputs[0].Focus(c) },
				)
				if !current {
					ui.Text(c, locales.TableLayoutStale()).
						FontSize(components.FontCaption).
						TextColor(c.Theme().Danger)
					return
				}
				if parseErr != nil {
					ui.Text(c, components.ErrorText(parseErr)).
						FontSize(components.FontCaption).
						TextColor(c.Theme().Danger)
					return
				}
			}
			if draft.err != nil {
				ui.Text(c, components.ErrorText(draft.err)).
					FontSize(components.FontCaption).
					TextColor(c.Theme().Danger)
			}
		})
	a.tableMove(c, table)
}

func (a *Page) applyTableLayout(c *ui.Context) {
	if a.state.Busy {
		return
	}
	pane := &a.panes[a.tab]
	table := a.state.Session.Records.Table(records.Kinds[a.tab])
	draft := &pane.layout
	if !draft.source.current(a.state, table) {
		draft.err = rom.ErrStale
		return
	}
	layout, err := draft.parse()
	if err != nil {
		draft.err = err
		return
	}
	if layout == table.Layout {
		draft.read(a.state, table)
		return
	}
	draft.err = a.state.Session.Records.UseLayout(table.Kind, layout)
	a.state.Report("editor.operation", draft.err)
	if draft.err != nil {
		return
	}
	a.tableLayoutChanged()
	draft.read(a.state, a.state.Session.Records.Table(table.Kind))
	draft.inputs[0].Focus(c)
}

func (a *Page) tableLayoutChanged() {
	pane := &a.panes[a.tab]
	pane.layout.move = nil
	pane.selected = 0
	pane.chosen.Clear()
	pane.initialized = false
	pane.list = ui.ListState{}
	a.form.ClearDrafts()
	a.state.HistoryEpoch++
}

func (a *Page) previewTableMove() {
	if a.state.Busy {
		return
	}
	table := a.state.Session.Records.Table(records.Kinds[a.tab])
	draft := &a.panes[a.tab].layout
	if table == nil || table.Manual || table.Err != nil || draft.dirty() {
		return
	}
	plan, err := a.state.Session.Records.PlanTableMove(table.Kind, nil)
	draft.err = err
	a.state.Report("editor.operation", err)
	if err != nil {
		return
	}
	draft.move = &tableMoveReview{
		open:   true,
		source: tableOriginFor(a.state, table),
		plan:   plan,
	}
}

func (a *Page) closeTableMove(c *ui.Context) {
	draft := &a.panes[a.tab].layout
	draft.move = nil
	draft.moveButton.Focus(c)
}

func (a *Page) tableMove(c *ui.Context, table *records.Table) {
	draft := &a.panes[a.tab].layout
	review := draft.move
	if review == nil {
		return
	}
	width, height := c.Size()
	modal := ui.Modal(c.Key(review), &review.open, func() {
		ui.Column(c).
			Width(min(float32(560), max(float32(0), width-64))).
			Gap(14).
			Children(func() {
				ui.Text(c, locales.MoveTable()).
					FontSize(components.FontPanelTitle).
					Bold()
				current := review.source.current(a.state, table)
				ui.Scroll(c).
					MaxHeight(max(float32(100), height-240)).
					Gap(12).
					Children(func() {
						ui.Text(c, locales.MoveTableScope())
						ui.Grid(c).Columns(2).Gap(8).Children(func() {
							fields := []struct{ label, value string }{
								{
									locales.TableMoveSource(),
									fmt.Sprintf("0x%08X", review.plan.Source()),
								},
								{
									locales.TableMoveTarget(),
									fmt.Sprintf(
										"0x%08X",
										review.plan.Destination(),
									),
								},
								{
									locales.TableMoveLength(),
									strconv.Itoa(review.plan.Length()),
								},
								{
									locales.TableMoveReferences(),
									strconv.Itoa(review.plan.ReferenceCount()),
								},
								{
									locales.TableMoveSize(),
									fmt.Sprintf(
										"%.2f MiB",
										float64(review.plan.Size())/(1<<20),
									),
								},
							}
							for _, field := range fields {
								ui.Text(c, field.label).
									TextColor(c.Theme().TextMuted)
								ui.Text(c, field.value).
									FontFeatures("tnum").
									Selectable()
							}
						})
						if !current {
							ui.Text(c, components.ErrorText(rom.ErrStale)).
								TextColor(c.Theme().Danger)
						}
						if review.err != nil {
							ui.Text(c, components.ErrorText(review.err)).
								TextColor(c.Theme().Danger)
						}
					})
				ui.Row(c).Wrap().Gap(8).Children(func() {
					ui.PrimaryButton(c, locales.ApplyMove()).
						Disabled(a.state.Busy || !current).
						OnClick(func() {
							if !review.source.current(
								a.state,
								a.state.Session.Records.Table(table.Kind),
							) {
								review.err = rom.ErrStale
								return
							}
							review.err = a.state.Edit(
								func() error { return a.state.Session.Records.ApplyTableMove(review.plan) },
							)
							if review.err != nil {
								return
							}
							a.tableLayoutChanged()
							draft.read(
								a.state,
								a.state.Session.Records.Table(table.Kind),
							)
							draft.moveButton.Focus(c)
						})
					if !current {
						ui.Button(c, locales.PreviewMoveAgain()).
							Disabled(a.state.Busy || table.Manual || table.Err != nil).
							OnClick(func() { a.previewTableMove() })
					}
					ui.Button(c, locales.Cancel()).
						Disabled(a.state.Busy).
						OnClick(func() { a.closeTableMove(c) })
				})
			})
	})
	if modal.Dismissed() {
		a.closeTableMove(c)
	}
}
