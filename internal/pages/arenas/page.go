// Package arenas owns drafts for native arena rules.
package arenas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/arena"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	rowKey struct {
		kind arena.Kind
		id   int
	}
	valueInput struct {
		text    string
		invalid bool
	}
	draftState struct {
		draft         *arena.Draft
		plan          *arena.Plan
		err           error
		inputs        map[rowKey]*valueInput
		selected      map[arena.Kind]*int
		lists         map[arena.Kind]*ui.ListState
		previewAction ui.Handle
		editAction    ui.Handle
	}
)

func (d *draftState) invalid() bool {
	for _, field := range d.inputs {
		if field.invalid {
			return true
		}
	}
	return false
}

type Page struct {
	state      *editor.State
	openRecord func(records.Kind, int)
	group      int
	entries    map[arena.Kind]*draftState
}

func New(
	state *editor.State,
	record func(records.Kind, int),
) *Page {
	return &Page{
		state:      state,
		openRecord: record,
		entries:    make(map[arena.Kind]*draftState),
	}
}
func (p *Page) Reset() { *p = *New(p.state, p.openRecord) }
func caption(kind arena.Kind) string {
	labels := [...]string{
		locales.ArenaMelee(),
		locales.ArenaRanged(),
		locales.ArenaMagic(),
		locales.ArenaBaseWeapons(),
		locales.ArenaUpgrades(),
		locales.ArenaLinkUnits(),
	}
	return labels[kind]
}

func recordKind(kind arena.Kind) records.Kind {
	if kind.ClassList() {
		return records.Classes
	}
	if kind == arena.LinkUnits {
		return records.Units
	}
	return records.Items
}

func (p *Page) name(kind arena.Kind, id uint16) string {
	if table := p.state.Session.Records.Table(
		recordKind(kind),
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == int(id) {
				return fmt.Sprintf("%02X · %s", id, row.Name)
			}
		}
	}
	return fmt.Sprintf("0x%04X", id)
}

func (p *Page) draft(group arena.Kind) *draftState {
	entry := p.entries[group]
	if entry != nil && entry.draft != nil &&
		!entry.draft.Current(p.state.Session.Arena) &&
		!entry.draft.Modified() &&
		!entry.invalid() {
		entry = nil
	}
	if entry == nil {
		entry = &draftState{
			inputs:   make(map[rowKey]*valueInput),
			selected: make(map[arena.Kind]*int),
			lists:    make(map[arena.Kind]*ui.ListState),
		}
		entry.draft, entry.err = p.state.Session.Arena.NewDraft(group)
		p.entries[group] = entry
	}
	return entry
}

func (p *Page) View(c *ui.Context) {
	groups := p.state.Session.Arena.Groups()
	labels := make([]string, len(groups))
	for i, group := range groups {
		labels[i] = caption(group)
		if group == arena.BaseWeapons {
			labels[i] = locales.ArenaWeapons()
		}
	}
	p.group = min(p.group, len(groups)-1)
	group := groups[p.group]
	help := locales.ArenaClassHelp()
	if group == arena.BaseWeapons {
		help = locales.ArenaWeaponHelp()
	}
	if group == arena.LinkUnits {
		help = locales.ArenaLinkHelp()
	}
	ui.Column(c).Fill().MinHeight(0).Children(func() {
		ui.ScrollHorizontal(c.Key("arena-tabs")).Shrink(0).Children(func() {
			ui.Tabs(c, &p.group, labels...).Tooltip(help).Disabled(p.state.Busy)
		})
		entry := p.draft(group)
		reload := func() { delete(p.entries, group) }
		ui.Scroll(c.Key(entry)).Fill().Padding(16).Gap(12).Children(func() {
			if entry.draft == nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
				ui.Button(c, locales.QuoteReload()).
					Disabled(p.state.Busy).
					OnClick(reload)
				return
			}
			e := p.state.Session.Arena
			stale := !entry.draft.Current(e)
			if stale {
				entry.plan = nil
				ui.Text(c, locales.SupportStale()).TextColor(c.Theme().Danger)
			}
			if entry.draft.Modified() || entry.invalid() || stale {
				ui.Row(c.Key("actions")).
					Wrap().
					Gap(8).
					AlignItems(ui.Center).
					Children(func() {
						if entry.plan == nil {
							ui.Button(c.Key("preview"), locales.ArenaPreview()).
								Bind(&entry.previewAction).
								Disabled(p.state.Busy || stale || entry.invalid() || !entry.draft.Modified()).
								OnClick(func() {
									entry.plan, entry.err = e.Plan(entry.draft)
									p.state.Report(
										"editor.operation",
										entry.err,
									)
									if entry.plan != nil {
										entry.editAction.Focus(c)
									}
								})
						} else {
							ui.Button(c.Key("edit"), locales.ArenaEdit()).
								Bind(&entry.editAction).
								Disabled(p.state.Busy).
								OnClick(func() {
									entry.plan = nil
									entry.previewAction.Focus(c)
								})
						}
						ui.Button(c, locales.DeployReload()).
							Disabled(p.state.Busy).
							OnClick(reload)
					})
			}
			if entry.invalid() {
				ui.Text(c, locales.SupportInvalidInput()).
					TextColor(c.Theme().Danger).
					FontSize(components.FontCaption)
			}
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if entry.plan != nil {
				p.preview(c, entry, reload)
				return
			}
			for _, kind := range entry.draft.Kinds() {
				ui.Column(c.Key(kind)).
					Gap(8).
					Children(func() { p.table(c, entry, kind, p.state.Busy || stale) })
			}
		})
	})
}

func (p *Page) table(
	c *ui.Context,
	entry *draftState,
	kind arena.Kind,
	disabled bool,
) {
	rows := entry.draft.Rows(kind)
	if entry.selected[kind] == nil {
		entry.selected[kind] = new(int)
	}
	selected := min(max(0, *entry.selected[kind]), max(0, len(rows)-1))
	*entry.selected[kind] = selected
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		title := caption(kind)
		if kind.ClassList() || kind == arena.LinkUnits {
			title = locales.Label(string(recordKind(kind)))
		}
		ui.Textf(c, "%s · %d", title, len(rows)).Bold()
	})
	list := entry.lists[kind]
	if list == nil {
		list = &ui.ListState{}
		entry.lists[kind] = list
	}
	list.Selected = entry.selected[kind]
	list.Key = func(i int) any { return rows[i].Key }
	if len(rows) > 0 {
		indexTitle, indexWidth, indexAlign := "#", float32(54), ui.End
		if kind == arena.BaseWeapons {
			indexTitle, indexWidth, indexAlign = locales.WeaponType(), 100, ui.Start
		}
		ui.Table(c.Key("rows"), list, []ui.TableColumn{{ID: "id", Title: indexTitle, Width: indexWidth, Align: indexAlign}, {ID: "value", Title: locales.Label(string(recordKind(kind)))}}, len(rows), func(index, column int) {
			row := rows[index]
			if column == 0 {
				label := strconv.Itoa(index + 1)
				if row.WeaponType >= 0 {
					label = locales.Label(records.WeaponNames[row.WeaponType])
				}
				ui.Text(c, label).
					FontSize(components.FontCaption).
					FontFeatures("tnum")
				return
			}
			label := p.name(kind, row.Value)
			if !row.Editable {
				label = fmt.Sprintf(
					"%02X · %s",
					row.Value,
					locales.ArenaReserved(),
				)
			}
			ui.Text(c, label).SingleLine()
		}).
			Height(min(float32(240), float32(len(rows)*28+32)))
	}
	if kind.ClassList() {
		p.structure(c, entry, kind, rows, selected, disabled || entry.invalid())
	}
	if selected < len(rows) {
		row := rows[selected]
		if row.Editable {
			p.field(c, entry, kind, row, disabled)
		}
	}
}

func (p *Page) field(
	c *ui.Context,
	entry *draftState,
	kind arena.Kind,
	row arena.Row,
	disabled bool,
) {
	key := rowKey{kind, row.Key}
	field := entry.inputs[key]
	if field == nil {
		field = &valueInput{text: p.name(kind, row.Value)}
		entry.inputs[key] = field
	}
	var choices []string
	values := make(map[string]int)
	if kind == arena.LinkUnits {
		choices = append(choices, "0")
		values["0"] = 0
	}
	if table := p.state.Session.Records.Table(
		recordKind(kind),
	); table != nil &&
		table.Err == nil {
		for _, r := range table.Rows {
			if r.ID <= 0 || r.ID > 255 ||
				kind == arena.Upgrades && r.ID == 255 {
				continue
			}
			name := p.name(kind, uint16(r.ID))
			choices = append(choices, name)
			values[name] = r.ID
		}
	}
	ui.Row(c.Key(key)).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		control := components.TextDraft(
			p.state,
			ui.Autocomplete(c.Key("value"), &field.text, choices).
				Width(320).
				MaxWidthPercent(100).
				Label(caption(kind)+" / "+strconv.Itoa(row.Index+1)).
				Disabled(disabled).
				OnChange(func() {
					entry.plan = nil
					value, ok := values[field.text]
					if !ok {
						parsed, err := input.ParseInteger(field.text)
						if err != nil || parsed < 0 || parsed > 255 {
							field.invalid = true
							return
						}
						value = int(parsed)
					}
					entry.err = entry.draft.Set(kind, row.Index, value)
					field.invalid = entry.err != nil
				}),
		)
		if field.invalid {
			control.Error(locales.ArenaValueHelp())
		}
		ui.Button(c, locales.ArenaOpenRecord()).
			Disabled(p.state.Busy || row.Value == 0 || row.Value > 255 || p.openRecord == nil).
			OnClick(func() { p.openRecord(recordKind(kind), int(row.Value)) })
	})
	if kind == arena.LinkUnits {
		ui.Text(c, locales.ArenaLinkSlotHelp()).FontSize(components.FontCaption)
	}
}

func (p *Page) structure(
	c *ui.Context,
	entry *draftState,
	kind arena.Kind,
	rows []arena.Row,
	selected int,
	disabled bool,
) {
	ui.Row(c.Key("structure")).Wrap().Gap(6).Children(func() {
		components.ActionButton(c, "+", locales.ArenaAdd()).
			Disabled(disabled || len(rows) >= arena.MaxClasses).
			OnClick(func() {
				value := 1
				if len(rows) > 0 {
					value = int(rows[selected].Value)
				}
				entry.err = entry.draft.Insert(kind, len(rows), value)
				if entry.err == nil {
					*entry.selected[kind] = len(rows)
					entry.plan = nil
				}
			})
		components.ActionButton(c, "⧉", locales.ArenaCopy()).
			Disabled(disabled || len(rows) == 0 || len(rows) >= arena.MaxClasses).
			OnClick(func() {
				entry.err = entry.draft.Insert(
					kind,
					selected+1,
					int(rows[selected].Value),
				)
				if entry.err == nil {
					*entry.selected[kind] = selected + 1
					entry.plan = nil
				}
			})
		components.ActionButton(c, "−", locales.ArenaDelete()).
			Disabled(disabled || len(rows) == 0).
			OnClick(func() {
				entry.err = entry.draft.Delete(kind, selected)
				if entry.err == nil {
					delete(entry.inputs, rowKey{kind, rows[selected].Key})
					entry.plan = nil
				}
			})
		for _, direction := range []int{-1, 1} {
			label, help := "↑", locales.MoveUp()
			if direction > 0 {
				label, help = "↓", locales.MoveDown()
			}
			ui.Button(c.Key(direction), label).
				Label(help).
				Tooltip(help).
				Disabled(disabled || len(rows) == 0 || selected+direction < 0 || selected+direction >= len(rows)).
				OnClick(func() {
					entry.err = entry.draft.Move(
						kind,
						selected,
						selected+direction,
					)
					if entry.err == nil {
						*entry.selected[kind] = selected + direction
						entry.plan = nil
					}
				})
		}
	})
}

func (p *Page) preview(c *ui.Context, entry *draftState, reload func()) {
	ui.Column(c.Key("preview")).
		Gap(12).
		Children(func() {
			help := locales.ArenaApplyHelp()
			if entry.draft.Kinds()[0] == arena.BaseWeapons {
				help = locales.ArenaWeaponApplyHelp()
			}
			ui.Text(c, help)
			for _, change := range entry.plan.Changes() {
				ui.Column(c.Key(change.Kind)).Gap(4).Children(func() {
					ui.Text(c, caption(change.Kind)).Bold()
					var changed []int
					for i := range max(len(change.BeforeValues), len(change.AfterValues)) {
						if i < len(change.BeforeValues) &&
							i < len(change.AfterValues) &&
							change.BeforeValues[i] == change.AfterValues[i] {
							continue
						}
						changed = append(changed, i)
					}
					ui.Table(c.Key("changes"), nil, []ui.TableColumn{
						{ID: "index", Title: "#", Width: 54, Align: ui.End},
						{ID: "before", Title: locales.Before()},
						{ID: "after", Title: locales.After()},
					}, len(changed), func(index, column int) {
						row := changed[index]
						if column == 0 {
							ui.Textf(c, "%d", row+1).
								FontSize(components.FontCaption).
								FontFeatures("tnum")
							return
						}
						values := change.BeforeValues
						if column == 2 {
							values = change.AfterValues
						}
						value := "—"
						if row < len(values) {
							value = p.name(change.Kind, values[row])
						}
						ui.Text(c, value).
							SingleLine().
							Tooltip(value)
					}).Height(min(float32(240), float32(len(changed)*28+32)))
					if len(change.Shared) > 0 {
						labels := make([]string, len(change.Shared))
						for i, kind := range change.Shared {
							labels[i] = caption(kind)
						}
						ui.Text(c, locales.ArenaShared()+strings.Join(labels, "、")).
							FontSize(components.FontCaption)
					}
				})
			}
			ui.PrimaryButton(c, locales.ArenaApply()).
				AlignSelf(ui.Start).
				Disabled(p.state.Busy || entry.invalid() || !entry.plan.Current(p.state.Session.Arena)).
				OnClick(func() {
					entry.err = p.state.Edit(
						func() error { return p.state.Session.Arena.Apply(entry.plan) },
					)
					if entry.err == nil {
						reload()
						p.state.HistoryEpoch++
						p.state.Session.Refresh()
					}
				})
		})
}
